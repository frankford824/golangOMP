<template>
  <main class="task-page task-detail-view">
    <div v-if="loading && !task" class="state">正在加载任务…</div>
    <div v-else-if="error && !task" class="state error">{{ error }}</div>
    <template v-else-if="task">
      <header class="task-head">
        <div>
          <p class="eyebrow">{{ taskTypeLabel }}</p>
          <h1>{{ task.task_no }}</h1>
          <p>{{ task.product_name_snapshot || task.sku_code || '未填写产品名称' }}</p>
        </div>
        <div class="head-actions">
          <TaskStatusTag :status="task.task_status" />
          <button @click="router.push('/tasks')">返回任务中心</button>
          <button :disabled="loading" @click="load">刷新</button>
        </div>
      </header>

      <section class="progress-card">
        <WorkflowProgress variant="horizontal" :task="task" />
      </section>

      <section class="facts">
        <div><span>主 SKU</span><strong>{{ task.primary_sku_code || task.sku_code || '—' }}</strong></div>
        <div><span>负责人</span><strong>{{ task.current_handler_name || task.designer_name || '未指派' }}</strong></div>
        <div><span>归属部门</span><strong>{{ task.owner_department || '—' }}</strong></div>
        <div><span>归属团队</span><strong>{{ task.owner_org_team || '—' }}</strong></div>
        <div><span>当前阶段</span><strong>{{ task.task_status }}</strong></div>
        <div><span>业务类型</span><strong>{{ task.business_lane === 'customization' ? '定制' : '常规' }}</strong></div>
      </section>

      <div v-if="error" class="message error" role="alert">{{ error }}</div>

      <section class="business-context" aria-label="任务业务信息">
        <article>
          <div class="section-title"><div><p class="eyebrow">任务需求</p><h2>{{ requirementHeading }}</h2></div></div>
          <p class="long-copy">{{ requirementText }}</p>
        </article>
        <article>
          <div class="section-title"><div><p class="eyebrow">协作说明</p><h2>运营备注</h2></div></div>
          <p class="long-copy">{{ operationNote }}</p>
        </article>
        <article class="reference-context">
          <div class="section-title"><div><p class="eyebrow">参考资料</p><h2>参考图与附件</h2></div><button @click="router.push(`/tasks/${task.id}/assets`)">管理附件</button></div>
          <div v-if="referenceFiles.length" class="reference-list">
            <a v-for="file in referenceFiles" :key="file.id || file.file_name" :href="file.download_url || file.preview_url || file.url" target="_blank" rel="noreferrer">{{ file.file_name || '参考附件' }}</a>
          </div>
          <p v-else class="long-copy">暂无参考附件。</p>
        </article>
      </section>

      <section class="collaboration-card" aria-label="审核协作">
        <div class="section-title"><div><p class="eyebrow">审核协作</p><h2>交班与接手</h2><p>审核人员可把当前任务交给另一位审核人员，接手后继续处理。</p></div><button v-if="canHandover" @click="showHandover = !showHandover">{{ showHandover ? '取消交班' : '发起交班' }}</button></div>
        <form v-if="showHandover" class="handover-form" @submit.prevent="submitHandover">
          <label>接手人员 ID<input v-model.number="handoverUserId" type="number" min="1" required /></label>
          <label>交班原因<input v-model.trim="handoverReason" maxlength="1000" required /></label>
          <button :disabled="handoverBusy || !handoverUserId || !handoverReason">{{ handoverBusy ? '提交中…' : '确认交班' }}</button>
        </form>
        <div v-if="handovers.length" class="handover-list"><div v-for="item in handovers" :key="item.id"><strong>{{ item.handover_no || `交班 ${item.id}` }}</strong><span>{{ item.status || '待接手' }}</span><button v-if="item.allowed_actions?.includes('task.audit.takeover')" :disabled="handoverBusy" @click="takeover(item.id)">接手</button></div></div>
        <p v-else class="long-copy">暂无交班记录。</p>
      </section>

      <template v-if="isPlanning">
        <section class="planning-card">
          <div><p class="eyebrow">生成结果</p><h2>策划 SKU 已生成</h2><p>产品图片用于记录策划信息，不进入设计成品或客户素材。</p></div>
          <a :href="`/v1/tasks/${task.id}/planning-skus/export.xlsx`">导出 Excel</a>
        </section>
      </template>
      <template v-else-if="bundle">
        <ResourceWorkflowPanel
          :task-id="task.id"
          :task-type="task.task_type"
          :bundle="bundle"
          :allowed-actions="task.allowed_actions || []"
          @updated="onWorkflowUpdated"
        />
        <section class="resource-card">
          <div class="resource-head"><div><p class="eyebrow">成品资料</p><h2>任务资源</h2></div><button @click="router.push(`/tasks/${task.id}/assets`)">独立查看</button></div>
          <SkuResourceMatrix :bundle="bundle" />
        </section>
      </template>

      <section class="timeline-card" aria-label="任务动态">
        <div class="section-title"><div><p class="eyebrow">处理记录</p><h2>任务动态</h2></div></div>
        <ol v-if="events.length"><li v-for="item in events" :key="item.id || `${item.event_type}-${item.created_at}`"><strong>{{ item.title || item.event_type || '任务更新' }}</strong><span>{{ item.operator_name || item.actor_name || '' }} {{ item.created_at || '' }}</span><p v-if="item.reason || item.remark">{{ item.reason || item.remark }}</p></li></ol>
        <p v-else class="long-copy">暂无任务动态。</p>
      </section>
    </template>
  </main>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { tasksApi } from '@/services/api/tasksApi'
