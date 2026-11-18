<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { RouterLink } from 'vue-router'
import PageHead from '../components/PageHead.vue'
import { api, dateInChina, download, errorText, opsURL, saveFile, send } from '../api'
import { can, currentProfile } from '../session'

type Person = {
  id: number; name: string; employee_no?: number | null; department_id: number; team_id: number
  department?: string; team?: string; management?: string; roles?: string[]; employment_type?: string
  joined?: string; left?: string; manager_id?: number; version: number; terms?: Term[]; events?: PersonEvent[]
  status?: string
}
type Term = { shop_id: string; shop_name?: string; from?: string; to?: string; start_date?: string; end_date?: string }
type PersonEvent = { kind: string; effective_date: string; target_id?: number; shop_id?: string; status?: string }
type Department = { id: number; name: string; team_items?: { id: number; name: string }[] }
type OnboardingAccount = { id: number; department_id: number; team_id: number }
type Handover = { items: { id: number; task_no: string; title: string; status: string }[]; shops: string[] }
type PreviewRow = { row: number; name?: string; error?: string; change?: string; status?: string; sheet?: string; employee_no?: number; previous_employee_no?: number; new_name?: string; previous_name?: string; employment_type?: string; shop_id?: string; from?: string }
type Preview = { token: string; items: PreviewRow[] }
const props = defineProps<{ mode: 'people' | 'roster' }>()
const allowed = computed(() => can('people.manage') || currentProfile.value?.super_admin)
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
const createDialog = ref<HTMLDialogElement>()
const organizations = ref<Department[]>([])
const onboardingAccount = ref<OnboardingAccount>()
const blankAccount = () => ({ username: '', employee_no: undefined as number | undefined, display_name: '', department_id: 0, team_id: 0, mobile: '', password: '', effective_date: dateInChina(), employment_type: 'full_time' })
const newAccount = ref(blankAccount())
const assignableTeams = computed(() => organizations.value.find(item => item.id === newAccount.value.department_id)?.team_items ?? [])
const canCreate = computed(() => Boolean(allowed.value && (currentProfile.value?.super_admin || can('access.manage'))))
const handover = ref<Handover>()
const handoverError = ref('')
const impactCheck = ref(false)
const operation = ref<ReturnType<typeof blank>>()
const kinds: Record<string, string> = { hire: '入职', transfer: '调岗', shop_transfer: '调店', leave: '离职', handover: '交接', rehire: '返聘', roles: '角色与管理权限' }
const mainRoles = ['定制设计师', '审核', '常规设计师', '运营']
const assetRoles = ['素材提交', '素材管理', '素材结算']
const roleLabels: Record<string, string> = { custom_designer: '定制设计师', auditor: '审核', designer: '常规设计师', operations: '运营', asset_submit: '素材提交', asset_manage: '素材管理', asset_settle: '素材结算' }
const managementLabels: Record<string, string> = { none: '无管理权限', department: '部门管理', super_admin: '超级管理员' }

