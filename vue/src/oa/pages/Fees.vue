<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { RouterLink, useRoute, useRouter } from 'vue-router'
import PageHead from '../components/PageHead.vue'
import { api, dateInChina, download, errorText, money, send } from '../api'
import { can, currentProfile } from '../session'

type FeeTemplate = { id: string; name: string; type: string; channels: string[]; fields: string[]; required: string[]; evidence: string; nodes: string[]; threshold_cents: number; extra: string; version: number }
type Approver = { id: number; name: string; employee_no?: number | null }
type Evidence = { id: string; name: string }
type Decision = { action: string; comment?: string; actor_id?: number; at: string }
type FeeDoc = { id: string; applicant_id: number; template: FeeTemplate; amount_cents: number; reason: string; channel: string; holder: string; account: string; bank: string; extra: Record<string, string>; files: string[]; site: string; state: string; route: (number | string | { id?: number; name?: string; user_id?: number })[]; current: number; version: number; history?: Decision[]; payment?: { amount_cents: number; date: string; receipt: string } }
type FeeReport = { amount_cents: number; count: number; items: FeeDoc[]; unpaid_cents?: number }
const props = defineProps<{ mode: string }>()
const route = useRoute()
const router = useRouter()
const titles: Record<string, string> = { apply: '发起费用申请', mine: '我的申请', approval: '审批与付款', templates: '费用模板与审核节点', report: '费用支出统计', document: '费用单据' }
const title = computed(() => titles[props.mode] || '费用单据')
const isAdmin = computed(() => Boolean(currentProfile.value?.super_admin))
const canPay = computed(() => can('finance.pay') || isAdmin.value)
const canReport = computed(() => can('finance.report') || canPay.value)
const queue = ref<'mine' | 'todo' | 'pay'>(props.mode === 'mine' ? 'mine' : 'todo')
const busy = ref(false)
const error = ref('')
const notice = ref('')
const templates = ref<FeeTemplate[]>([])
const docs = ref<FeeDoc[]>([])
const detail = ref<FeeDoc>()
const report = ref<FeeReport>()
const period = ref(dateInChina().slice(0, 7))
const selection = ref('')
const form = ref({ request_id: '', template_id: '', amount: '', reason: '', channel: '', holder: '', account: '', bank: '', extra: {} as Record<string, string>, files: [] as Evidence[], site: route.query.site === 'asset' ? 'asset' : 'main' })
const returned = ref<string>()
const returnedTemplate = ref<FeeTemplate>()
const level = ref('')
const delegates = ref<{ id: number; level: string }[]>([])
const revealed = ref(false)
const approval = ref({ decision: 'approve', comment: '' })
const payment = ref({ date: dateInChina(), receipt: '' })
const draft = ref<FeeTemplate>()
const editingId = ref('')
const templateFields = ref({ fields: '', threshold: '0.00' })
const templateForm = ref<HTMLFormElement>()
const approverQuery = ref('')
const approverOptions = ref<Approver[]>([])
const knownApprovers = ref<Record<number, Approver>>({})
const approverBusy = ref(false)
const approverError = ref('')
let approverSearch = 0
const grants = ref({ id: '', pay: false, report: false })
const delegate = ref({ id: '', level: 'draft' })
const files = ref<HTMLInputElement>()
const receipt = ref<HTMLInputElement>()
const channels: Record<string, string> = { bank: '对公银行转账', person: '对私银行卡', wallet: '对私收款平台' }
const states: Record<string, string> = { waiting: '审批中', pending: '审批中', returned: '已退回', approved: '已批未付', paid: '已付款', rejected: '已拒绝' }
const decisions: Record<string, string> = { approve: '同意', return: '退回修改', reject: '拒绝' }
const active = computed(() => returnedTemplate.value || templates.value.find(item => item.id === form.value.template_id))
const own = computed(() => String(detail.value?.applicant_id) === String(currentProfile.value?.id))
const mayDecide = computed(() => detail.value?.state === 'waiting' && !own.value && String(detail.value.route?.[detail.value.current]) === String(currentProfile.value?.id))
const major = (cents?: number) => cents == null ? '—' : money(cents / 100)
const status = (state: string) => states[state] || state
const node = (step: FeeDoc['route'][number]) => typeof step === 'object' ? step.name || ('#' + (step.user_id || step.id)) : '#' + step
function approverLabel(approver: Approver) {
  const number = approver.employee_no == null ? '' : String(approver.employee_no).padStart(4, '0')
  return number ? `${approver.name} · ${number}` : approver.name
}
function nodeLabel(value: string) {
  if (value === 'manager') return '申请人直属负责人'
  const person = knownApprovers.value[Number(value.replace(/^fixed:/, ''))]
  return person ? approverLabel(person) : '原指定审批人'
}
function approvalChoices(value: string) {
  const results = approverOptions.value.map(person => ({ value: `fixed:${person.id}`, label: approverLabel(person) }))
  if (value.startsWith('fixed:') && !results.some(option => option.value === value)) results.unshift({ value, label: nodeLabel(value) })
  return [{ value: 'manager', label: '申请人直属负责人' }, ...results]
}
async function findApprovers() {
  const search = ++approverSearch
  approverBusy.value = true
  approverError.value = ''
  try {
    const query = new URLSearchParams({ q: approverQuery.value.trim() })
    const selected = [...new Set([...(draft.value?.nodes ?? []), draft.value?.extra ?? ''])]
      .filter(value => value.startsWith('fixed:'))
      .map(value => value.slice(6))
    if (selected.length) query.set('ids', selected.join(','))
    const { items, selected: resolved } = await api<{ items: Approver[]; selected?: Approver[] }>('/oa-api/finance/approvers?' + query)
    if (search !== approverSearch) return
    approverOptions.value = items || []
    for (const person of [...approverOptions.value, ...(resolved ?? [])]) knownApprovers.value[person.id] = person
  } catch (cause) {
    if (search === approverSearch) approverError.value = errorText(cause)
  } finally {
    if (search === approverSearch) approverBusy.value = false
  }
}
const historyLabel = (action: string) => decisions[action] || ({ submit: '提交申请', resubmit: '重新提交', payment: '登记付款' })[action] || action
const movingNode = (from: number, delta: number) => {
  const nodes = draft.value?.nodes
  const target = from + delta
  if (!nodes || target < 0 || target >= nodes.length) return
  ;[nodes[from], nodes[target]] = [nodes[target], nodes[from]]
}
const list = (value: string) => [...new Set(value.split(/[,，\n]/).map(item => item.trim()).filter(Boolean))]