import { resourceGroupsApi, type ResourceBundle } from '@/services/api/resourceGroupsApi'
import WorkflowProgress from '@/components/task/WorkflowProgress.vue'
import TaskStatusTag from '@/components/task/TaskStatusTag.vue'
import SkuResourceMatrix from '@/components/task/SkuResourceMatrix.vue'
import ResourceWorkflowPanel from '@/components/task/ResourceWorkflowPanel.vue'

interface V8Task {
  id: number
  task_no: string
  task_type: string
  task_status: string
  sku_code?: string
  primary_sku_code?: string
  product_name_snapshot?: string
  current_handler_name?: string
  designer_name?: string
  owner_department?: string
  owner_org_team?: string
  workflow_revision: number
  workflow_contract_version: 2
  business_lane?: string
  allowed_actions: string[]
  description?: string
  requirement_description?: string
  note?: string
  remark?: string
  operation_note?: string
  reference_file_refs?: ReferenceFile[]
}

interface ReferenceFile { id?: number; file_name?: string; download_url?: string; preview_url?: string; url?: string }
interface TaskEvent { id?: number; event_type?: string; title?: string; operator_name?: string; actor_name?: string; created_at?: string; reason?: string; remark?: string }
interface AuditHandover { id: number; handover_no?: string; status?: string; allowed_actions?: string[] }

const route = useRoute()
const router = useRouter()
const task = ref<V8Task | null>(null)
const bundle = ref<ResourceBundle | null>(null)
const loading = ref(false)
const error = ref('')
const events = ref<TaskEvent[]>([])
const handovers = ref<AuditHandover[]>([])
const showHandover = ref(false)
const handoverUserId = ref<number | null>(null)
const handoverReason = ref('')
const handoverBusy = ref(false)
const taskId = computed(() => Number(route.params.id))
const isPlanning = computed(() => task.value?.task_type === 'sku_planning')
const taskTypeLabel = computed(() => ({ original_product_development: '原品开发', new_product_development: '新品开发', retouch_task: '修图任务', sku_planning: '策划 SKU' }[task.value?.task_type || ''] || task.value?.task_type || '任务'))
const requirementHeading = computed(() => isPlanning.value ? '策划说明' : isRetouch.value ? '修图要求' : task.value?.business_lane === 'customization' ? '定制需求' : '需求说明')
const isRetouch = computed(() => ['retouch', 'retouch_task'].includes(task.value?.task_type || ''))
const requirementText = computed(() => task.value?.requirement_description || task.value?.description || '未填写需求说明。')
const operationNote = computed(() => task.value?.operation_note || task.value?.note || task.value?.remark || '未填写运营备注。')
const referenceFiles = computed(() => task.value?.reference_file_refs || [])
const actionSet = computed(() => new Set(task.value?.allowed_actions || []))
const canHandover = computed(() => actionSet.value.has('task.audit.handover'))

function unwrap<T>(response: { data?: unknown }): T {
  const body = response.data as Record<string, unknown> | undefined
  return (body?.data && typeof body.data === 'object' ? body.data : body) as T
}
async function load() {
  if (!Number.isInteger(taskId.value) || taskId.value <= 0) { error.value = '任务 ID 无效。'; return }
  loading.value = true; error.value = ''
  try {
    task.value = unwrap<V8Task>(await tasksApi.getById(String(taskId.value)))
    const [nextBundle, eventResponse, handoverResponse] = await Promise.all([
      task.value.task_type === 'sku_planning' ? Promise.resolve(null) : resourceGroupsApi.taskBundle(taskId.value),
      tasksApi.listTaskEvents(String(taskId.value)).catch(() => null),
      tasksApi.listAuditHandovers(String(taskId.value)).catch(() => null),
    ])
    bundle.value = nextBundle
    events.value = eventResponse ? unwrap<TaskEvent[]>(eventResponse) || [] : []
    handovers.value = handoverResponse ? unwrap<AuditHandover[]>(handoverResponse) || [] : []
  } catch (cause) { error.value = cause instanceof Error ? cause.message : '任务加载失败。' }
  finally { loading.value = false }
}
async function onWorkflowUpdated(next: ResourceBundle) { bundle.value = next; await load() }
async function submitHandover() {
  if (!handoverUserId.value || !handoverReason.value || handoverBusy.value) return
  handoverBusy.value = true; error.value = ''
  try { await tasksApi.auditHandover(String(taskId.value), { to_auditor_id: handoverUserId.value, reason: handoverReason.value }); showHandover.value = false; handoverReason.value = ''; await load() }
  catch (cause) { error.value = cause instanceof Error ? cause.message : '审核交班失败。' }
  finally { handoverBusy.value = false }
}
async function takeover(handoverId: number) {
  if (handoverBusy.value) return
  handoverBusy.value = true; error.value = ''
  try { await tasksApi.auditTakeover(String(taskId.value), { handover_id: handoverId }); await load() }
  catch (cause) { error.value = cause instanceof Error ? cause.message : '接手任务失败。' }
  finally { handoverBusy.value = false }
}
onMounted(load)
</script>

