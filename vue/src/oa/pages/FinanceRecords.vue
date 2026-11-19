<script setup lang="ts">
import BaseSelect from '@/components/base/BaseSelect.vue'
import OaDateField from '../components/OaDateField.vue'
import { computed, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useOAAccess } from '../access'
import { api, dateInChina, download, errorText } from '../api'

const props = defineProps<{ mode: 'report' | 'all' | 'done' | 'cc' }>()
const route = useRoute()
const router = useRouter()
const { can } = useOAAccess()

const states: Record<string, string> = {
  waiting: '待审批', returned: '退回待修改', approved: '待付款', paid: '已付款',
  rejected: '已拒绝', withdrawn: '已撤回', voided: '已作废',
}
const paymentStates: Record<string, string> = { paid: '已付款', pending: '待付款', voided: '已作废', not_due: '未进入付款' }
const bases: Record<string, string> = { submitted: '首次提交日期', approved: '完成审批日期', paid: '实际付款日期' }
const groups: Record<string, string> = {
  department: '部门', team: '团队', person: '申请人', site: '店铺', template: '模板', payee: '收款方',
  channel: '付款渠道', status: '单据状态',
}
function choices(values: Record<string,string>, all = false) {
  const options = Object.entries(values).map(([value,label]) => ({value,label}))
  return all ? [{value:'',label:'全部'}, ...options] : options
}
function paidShare(paid: string, total: string) {
  const denominator = BigInt(total || '0')
  return denominator > 0n ? Number(BigInt(paid || '0') * 1000n / denominator) / 10 : null
}
const metrics = [
  { key: 'requested_cents', label: '申请金额' }, { key: 'approved_cents', label: '核准金额' },
  { key: 'paid_cents', label: '实付金额' }, { key: 'unpaid_cents', label: '已批未付' },
  { key: 'in_flight_cents', label: '在途申请' },
] as const
type Metrics = Record<typeof metrics[number]['key'], string> & { count: number }
type Row = {
  id: string; state: string; applicant_id: string; applicant_name: string; applicant_employee_no: string;
  department_name: string; team_name: string; site: string; template_name: string; expense_type: string; channel: string;
  submitted_date: string; approved_date: string; paid_date: string;
  requested_cents: string; approved_cents: string; paid_cents: string;
  purpose_summary?: string; payee?: string; approval_status?: string; payment_status?: string;
  payment_operator_id?: string; payment_operator_name?: string;
}
type Result = {
  items: Row[]; total: number; page: number; size: number;
  summary?: Metrics; groups?: (Metrics & { key: string; label: string })[];
}
const report = computed(() => props.mode === 'report')
const personal = computed(() => props.mode === 'done' || props.mode === 'cc')
const result = ref<Result>()
const loading = ref(false)
const exporting = ref(false)
const error = ref('')
const formError = ref('')
let requestNumber = 0
const size = 15
const pageCount = computed(() => Math.max(1, Math.ceil((result.value?.total ?? 0) / size)))
const emptyText = computed(() => ({
  all: '当前条件下没有费用单据。', report: '当前筛选条件下没有费用记录。',
  done: '当前条件下没有您实际处理过的费用单据。', cc: '当前没有符合条件且明确抄送给您的费用单据。',
})[props.mode])

