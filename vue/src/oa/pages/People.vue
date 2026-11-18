<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { RouterLink } from 'vue-router'
import PageHead from '../components/PageHead.vue'
import { api, dateInChina, download, errorText, saveFile, send } from '../api'
import { useOAAccess } from '../access'
import { usePermissionsStore } from '@/stores/permissions'
import '../styles.css'

const { can } = useOAAccess()
const permissions = usePermissionsStore()

type Person = {
  id: number; name: string; employee_no?: number | null; department_id: number; team_id: number
  department?: string; team?: string; management?: string; roles?: string[]; employment_type?: string
  joined?: string; left?: string; manager_id?: number; version: number; terms?: Term[]; events?: PersonEvent[]
  status?: string
}
type Term = { shop_id: string; shop_name?: string; from?: string; to?: string; start_date?: string; end_date?: string }
type PersonEvent = { kind: string; effective_date: string; target_id?: number; shop_id?: string; status?: string }
type Handover = { items: { id: number; task_no: string; title: string; status: string }[]; shops: string[] }
type PreviewRow = { row: number; name?: string; error?: string; change?: string; status?: string; sheet?: string; failure?: string; result?: 'applied' | 'unchanged'; employee_no?: number; previous_employee_no?: number; new_name?: string; previous_name?: string; employment_type?: string; shop_id?: string; from?: string }
type Preview = { token: string; items: PreviewRow[] }
const props = defineProps<{ mode: 'people' | 'roster' }>()
const allowed = computed(() => can('people.manage'))
const query = ref('')
const items = ref<Person[]>([])
const person = ref<Person>()
const tab = ref('profile')
const busy = ref(false)
const error = ref('')
const notice = ref('')
const selectedFile = ref<File>()
const preview = ref<Preview>()
const editor = ref<HTMLDialogElement>()
const canManageAccounts = computed(() => permissions.hasMenu('user_admin') && permissions.hasAction('access.manage'))
const handover = ref<Handover>()
const handoverError = ref('')
const impactCheck = ref(false)
const operation = ref<ReturnType<typeof blank>>()
const kinds: Record<string, string> = { hire: '入职', transfer: '调岗', shop_transfer: '调店', leave: '离职', handover: '交接', rehire: '返聘', roles: '角色与管理权限' }
const roleLabels: Record<string, string> = { custom_designer: '定制设计师', auditor: '审核', designer: '常规设计师', operations: '运营', asset_submit: '素材提交', asset_manage: '素材管理', asset_settle: '素材结算' }
const managementLabels: Record<string, string> = { none: '无管理权限', department: '部门管理', super_admin: '超级管理员' }

function blank(p: Person, kind: string) {
  return {
    kind, version: p.version, effective_date: dateInChina(),
    shop_id: '', from_shop_id: '', target_id: 0,
  }
}
const effectiveManagement = computed(() => managementLabels[person.value?.management ?? ''] || '待核实')
const currentRoles = computed(() => person.value?.roles?.map(role => roleLabels[role] || role).join('、') || '无')
const employment = (kind?: string) => ({ full_time: '全职', part_time: '兼职' }[kind || ''] || kind || '—')
const employeeNo = (no?: number | null) => no == null ? '—' : String(no).padStart(4, '0')
const identifier = (value?: number, name?: string) => name || (value ? '#' + value : '—')
const personName = (id?: number) => items.value.find(item => item.id === id)?.name || (id ? '#' + id : '—')

async function run(action: () => Promise<void>) {
  if (busy.value) return
  busy.value = true
  error.value = ''
  notice.value = ''
  try { await action() }
  catch (cause) { error.value = errorText(cause) }
  finally { busy.value = false }
}

async function readPerson(id: number) {
  person.value = await api<Person>(`/oa-api/people/${id}`)
  tab.value = 'profile'
  handover.value = undefined
  handoverError.value = ''
}
async function openHandover() {
  tab.value = 'terms'
  if (!person.value) return
  const id = person.value.id
  handoverError.value = ''
  handover.value = undefined
  try {
    const result = await api<Handover>(`/oa-api/people/${id}/handover`)
    if (person.value?.id === id) handover.value = result
  } catch (cause) {
    if (person.value?.id === id) handoverError.value = errorText(cause)
  }
}
async function refresh() {
  if (!allowed.value) return
  const selected = person.value?.id
  const data = await api<{ items: Person[] }>('/oa-api/people/list?' + new URLSearchParams({ q: query.value.trim() }))
  items.value = data.items ?? []
  const next = items.value.find(item => item.id === selected) ?? items.value[0]
  if (next) await readPerson(next.id)
  else person.value = undefined
}

function open(kind: string) {
  if (!person.value) return
  operation.value = blank(person.value, kind)
  impactCheck.value = false
  editor.value?.showModal()
}
function saveChange() {
  void run(async () => {
    if (!operation.value || !person.value) return
    const id = person.value.id
    await send(`/oa-api/people/${id}/change`, operation.value)
    editor.value?.close()
    await readPerson(id)
    notice.value = '变更已保存'
  })
}