<style scoped>
.task-page{max-width:1240px;margin:0 auto;padding:28px;display:grid;gap:20px}.task-head,.head-actions,.resource-head,.planning-card,.section-title{display:flex;align-items:flex-start;justify-content:space-between;gap:18px}.task-head h1{margin:4px 0;font-size:32px}.task-head p,.planning-card p,.section-title p{margin:0;color:rgb(var(--yb-text-muted))}.eyebrow{font-size:11px;letter-spacing:.13em;font-weight:900;color:rgb(var(--yb-brand))}.head-actions{align-items:center}.head-actions button,.resource-head button,.planning-card a,.section-title button,.handover-form button,.handover-list button{min-height:38px;padding:0 13px;display:inline-flex;align-items:center;border:1px solid rgb(var(--yb-border));border-radius:10px;background:rgb(var(--yb-surface));color:rgb(var(--yb-text));text-decoration:none;cursor:pointer}.progress-card,.resource-card,.planning-card,.business-context article,.collaboration-card,.timeline-card{padding:20px;border:1px solid rgb(var(--yb-border));border-radius:18px;background:rgb(var(--yb-surface))}.facts{display:grid;grid-template-columns:repeat(3,minmax(0,1fr));gap:1px;border:1px solid rgb(var(--yb-border));border-radius:16px;overflow:hidden;background:rgb(var(--yb-border))}.facts>div{display:grid;gap:4px;padding:15px;background:rgb(var(--yb-surface))}.facts span{font-size:12px;color:rgb(var(--yb-text-muted))}.business-context{display:grid;grid-template-columns:1fr 1fr;gap:14px}.business-context .reference-context{grid-column:1/-1}.section-title h2{margin:3px 0}.long-copy{margin:14px 0 0;white-space:pre-wrap;color:rgb(var(--yb-text-muted))}.reference-list{display:flex;flex-wrap:wrap;gap:8px;margin-top:14px}.reference-list a{padding:8px 10px;border-radius:9px;background:rgb(var(--yb-surface-muted));color:rgb(var(--yb-text));text-decoration:none}.resource-card{display:grid;gap:18px}.resource-head{align-items:center}.resource-head h2,.planning-card h2{margin:3px 0}.collaboration-card,.timeline-card{display:grid;gap:14px}.handover-form{display:grid;grid-template-columns:1fr 2fr auto;align-items:end;gap:10px}.handover-form label{display:grid;gap:5px}.handover-form input{min-height:38px;border:1px solid rgb(var(--yb-border));border-radius:9px;padding:0 10px}.handover-list{display:grid;gap:8px}.handover-list>div{display:flex;align-items:center;gap:10px;padding:10px;border-radius:10px;background:rgb(var(--yb-surface-muted))}.handover-list span{margin-left:auto}.timeline-card ol{display:grid;gap:8px;margin:0;padding:0;list-style:none}.timeline-card li{display:grid;grid-template-columns:1fr auto;gap:4px;padding:11px;border-radius:10px;background:rgb(var(--yb-surface-muted))}.timeline-card li span{color:rgb(var(--yb-text-muted));font-size:12px}.timeline-card li p{grid-column:1/-1;margin:0}.message,.state{padding:16px;border-radius:12px;background:rgb(var(--yb-surface-muted))}.error{background:rgb(var(--yb-danger-soft));color:rgb(var(--yb-danger-text))}@media(max-width:760px){.task-page{padding:16px}.task-head,.planning-card,.section-title{flex-direction:column}.head-actions{flex-wrap:wrap}.facts,.business-context{grid-template-columns:1fr}.business-context .reference-context{grid-column:auto}.handover-form{grid-template-columns:1fr}.progress-card{overflow:auto}}
</style>