function queryString(key: string) {
  const value = route.query[key]
  return typeof value === 'string' ? value : ''
}
function periodDates(period: string) {
  const [year, month, day] = period.split('-')
  if (/^\d{4}$/.test(period)) return { start: year + '-01-01', end: year + '-12-31' }
  if (/^\d{4}-\d{2}$/.test(period) && Number(month) >= 1 && Number(month) <= 12) {
    const last = new Date(Date.UTC(Number(year), Number(month), 0)).getUTCDate()
    return { start: period + '-01', end: period + '-' + last }
  }
  if (/^\d{4}-\d{2}-\d{2}$/.test(period) && day) return { start: period, end: period }
  return { start: '', end: '' }
}
function yuan(raw: string) {
  if (!/^\d+$/.test(raw)) return ''
  const value = BigInt(raw)
  return String(value / 100n) + '.' + String(value % 100n).padStart(2, '0')
}
function amount(raw: string) {
  if (!/^\d+$/.test(raw)) return '—'
  const value = BigInt(raw)
  return '¥' + (value / 100n).toLocaleString('zh-CN') + '.' + String(value % 100n).padStart(2, '0')
}
function cents(raw: string) {
  const match = /^(\d{1,17})(?:\.(\d{1,2}))?$/.exec(raw.trim())
  return match ? (BigInt(match[1]) * 100n + BigInt((match[2] || '').padEnd(2, '0'))).toString() : ''
}
function filtersFromRoute() {
  const defaults = report.value ? periodDates(queryString('period') || dateInChina().slice(0, 7)) : { start: '', end: '' }
  return {
    basis: queryString('basis') || (report.value ? 'paid' : 'submitted'),
    group_by: queryString('group_by') || 'department',
    sort: queryString('sort') || 'date', order: queryString('order') || 'desc',
    start: queryString('start') || defaults.start, end: queryString('end') || defaults.end,
    state: queryString('state'), q: queryString('q'), template: queryString('template'), expense_type: queryString('expense_type'),
    department: queryString('department'), team: queryString('team'), person: queryString('person'),
    site: queryString('site'), channel: queryString('channel'),
    min: yuan(queryString('min_amount_cents')), max: yuan(queryString('max_amount_cents')),
  }
}
const form = ref(filtersFromRoute())

function endpoint(forExport = false) {
  const url = '/oa-api/finance/' + (report.value ? 'report' : 'documents')
  const filters = filtersFromRoute()
  const params = new URLSearchParams()
  for (const [key, value] of Object.entries(filters)) {
    if (value && key !== 'min' && key !== 'max') params.set(key, value)
  }
  if (!report.value) params.set('queue', props.mode)
  for (const key of ['min_amount_cents', 'max_amount_cents']) {
    if (queryString(key)) params.set(key, queryString(key))
  }
  if (forExport) params.set('format', 'xlsx')
  else {
    params.set('page', queryString('page') || '1')
    params.set('size', String(size))
  }
  return url + '?' + params
}

async function load() {
  const id = ++requestNumber
  error.value = ''
  formError.value = ''
  loading.value = true
  result.value = undefined
  try {
    const response = await api<Result>(endpoint())
    if (id === requestNumber) result.value = response
  } catch (cause) {
    if (id === requestNumber) error.value = errorText(cause)
  } finally {
    if (id === requestNumber) loading.value = false
  }
}

watch(() => [props.mode, route.fullPath], () => {
  if (route.path !== '/fees/' + (props.mode === 'all' ? 'ledger' : props.mode)) return
  form.value = filtersFromRoute()
  void load()
}, { immediate: true })

function search() {
  formError.value = ''
  const query: Record<string, string> = {}
  for (const [key, raw] of Object.entries(form.value)) {
    const value = raw.trim()
    if (!value) continue
    if (key === 'min' || key === 'max') {
      const parsed = cents(value)
      if (!parsed || BigInt(parsed) > 9223372036854775807n) {
        formError.value = '请输入有效的金额范围，金额最多保留两位小数。'
        return
      }
      query[key + '_amount_cents'] = parsed
    } else query[key] = value
  }
  if (query.start && query.end && query.start > query.end) {
    formError.value = '结束日期须等于或晚于开始日期。'
    return
  }
  if (Boolean(query.start) !== Boolean(query.end)) {
    formError.value = '请选择完整的起止日期。'
    return
  }
  if (query.min_amount_cents && query.max_amount_cents && BigInt(query.min_amount_cents) > BigInt(query.max_amount_cents)) {
    formError.value = '金额上限须等于或高于金额下限。'
    return
  }
  const location = { path: route.path, query }
  if (router.resolve(location).fullPath === route.fullPath) void load()
  else void router.replace(location)
}
function reset() {
  if (Object.keys(route.query).length) void router.replace({ path: route.path })
  else {
    form.value = filtersFromRoute()
    void load()
  }
}
function turn(page: number) {
  if (page < 1 || page > pageCount.value) return
  void router.replace({ path: route.path, query: { ...route.query, page: page === 1 ? undefined : String(page) } })
}
async function exportReport() {
  exporting.value = true
  formError.value = ''
  try {
    await download(endpoint(true), '费用报表.xlsx')
  } catch (cause) {
    formError.value = errorText(cause)
  } finally {
    exporting.value = false
  }
}
</script>

