<script setup lang="ts">
import OAEmpty from '../components/OAEmpty.vue'
import { computed, onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import OAPageReturn from '../components/OAPageReturn.vue'
import { directoryQuery, positiveQueryNumber } from '../peopleNavigation'
import BaseTablePager from '@/components/base/BaseTablePager.vue'
import PageHead from '../components/PageHead.vue'
import PersonnelProfile from '../components/PersonnelProfile.vue'
import OrganizationManager from '../components/OrganizationManager.vue'
import PersonnelTerms from '../components/PersonnelTerms.vue'
import { api, download, errorText, saveFile, send } from '../api'
import { useOAAccess } from '../access'
import '../styles.css'

const { can, currentProfile } = useOAAccess()
const route = useRoute()
const router = useRouter()

type Person = { id: number; name: string; employee_no?: number | null; department?: string; team?: string; department_id: number; team_id: number; status: string; employment_type?: string; manager_name?: string; joined?: string; job_title?: string; job_level?: string; cost_center?: string }
type PreviewRow = { row: number; name?: string; error?: string; change?: string; status?: string; sheet?: string; failure?: string; result?: 'applied' | 'unchanged'; employee_no?: number; previous_employee_no?: number; new_name?: string; previous_name?: string; employment_type?: string; shop_id?: string; from?: string; profile?: { manager_id: number; joined: string; job_title: string; job_level: string; cost_center: string } }
type Preview = { token: string; items: PreviewRow[] }
const props = defineProps<{ mode: 'people' | 'roster' }>()
const allowed = computed(() => can('people.manage'))
const query = ref(typeof route.query.q === 'string' ? route.query.q : '')
const applied = ref(query.value)
const items = ref<Person[]>([])
const hasLoadedList = ref(false)
const selected = computed(() => typeof route.query.user_id === 'string' && /^[1-9][0-9]*$/.test(route.query.user_id) ? route.query.user_id : undefined)
const directoryTarget = computed(() => ({ path: '/users', query: directoryQuery(route.query) }))
const selectedRecord = ref<{ id: number; name: string }>()
const selectedName = computed(() => String(selectedRecord.value?.id) === selected.value ? selectedRecord.value?.name : items.value.find(item => String(item.id) === selected.value)?.name)
const busy = ref(false)
const error = ref('')
const notice = ref('')
const selectedFile = ref<File>()
const preview = ref<Preview>()
const pageSize = 20
const page = ref(props.mode === 'people' ? positiveQueryNumber(route.query.page) : 1)
const previewPage = ref(1)
const pageStart = computed(() => (previewPage.value - 1) * pageSize)
const total = ref(0)
const status = ref('')
const missing = ref('')
const department = ref('')
const workerType = ref('')
const rosterFilters = ref<Record<string, string>>({ q: query.value.trim() })
const orgChoice = ref('')
type OrgManager = { kind: 'department' | 'team'; id: number; name: string; department_id: number }
const orgOptions = ref<OrgManager[]>([])
const departments = computed(() => orgOptions.value.filter(item => item.kind === 'department'))
const chosenOrg = computed(() => orgOptions.value.find(item => item.kind + '/' + item.id === orgChoice.value))
async function loadOrgFilters() {
  const result = await api<{ items: OrgManager[] }>('/oa-api/people/org-managers')
  orgOptions.value = result.items
}
const employment = (kind?: string) => ({ full_time: '全职', part_time: '兼职' }[kind || ''] || kind || '—')
const employeeNo = (no?: number | null) => no == null ? '—' : String(no).padStart(4, '0')

async function run(action: () => Promise<void>) {
  if (busy.value) return
  busy.value = true
  error.value = ''
  notice.value = ''
  try { await action() }
  catch (cause) { error.value = errorText(cause) }
  finally { busy.value = false }
}

async function refresh(requestedPage = 1) {
  if (!allowed.value) return
  applied.value = props.mode === 'roster' ? rosterFilters.value.q : query.value.trim()
  items.value = []
  const params = new URLSearchParams({
    ...(props.mode === 'roster' ? rosterFilters.value : { q: applied.value }),
    page: String(requestedPage), page_size: String(pageSize),
  })
  const result = await api<{ items: Person[]; total: number; page: number }>('/oa-api/people/roster?' + params)
  const lastPage = Math.max(1, Math.ceil(result.total / pageSize))
  if (result.page > lastPage) {
    await refresh(lastPage)
    return
  }
  items.value = result.items
  hasLoadedList.value = true
  total.value = result.total
  page.value = result.page
}

function applyRosterFilters() {
  const filters: Record<string, string> = { q: query.value.trim() }
  if (status.value) filters.status = status.value
  if (missing.value) filters.missing = missing.value
  if (department.value) filters.department_id = department.value
  if (workerType.value) filters.employment_type = workerType.value
  rosterFilters.value = filters
  void run(() => refresh())
}

function chooseFile(event: Event) {
  selectedFile.value = (event.target as HTMLInputElement).files?.[0]
  preview.value = undefined
  previewPage.value = 1
  notice.value = ''
}
function previewImport() {
  void run(async () => {
    if (!selectedFile.value) return
    const form = new FormData()
    form.set('file', selectedFile.value)
    preview.value = await api<Preview>('/oa-api/people/preview', { method: 'POST', body: form })
  })
}
function confirmImport() {
  void run(async () => {
    if (!preview.value?.token || !hasEligibleLines.value) return
    const result = await send<{ count: number; items: PreviewRow[] }>('/oa-api/people/confirm', { token: preview.value.token })
    preview.value.items = result.items ?? preview.value.items
    const failures = preview.value.items.filter(row => row.failure).length
    notice.value = failures ? `已更新 ${result.count} 条，${failures} 条确认失败，可重试。` : `已确认回导，共更新 ${result.count} 条记录`
    await refresh(page.value)
  })
}
function importDifference(row: PreviewRow) {
  if (row.failure) return row.failure
  if (row.error) return row.error
  if (row.result === 'applied') return '已更新'
  if (row.result === 'unchanged') return '无变化'
  const changes = [
    row.employee_no != null ? `工号 ${employeeNo(row.previous_employee_no)} → ${employeeNo(row.employee_no)}` : '',
    row.new_name ? `姓名 ${row.previous_name || row.name} → ${row.new_name}` : '',
    row.employment_type ? `用工类型 → ${employment(row.employment_type)}` : '',
    row.profile ? `档案 · 直属负责人 ${row.profile.manager_id ? '#' + row.profile.manager_id : '随组织'} · 入职 ${row.profile.joined || '未填'} · 岗位 ${row.profile.job_title || '未填'} · 职级 ${row.profile.job_level || '未填'} · 成本中心 ${row.profile.cost_center || '未填'}` : '',
    row.shop_id && row.from ? `店铺 ${row.shop_id} · ${row.from} 生效` : '',
  ].filter(Boolean)
  return changes.join('；') || row.change || row.status || '无变化'
}
const importErrors = computed(() => preview.value?.items?.filter(row => row.error || row.failure) ?? [])
const hasEligibleLines = computed(() => preview.value?.items?.some(row => !row.error && (!row.result || row.failure)))
function downloadConflicts() {
  const quote = (value: unknown) => {
    const source = String(value ?? '')
    const safe = /^[=+\-@\t\r]/.test(source) ? "'" + source : source
    return '"' + safe.replace(/"/g, '""') + '"'
  }
  const rows = [['工作表', '行号', '姓名', '冲突原因'], ...importErrors.value.map(row => [row.sheet || '人员', row.row, row.name, row.error || row.failure])]
  saveFile(new Blob(['\uFEFF', rows.map(row => row.map(quote).join(',')).join('\r\n')], { type: 'text/csv;charset=utf-8' }), '花名册回导冲突.csv')
}
async function submitDirectorySearch() {
  await run(refresh)
  if (!error.value) await router.replace({ query: { ...route.query, q: applied.value || undefined, page: undefined } })
}

function changeDirectoryPage(next: number) {
  if (props.mode === 'roster') {
    void run(() => refresh(next))
    return
  }
  void router.replace({ query: { ...route.query, page: next > 1 ? String(next) : undefined } })
}

watch(() => route.query, (current) => {
  if (props.mode !== 'people' || route.path !== '/users' || selected.value) return
  const nextKeyword = typeof current.q === 'string' ? current.q : ''
  const nextPage = positiveQueryNumber(current.page)
  if (nextKeyword === applied.value && nextPage === page.value && hasLoadedList.value) return
  query.value = nextKeyword
  void run(() => refresh(nextPage))
})
onMounted(() => {
  if (props.mode === 'roster') void run(async () => { await Promise.all([refresh(page.value), loadOrgFilters()]) })
  else if (!selected.value) void run(() => refresh(page.value))
})
</script>

<template>
<div class="oa-page">
  <OAPageReturn v-if="selected && mode !== 'roster'" :to="directoryTarget">返回目录</OAPageReturn>
  <PageHead :title="mode === 'roster' ? '人员花名册' : selectedName || (selected ? '人员详情' : '人员管理')">
    <RouterLink v-if="selected && mode !== 'roster' && currentProfile?.super_admin" class="oa-button" :to="{ name: 'PeopleShopBindings', query: { user_id: selected || undefined, peopleFrom: route.fullPath } }">店铺人员绑定</RouterLink>
  </PageHead>
  <p v-if="!allowed" class="oa-error" role="alert">当前账号没有人员管理权限。</p>
  <template v-else>
    <div v-if="error" class="oa-error" role="alert">{{ error }} <button v-if="mode === 'people' && !selected" type="button" @click="run(refresh)">重新加载人员</button></div>
    <p v-if="notice" class="oa-notice" role="status">{{ notice }}</p>
    <section v-if="mode === 'people' && selected" class="oa-panel">
      <PersonnelProfile :key="selected" :user-id="selected" />
      <PersonnelTerms :key="selected" :user-id="selected" show-profile @loaded="selectedRecord = $event" />
    </section>
    <section v-else-if="mode === 'people'" class="oa-panel">
      <form class="oa-actions oa-filter" @submit.prevent="submitDirectorySearch">
        <input v-model="query" aria-label="搜索姓名或工号" placeholder="搜索姓名或工号" />
        <button :disabled="busy">搜索</button>
      </form>
      <p v-if="busy && !items.length" class="oa-empty" role="status">正在读取人员…</p>
      <OAEmpty v-else-if="!items.length && !error" kind="search" title="暂无匹配人员" description="试试其他姓名或工号，或清除搜索条件。" />
      <ul class="oa-list oa-link-list">
        <li v-for="item in items" :key="item.id">
          <RouterLink :to="{ path: '/users', query: { ...route.query, user_id: item.id, tab: 'terms', q: applied || undefined } }">
            <strong>{{ item.name }}</strong><span>{{ employeeNo(item.employee_no) }} · {{ item.team || item.department || '未分配' }}</span>
          </RouterLink>
        </li>
      </ul>
      <BaseTablePager v-if="total > pageSize" :page="page" @update:page="changeDirectoryPage" :total="total" :page-size="pageSize" :show-page-size="false" compact />
    </section>
    <template v-else>
      <section class="oa-panel">
        <form class="oa-actions oa-filter" aria-label="花名册筛选" @submit.prevent="applyRosterFilters">
          <input v-model="query" type="search" aria-label="搜索姓名或工号" placeholder="姓名或工号" />
          <select v-model="status" aria-label="账号状态"><option value="">全部账号状态</option><option value="active">启用</option><option value="disabled">禁用</option></select>
          <select v-model="department" aria-label="部门"><option value="">全部部门</option><option v-for="org in departments" :key="org.id" :value="String(org.id)">{{ org.name }}</option></select>
          <select v-model="workerType" aria-label="用工类型"><option value="">全部用工类型</option><option value="full_time">全职</option><option value="part_time">兼职</option></select>
          <select v-model="missing" aria-label="缺项"><option value="">全部档案完整度</option><option value="manager">缺直属负责人</option><option value="joined">缺入职日期</option><option value="employee_no">缺工号</option></select>
          <button :disabled="busy">筛选</button>
        </form>
        <p v-if="busy && !hasLoadedList" role="status">读取花名册中…</p>
        <OAEmpty v-else-if="!items.length && !error" kind="search" title="暂无匹配人员" />
        <div v-if="items.length" class="oa-table-wrap desktop-region-scroll" tabindex="0" role="region" aria-label="在线人员花名册">
          <table class="oa-table">
            <thead><tr><th>姓名</th><th>工号</th><th>部门 / 小组</th><th>账号状态</th><th>用工类型</th><th>直属负责人</th><th>入职日期</th><th>岗位 / 职级</th><th>成本中心</th></tr></thead>
            <tbody><tr v-for="item in items" :key="item.id">
              <td><RouterLink :to="{ path: '/users', query: { user_id: item.id, tab: 'profile' } }">{{ item.name }}</RouterLink></td>
              <td>{{ employeeNo(item.employee_no) }}</td><td>{{ [item.department, item.team].filter(Boolean).join(' / ') || '—' }}</td>
              <td>{{ item.status === 'active' ? '启用' : '禁用' }}</td><td>{{ employment(item.employment_type) }}</td>
              <td>{{ item.manager_name || '—' }}</td><td>{{ item.joined || '—' }}</td>
              <td>{{ [item.job_title, item.job_level].filter(Boolean).join(' / ') || '—' }}</td><td>{{ item.cost_center || '—' }}</td>
            </tr></tbody>
          </table>
        </div>
        <BaseTablePager v-if="total > pageSize" :page="page" :total="total" :page-size="pageSize" :show-page-size="false" compact @update:page="changeDirectoryPage" />
      </section>
      <details v-if="orgOptions.length" class="oa-panel">
        <summary>组织默认负责人</summary>
        <div class="oa-form">
          <label>部门 / 小组<select v-model="orgChoice" aria-label="选择部门或小组"><option value="">选择组织</option><option v-for="org in orgOptions" :key="org.kind + '/' + org.id" :value="org.kind + '/' + org.id">{{ org.kind === 'team' ? (departments.find(department => department.id === org.department_id)?.name || '小组') + ' / ' : '' }}{{ org.name }}</option></select></label>
          <OrganizationManager v-if="chosenOrg" :kind="chosenOrg.kind" :id="chosenOrg.id" @saved="run(() => refresh(page))" />
        </div>
      </details>
      <div class="oa-split">
        <section class="oa-panel">
          <h2>回导 Excel</h2>
          <form class="oa-form" @submit.prevent="previewImport">
            <label><span>名册 Excel 文件 <span class="personnel-required" aria-hidden="true">*</span></span><input type="file" accept=".xlsx" required :disabled="busy" aria-describedby="people-roster-upload-hint" @change="chooseFile" /><small id="people-roster-upload-hint" class="oa-muted">上传 .xlsx 文件，预览差异后再确认回导。</small></label>
            <button class="oa-button-primary" :disabled="busy || !selectedFile">预览差异</button>
          </form>
        </section>
        <section class="oa-panel oa-form">
          <h2>下载花名册</h2>
          <p class="oa-muted">按人员 ID 核对身份。直属负责人可填用户 ID、唯一工号或姓名。负责人两列留空时跟随所在组织。</p>
          <button :disabled="busy" @click="run(() => download('/oa-api/people/export', '永箔花名册.xlsx'))">导出花名册</button>
        </section>
      </div>
      <section v-if="preview" class="oa-panel">
        <h2>回导差异预览</h2>
        <p class="oa-muted">{{ preview.items?.length || 0 }} 行 · {{ importErrors.length }} 行冲突</p>
        <div class="oa-table-wrap desktop-region-scroll" tabindex="0" role="region" aria-label="花名册差异预览">
          <table class="oa-table"><thead><tr><th>工作表</th><th class="personnel-numeric" scope="col">行号</th><th>姓名</th><th>预览结果</th></tr></thead><tbody>
            <tr v-for="line in preview.items.slice(pageStart, pageStart + pageSize)" :key="(line.sheet || '人员') + line.row"><td>{{ line.sheet || '人员' }}</td><td class="personnel-numeric">{{ line.row }}</td><td>{{ line.name || '—' }}</td><td><span :class="['oa-pill', { 'oa-pill-warn': line.error || line.failure }]">{{ importDifference(line) }}</span></td></tr>
          </tbody></table>
        </div>
        <BaseTablePager v-if="preview.items.length > pageSize" v-model:page="previewPage" :total="preview.items.length" :page-size="pageSize" :show-page-size="false" compact />
        <div class="oa-actions oa-person-tabs"><button v-if="importErrors.length" @click="downloadConflicts">下载冲突行</button><button class="oa-button-primary" :disabled="busy || !hasEligibleLines || !preview.token" @click="confirmImport">{{ preview.items?.some(row => row.failure) ? '重试失败行' : '确认回导' }}</button></div>
      </section>
    </template>
  </template>
</div>
</template>

<style scoped>
.personnel-numeric { text-align: right; font-variant-numeric: tabular-nums; }
.personnel-required { color: var(--oa-danger, rgb(var(--yb-danger-text))); }
.oa-link-list a { min-height: 32px; min-width: 32px; }
</style>