function blank(p: Person, kind: string) {
  return {
    kind, version: p.version, effective_date: dateInChina(),
    department_id: p.department_id || 0, team_id: p.team_id || 0,
    roles: [...new Set((p.roles || []).map(role => roleLabels[role] || role))],
    management: p.management || 'none', employment_type: p.employment_type || 'full_time',
    shop_id: '', from_shop_id: '', target_id: 0, manager_id: p.manager_id || 0,
  }
}
const pendingHire = computed(() => person.value?.events?.find(event => event.kind === 'hire' && event.effective_date > dateInChina()))
const canRegister = computed(() => person.value?.status === 'disabled' && !person.value.joined && !person.value.left && !pendingHire.value)
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
function beginOnboarding() {
  void run(async () => {
    if (!organizations.value.length) organizations.value = (await api<{ departments: Department[] }>('/v1/org/options')).departments || []
    createDialog.value?.showModal()
  })
}
function registerEmployee() {
  void run(async () => {
    const person = newAccount.value
    if (!onboardingAccount.value) {
      const department = organizations.value.find(item => item.id === person.department_id)
      const team = assignableTeams.value.find(item => item.id === person.team_id)
      if (!department || !team) throw new Error('请选择部门及团队')
      onboardingAccount.value = await send<OnboardingAccount>('/v1/users', {
        username: person.username, display_name: person.display_name, employee_no: person.employee_no,
        department: department.name, team: team.name, department_id: department.id, team_id: team.id,
        mobile: person.mobile, password: person.password, status: 'disabled', employment_type: person.employment_type,
      })
      person.password = ''
    }
    const account = onboardingAccount.value
    await send(`/oa-api/people/${account.id}/change`, {
      kind: 'hire', effective_date: person.effective_date, version: 0,
      department_id: person.department_id, team_id: person.team_id, employment_type: person.employment_type,
    })
    onboardingAccount.value = undefined
    newAccount.value = blankAccount()
    createDialog.value?.close()
    query.value = ''
    await refresh()
    await readPerson(account.id)
    notice.value = '入职记录已保存。业务角色可在人员档案中单独分配。'
  })
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
    if (!preview.value?.token || importErrors.value.length) return
    const result = await send<{ count: number }>('/oa-api/people/confirm', { token: preview.value.token })
    preview.value = undefined
    notice.value = `已确认回导，共更新 ${result.count} 条记录`
  })
}
function importDifference(row: PreviewRow) {
  if (row.error) return row.error
  const changes = [
    row.employee_no != null ? `工号 ${employeeNo(row.previous_employee_no)} → ${employeeNo(row.employee_no)}` : '',
    row.new_name ? `姓名 ${row.previous_name || row.name} → ${row.new_name}` : '',
    row.employment_type ? `用工类型 → ${employment(row.employment_type)}` : '',
    row.shop_id && row.from ? `店铺 ${row.shop_id} · ${row.from} 生效` : '',
  ].filter(Boolean)
  return changes.join('；') || row.change || row.status || '无变化'
}
const importErrors = computed(() => preview.value?.items?.filter(row => row.error) ?? [])
function downloadConflicts() {
  const quote = (value: unknown) => {
    const source = String(value ?? '')
    const safe = /^[=+\-@\t\r]/.test(source) ? "'" + source : source
    return '"' + safe.replace(/"/g, '""') + '"'
  }
  const rows = [['工作表', '行号', '姓名', '冲突原因'], ...importErrors.value.map(row => [row.sheet || '人员', row.row, row.name, row.error])]
  saveFile(new Blob(['\uFEFF', rows.map(row => row.map(quote).join(',')).join('\r\n')], { type: 'text/csv;charset=utf-8' }), '花名册回导冲突.csv')
}
onMounted(() => { if (props.mode === 'people') void run(refresh) })
</script>

<template>
  <PageHead section="公司事务 / 组织与人员" :title="mode === 'people' ? '人员管理' : '组织任职与花名册'">
    <button v-if="mode === 'people' && canCreate" class="oa-button-primary" @click="beginOnboarding">新员工入职</button>
    <RouterLink v-if="mode === 'people' && allowed" class="oa-button" to="/roster">名册导入 / 导出</RouterLink>
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
            <div><dt>入职日期</dt><dd>{{ person.joined || (pendingHire ? pendingHire.effective_date + ' · 待生效' : '未登记') }}</dd></div>
            <div><dt>离职日期</dt><dd>{{ person.left || '—' }}</dd></div>
          </dl>
          <div class="oa-actions"><button v-if="canRegister" @click="open('hire')">登记入职</button><button v-if="person.status === 'active'" @click="open('transfer')">办理调岗</button><button v-if="person.status === 'active' && person.management !== 'super_admin' && String(currentProfile?.id) !== String(person.id)" @click="open('leave')">登记离职</button><button v-if="person.left && person.left <= dateInChina()" @click="open('rehire')">办理返聘</button></div>
        </template>
        <template v-else-if="tab === 'access'">
          <dl class="oa-kv"><div><dt>业务角色</dt><dd>{{ currentRoles }}</dd></div><div><dt>管理权限</dt><dd>{{ effectiveManagement }}</dd></div></dl>
          <p v-if="person.management === 'super_admin'" class="oa-muted">超级管理员权限由受保护的账号设置维护。</p>
          <button v-else-if="currentProfile?.super_admin" @click="open('roles')">调整角色和权限</button>
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
          <dl v-else-if="handover?.items?.length" class="oa-kv"><div v-for="task in handover.items" :key="task.id"><dt>{{ task.task_no }} · {{ task.title || task.status }}</dt><dd><a :href="opsURL + '/tasks/' + task.id" target="_blank" rel="noopener noreferrer">打开任务办理</a></dd></div></dl>
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
          <p class="oa-muted">使用导出表保留稳定人员 ID。缺行不会触发离职，空单元格不会清空原资料。</p>
          <button :disabled="busy" @click="run(() => download('/oa-api/people/export', '永箔花名册.xlsx'))">导出花名册</button>
        </section>
      </div>
      <section v-if="preview" class="oa-panel">
        <h2>回导差异预览</h2>
        <p class="oa-subtitle">{{ preview.items?.length || 0 }} 行 · {{ importErrors.length }} 行冲突</p>
        <div class="oa-table-wrap">
          <table class="oa-table"><thead><tr><th>工作表</th><th>行号</th><th>姓名</th><th>预览结果</th></tr></thead><tbody>
            <tr v-for="(line, index) in preview.items || []" :key="index"><td>{{ line.sheet || '人员' }}</td><td>{{ line.row }}</td><td>{{ line.name || '—' }}</td><td><span :class="['oa-pill', { 'oa-pill-warn': line.error }]">{{ importDifference(line) }}</span></td></tr>
          </tbody></table>
        </div>
        <div class="oa-actions oa-person-tabs"><button v-if="importErrors.length" @click="downloadConflicts">下载冲突行</button><button class="oa-button-primary" :disabled="busy || !!importErrors.length || !preview.token" @click="confirmImport">确认回导</button></div>
      </section>
    </template>
  </template>
  <dialog ref="createDialog" class="oa-dialog">
    <h2>新员工入职</h2>
    <p v-if="error" class="oa-error" role="alert">{{ error }}</p>
    <p v-if="onboardingAccount" class="oa-notice">账号已创建。继续登记其入职即可。</p>
    <form class="oa-form" @submit.prevent="registerEmployee">
      <fieldset v-if="!onboardingAccount" class="oa-form-row oa-fieldset">
        <label>登录账号<input v-model.trim="newAccount.username" autocomplete="off" required /></label>
        <label>姓名<input v-model.trim="newAccount.display_name" required /></label>
        <label>工号<input v-model.number="newAccount.employee_no" type="number" min="0" max="9999" required /></label>
        <label>手机号<input v-model.trim="newAccount.mobile" type="tel" required /></label>
        <label>初始密码<input v-model="newAccount.password" type="password" autocomplete="new-password" required /></label>
        <label>部门<select v-model.number="newAccount.department_id" required @change="newAccount.team_id = 0"><option :value="0" disabled>选择部门</option><option v-for="item in organizations" :key="item.id" :value="item.id">{{ item.name }}</option></select></label>
        <label>团队<select v-model.number="newAccount.team_id" required><option :value="0" disabled>选择团队</option><option v-for="item in assignableTeams" :key="item.id" :value="item.id">{{ item.name }}</option></select></label>
        <label>用工类型<select v-model="newAccount.employment_type"><option value="full_time">全职</option><option value="part_time">兼职</option></select></label>
      </fieldset>
      <label>入职生效日期<input v-model="newAccount.effective_date" type="date" :min="dateInChina()" required /></label>
      <div class="oa-actions"><button type="button" :disabled="busy" @click="createDialog?.close()">返回</button><button class="oa-button-primary" :disabled="busy">{{ onboardingAccount ? '继续登记入职' : '创建账号并登记入职' }}</button></div>
    </form>
  </dialog>
  <dialog ref="editor" class="oa-dialog" @close="operation = undefined">
    <template v-if="operation && person">
      <h2>{{ kinds[operation.kind] }} · {{ person.name }}</h2>
      <p v-if="error" class="oa-error" role="alert">{{ error }}</p>
      <form class="oa-form" @submit.prevent="impactCheck ? saveChange() : impactCheck = true">
        <template v-if="!impactCheck">
          <label>生效日期<input v-model="operation.effective_date" type="date" required /></label>
          <div v-if="['hire', 'transfer', 'rehire'].includes(operation.kind)" class="oa-form-row">
            <label>部门 ID<input v-model.number="operation.department_id" type="number" min="1" required /></label>
            <label>团队 ID<input v-model.number="operation.team_id" type="number" min="1" required /></label>
            <label>直属负责人<select v-model.number="operation.manager_id"><option :value="0">未设置</option><option v-for="item in items.filter(item => item.id !== person?.id)" :key="item.id" :value="item.id">{{ item.name }}</option></select></label>
            <label>用工类型<select v-model="operation.employment_type"><option value="full_time">全职</option><option value="part_time">兼职</option></select></label>
          </div>
          <div v-if="['shop_transfer', 'handover'].includes(operation.kind)" class="oa-form-row">
            <label v-if="operation.kind === 'shop_transfer'">移交原店<input v-model="operation.from_shop_id" placeholder="可填写原店铺 ID" /></label>
            <label>{{ operation.kind === 'shop_transfer' ? '接管新店' : '交接店铺' }}<input v-model="operation.shop_id" :required="operation.kind === 'handover' || !operation.from_shop_id" /></label>
            <label>接手人<select v-model.number="operation.target_id" :required="operation.kind === 'handover'"><option :value="0">选择接手人</option><option v-for="item in items.filter(item => item.id !== person?.id)" :key="item.id" :value="item.id">{{ item.name }}</option></select></label>
          </div>
          <template v-if="operation.kind === 'roles'">
            <div v-for="(roles, index) in [mainRoles, assetRoles]" :key="index">
              <h3>{{ index ? '素材工作台业务角色' : '运营平台业务角色' }}</h3>
              <div class="oa-checks"><label v-for="role in roles" :key="role"><input v-model="operation.roles" type="checkbox" :value="role" />{{ role }}</label></div>
            </div>
            <label>管理权限<select v-model="operation.management"><option value="none">无管理权限</option><option value="department">部门管理</option><option value="super_admin" disabled>超级管理员（受保护）</option></select></label>
          </template>
        </template>
        <dl v-else class="oa-kv">
          <div><dt>人员 / 生效</dt><dd>{{ person.name }} · {{ operation.effective_date }}</dd></div>
          <div v-if="operation.kind === 'roles'"><dt>业务角色</dt><dd>{{ currentRoles }} → {{ operation.roles.join('、') || '无' }}</dd></div>
          <div v-if="operation.kind === 'roles'"><dt>管理权限</dt><dd>{{ effectiveManagement }} → {{ managementLabels[operation.management] }}</dd></div>
          <div v-if="operation.shop_id"><dt>店铺与接手人</dt><dd>{{ operation.from_shop_id || '—' }} → {{ operation.shop_id || '—' }} · {{ personName(operation.target_id) }}</dd></div>
          <div v-if="['hire', 'transfer', 'rehire'].includes(operation.kind)"><dt>部门 / 团队</dt><dd>#{{ operation.department_id }} / #{{ operation.team_id }}</dd></div>
        </dl>
        <div class="oa-actions"><button type="button" @click="impactCheck ? impactCheck = false : editor?.close()">{{ impactCheck ? '返回修改' : '取消' }}</button><button :disabled="busy" class="oa-button-primary">{{ impactCheck ? '确认变更' : '预览变更' }}</button></div>
      </form>
    </template>
  </dialog>
</template>