<template>
  <section class="finance-records" :aria-label="report ? '费用报表' : '费用单据'" :aria-busy="loading">
    <form class="finance-filters oa-panel" aria-label="筛选费用单据" @submit.prevent="search">
      <div class="finance-fields">
        <label>搜索<input v-model="form.q" type="search" :placeholder="report ? '单号、用途、收款方、人员、模板' : '单号、人员、模板、组织、店铺'" maxlength="80"></label>
        <BaseSelect v-model="form.state" label="状态" :options="choices(states, true)" />
        <BaseSelect v-model="form.basis" label="日期口径" :options="choices(bases)" />
        <OaDateField v-model="form.start" label="开始日期" />
        <OaDateField v-model="form.end" label="结束日期" />
        <BaseSelect v-if="report" v-model="form.group_by" label="汇总维度" :options="choices(groups)" />
      </div>
      <details class="finance-advanced">
        <summary>更多筛选</summary>
        <div class="finance-fields">
          <BaseSelect v-model="form.expense_type" label="费用类型" :options="choices({reimbursement:'报销',payment:'付款'}, true)" />
          <BaseSelect v-model="form.sort" label="明细排序" :options="choices({date:'所选口径日期',amount:'申请金额'})" />
          <BaseSelect v-model="form.order" label="顺序" :options="choices({desc:'从高到低',asc:'从低到高'})" />
          <label>模板<input v-model="form.template" placeholder="模板全称" maxlength="80"></label>
          <label>部门<input v-model="form.department" placeholder="部门全称" maxlength="80"></label>
          <label>团队<input v-model="form.team" placeholder="团队全称" maxlength="80"></label>
          <label>申请人 ID<input v-model="form.person" inputmode="numeric" placeholder="账号编号" maxlength="19"></label>
          <label>店铺<input v-model="form.site" placeholder="店铺全称" maxlength="80"></label>
          <label>付款渠道<input v-model="form.channel" placeholder="渠道全称" maxlength="80"></label>
          <label>申请金额下限（元）<input v-model="form.min" inputmode="decimal"></label>
          <label>申请金额上限（元）<input v-model="form.max" inputmode="decimal"></label>
        </div>
      </details>
      <div class="oa-actions">
        <button type="submit" class="oa-button oa-button-primary" :disabled="loading">查询</button>
        <button type="button" class="oa-button" @click="reset">重置</button>
        <button v-if="report" type="button" class="oa-button" :disabled="loading || exporting || !result" @click="exportReport">{{ exporting ? '正在导出…' : '导出 Excel' }}</button>
      </div>
      <p v-if="formError" role="alert">{{ formError }}</p>
    </form>
    <div v-if="loading" class="finance-loading oa-panel" role="status"><span>正在读取费用单据…</span><i v-for="n in 3" :key="n" aria-hidden="true"></i></div>
    <div v-else-if="error" class="oa-panel finance-message" role="alert">
      <p>{{ error }}</p>
      <button type="button" class="oa-button" @click="load">重试</button>
    </div>
    <template v-else-if="result">
      <template v-if="report && result.summary">
        <dl class="finance-metrics">
          <div class="oa-panel"><dt>费用单据</dt><dd>{{ result.summary.count.toLocaleString('zh-CN') }} 笔</dd></div>
          <div v-for="metric in metrics" :key="metric.key" class="oa-panel">
            <dt>{{ metric.label }}</dt><dd>{{ amount(result.summary[metric.key]) }}</dd>
          </div>
        </dl>
        <p class="finance-hint">金额按所选日期范围和当前单据状态汇总。已批未付为当前待付款金额，在途为当前待审批及退回待修改的申请金额。按实付日期只统计已有实付记录的单据。</p>
        <div v-if="result.groups?.length" class="oa-panel finance-table" tabindex="0" role="region" aria-label="分组汇总表">
          <table class="oa-table">
            <caption>按{{ groups[filtersFromRoute().group_by] }}汇总</caption>
            <thead><tr><th scope="col">{{ groups[filtersFromRoute().group_by] }}</th><th scope="col">实付占比</th><th scope="col">单据数</th><th v-for="m in metrics" :key="m.key" scope="col">{{ m.label }}</th></tr></thead>
            <tbody>
              <tr v-for="group in result.groups" :key="group.key">
                <th scope="row">{{ (filtersFromRoute().group_by === 'status' ? states[group.label] : group.label) || (filtersFromRoute().group_by === 'person' ? '账号 #' + group.key : '未记录') }}</th>
                <td class="paid-share"><template v-if="paidShare(group.paid_cents, result.summary.paid_cents) !== null"><progress max="100" :value="paidShare(group.paid_cents,result.summary.paid_cents) ?? 0" aria-label="实付金额占比" /> {{ paidShare(group.paid_cents,result.summary.paid_cents) }}%</template><template v-else>—</template></td>
                <td>{{ group.count }}</td><td v-for="m in metrics" :key="m.key">{{ amount(group[m.key]) }}</td>
              </tr>
            </tbody>
          </table>
        </div>
      </template>
      <div v-if="result.items.length" class="oa-panel finance-table" tabindex="0" role="region" aria-label="费用明细表">
        <table class="oa-table">
          <caption>{{ report ? '费用明细' : '费用单据' }}</caption>
          <thead><tr>
            <th scope="col">单据 / 模板</th><th scope="col">费用类型</th><th v-if="report" scope="col">用途摘要 / 收款方</th><th scope="col">申请人</th><th scope="col">部门 / 团队</th>
            <th scope="col">店铺 / 渠道</th><th scope="col">{{ report ? '审批 / 付款状态' : '状态' }}</th><th scope="col">申请金额</th><th scope="col">核准金额</th>
            <th scope="col">实付金额</th><th scope="col">首次提交</th><th scope="col">完成审批</th><th scope="col">{{ report ? '实付日期 / 经办人' : '实付日期' }}</th>
          </tr></thead>
          <tbody><tr v-for="row in result.items" :key="row.id">
            <th scope="row">
              <RouterLink v-if="personal || (report && row.state === 'paid') || (mode === 'all' && row.state === 'approved' && can('finance.admin'))" :to="{ path: '/fees/document', query: { id: row.id, return_to: route.fullPath } }">{{ row.template_name || '费用单据' }}<span>{{ row.id }}</span></RouterLink>
              <template v-else>{{ row.template_name || '费用单据' }}<span>{{ row.id }}</span></template>
            </th>
            <td>{{ row.expense_type === 'reimbursement' ? '报销' : row.expense_type === 'payment' ? '付款' : '未记录' }}</td>
            <td v-if="report" class="finance-purpose">{{ row.purpose_summary || '历史未记录' }}<span>收款方 {{ row.payee || '历史未记录' }}</span></td>
            <td>{{ row.applicant_name || '账号 #' + row.applicant_id }}<span v-if="row.applicant_employee_no">员工编号 {{ row.applicant_employee_no }}</span></td>
            <td>{{ row.department_name || '—' }}<span v-if="row.team_name">{{ row.team_name }}</span></td>
            <td>{{ row.site || '—' }}<span v-if="row.channel">{{ row.channel }}</span></td>
            <td v-if="report">
              {{ row.approval_status === 'approved' ? '已批准' : states[row.approval_status || row.state] || '历史未记录' }}
              <span>{{ paymentStates[row.payment_status || ''] || '历史未记录' }}</span>
            </td><td v-else>{{ states[row.state] || row.state }}</td><td>{{ amount(row.requested_cents) }}</td><td>{{ amount(row.approved_cents) }}</td>
            <td>{{ amount(row.paid_cents) }}</td><td>{{ row.submitted_date || '—' }}</td><td>{{ row.approved_date || '—' }}</td><td>{{ row.paid_date || '—' }}
              <span v-if="report && row.state === 'paid'">{{ row.payment_operator_name || (row.payment_operator_id ? '账号 #' + row.payment_operator_id : '历史未记录') }}</span>
            </td>
          </tr></tbody>
        </table>
      </div>
      <p v-else class="oa-panel finance-message">{{ result.total ? '本页没有单据，请返回有记录的页。' : emptyText }}</p>
      <nav v-if="result.total || result.page > 1" class="oa-actions finance-pagination" aria-label="费用单据分页">
        <button type="button" class="oa-button" :disabled="result.page <= 1" @click="turn(Math.min(result.page - 1, pageCount))">上一页</button>
        <span>第 {{ result.page }} / {{ pageCount }} 页，共 {{ result.total.toLocaleString('zh-CN') }} 笔</span>
        <button type="button" class="oa-button" :disabled="result.page >= pageCount" @click="turn(result.page + 1)">下一页</button>
      </nav>
    </template>
  </section>