function cents(value: string) {
  const parts = /^(\d{1,10})(?:\.(\d{1,2}))?$/.exec(value.trim())
  if (!parts) throw new Error('请输入有效金额，最多两位小数')
  const result = Number(parts[1]) * 100 + Number((parts[2] || '').padEnd(2, '0'))
  if (!result || !Number.isSafeInteger(result)) throw new Error('申请金额必须大于零')
  return result
}
async function run(action: () => Promise<void>) {
  if (busy.value) return
  busy.value = true
  error.value = ''
  notice.value = ''
  try { await action() }
  catch (cause) { error.value = errorText(cause) }
  finally { busy.value = false }
}

function selectTemplate() {
  form.value.channel = active.value?.channels?.[0] || ''
  form.value.extra = {}
}
async function loadTemplates() {
  const data = await api<{ items: FeeTemplate[]; level?: string }>('/oa-api/finance/templates' + (props.mode === 'templates' ? '?draft=1' : ''))
  level.value = data.level || ''
  templates.value = data.items ?? []
  if (!form.value.template_id && props.mode === 'apply') {
    form.value.template_id = templates.value[0]?.id || ''
    selectTemplate()
  }
}
async function loadDocs() {
  detail.value = undefined
  const data = await api<{ items: FeeDoc[] }>('/oa-api/finance/documents?' + new URLSearchParams({ queue: queue.value }))
  docs.value = data.items ?? []
}
async function open(id: string) {
  revealed.value = false
  detail.value = await api<FeeDoc>('/oa-api/finance/documents/' + encodeURIComponent(id))
  approval.value = { decision: 'approve', comment: '' }
  payment.value = { date: dateInChina(), receipt: '' }
}
async function revealAccount() {
  if (!detail.value?.id) return
  detail.value = await api<FeeDoc>('/oa-api/finance/documents/' + encodeURIComponent(detail.value.id) + '?reveal=1')
  revealed.value = true
}
async function upload(file: File) {
  const data = new FormData()
  data.set('file', file)
  return api<Evidence>('/oa-api/finance/files', { method: 'POST', body: data })
}
function uploadEvidence() {
  void run(async () => {
    for (const file of Array.from(files.value?.files ?? [])) form.value.files.push(await upload(file))
    if (files.value) files.value.value = ''
  })
}
function uploadReceipt() {
  void run(async () => {
    const file = receipt.value?.files?.[0]
    if (file) payment.value.receipt = (await upload(file)).id
  })
}
async function submit() {
  if (!active.value) return
  if (!form.value.files.length) throw new Error('请上传至少一份单据凭证')
  if (!form.value.request_id) form.value.request_id = crypto.randomUUID()
  const { amount, files: evidence, ...fields } = form.value
  const path = '/oa-api/finance/documents' + (returned.value ? '/' + encodeURIComponent(returned.value) + '/resubmit' : '')
  const result = await send<{ id?: string }>(path, { ...fields, amount_cents: cents(amount), files: evidence.map(file => file.id) })
  if (returned.value && result?.id) {
    returned.value = undefined
    returnedTemplate.value = undefined
    await loadDocs()
    await open(result.id)
    notice.value = '申请已重新提交'
  } else {
    await router.push({ path: '/fees/mine', query: result?.id ? { id: result.id } : {} })
  }
}
function editReturned() {
  if (!detail.value || !own.value || detail.value.state !== 'returned') return
  const document = detail.value
  void run(async () => {
    returned.value = document.id
    returnedTemplate.value = document.template
    form.value = {
      request_id: crypto.randomUUID(), template_id: document.template.id,
      amount: (document.amount_cents / 100).toFixed(2), reason: document.reason, channel: document.channel,
      holder: document.holder, account: '', bank: document.bank, extra: { ...document.extra },
      files: (document.files || []).map(id => ({ id, name: '原附件 ' + id.slice(0, 6) })), site: document.site,
    }
  })
}
async function act() {
  if (!detail.value) return
  const id = detail.value.id
  await send(`/oa-api/finance/documents/${encodeURIComponent(id)}/decision`, { ...approval.value, current: detail.value.current, version: detail.value.version })
  await open(id)
  notice.value = '审批结果已保存'
}
async function pay() {
  if (!detail.value || !payment.value.receipt) return
  const id = detail.value.id
  await send(`/oa-api/finance/documents/${encodeURIComponent(id)}/payment`, { ...payment.value, amount_cents: detail.value.amount_cents })
  await open(id)
  notice.value = '实付记录已保存'
}
async function loadReport() {
  report.value = await api<FeeReport>('/oa-api/finance/report?' + new URLSearchParams({ period: period.value.trim() }))
}
function beginTemplate(template?: FeeTemplate) {
  draft.value = template ? { ...template } : { id: '', name: '', type: '报销', channels: [], fields: [], required: [], evidence: '', nodes: [], threshold_cents: 0, extra: '', version: 0 }
  editingId.value = template?.id || ''
  draft.value.channels = [...(draft.value.channels || [])]
  draft.value.required = [...(draft.value.required || [])]
  draft.value.nodes = [...(draft.value.nodes || [])]
  templateFields.value = { fields: draft.value.fields?.join(', ') || '', threshold: (draft.value.threshold_cents / 100).toFixed(2) }
  approverQuery.value = ''
  approverOptions.value = []
  void findApprovers()
}
async function saveTemplate(publish: boolean) {
  if (!draft.value?.id) throw new Error('请填写模板 ID')
  if (!templateForm.value?.reportValidity()) return
  if (!draft.value.channels.length) throw new Error('请选择至少一种支付方式')
  if (!draft.value.nodes.length) throw new Error('请添加至少一个审批节点')
  const fields = list(templateFields.value.fields)
  const threshold = Number(templateFields.value.threshold)
  const body = {
    ...draft.value, fields, required: draft.value.required.filter(field => fields.includes(field)),
    nodes: draft.value.nodes.map(value => value.trim()),
    threshold_cents: threshold === 0 ? 0 : cents(templateFields.value.threshold), publish,
  }
  await send('/oa-api/finance/templates/' + encodeURIComponent(draft.value.id), body, 'PUT')
  await loadTemplates()
  draft.value = undefined
  notice.value = publish ? '模板已发布' : '草稿已保存'
}
async function saveFinanceAccess() {
  const { id, pay, report } = grants.value
  if (!/^[1-9]\d*$/.test(id)) throw new Error('请输入有效账号 ID')
  await send('/oa-api/finance/actions/' + encodeURIComponent(id), { pay, report }, 'PUT')
  notice.value = '财务经办权限已保存'
}
async function saveDelegate() {
  await send('/oa-api/finance/delegates/' + encodeURIComponent(delegate.value.id), { level: delegate.value.level }, 'PUT')
  delegates.value = (await api<{ items: { id: number; level: string }[] }>('/oa-api/finance/delegates')).items || []
  notice.value = '模板维护委托已保存'
}

