<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { api, dateInChina, errorText, send } from '../api'
import '../styles.css'

type Term = { shop_id: string; shop_name?: string; from?: string; to?: string; start_date?: string; end_date?: string }
type Entry = { kind: string; effective_date: string; target_id?: number; shop_id?: string; status?: string }
type Task = { id: number; task_no: string; title: string; status: string }
type Person = { id: number; name: string; version: number; left?: string; terms?: Term[]; events?: Entry[]; employee_no?: number | null; department?: string; team?: string; roles?: string[] }
type Candidate = { id: number; name: string; employee_no?: number | null }
type Kind = 'shop_transfer' | 'handover'

const props = defineProps<{ userId: string | number; showProfile?: boolean }>()
const emit = defineEmits<{ loaded: [person: Person] }>()
const person = ref<Person>()
const handover = ref<{ items: Task[]; shops: string[] }>()
const loading = ref(true)
const saving = ref(false)
const error = ref('')
const taskError = ref('')
const notice = ref('')
const candidates = ref<Candidate[]>([])
const candidateQuery = ref('')
const candidateError = ref('')
const candidateLoading = ref(false)
const preview = ref(false)
const operation = ref<{ kind: Kind; version: number; effective_date: string; shop_id: string; from_shop_id: string; target_id: number }>()
const labels: Record<string, string> = { hire: '入职', transfer: '调岗', shop_transfer: '调店', leave: '离职', handover: '交接', rehire: '返聘', roles: '角色与管理权限' }
const candidateLabels = ref<Record<number, string>>({})
const recipient = computed(() => operation.value?.target_id ? candidateLabels.value[operation.value.target_id] : undefined)

async function loadDetail() {
  loading.value = true
  error.value = ''
  taskError.value = ''
  try {
    person.value = await api<Person>(`/oa-api/people/${props.userId}`)
    emit('loaded', person.value)
    try { handover.value = await api(`/oa-api/people/${props.userId}/handover`) }
    catch (cause) { taskError.value = errorText(cause) }
  } catch (cause) {
    person.value = undefined
    error.value = errorText(cause)
  } finally { loading.value = false }
}
async function loadCandidates() {
  if (candidateLoading.value) return
  candidateLoading.value = true
  candidateError.value = ''
  try {
    const data = await api<{ items: Candidate[] }>('/oa-api/people/list?' + new URLSearchParams({ q: candidateQuery.value.trim() }))
    candidates.value = (data.items || []).filter(item => item.id !== person.value?.id)
    for (const item of candidates.value) {
      candidateLabels.value[item.id] = item.employee_no == null ? item.name : `${item.name} · ${String(item.employee_no).padStart(4, '0')}`
    }
  } catch (cause) { candidateError.value = errorText(cause) }
  finally { candidateLoading.value = false }
}
function begin(kind: Kind) {
  if (!person.value) return
  operation.value = { kind, version: person.value.version, effective_date: dateInChina(), shop_id: '', from_shop_id: '', target_id: 0 }
  preview.value = false
  error.value = ''
  notice.value = ''
}
async function submit() {
  const next = operation.value
  if (!next || saving.value) return
  if (!preview.value) {
    if (next.kind === 'handover' && !next.target_id) { error.value = '请选择接手人'; return }
    error.value = ''
    preview.value = true
    return
  }
  saving.value = true
  error.value = ''
  try {
    await send(`/oa-api/people/${props.userId}/change`, next)
    operation.value = undefined
    notice.value = '变更已保存'
    await loadDetail()
  } catch (cause) { error.value = errorText(cause) }
  finally { saving.value = false }
}
onMounted(loadDetail)
</script>