</template>

<style scoped>
.finance-records { display: grid; gap: 14px; min-width: 0; }
.finance-filters, .finance-message { padding: 14px; }
.finance-fields { display: grid; grid-template-columns: repeat(auto-fit, minmax(144px, 1fr)); align-items: end; gap: 12px; }
.finance-fields label { display: grid; min-width: 0; gap: 4px; }
.finance-fields input, .finance-fields select { width: 100%; min-width: 0; }
.finance-advanced { margin-block: 12px; }
.finance-advanced summary { width: fit-content; cursor: pointer; }
.finance-advanced .finance-fields { margin-block-start: 10px; }
.finance-metrics { display: grid; grid-template-columns: repeat(auto-fit, minmax(140px, 1fr)); gap: 8px; margin: 0; }
.finance-metrics div { padding: 12px; }
.finance-metrics dd { margin: 5px 0 0; font-variant-numeric: tabular-nums; font-weight: 600; overflow-wrap: anywhere; }
.finance-hint { margin: 0; color: var(--oa-muted); }
.finance-table { min-width: 0; overflow: auto; }
.finance-table table { width: 100%; border-collapse: collapse; }
.finance-table :is(td, th) { text-align: start; padding: 8px 12px; white-space: nowrap; }
.finance-table caption { text-align: start; padding: 10px 12px; font-weight: 600; }
.finance-table .finance-purpose { min-width: 180px; max-width: 300px; white-space: normal; overflow-wrap: anywhere; }
.finance-table span { display: block; color: var(--oa-muted); }
.finance-table a { text-decoration: underline; }
.finance-pagination { justify-content: flex-end; }
.finance-loading { display:grid; gap:8px; padding:16px; color:var(--oa-muted); }
.finance-loading i { height:12px; width:80%; background:var(--yb-line-hairline); border-radius:4px; }
.finance-loading i:nth-child(3){width:60%}
.paid-share progress { width:64px; accent-color:var(--oa-accent); vertical-align:middle; }
</style>
