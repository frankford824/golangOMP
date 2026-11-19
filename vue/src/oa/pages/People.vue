<script setup lang="ts">
import OAEmpty from '../components/OAEmpty.vue'
import { computed, onMounted, ref, watch } from 'vue'
import { RouterLink, useRoute, useRouter } from 'vue-router'
import OAReturnLink from '../components/OAReturnLink.vue'
import { directoryQuery, peopleReturnSource, positiveQueryNumber } from '../peopleNavigation'
import BaseTablePager from '@/components/base/BaseTablePager.vue'
import PageHead from '../components/PageHead.vue'
import PersonnelTerms from '../components/PersonnelTerms.vue'
import { api, download, errorText, saveFile, send } from '../api'
import { useOAAccess } from '../access'
import '../styles.css'

const { can, currentProfile } = useOAAccess()
const route = useRoute()
const router = useRouter()

type Person = { id: number; name: string; employee_no?: number | null; department?: string; team?: string }
type PreviewRow = { row: number; name?: string; error?: string; change?: string; status?: string; sheet?: string; failure?: string; result?: 'applied' | 'unchanged'; employee_no?: number; previous_employee_no?: number; new_name?: string; previous_name?: string; employment_type?: string; shop_id?: string; from?: string }
type Preview = { token: string; items: PreviewRow[] }
const props = defineProps<{ mode: 'people' | 'roster' }>()
const allowed = computed(() => can('people.manage'))
const query = ref(typeof route.query.q === 'string' ? route.query.q : '')
const applied = ref(query.value)
const items = ref<Person[]>([])
const hasLoadedList = ref(false)
const selected = computed(() => typeof route.query.user_id === 'string' && /^[1-9][0-9]*$/.test(route.query.user_id) ? route.query.user_id : undefined)
const rosterReturnTarget = computed(() => peopleReturnSource(route.query.peopleFrom)
  ?? peopleReturnSource(window.history.state?.back)
  ?? { path: '/users', query: {
    q: typeof route.query.q === 'string' ? route.query.q : undefined,
    department: route.query.department,
    team: route.query.team,
    status: route.query.status,
    page: route.query.page,
    page_size: route.query.page_size,
    user_id: selected.value,
    tab: selected.value ? 'terms' : undefined,
  } })
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
const pageStart = computed(() => (page.value - 1) * pageSize)
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
  applied.value = query.value.trim()
  items.value = []
  const result = await api<{ items: Person[] }>('/oa-api/people/list?' + new URLSearchParams({ q: applied.value }))
  items.value = result.items ?? []
  hasLoadedList.value = true
  page.value = Math.min(requestedPage, Math.max(1, Math.ceil(items.value.length / pageSize)))
}

function chooseFile(event: Event) {
  selectedFile.value = (event.target as HTMLInputElement).files?.[0]
  preview.value = undefined
  page.value = 1
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
  page.value = next
  void router.replace({ query: { ...route.query, page: next > 1 ? String(next) : undefined } })
}

watch(() => route.query, (current) => {
  if (props.mode !== 'people' || route.path !== '/users' || selected.value) return
  const nextKeyword = typeof current.q === 'string' ? current.q : ''
  const nextPage = positiveQueryNumber(current.page)
  if (nextKeyword === applied.value && hasLoadedList.value) {
    page.value = Math.min(nextPage, Math.max(1, Math.ceil(items.value.length / pageSize)))
    return
  }
  query.value = nextKeyword
  void run(() => refresh(nextPage))
})
onMounted(() => { if (props.mode === 'people' && !selected.value) void run(() => refresh(page.value)) })
</script>

<template>
<div class="oa-page">
  <PageHead :title="mode === 'roster' ? '人员花名册' : selectedName || (selected ? '人员详情' : '人员管理')">
    <OAReturnLink v-if="mode === 'roster'" class="oa-button" :to="rosterReturnTarget">返回人员管理</OAReturnLink>
    <OAReturnLink v-else-if="selected" class="oa-button" :to="directoryTarget">返回目录</OAReturnLink>
    <RouterLink v-else class="oa-button" :to="{ path: '/users/roster', query: { peopleFrom: route.fullPath } }">人员花名册</RouterLink>
    <RouterLink v-if="currentProfile?.super_admin" class="oa-button" :to="{ name: 'PeopleShopBindings', query: { user_id: selected || undefined, peopleFrom: route.fullPath } }">店铺人员绑定</RouterLink>
  </PageHead>
  <p v-if="!allowed" class="oa-error" role="alert">当前账号没有人员管理权限。</p>
  <template v-else>
    <div v-if="error" class="oa-error" role="alert">{{ error }} <button v-if="mode === 'people' && !selected" type="button" @click="run(refresh)">重新加载人员</button></div>
    <p v-if="notice" class="oa-notice" role="status">{{ notice }}</p>
    <section v-if="mode === 'people' && selected" class="oa-panel">
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
        <li v-for="item in items.slice(pageStart, pageStart + pageSize)" :key="item.id">
          <RouterLink :to="{ path: '/users', query: { ...route.query, user_id: item.id, tab: 'terms', q: applied || undefined } }">
            <strong>{{ item.name }}</strong><span>{{ employeeNo(item.employee_no) }} · {{ item.team || item.department || '未分配' }}</span>
          </RouterLink>
        </li>
      </ul>
      <BaseTablePager v-if="items.length > pageSize" :page="page" @update:page="changeDirectoryPage" :total="items.length" :page-size="pageSize" :show-page-size="false" compact />
    </section>
    <template v-else>
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
          <p class="oa-muted">按稳定人员 ID 回导。每行预览姓名、工号、用工类型或店铺任职变化，冲突行单独跳过。</p>
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
        <BaseTablePager v-if="preview.items.length > pageSize" v-model:page="page" :total="preview.items.length" :page-size="pageSize" :show-page-size="false" compact />
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