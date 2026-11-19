<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import BaseSelect from '@/components/base/BaseSelect.vue'
import ManagerPicker from './ManagerPicker.vue'
import PersonnelFinanceHandover from './PersonnelFinanceHandover.vue'
import { useRoute, useRouter } from 'vue-router'
import { taskDetailLocation } from '@/oa/taskNavigation'
import { api, dateInChina, errorText, send } from '../api'
import '../styles.css'

type Term = { shop_id: string; shop_name?: string; from?: string; to?: string; start_date?: string; end_date?: string }
type Entry = { kind: string; effective_date: string; target_id?: number; shop_id?: string; from_shop_id?: string; shop_ids?: string[]; from_shop_ids?: string[]; note?: string; org_from?: string; org_to?: string; actor_id?: number; recorded_at?: string; id?: number; job_title?: string; job_level?: string; previous_job_title?: string; previous_job_level?: string }
type Task = { id: number; task_no: string; title: string; status: string }
type Person = { id: number; name: string; version: number; status: string; hr_status?: string; joined?: string; confirmed?: string; left?: string; manager_name?: string; terms?: Term[]; events?: Entry[]; employee_no?: number | null; department?: string; team?: string; roles?: string[] }
type Shop = { shop_id: string; shop_name: string; platform: string; bound: boolean; handover: boolean }
type ShopResults = { bound: Shop[]; items: Shop[] }
type Kind = 'shop_transfer' | 'handover' | 'confirm' | 'leave_plan' | 'leave' | 'rehire' | 'hire' | 'salary_change'

const route = useRoute()
const router = useRouter()
const props = defineProps<{ userId: string | number; showProfile?: boolean; mode?: 'shops' | 'records' }>()
const personnelTaskSource = computed(() => router.resolve({
  path: '/users',
  query: { ...route.query, user_id: String(props.userId), tab: 'records' },
}).fullPath)
const emit = defineEmits<{ loaded: [person: Person] }>()
const person = ref<Person>()
const handover = ref<{ items: Task[]; shops: string[] }>()
const loading = ref(true)
const saving = ref(false)
const error = ref('')
const taskError = ref('')
const notice = ref('')
const shops = ref<ShopResults>({ bound: [], items: [] })
const shopLabels = ref<Record<string, string>>({})
const shopQuery = ref('')
const shopError = ref('')
const shopBusy = ref(false)
let shopSequence = 0
const financeReady = ref(false)
const preview = ref(false)
const operation = ref<{ kind: Kind; version: number; effective_date: string; shop_ids: string[]; from_shop_ids: string[]; target_id: number; note: string }>()
const labels: Record<string, string> = {
  hire: '入职', hire_profile: '入职日期登记', confirm: '转正', transfer: '组织调动', shop_transfer: '调店',
  leave_plan: '待离职', leave: '离职', handover: '交接店铺', shop_received: '接收店铺', rehire: '返聘',
  roles: '角色与管理权限', job_change: '岗位调整', level_change: '职级变动', salary_change: '调薪', manager_change: '直属负责人变更', account_disabled: '停用账号', account_disable_requested: '申请停用账号', account_disabled_observed: '确认账号已停用', transfer_requested: '提交组织调整', transfer_unchanged: '组织归属未变化', transfer_observed: '核对当前组织归属',
}
const isShopOperation = computed(() => operation.value?.kind === 'shop_transfer' || operation.value?.kind === 'handover')
const boundShops = computed(() => shops.value.bound.filter(item => item.bound || operation.value?.kind === 'handover'))
const shopOptions = computed(() => shops.value.items
  .filter(item => !operation.value?.shop_ids.includes(item.shop_id))
  .map(item => ({ value: item.shop_id, label: shopLabels.value[item.shop_id] })))