onMounted(() => void run(async () => {
  if (props.mode === 'apply' || props.mode === 'templates') await loadTemplates()
  if (props.mode === 'templates' && isAdmin.value) delegates.value = (await api<{ items: { id: number; level: string }[] }>('/oa-api/finance/delegates')).items || []
  if (props.mode === 'approval' || props.mode === 'mine') await loadDocs()
  if (props.mode === 'report' && canReport.value) await loadReport()
  if (typeof route.query.id === 'string') await open(route.query.id)
}))
</script>

<template>
  <PageHead section="个人与公司 / 费用与审批" :title="title">
    <RouterLink v-if="mode !== 'apply'" class="oa-button" to="/fees/apply">发起申请</RouterLink>
    <RouterLink v-if="mode !== 'mine'" class="oa-button" to="/fees/mine">我的申请</RouterLink>
  </PageHead>
  <p v-if="error" class="oa-error" role="alert">{{ error }}</p>
  <p v-if="notice" class="oa-notice" role="status">{{ notice }}</p>
  <p v-if="busy" class="oa-muted" role="status">正在处理…</p>

  <div v-if="mode === 'apply' || returned" class="oa-split">
    <section class="oa-panel">
      <h2>{{ returned ? '修改退回单据' : '填写申请' }}</h2>
      <p v-if="!templates.length && !returnedTemplate && !busy" class="oa-empty">暂无可用的费用模板</p>
      <form v-else class="oa-form" @submit.prevent="run(submit)">
        <div class="oa-form-row">
          <label>申请模板<select v-model="form.template_id" :disabled="!!returned" required @change="selectTemplate"><option v-for="item in returnedTemplate ? [returnedTemplate] : templates" :key="item.id" :value="item.id">{{ item.name }}</option></select></label>
          <label>金额（元）<input v-model="form.amount" inputmode="decimal" placeholder="0.00" required /></label>
          <label>付款方式<select v-model="form.channel" required><option v-for="item in active?.channels || []" :key="item" :value="item">{{ channels[item] || item }}</option></select></label>
          <label>{{ form.channel === 'bank' ? '单位开户全称' : '收款人姓名' }}<input v-model="form.holder" required /></label>
          <label>{{ form.channel === 'wallet' ? '收款平台账号' : form.channel === 'bank' ? '对公银行账号' : '银行卡号' }}<input v-model="form.account" autocomplete="off" :placeholder="returned ? '留空保留原收款账号' : undefined" :required="!returned || form.channel !== detail?.channel" /></label>
          <label>{{ form.channel === 'wallet' ? '收款平台' : '开户银行' }}<input v-model="form.bank" required /></label>
          <label v-for="field in active?.fields || []" :key="field">{{ field }}<input v-model="form.extra[field]" :required="active?.required?.includes(field)" /></label>
        </div>
        <label>用途与说明<textarea v-model="form.reason" required /></label>
        <label>{{ active?.evidence || '凭据附件' }}<input ref="files" type="file" multiple @change="uploadEvidence" /></label>
        <div v-for="file in form.files" :key="file.id" class="oa-actions"><span>{{ file.name }}</span><button type="button" @click="form.files = form.files.filter(item => item.id !== file.id)">移除</button></div>
        <div class="oa-actions"><button v-if="returned" type="button" @click="returned = undefined; returnedTemplate = undefined">返回单据</button><button class="oa-button-primary" :disabled="busy || !active">{{ returned ? '重新提交' : '提交申请' }}</button></div>
      </form>
    </section>
    <section class="oa-panel">
      <h2>审批路径</h2>
      <ol v-if="active" class="oa-steps">
        <li v-for="item in active.nodes || []" :key="item">{{ nodeLabel(item) }}</li>
        <li v-if="active.threshold_cents && active.extra">达到 {{ major(active.threshold_cents) }} 时加签 {{ nodeLabel(active.extra) }}</li>
        <li>审批通过后登记付款</li>
      </ol>
      <p v-else class="oa-empty">选择模板后查看审批规则</p>
    </section>
  </div>

  <template v-else-if="mode === 'mine' || mode === 'approval' || mode === 'document'">
    <div v-if="mode === 'approval'" class="oa-actions oa-person-tabs">
      <button :aria-pressed="queue === 'todo'" :disabled="busy" @click="queue = 'todo'; run(loadDocs)">审批待办</button>
      <button v-if="canPay" :aria-pressed="queue === 'pay'" :disabled="busy" @click="queue = 'pay'; run(loadDocs)">待付款</button>
    </div>
    <section v-if="!detail" class="oa-panel">
      <h2>{{ queue === 'mine' ? '本人单据' : queue === 'pay' ? '待付款单据' : '待审批单据' }}</h2>
      <p v-if="!docs.length && !busy && !error" class="oa-empty">{{ mode === 'document' ? '未选中费用单据' : '暂无单据' }}</p>
      <div v-if="docs.length" class="oa-table-wrap">
        <table class="oa-table"><thead><tr><th>申请 / 单号</th><th>申请人</th><th>金额</th><th>来源</th><th>状态</th><th>操作</th></tr></thead><tbody>
          <tr v-for="item in docs" :key="item.id"><td><b>{{ item.template?.name }}</b><br><small class="oa-muted">{{ item.id }}</small></td><td>#{{ item.applicant_id }}</td><td>{{ major(item.amount_cents) }}</td><td>{{ item.site === 'asset' ? '资产办理区' : 'OA' }}</td><td><span class="oa-pill">{{ status(item.state) }}</span></td><td><button :disabled="busy" @click="run(() => open(item.id))">打开单据</button></td></tr>
        </tbody></table>
      </div>
    </section>
    <div v-else class="oa-split">
      <section class="oa-panel">
        <h2>{{ detail.template?.name || '费用单据' }} <span class="oa-pill">{{ status(detail.state) }}</span></h2>
        <dl class="oa-kv">
          <div><dt>单号</dt><dd>{{ detail.id }}</dd></div><div><dt>申请人</dt><dd>#{{ detail.applicant_id }}</dd></div>
          <div><dt>申请金额</dt><dd>{{ major(detail.amount_cents) }}</dd></div><div><dt>支付渠道</dt><dd>{{ channels[detail.channel] || detail.channel }}</dd></div>
          <div><dt>收款对象</dt><dd>{{ detail.holder }}</dd></div><div><dt>收款账号</dt><dd>{{ detail.account }} <button v-if="canPay && !own && detail.state === 'approved' && !revealed" :disabled="busy" @click="run(revealAccount)">查看完整账号</button></dd></div><div><dt>银行 / 平台</dt><dd>{{ detail.bank || '—' }}</dd></div>
          <div><dt>用途与说明</dt><dd>{{ detail.reason }}</dd></div><div v-for="(value, key) in detail.extra" :key="key"><dt>{{ key }}</dt><dd>{{ value }}</dd></div>
        </dl>
        <div class="oa-actions">
          <button v-for="id in detail.files || []" :key="id" :disabled="busy" @click="run(() => download('/oa-api/finance/files/' + id, '凭证-' + id))">下载凭证</button>
        </div>
        <div class="oa-actions oa-person-tabs"><RouterLink v-if="mode === 'document'" class="oa-button" :to="own ? '/fees/mine' : '/fees/approval'">返回列表</RouterLink><button v-else @click="run(loadDocs)">返回列表</button><button v-if="own && detail.state === 'returned'" class="oa-button-primary" @click="editReturned">修改并重提</button></div>
      </section>
      <section class="oa-panel">
        <h2>审批与付款记录</h2>
        <ol v-if="detail.route?.length" class="oa-steps"><li v-for="(item, index) in detail.route" :key="index">{{ node(item) }}<small v-if="detail.current === index"> · 当前节点</small></li></ol>
        <dl v-if="detail.history?.length" class="oa-kv"><div v-for="(entry, index) in detail.history" :key="index"><dt>{{ historyLabel(entry.action) }} <small>{{ new Date(entry.at).toLocaleString('zh-CN') }}</small></dt><dd>{{ entry.comment || '—' }}</dd></div></dl>
        <form v-if="(queue === 'todo' || mode === 'document') && mayDecide" class="oa-form" @submit.prevent="run(act)">
          <label>审批结果<select v-model="approval.decision"><option v-for="(text, key) in decisions" :key="key" :value="key">{{ text }}</option></select></label>
          <label>审批意见<textarea v-model="approval.comment" :required="approval.decision !== 'approve'" /></label>
          <button class="oa-button-primary" :disabled="busy">提交审批</button>
        </form>
        <form v-if="(queue === 'pay' || mode === 'document') && !own && canPay && detail.state === 'approved'" class="oa-form" @submit.prevent="run(pay)">
          <label>实付日期<input v-model="payment.date" type="date" :max="dateInChina()" required /></label>
          <label>金额<input :value="major(detail.amount_cents)" readonly /></label>
          <label>付款回单<input ref="receipt" type="file" @change="uploadReceipt" /></label>
          <p v-if="payment.receipt" class="oa-muted">回单已上传</p>
          <button class="oa-button-primary" :disabled="busy || !payment.receipt">登记已付款</button>
        </form>
        <template v-if="detail.payment">
          <dl class="oa-kv"><div><dt>实付日期</dt><dd>{{ detail.payment.date }}</dd></div><div><dt>实付金额</dt><dd>{{ major(detail.payment.amount_cents) }}</dd></div></dl>
          <button v-if="detail.payment.receipt" @click="run(() => download('/oa-api/finance/files/' + detail?.payment?.receipt, '付款回单'))">下载回单</button>
        </template>
      </section>
    </div>
  </template>

  <template v-else-if="mode === 'templates'">
    <div class="oa-split oa-sidebar-split">
      <section class="oa-panel">
        <h2>费用模板</h2>
        <div class="oa-list"><button v-for="item in templates" :key="item.id" @click="selection = item.id; beginTemplate(item)" :aria-current="selection === item.id">{{ item.name }} · v{{ item.version }}</button></div>
        <p v-if="!templates.length && !busy" class="oa-empty">暂无费用模板</p>
        <button v-if="level === 'draft' || level === 'publish'" @click="selection = ''; beginTemplate()">新建模板</button>
      </section>
      <section v-if="draft && (level === 'draft' || level === 'publish')" class="oa-panel">
        <h2>维护模板</h2>
        <form ref="templateForm" class="oa-form" @submit.prevent="run(() => saveTemplate(false))">
          <div class="oa-form-row">
            <label>模板 ID<input v-model.trim="draft.id" :readonly="Boolean(editingId)" pattern="[A-Za-z0-9_-]+" required /></label>
            <label>名称<input v-model.trim="draft.name" required /></label>
            <label>单据类型<select v-model="draft.type"><option>报销</option><option>付款</option></select></label>
            <label>可填写字段<input v-model="templateFields.fields" placeholder="费用归属，合同编号" /></label>
            <label>凭证要求<input v-model.trim="draft.evidence" placeholder="例如发票" required /></label>
          </div>
          <fieldset class="oa-fieldset"><legend>支付方式</legend><div class="oa-checks"><label v-for="(label, key) in channels" :key="key"><input v-model="draft.channels" type="checkbox" :value="key" />{{ label }}</label></div></fieldset>
          <fieldset v-if="list(templateFields.fields).length" class="oa-fieldset"><legend>必填字段</legend><div class="oa-checks"><label v-for="field in list(templateFields.fields)" :key="field"><input v-model="draft.required" type="checkbox" :value="field" />{{ field }}</label></div></fieldset>
          <fieldset class="oa-fieldset oa-form"><legend>审批节点</legend>
            <div class="oa-actions oa-filter"><label>搜索审批人<input v-model="approverQuery" placeholder="姓名或工号" @keydown.enter.prevent="findApprovers" /></label><button type="button" :disabled="approverBusy" @click="findApprovers">{{ approverBusy ? '搜索中…' : '搜索' }}</button></div>
            <p v-if="approverError" class="oa-error" role="alert">{{ approverError }}</p>
            <p v-else-if="!approverBusy && approverQuery && !approverOptions.length" class="oa-muted" role="status">没有匹配的在职人员。</p>
            <div v-for="(item, index) in draft.nodes" :key="index" class="oa-actions">
              <label>第 {{ index + 1 }} 步<select v-model="draft.nodes[index]" :aria-label="'第 ' + (index + 1) + ' 步'" required><option value="" disabled>选择审批人</option><option v-for="option in approvalChoices(item)" :key="option.value" :value="option.value">{{ option.label }}</option></select></label>
              <button type="button" :disabled="index === 0" @click="movingNode(index, -1)">上移</button>
              <button type="button" :disabled="index + 1 === draft.nodes.length" @click="movingNode(index, 1)">下移</button>
              <button type="button" @click="draft.nodes.splice(index, 1)">移除</button>
            </div>
            <button type="button" @click="draft.nodes.push(draft.nodes.length ? '' : 'manager')">添加审批节点</button>
          </fieldset>
          <div class="oa-form-row">
            <label>金额达到时加签（元）<input v-model.trim="templateFields.threshold" inputmode="decimal" pattern="[0-9]+(\.[0-9]{1,2})?" required /></label>
            <label v-if="Number(templateFields.threshold) > 0">加签审批人<select v-model="draft.extra" aria-label="加签审批人" required><option value="" disabled>选择审批人</option><option v-for="option in approvalChoices(draft.extra)" :key="option.value" :value="option.value">{{ option.label }}</option></select></label>
          </div>
          <div class="oa-actions"><button :disabled="busy">保存草稿</button><button v-if="level === 'publish'" class="oa-button-primary" type="button" :disabled="busy" @click="run(() => saveTemplate(true))">发布模板</button></div>
        </form>
      </section>
    </div>
    <section v-if="isAdmin" class="oa-panel">
      <h2>模板维护委托</h2>
      <dl v-if="delegates.length" class="oa-kv"><div v-for="item in delegates" :key="item.id"><dt>账号 #{{ item.id }}</dt><dd>{{ { none: '无模板维护权', draft: '编辑草稿', publish: '编辑并发布' }[item.level] || item.level }}</dd></div></dl>
      <form class="oa-form" @submit.prevent="run(saveDelegate)">
        <div class="oa-form-row"><label>受托人账号 ID<input v-model.trim="delegate.id" pattern="[0-9]+" required /></label><label>维护权限<select v-model="delegate.level"><option value="none">无模板维护权</option><option value="draft">编辑草稿</option><option value="publish">编辑并发布</option></select></label></div>
        <button class="oa-button-primary" :disabled="busy">保存委托</button>
      </form>
    </section>
    <section v-if="isAdmin" class="oa-panel">
      <h2>财务经办授权</h2><p class="oa-subtitle">付款登记与支出统计可以分别授权。</p>
      <form class="oa-form oa-person-tabs" @submit.prevent="run(saveFinanceAccess)">
        <label>经办账号 ID<input v-model.trim="grants.id" pattern="[1-9][0-9]*" required /></label>
        <div class="oa-checks"><label><input v-model="grants.pay" type="checkbox" />付款登记</label><label><input v-model="grants.report" type="checkbox" />支出统计</label></div>
        <button class="oa-button-primary" :disabled="busy">保存财务授权</button>
      </form>
    </section>
  </template>

  <section v-else-if="mode === 'report'" class="oa-panel">
    <h2>按实付日期统计</h2>
    <p v-if="!canReport" class="oa-error" role="alert">当前账号没有财务报表权限。</p>
    <template v-else>
      <form class="oa-actions oa-filter" @submit.prevent="run(loadReport)"><label>期间（年 / 月 / 日）<input v-model.trim="period" pattern="[0-9]{4}(-[0-9]{2}){0,2}" required /></label><button :disabled="busy">查询</button></form>
      <dl v-if="report" class="oa-kv"><div><dt>期间实付</dt><dd>{{ major(report.amount_cents) }}</dd></div><div><dt>实付张数</dt><dd>{{ report.count }}</dd></div><div v-if="report.unpaid_cents != null"><dt>已批未付</dt><dd>{{ major(report.unpaid_cents) }}</dd></div></dl>
      <div v-if="report?.items?.length" class="oa-table-wrap"><table class="oa-table"><thead><tr><th>单号</th><th>实付日期</th><th>实付金额</th><th>操作</th></tr></thead><tbody><tr v-for="item in report.items" :key="item.id"><td>{{ item.id }}</td><td>{{ item.payment?.date }}</td><td>{{ major(item.payment?.amount_cents) }}</td><td><RouterLink :to="{ path: '/fees/document', query: { id: item.id } }">打开单据</RouterLink></td></tr></tbody></table></div>
    </template>
  </section>
</template>