function chooseFile(event: Event) {
  selectedFile.value = (event.target as HTMLInputElement).files?.[0]
  preview.value = undefined
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
onMounted(() => { if (props.mode === 'people') void run(refresh) })
</script>

<template>
<div class="oa-page">
  <PageHead section="公司事务 / 组织与人员" :title="mode === 'people' ? '人员管理' : '组织任职与花名册'">
    <RouterLink v-if="mode === 'people' && canManageAccounts" class="oa-button oa-button-primary" to="/users">办理入职与账号管理</RouterLink>
    <RouterLink v-if="mode === 'people' && allowed" class="oa-button" to="/people/roster">名册导入 / 导出</RouterLink>
    <RouterLink v-if="mode === 'roster' && allowed" class="oa-button" to="/people">返回人员管理</RouterLink>
  </PageHead>
  <p v-if="!allowed" class="oa-error" role="alert">当前账号没有人员管理权限。</p>
  <template v-else>
    <p v-if="error" class="oa-error" role="alert">{{ error }}</p>
    <p v-if="notice" class="oa-notice" role="status">{{ notice }}</p>
    <div v-if="mode === 'people'" class="oa-split oa-sidebar-split">
      <section class="oa-panel">
        <h2>人员目录</h2>
        <form class="oa-actions oa-filter" @submit.prevent="run(refresh)">
          <input v-model="query" aria-label="搜索姓名或工号" placeholder="搜索姓名或工号" />
          <button :disabled="busy">搜索</button>
        </form>
        <p v-if="busy && !items.length" class="oa-empty" role="status">正在读取人员…</p>
        <p v-else-if="!items.length && !error" class="oa-empty">暂无匹配人员</p>
        <div class="oa-list">
          <button v-for="item in items" :key="item.id" :aria-current="person?.id === item.id" :disabled="busy" @click="run(() => readPerson(item.id))">
            <span class="oa-avatar">{{ item.name.slice(-1) }}</span>
            <span class="oa-person-choice"><strong>{{ item.name }}</strong><small>{{ employeeNo(item.employee_no) }} · {{ identifier(item.team_id, item.team) }}</small></span>
            <span v-if="item.left && item.left <= dateInChina()" class="oa-pill oa-pill-warn">已离职</span>
          </button>
        </div>
      </section>
      <section v-if="person" class="oa-panel">
        <h2>{{ person.name }} · {{ employeeNo(person.employee_no) }}</h2>
        <p class="oa-muted">人员 ID {{ person.id }}</p>
        <div class="oa-actions oa-person-tabs">
          <button :aria-pressed="tab === 'profile'" @click="tab = 'profile'">人员资料</button>
          <button :aria-pressed="tab === 'access'" @click="tab = 'access'">角色与管理权限</button>
          <button :aria-pressed="tab === 'terms'" @click="openHandover">任职与交接</button>
        </div>
        <template v-if="tab === 'profile'">
          <dl class="oa-kv">
            <div><dt>工号 / 用工</dt><dd>{{ employeeNo(person.employee_no) }} · {{ employment(person.employment_type) }}</dd></div>
            <div><dt>部门 / 团队</dt><dd>{{ identifier(person.department_id, person.department) }} / {{ identifier(person.team_id, person.team) }}</dd></div>
            <div><dt>入职日期</dt><dd>{{ person.joined || '未登记' }}</dd></div>
            <div><dt>离职日期</dt><dd>{{ person.left || '—' }}</dd></div>
          </dl>
          <RouterLink v-if="canManageAccounts" class="oa-button" :to="{ path: '/users', query: { user_id: person.id } }">办理正式资料与状态</RouterLink>
        </template>
        <template v-else-if="tab === 'access'">
          <dl class="oa-kv"><div><dt>业务角色</dt><dd>{{ currentRoles }}</dd></div><div><dt>管理权限</dt><dd>{{ effectiveManagement }}</dd></div></dl>
          <p v-if="person.management === 'super_admin'" class="oa-muted">超级管理员权限由受保护的账号设置维护。</p>
          <RouterLink v-else-if="canManageAccounts" class="oa-button" :to="{ path: '/users', query: { user_id: person.id } }">前往正式角色授权</RouterLink>
        </template>
        <template v-else>
          <h3>店铺任职</h3>
          <dl class="oa-kv"><div v-for="(term, index) in person.terms || []" :key="index"><dt>{{ term.shop_name || term.shop_id }}</dt><dd>{{ term.from || term.start_date || '—' }} {{ term.to || term.end_date ? '至 ' + (term.to || term.end_date) : '至今' }}</dd></div></dl>
          <p v-if="!person.terms?.length" class="oa-empty">暂无店铺任职记录</p>
          <h3>任职与交接记录</h3>
          <div v-if="person.events?.length" class="oa-table-wrap">
            <table class="oa-table"><thead><tr><th>事项</th><th>生效时间</th><th>店铺</th><th>接手人</th><th>状态</th></tr></thead>
              <tbody><tr v-for="(event, index) in person.events" :key="index"><td>{{ kinds[event.kind] || event.kind }}</td><td>{{ event.effective_date }}</td><td>{{ event.shop_id || '—' }}</td><td>{{ personName(event.target_id) }}</td><td>{{ event.status || '已登记' }}</td></tr></tbody>
            </table>
          </div>
          <p v-else class="oa-empty">暂无人员变动</p>
          <h3>待交接任务</h3>
          <p v-if="handoverError" class="oa-error">{{ handoverError }}</p>
          <dl v-else-if="handover?.items?.length" class="oa-kv"><div v-for="task in handover.items" :key="task.id"><dt>{{ task.task_no }} · {{ task.title || task.status }}</dt><dd><RouterLink :to="'/tasks/' + task.id">打开任务办理</RouterLink></dd></div></dl>
          <p v-else-if="handover" class="oa-empty">暂无待交接任务</p>
          <p v-if="handover?.shops?.length" class="oa-muted">待交接店铺 {{ handover.shops.join('、') }}</p>
          <div class="oa-actions"><button v-if="!person.left" @click="open('shop_transfer')">办理调店</button><button @click="open('handover')">办理交接</button></div>
        </template>
      </section>
    </div>
    <template v-else>
      <div class="oa-split">
        <section class="oa-panel">
          <h2>回导 Excel</h2>
          <form class="oa-form" @submit.prevent="previewImport">
            <label>名册 Excel 文件<input type="file" accept=".xlsx" @change="chooseFile" /></label>
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
        <p class="oa-subtitle">{{ preview.items?.length || 0 }} 行 · {{ importErrors.length }} 行冲突</p>
        <div class="oa-table-wrap">
          <table class="oa-table"><thead><tr><th>工作表</th><th>行号</th><th>姓名</th><th>预览结果</th></tr></thead><tbody>
            <tr v-for="(line, index) in preview.items || []" :key="index"><td>{{ line.sheet || '人员' }}</td><td>{{ line.row }}</td><td>{{ line.name || '—' }}</td><td><span :class="['oa-pill', { 'oa-pill-warn': line.error || line.failure }]">{{ importDifference(line) }}</span></td></tr>
          </tbody></table>
        </div>
        <div class="oa-actions oa-person-tabs"><button v-if="importErrors.length" @click="downloadConflicts">下载冲突行</button><button class="oa-button-primary" :disabled="busy || !hasEligibleLines || !preview.token" @click="confirmImport">{{ preview.items?.some(row => row.failure) ? '重试失败行' : '确认回导' }}</button></div>
      </section>
    </template>
  </template>
  <dialog ref="editor" class="oa-dialog" @close="operation = undefined">
    <template v-if="operation && person">
      <h2>{{ kinds[operation.kind] }} · {{ person.name }}</h2>
      <p v-if="error" class="oa-error" role="alert">{{ error }}</p>
      <form class="oa-form" @submit.prevent="impactCheck ? saveChange() : impactCheck = true">
        <template v-if="!impactCheck">
          <label>生效日期<input v-model="operation.effective_date" type="date" required /></label>
          <div v-if="['shop_transfer', 'handover'].includes(operation.kind)" class="oa-form-row">
            <label v-if="operation.kind === 'shop_transfer'">移交原店<input v-model="operation.from_shop_id" placeholder="可填写原店铺 ID" /></label>
            <label>{{ operation.kind === 'shop_transfer' ? '接管新店' : '交接店铺' }}<input v-model="operation.shop_id" :required="operation.kind === 'handover' || !operation.from_shop_id" /></label>
            <label>接手人<select v-model.number="operation.target_id" :required="operation.kind === 'handover'"><option :value="0">选择接手人</option><option v-for="item in items.filter(item => item.id !== person?.id)" :key="item.id" :value="item.id">{{ item.name }}</option></select></label>
          </div>
        </template>
        <dl v-else class="oa-kv">
          <div><dt>人员 / 生效</dt><dd>{{ person.name }} · {{ operation.effective_date }}</dd></div>
          <div v-if="operation.shop_id"><dt>店铺与接手人</dt><dd>{{ operation.from_shop_id || '—' }} → {{ operation.shop_id || '—' }} · {{ personName(operation.target_id) }}</dd></div>
        </dl>
        <div class="oa-actions"><button type="button" @click="impactCheck ? impactCheck = false : editor?.close()">{{ impactCheck ? '返回修改' : '取消' }}</button><button :disabled="busy" class="oa-button-primary">{{ impactCheck ? '确认变更' : '预览变更' }}</button></div>
      </form>
    </template>
  </dialog>
</div>
</template>