function addTargetShop(shop: unknown) {
  if (typeof shop !== 'string' || !shop || !operation.value || operation.value.shop_ids.includes(shop)) return
  operation.value.shop_ids.push(shop)
}
const eventTimeline = computed(() => [...(person.value?.events || [])].sort((a, b) => b.effective_date.localeCompare(a.effective_date) || (b.id || 0) - (a.id || 0)))

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
async function loadShops() {
  const request = ++shopSequence
  shopBusy.value = true
  shopError.value = ''
  try {
    const query = new URLSearchParams({ q: shopQuery.value.trim(), effective_date: operation.value?.effective_date || dateInChina() })
    const result = await api<ShopResults>(`/oa-api/people/${props.userId}/term-shops?${query}`)
    if (request !== shopSequence) return
    shops.value = result
    for (const item of [...result.bound, ...result.items]) {
      shopLabels.value[item.shop_id] = [item.shop_name, item.platform, item.shop_id].filter(Boolean).join(' · ')
    }
  } catch (cause) { if (request === shopSequence) shopError.value = errorText(cause) }
  finally { if (request === shopSequence) shopBusy.value = false }
}
watch(() => operation.value?.effective_date, () => { if (isShopOperation.value) void loadShops() })
function toggleBound(shop: string) {
  const selected = operation.value
  if (!selected) return
  const list = selected.kind === 'handover' ? selected.shop_ids : selected.from_shop_ids
  const i = list.indexOf(shop)
  if (i === -1) list.push(shop)
  else list.splice(i, 1)
}
function recordDetails(event: Entry) {
  return [
    (event.from_shop_ids || (event.from_shop_id ? [event.from_shop_id] : [])).map(shop => '移出 ' + (shopLabels.value[shop] || shop)).join('、'),
    (event.shop_ids || (event.shop_id ? [event.shop_id] : [])).map(shop => shopLabels.value[shop] || shop).join('、'),
    event.kind === 'transfer_observed' ? event.org_to : event.org_from || event.org_to ? `${event.org_from || '未分配'} → ${event.org_to || '未分配'}` : '',
    event.target_id ? '关联人员 #' + event.target_id : '',
    event.note || '',
    event.previous_job_title || event.job_title ? `岗位 ${event.previous_job_title || '未填'} → ${event.job_title || '未填'}` : '',
    event.previous_job_level || event.job_level ? `职级 ${event.previous_job_level || '未填'} → ${event.job_level || '未填'}` : '',
  ].filter(Boolean).join(' · ') || '—'
}
function begin(kind: Kind) {
  if (!person.value) return
  operation.value = { kind, version: person.value.version, effective_date: dateInChina(), shop_ids: [], from_shop_ids: [], target_id: 0, note: '' }
  preview.value = false
  shopQuery.value = ''
  error.value = ''
  notice.value = ''
}
async function submit() {
  const next = operation.value
  if (!next || saving.value) return
  if (!preview.value) {
    if (next.kind === 'handover' && (!next.target_id || !next.shop_ids.length)) { error.value = '请勾选交接店铺并选择接手人'; return }
    if (next.kind === 'shop_transfer' && !next.shop_ids.length && !next.from_shop_ids.length) { error.value = '请选择移交原店或接管新店'; return }
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
    if (props.mode !== 'records') await loadShops()
  } catch (cause) { error.value = errorText(cause) }
  finally { saving.value = false }
}
onMounted(async () => { await loadDetail(); if (props.mode !== 'records' && person.value) await loadShops() })
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
      <section v-if="mode !== 'records'" class="term-section">
        <div class="term-heading">
          <h4>店铺任职</h4>
          <div v-if="!operation" class="oa-actions">
            <button v-if="person.status === 'active' && shops.bound.some(item => item.bound)" type="button" @click="begin('shop_transfer')">办理调店</button>
            <button v-else-if="person.status === 'active'" type="button" @click="begin('shop_transfer')">接管店铺</button>
            <button v-if="shops.bound.length" type="button" @click="begin('handover')">办理交接</button>
          </div>
        </div>
        <p v-if="shopError && !operation" class="oa-error" role="alert">{{ shopError }} <button type="button" @click="loadShops">重新读取店铺</button></p>
        <ul v-if="shops.bound.length && !operation" class="personnel-shops">
          <li v-for="item in shops.bound" :key="item.shop_id">{{ shopLabels[item.shop_id] }} <span v-if="item.handover" class="oa-pill oa-pill-warn">待交接</span></li>
        </ul>
        <p v-else-if="!operation && !shopBusy && !shopError" class="oa-muted">暂无绑定店铺</p>
      </section>
      <section v-if="mode !== 'shops'" class="term-section">
        <PersonnelFinanceHandover :user-id="userId" @ready="financeReady = $event" />
        <div class="oa-actions">
          <button v-if="person.status === 'active' && !person.joined && !person.left" type="button" :disabled="!!operation" @click="begin('hire')">登记入职</button>
          <button v-if="person.status === 'active' && !person.confirmed && !person.left" type="button" :disabled="!!operation" @click="begin('confirm')">登记转正</button>
          <button v-if="person.status === 'active'" type="button" :disabled="!!operation" @click="begin('salary_change')">登记调薪</button>
          <button v-if="person.status === 'active' && !person.left" type="button" :disabled="!!operation || !financeReady" @click="begin('leave_plan')">登记待离职</button>
          <button v-if="person.status === 'disabled' && !person.left" type="button" :disabled="!!operation || !financeReady" @click="begin('leave')">登记离职</button>
          <button v-if="person.status === 'active' && person.left" type="button" :disabled="!!operation" @click="begin('rehire')">登记返聘</button>
        </div>
      </section>
      <form v-if="operation" class="oa-form term-editor" @submit.prevent="submit">
        <h5>{{ labels[operation.kind] }}</h5>
        <template v-if="!preview">
          <label>生效日期<input v-model="operation.effective_date" type="date" required :min="isShopOperation || operation.kind === 'leave_plan' ? dateInChina() : '1000-01-01'" :max="['hire', 'leave', 'rehire'].includes(operation.kind) ? dateInChina() : undefined" /></label>
          <label v-if="operation.kind === 'salary_change'">备注<textarea v-model.trim="operation.note" maxlength="300" rows="3" placeholder="可填写变更原因或薪酬审批单号" /></label>
          <p v-if="operation.kind === 'leave_plan'" class="oa-muted">到生效日，在账号状态中核对交接并停用账号，再登记离职。</p>
          <template v-if="isShopOperation">
            <p v-if="shopError" class="oa-error" role="alert">{{ shopError }}</p>
            <fieldset v-if="boundShops.length" class="personnel-shop-picker">
              <legend>{{ operation.kind === 'shop_transfer' ? '移交原店' : '交接店铺' }}</legend>
              <label v-for="item in boundShops" :key="item.shop_id">
                <input type="checkbox" :checked="(operation.kind === 'handover' ? operation.shop_ids : operation.from_shop_ids).includes(item.shop_id)" @change="toggleBound(item.shop_id)" />
                {{ shopLabels[item.shop_id] }} <span v-if="item.handover" class="oa-pill oa-pill-warn">待交接</span>
              </label>
            </fieldset>
            <div v-if="operation.kind === 'shop_transfer'" class="oa-form">
              <div class="oa-actions oa-filter">
                <input v-model="shopQuery" type="search" aria-label="搜索目标店铺" @keydown.enter.prevent="loadShops" placeholder="店铺名称、平台或店铺 ID" />
                <button type="button" :disabled="shopBusy || !shopQuery.trim()" @click="loadShops">搜索店铺</button>
              </div>
              <BaseSelect :model-value="''" :options="shopOptions" label="接管新店" placeholder="请选择并添加目标店铺" :disabled="!shopOptions.length" @update:model-value="addTargetShop" />
              <ul v-if="operation.shop_ids.length" class="personnel-shops">
                <li v-for="shop in operation.shop_ids" :key="shop">{{ shopLabels[shop] }} <button type="button" @click="operation.shop_ids = operation.shop_ids.filter(id => id !== shop)">移除</button></li>
              </ul>
            </div>
            <ManagerPicker v-model="operation.target_id" label="接手人" empty-label="请选择接手人" :exclude="Number(userId)" :disabled="saving" />
          </template>
        </template>
        <dl v-else class="oa-kv">
          <div><dt>生效日期</dt><dd>{{ operation.effective_date }}</dd></div>
          <div v-if="operation.from_shop_ids.length"><dt>移交原店</dt><dd>{{ operation.from_shop_ids.map(shop => shopLabels[shop]).join('、') }}</dd></div>
          <div v-if="operation.shop_ids.length"><dt>{{ operation.kind === 'handover' ? '交接店铺' : '接管新店' }}</dt><dd>{{ operation.shop_ids.map(shop => shopLabels[shop]).join('、') }}</dd></div>
          <div v-if="operation.note"><dt>备注</dt><dd>{{ operation.note }}</dd></div>
          <div v-if="operation.target_id"><dt>接手人员</dt><dd>用户 #{{ operation.target_id }}</dd></div>
        </dl>
        <div class="oa-actions">
          <button type="button" :disabled="saving" @click="preview ? preview = false : operation = undefined">{{ preview ? '返回修改' : '取消' }}</button>
          <button :disabled="saving || (isShopOperation && shopBusy)" class="oa-button-primary">{{ preview ? '确认变更' : '预览变更' }}</button>
        </div>
      </form>
      <section v-if="mode !== 'records' && person.terms?.length" class="term-section">
        <h4>店铺任职记录</h4>
        <dl class="oa-kv"><div v-for="(term, index) in person.terms" :key="index">
          <dt>{{ shopLabels[term.shop_id] || term.shop_name || term.shop_id }}</dt>
          <dd>{{ term.from || term.start_date || '—' }} {{ term.to || term.end_date ? '至 ' + (term.to || term.end_date) : '至今' }}</dd>
        </div></dl>
      </section>
      <section v-if="mode !== 'shops'" class="term-section">
        <h4>待交接任务</h4>
        <p v-if="taskError" class="oa-error" role="alert">{{ taskError }}</p>
        <dl v-else-if="handover?.items?.length" class="oa-kv"><div v-for="task in handover.items" :key="task.id"><dt>{{ task.task_no }} · {{ task.title || task.status }}</dt><dd><RouterLink :to="taskDetailLocation(task.id, personnelTaskSource)">打开任务办理</RouterLink></dd></div></dl>
        <p v-else-if="handover" class="oa-muted">暂无待交接任务</p>
        <p v-if="handover?.shops?.length" class="oa-muted">待交接店铺 {{ handover.shops.join('、') }}</p>
      </section>
      <section v-if="mode !== 'shops'" class="term-section">
        <h4>异动时间轴</h4>
        <div v-if="eventTimeline.length" class="oa-table-wrap"><table class="oa-table">
          <thead><tr><th>事项</th><th class="personnel-numeric" scope="col">生效日期</th><th>变更内容</th><th>登记人</th><th>登记时间</th></tr></thead>
          <tbody><tr v-for="(event, index) in eventTimeline" :key="event.id ?? 'legacy-' + index"><td>{{ labels[event.kind] || event.kind }}</td><td class="personnel-numeric">{{ event.effective_date }}</td><td>{{ recordDetails(event) }}</td><td>{{ event.actor_id ? '#' + event.actor_id : '—' }}</td><td>{{ event.recorded_at || '—' }}</td></tr></tbody>
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
.personnel-shop-picker, .personnel-shops { display: grid; gap: .45rem; }
.personnel-shop-picker label { display: flex; flex-direction: row; align-items: center; gap: .5rem; padding: .45rem; border: 1px solid rgb(var(--yb-border)); border-radius: 6px; }
</style>