<template>
  <section class="oa-page personnel-terms" aria-label="任职与交接">
    <p v-if="error" class="oa-error" role="alert">{{ error }} <button v-if="!person" type="button" @click="loadDetail">重试</button></p>
    <p v-if="notice" class="oa-notice" role="status">{{ notice }}</p>
    <p v-if="loading" class="oa-muted" role="status">正在读取任职与交接…</p>
    <template v-if="person">
      <section v-if="showProfile" class="term-section">
        <dl class="oa-kv">
          <div><dt>工号</dt><dd>{{ person.employee_no == null ? '—' : String(person.employee_no).padStart(4, '0') }}</dd></div>
          <div><dt>组织归属</dt><dd>{{ [person.department, person.team].filter(Boolean).join(' / ') || '未分配' }}</dd></div>
          <div v-if="person.roles?.length"><dt>业务角色</dt><dd>{{ person.roles.join('、') }}</dd></div>
        </dl>
      </section>
      <section class="term-section">
        <div class="term-heading">
          <h4>店铺任职</h4>
          <div v-if="!operation" class="oa-actions">
            <button v-if="!person.left" type="button" @click="begin('shop_transfer')">办理调店</button>
            <button type="button" @click="begin('handover')">办理交接</button>
          </div>
        </div>
        <form v-if="operation" class="oa-form term-editor" @submit.prevent="submit">
          <h5>{{ labels[operation.kind] }}</h5>
          <template v-if="!preview">
            <div class="oa-form-row">
              <label><span>生效日期 <span class="personnel-required" aria-hidden="true">*</span></span><input v-model="operation.effective_date" type="date" required /></label>
              <label v-if="operation.kind === 'shop_transfer'">移交原店<input v-model.trim="operation.from_shop_id" placeholder="店铺 ID" /></label>
              <label><span>{{ operation.kind === 'shop_transfer' ? '接管新店' : '交接店铺' }} <span v-if="operation.kind === 'handover' || !operation.from_shop_id" class="personnel-required" aria-hidden="true">*</span></span><input v-model.trim="operation.shop_id" :required="operation.kind === 'handover' || !operation.from_shop_id" placeholder="店铺 ID" /></label>
            </div>
            <div class="oa-actions oa-filter">
              <label>搜索接手人<input v-model="candidateQuery" placeholder="姓名或工号" @keydown.enter.prevent="loadCandidates" /></label>
              <button type="button" :disabled="candidateLoading" @click="loadCandidates">查询接手人</button>
            </div>
            <p v-if="candidateError" class="oa-error" role="alert">{{ candidateError }}</p>
            <label><span>接手人 <span v-if="operation.kind === 'handover'" class="personnel-required" aria-hidden="true">*</span></span><select v-model="operation.target_id" aria-label="接手人" :required="operation.kind === 'handover'">
              <option :value="0" :disabled="operation.kind === 'handover'">选择接手人</option>
              <option v-if="recipient && !candidates.some(item => item.id === operation?.target_id)" :value="operation.target_id">{{ recipient }}</option>
              <option v-for="item in candidates" :key="item.id" :value="item.id">{{ candidateLabels[item.id] }}</option>
            </select></label>
          </template>
          <dl v-else class="oa-kv">
            <div><dt>生效日期</dt><dd>{{ operation.effective_date }}</dd></div>
            <div v-if="operation.from_shop_id"><dt>移交原店</dt><dd>{{ operation.from_shop_id }}</dd></div>
            <div v-if="operation.shop_id"><dt>{{ operation.kind === 'handover' ? '交接店铺' : '接管新店' }}</dt><dd>{{ operation.shop_id }}</dd></div>
            <div v-if="recipient"><dt>接手人</dt><dd>{{ recipient }}</dd></div>
          </dl>
          <div class="oa-actions">
            <button type="button" :disabled="saving" @click="preview ? preview = false : operation = undefined">{{ preview ? '返回修改' : '取消' }}</button>
            <button :disabled="saving || candidateLoading" class="oa-button-primary">{{ preview ? '确认变更' : '预览变更' }}</button>
          </div>
        </form>
        <dl v-if="person.terms?.length" class="oa-kv"><div v-for="(term, index) in person.terms" :key="index">
          <dt>{{ term.shop_name || term.shop_id }}</dt>
          <dd>{{ term.from || term.start_date || '—' }} {{ term.to || term.end_date ? '至 ' + (term.to || term.end_date) : '至今' }}</dd>
        </div></dl>
        <p v-else class="oa-muted">暂无店铺任职记录</p>
      </section>
      <section class="term-section">
        <h4>待交接任务</h4>
        <p v-if="taskError" class="oa-error" role="alert">{{ taskError }}</p>
        <dl v-else-if="handover?.items?.length" class="oa-kv"><div v-for="task in handover.items" :key="task.id"><dt>{{ task.task_no }} · {{ task.title || task.status }}</dt><dd><RouterLink :to="'/tasks/' + task.id">打开任务办理</RouterLink></dd></div></dl>
        <p v-else-if="handover" class="oa-muted">暂无待交接任务</p>
        <p v-if="handover?.shops?.length" class="oa-muted">待交接店铺 {{ handover.shops.join('、') }}</p>
      </section>
      <section class="term-section">
        <h4>任职变更记录</h4>
        <div v-if="person.events?.length" class="oa-table-wrap"><table class="oa-table">
          <thead><tr><th>事项</th><th class="personnel-numeric" scope="col">生效日期</th><th>店铺</th><th>接手人</th><th>状态</th></tr></thead>
          <tbody><tr v-for="(event, index) in person.events" :key="index"><td>{{ labels[event.kind] || event.kind }}</td><td class="personnel-numeric">{{ event.effective_date }}</td><td>{{ event.shop_id || '—' }}</td><td>{{ event.target_id ? candidateLabels[event.target_id] || '#' + event.target_id : '—' }}</td><td>{{ event.status || '已登记' }}</td></tr></tbody>
        </table></div>
        <p v-else class="oa-muted">暂无任职变更</p>
      </section>
    </template>
  </section>
</template>

<style scoped>
.personnel-numeric { text-align: right; font-variant-numeric: tabular-nums; }
.personnel-required { color: var(--oa-danger, rgb(var(--yb-danger-text))); }
.personnel-terms .oa-kv a { display: inline-flex; min-height: 32px; min-width: 32px; align-items: center; }
.term-section + .term-section { margin-top: 14px; padding-top: 14px; border-top: 1px solid rgb(var(--yb-border)); }
.term-section > h4, .term-heading { margin: 0 0 8px; font-weight: 700; }
.term-heading { display: flex; flex-wrap: wrap; align-items: center; justify-content: space-between; gap: 8px; }
.term-editor { border: 1px solid rgb(var(--yb-border)); border-radius: 8px; padding: 12px; margin-bottom: 12px; }
.term-editor h5 { font-weight: 700; }
</style>
