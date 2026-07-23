<template>
  <main class="task-page task-detail-view">
    <div v-if="loading && !task" class="state">正在打开任务…</div>
    <div v-else-if="error && !task" class="state error" role="alert">{{ error }}</div>
    <template v-else-if="task">
      <section class="task-hero" :class="{ 'is-terminal': isTerminal }">
        <TaskDetailAtmosphere />
        <div class="hero-content">
          <nav class="hero-nav" aria-label="页面位置">
            <button class="back-button" @click="goBack"><ArrowLeft :size="16" :stroke-width="1.9" aria-hidden="true" /><span>返回任务中心</span></button>
            <div class="hero-actions">
              <TaskStatusTag :status="task.task_status" />
              <button class="refresh-button" :disabled="loading" aria-label="刷新任务" @click="load"><RefreshCw :size="16" :stroke-width="1.9" aria-hidden="true" /><span>刷新</span></button>
            </div>
          </nav>
          <div class="hero-main">
            <div class="hero-identity">
              <h1>{{ task.product_name_snapshot || task.primary_sku_code || task.task_no }}</h1>
              <p class="task-number"><span>{{ task.task_no }}</span><i aria-hidden="true" /><span>{{ task.primary_sku_code || task.sku_code || '尚未生成 SKU' }}</span></p>
              <div class="identity-badges" aria-label="任务身份">
                <span v-if="!isCustomization" class="identity-badge task-kind"><Tag :size="14" aria-hidden="true" />{{ taskTypeLabel }}</span>
                <span class="identity-badge lane" :class="{ customization: isCustomization }"><Layers3 :size="14" aria-hidden="true" />{{ isCustomization ? `${taskTypeLabel} · ${businessLaneLabel}` : `${businessLaneLabel}任务` }}</span>
                <span class="identity-badge sku-scope"><Boxes :size="14" aria-hidden="true" />{{ skuScopeLabel }}</span>
              </div>
            </div>
            <dl class="hero-facts">
              <div v-for="fact in heroFacts" :key="fact.label"><dt>{{ fact.label }}</dt><dd :class="{ 'is-empty': fact.tone === 'empty', 'is-danger': fact.tone === 'danger' }">{{ fact.value }}</dd></div>
            </dl>
          </div>
          <div class="hero-progress"><WorkflowProgress variant="horizontal" :task="task" /></div>
        </div>
      </section>

      <div v-if="error" class="message error" role="alert">{{ error }}</div>

      <TaskResourceRail
        v-if="bundle"
        :bundle="bundle"
        :references="referenceFiles"
        :task-status="task.task_status"
        :can-operate="canOperateResources"
        :action-label="workflowButtonLabel"
        @open-attachments="openWorkspace('attachments')"
        @open-resources="openWorkspace('resources')"
        @open-workflow="openWorkspace('workflow')"
      />

      <section class="command-strip" :class="{ 'is-complete': isTerminal }" aria-label="当前阶段操作">
        <div class="stage-summary">
          <span class="stage-orb" :class="{ 'is-complete': isTerminal }" aria-hidden="true"><CheckCircle2 v-if="isTerminal" :size="20" /><PanelTopOpen v-else :size="19" /></span>
          <div class="stage-copy"><p class="stage-label">当前节点</p><h2>{{ currentStageTitle }}</h2><p>{{ currentStageDescription }}</p></div>
          <dl class="stage-context" aria-label="当前协作信息">
            <div><dt>当前指派</dt><dd>{{ currentOwner }}</dd></div>
            <div><dt>最近动态</dt><dd>{{ latestEventTitle }}</dd></div>
          </dl>
        </div>
        <div class="command-actions">
          <button v-if="isPlanning" class="primary-button" :disabled="planningExporting" @click="downloadPlanningResult"><Download :size="16" aria-hidden="true" />{{ planningExporting ? '正在导出…' : '导出策划结果' }}</button>
          <button v-else-if="canOperateResources" class="primary-button" @click="openWorkspace('workflow')"><PanelTopOpen :size="16" aria-hidden="true" />{{ workflowButtonLabel }}</button>
          <button v-if="referenceFiles.length" class="secondary-button" @click="openWorkspace('attachments')"><Paperclip :size="16" aria-hidden="true" />查看参考附件</button>
          <button class="secondary-button" @click="openWorkspace('details')"><FileText :size="16" aria-hidden="true" />完整任务信息</button>
        </div>
      </section>

      <section class="overview-grid" aria-label="任务关键信息">
        <article class="brief-card mission-card">
          <header><div class="card-heading"><span class="heading-icon"><ClipboardList :size="18" aria-hidden="true" /></span><div><h2>{{ requirementHeading }}</h2><p>需求与运营交代</p></div></div><button @click="openWorkspace('details')"><FileText :size="15" aria-hidden="true" />完整资料</button></header>
          <div class="mission-copy">
            <section><span class="section-label">需求说明</span><p class="clamped-copy">{{ requirementText }}</p></section>
            <aside><span class="section-label">运营备注</span><p class="clamped-copy">{{ operationNote }}</p></aside>
          </div>
          <div v-if="specSummary.length" class="detail-chips"><span v-for="item in specSummary" :key="item">{{ item }}</span></div>
        </article>

        <article class="brief-card references-card">
          <header><div class="card-heading"><span class="heading-icon"><Paperclip :size="18" aria-hidden="true" /></span><div><h2>参考资料</h2><p>{{ referenceFiles.length }} 个附件</p></div></div><div class="card-actions"><button @click="openWorkspace('attachments')"><ExternalLink :size="15" aria-hidden="true" />查看附件</button><button v-if="canManageReferences" class="upload-button" :disabled="referenceUploading" @click="referenceInput?.click()"><Plus :size="15" aria-hidden="true" />{{ referenceUploading ? '上传中…' : '补充附件' }}</button></div></header>
          <div v-if="referenceFiles.length" class="reference-preview">
            <a v-for="(file,index) in referenceFiles.slice(0,4)" :key="referenceKey(file,index)" :href="referenceUrl(file)" target="_blank" rel="noreferrer">
              <img v-if="isPreviewable(file) && referencePreviewUrl(file) && !brokenReferences.has(referenceKey(file,index))" :src="referencePreviewUrl(file)" :alt="referenceName(file)" @error="markReferenceBroken(file,index)" />
              <span v-else class="file-glyph" aria-hidden="true">{{ fileExtension(file) }}</span>
              <small>{{ referenceName(file) }}</small>
            </a>
          </div>
          <p v-else class="muted-copy">暂无参考附件。</p>
        </article>

        <article class="brief-card collaboration-card">
          <header><div class="card-heading"><span class="heading-icon"><Activity :size="18" aria-hidden="true" /></span><div><h2>处理与动态</h2><p>当前协作状态</p></div></div><button @click="openWorkspace('history')"><History :size="15" aria-hidden="true" />历史记录 · {{ events.length }}</button></header>
          <div class="assignment-snapshot">
            <span class="person-avatar" aria-hidden="true">{{ ownerInitial }}</span>
            <div><small>当前处理人</small><strong>{{ currentOwner }}</strong><p>{{ ownerOrg }}</p></div>
          </div>
          <div v-if="latestEvent" class="current-event">
            <span class="event-dot" />
            <div><small>最新动态</small><strong>{{ eventTitle(latestEvent) }}</strong><p>{{ eventActor(latestEvent) }} · {{ displayDate(latestEvent.created_at) }}</p></div>
          </div>
          <p v-else class="muted-copy">任务刚刚创建，等待首次处理。</p>
          <div class="collaboration-actions">
            <button @click="openWorkspace('details')"><UserRound :size="15" aria-hidden="true" />人员与组织</button>
            <button v-if="canAssignDesigner" @click="openAssignDialog">{{ task.designer_name ? '改派设计' : '指派设计' }}</button>
            <button v-if="supportsAuditCollaboration" @click="openWorkspace('collaboration')">审核协作</button>
          </div>
        </article>
      </section>

      <Teleport to="body">
      <div v-if="workspaceMode" class="workspace-backdrop" @mousedown.self="closeWorkspace">
        <section ref="workspaceDialog" class="workspace-dialog" role="dialog" aria-modal="true" :aria-label="workspaceTitle" tabindex="-1" @keydown="handleWorkspaceKeydown">
          <header class="workspace-head">
            <div><p class="eyebrow">{{ task.task_no }}</p><h2>{{ workspaceTitle }}</h2><p>{{ workspaceSubtitle }}</p></div>
            <button class="close-button" aria-label="关闭" @click="closeWorkspace">×</button>
          </header>

          <div v-if="workspaceMode === 'workflow' && bundle" class="workspace-body workflow-body">
            <ResourceWorkflowPanel
              :task-id="task.id"
              :task-type="task.task_type"
              :bundle="bundle"
              :reference-count="referenceFiles.length"
              :sku-mode-hints="skuModeHints"
              :allowed-actions="task.allowed_actions || []"
              @updated="onWorkflowUpdated"
              @dirty-change="workflowDirty = $event"
            />
          </div>

          <div v-else-if="workspaceMode === 'resources' && bundle" class="workspace-body"><SkuResourceMatrix :bundle="bundle" :enable-revision-history="can('asset.view')" /></div>

          <div v-else-if="workspaceMode === 'attachments'" class="workspace-body attachment-body">
            <TaskAttachmentWorkspace :files="referenceFiles" :can-upload="canManageReferences" :uploading="referenceUploading" @upload="referenceInput?.click()" />
          </div>

          <div v-else-if="workspaceMode === 'details'" class="workspace-body detail-sections">
            <section class="detail-summary-strip"><div><span>当前状态</span><strong>{{ currentStageTitle }}</strong></div><div><span>任务类型</span><strong>{{ taskTypeLabel }} · {{ businessLaneLabel }}</strong></div><div><span>当前指派</span><strong>{{ currentOwner }}</strong></div><div><span>附件</span><strong>{{ referenceFiles.length }} 个</strong></div><div><span>最近更新</span><strong>{{ displayDate(task.updated_at) }}</strong></div></section>
            <section class="detail-requirement"><p class="eyebrow">需求与运营交代</p><div class="detail-copy-grid"><div><h3>{{ requirementHeading }}</h3><p class="long-copy">{{ requirementText }}</p></div><aside><h3>运营备注</h3><p class="long-copy">{{ operationNote }}</p></aside></div></section>
            <section><p class="eyebrow">人员与组织</p><dl class="detail-list"><div><dt>创建人</dt><dd>{{ task.creator_name || '—' }}</dd></div><div><dt>设计人员</dt><dd>{{ task.designer_name || '—' }}</dd></div><div><dt>当前处理人</dt><dd>{{ currentOwner }}</dd></div><div><dt>归属组织</dt><dd>{{ ownerOrg }}</dd></div></dl></section>
            <section><p class="eyebrow">产品与规格</p><dl class="detail-list"><div><dt>主 SKU</dt><dd>{{ task.primary_sku_code || task.sku_code || '—' }}</dd></div><div><dt>产品名称</dt><dd>{{ task.product_name_snapshot || '—' }}</dd></div><div><dt>规格</dt><dd>{{ detailValue('spec_text') }}</dd></div><div><dt>尺寸</dt><dd>{{ detailValue('size_text') }}</dd></div><div><dt>材质</dt><dd>{{ detailValue('material') }}</dd></div><div><dt>工艺</dt><dd>{{ detailValue('craft_text') }}</dd></div></dl></section>
            <section v-if="resourceSKUProfiles.length"><p class="eyebrow">SKU 规格与成本</p><div class="detail-sku-costs"><article v-for="item in resourceSKUProfiles" :key="item.groupId"><div><strong>{{ item.sku }}</strong><span>{{ item.product }}</span></div><dl><div><dt>规格</dt><dd>{{ item.specification }}</dd></div><div><dt>面积</dt><dd>{{ item.area }}</dd></div><div><dt>成本</dt><dd>{{ item.cost }}</dd></div><div><dt>规则</dt><dd>{{ item.rule }}</dd></div></dl></article></div></section>
            <section><p class="eyebrow">业务与时效</p><dl class="detail-list"><div v-for="item in businessDetailItems" :key="item.label"><dt>{{ item.label }}</dt><dd>{{ item.value }}</dd></div></dl></section>
            <section><p class="eyebrow">文案与同步</p><dl class="detail-list"><div v-for="item in contentDetailItems" :key="item.label"><dt>{{ item.label }}</dt><dd>{{ item.value }}</dd></div></dl></section>
            <section v-if="retouchRequirements.length"><p class="eyebrow">逐项修图要求</p><ol class="retouch-requirement-list"><li v-for="(item,index) in retouchRequirements" :key="String(item.id || index)"><span>{{ index + 1 }}</span><div><strong>{{ item.description || item.requirement || `修图要求 ${index + 1}` }}</strong><p v-if="item.remark || item.note">{{ item.remark || item.note }}</p></div></li></ol></section>
            <section class="reference-summary"><p class="eyebrow">参考附件</p><div><strong>{{ referenceFiles.length }} 个文件</strong><p>参考资料已集中到独立附件工作台，可预览并下载，不再与完整任务信息混在一起。</p><button type="button" @click="openWorkspace('attachments')">打开附件工作台</button></div></section>
            <section v-if="skuItems.length"><p class="eyebrow">SKU 清单</p><div class="sku-list"><span v-for="item in skuItems" :key="String(item.id || item.sku_code)">{{ item.sku_code || `子项 ${item.sequence_no || ''}` }}<em v-if="item.set_mode_hint || item.setModeHint">运营建议套装 · 设计可调整</em></span></div></section>
          </div>

          <div v-else-if="workspaceMode === 'history'" class="workspace-body history-body">
            <div v-if="latestEvent" class="history-now"><span class="event-dot" /><div><p class="eyebrow">当前动态</p><h3>{{ eventTitle(latestEvent) }}</h3><p>{{ eventActor(latestEvent) }} · {{ displayDate(latestEvent.created_at) }}</p></div></div>
            <header class="history-heading"><div><p class="eyebrow">历史动态</p><h3>完整处理轨迹</h3></div><span>{{ events.length }} 条记录</span></header>
            <ol v-if="groupedEvents.length" class="full-timeline"><li v-for="item in groupedEvents" :key="item.key"><span class="event-dot" /><div><strong>{{ item.title }}<em v-if="item.count > 1" class="event-count">×{{ item.count }}</em></strong><p>{{ item.actor }} · {{ item.timeRange }}</p><small v-if="item.note">{{ item.note }}</small></div></li></ol>
            <p v-else class="muted-copy">暂无任务动态。</p>
          </div>

          <div v-else-if="workspaceMode === 'collaboration'" class="workspace-body collaboration-body">
            <form v-if="canHandover" class="handover-form" @submit.prevent="submitHandover">
              <label>交给哪位审核人员
                <select v-model="handoverUserId" required>
                  <option :value="null" disabled>{{ auditorsLoading ? '正在加载审核人员…' : '请选择接手的审核人员' }}</option>
                  <option v-for="person in auditorCandidates" :key="person.id" :value="Number(person.id)">{{ person.name }}</option>
                </select>
              </label>
              <label>交班原因<textarea v-model.trim="handoverReason" maxlength="1000" placeholder="例如：今天请假，请同事接手继续审核" required /></label>
              <button class="primary-button" :disabled="handoverBusy || !handoverUserId || !handoverReason">{{ handoverBusy ? '提交中…' : '确认交班' }}</button>
            </form>
            <div class="handover-side">
              <div v-if="handovers.length" class="handover-list"><article v-for="item in handovers" :key="item.id"><div><strong>{{ item.handover_no || `交班 ${item.id}` }}</strong><p>{{ handoverStatusLabel(item.status) }}</p></div><button v-if="item.allowed_actions?.includes('task.audit.takeover')" class="secondary-button" :disabled="handoverBusy" @click="takeover(item.id)">接手</button></article></div>
              <p v-else class="muted-copy">暂无交班记录。</p>
              <p v-if="!canHandover" class="muted-copy">只有当前正在审核这张任务的人员可以发起交班；如需批量交班，请到任务中心列表右上角的「审核交班」入口。</p>
            </div>
          </div>
        </section>
      </div>
      </Teleport>

      <button v-if="supportsAuditCollaboration" class="collaboration-fab" @click="openWorkspace('collaboration')">审核协作<span v-if="handovers.length">{{ handovers.length }}</span></button>

      <ReassignDesignerDialog
        v-model="assignDialogOpen"
        :designers="assignDesigners"
        :loading="assignDesignersLoading"
        :submitting="assignSubmitting"
        :submit-error="assignError"
        :current-assignee-id="currentDesignerId"
        :current-assignee-name="task.designer_name || null"
        :has-design-output-hint="Boolean(bundle?.groups.some((group) => group.working_revision || group.finalized_revision))"
        @confirm="submitAssign"
      />
      <input ref="referenceInput" class="sr-only" type="file" accept="image/*,.pdf,.zip" multiple aria-label="补充任务参考附件" @change="uploadReferenceFiles" />
    </template>
  </main>
</template>

<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { onBeforeRouteLeave, onBeforeRouteUpdate, useRoute, useRouter } from 'vue-router'
import {
  Activity,
  ArrowLeft,
  Boxes,
  CheckCircle2,
  ClipboardList,
  Download,
  ExternalLink,
  FileText,
  History,
  Layers3,
  PanelTopOpen,
  Paperclip,
  Plus,
  RefreshCw,
  Tag,
  UserRound,
} from 'lucide-vue-next'
import { tasksApi } from '@/services/api/tasksApi'
import { resourceGroupsApi, type ResourceBundle } from '@/services/api/resourceGroupsApi'
import { mergeDetailEnvelopeIntoTaskRaw } from '@/domain/mappers/task-detail-envelope'
import { useDesignerOptions } from '@/composables/useDesignerOptions'
import { usePermission } from '@/composables/usePermission'
import WorkflowProgress from '@/components/task/WorkflowProgress.vue'
import TaskStatusTag from '@/components/task/TaskStatusTag.vue'
import SkuResourceMatrix from '@/components/task/SkuResourceMatrix.vue'
import ResourceWorkflowPanel from '@/components/task/ResourceWorkflowPanel.vue'
import TaskAttachmentWorkspace from '@/components/task/TaskAttachmentWorkspace.vue'
import TaskResourceRail from '@/components/task/TaskResourceRail.vue'
import TaskDetailAtmosphere from '@/components/task/TaskDetailAtmosphere.vue'
import ReassignDesignerDialog from '@/components/task/ReassignDesignerDialog.vue'
import { uploadReferenceFileRef } from '@/services/upload/assetUploadFlow'
import { planningSkuApi } from '@/services/api/planningSkuApi'
import { handoverStatusLabel, taskDetailDisplayValue } from '@/domain/task-detail-display'

interface V8Task extends Record<string, unknown> {
  id: number; task_no: string; task_type: string; task_status: string; workflow_revision: number; workflow_contract_version: 2; allowed_actions: string[]
  sku_code?: string; primary_sku_code?: string; product_name_snapshot?: string; current_handler_name?: string; designer_id?: number | string | null; designer_name?: string; creator_name?: string; owner_department?: string; owner_org_team?: string; business_lane?: string
  requirement_description?: string; description?: string; design_requirement?: string; change_request?: string; note?: string; remark?: string; operation_note?: string; reference_file_refs?: ReferenceFile[]; updated_at?: string; due_at?: string; deadline_at?: string
}
interface ReferenceFile extends Record<string, unknown> { id?: number; asset_id?: string; file_name?: string; filename?: string; mime_type?: string; download_url?: string; preview_url?: string; url?: string }
interface TaskEvent extends Record<string, unknown> { id?: number; event_type?: string; title?: string; operator_name?: string; actor_name?: string; created_at?: string; reason?: string; remark?: string }
interface AuditHandover { id: number; handover_no?: string; status?: string; allowed_actions?: string[] }
type WorkspaceMode = 'workflow' | 'resources' | 'attachments' | 'details' | 'history' | 'collaboration'

const route = useRoute()
const router = useRouter()
const { can } = usePermission()
const task = ref<V8Task | null>(null)
const aggregate = ref<Record<string, unknown>>({})
const bundle = ref<ResourceBundle | null>(null)
const loading = ref(false)
const error = ref('')
const events = ref<TaskEvent[]>([])
const handovers = ref<AuditHandover[]>([])
const handoverUserId = ref<number | null>(null)
const handoverReason = ref('')
const handoverBusy = ref(false)
const assignDialogOpen = ref(false)
const assignSubmitting = ref(false)
const referenceInput = ref<HTMLInputElement | null>(null)
const referenceUploading = ref(false)
const brokenReferences = ref(new Set<string>())
const planningExporting = ref(false)
const assignError = ref('')
const {
  designers: assignDesigners,
  loading: assignDesignersLoading,
  loadDesigners: loadAssignDesigners,
} = useDesignerOptions({ includeEmpty: false, autoLoad: false, requiredActions: ['task.assign', 'task.assign.team', 'task.assign.department'] })
const {
  designers: auditorCandidates,
  loading: auditorsLoading,
  loadDesigners: loadAuditorCandidates,
} = useDesignerOptions({ includeEmpty: false, autoLoad: false, workflowLane: 'audit', requiredActions: ['task.audit.decision'] })
const workflowDirty = ref(false)
const workspaceMode = ref<WorkspaceMode | null>(null)
const workspaceDialog = ref<HTMLElement | null>(null)
const workspaceReturnFocus = ref<HTMLElement | null>(null)
let previousBodyOverflow = ''
const taskId = computed(() => Number(route.params.id))
const isPlanning = computed(() => task.value?.task_type === 'sku_planning')
const isRetouch = computed(() => ['retouch', 'retouch_task'].includes(task.value?.task_type || ''))
const taskTypeLabel = computed(() => ({ original_product_development: '原品开发', new_product_development: '新品开发', retouch_task: '修图任务', sku_planning: '策划 SKU', regular_customization: '常规定制', customer_customization: '客户定制' }[task.value?.task_type || ''] || '其他任务'))
const isCustomization = computed(() => task.value?.business_lane === 'customization' || ['regular_customization', 'customer_customization'].includes(task.value?.task_type || ''))
const businessLaneLabel = computed(() => isCustomization.value ? '定制' : '常规')
const requirementHeading = computed(() => isPlanning.value ? '策划说明' : isRetouch.value ? '修图要求' : isCustomization.value ? '定制需求' : '设计需求')
const requirementText = computed(() => task.value?.design_requirement || task.value?.change_request || task.value?.requirement_description || task.value?.description || '未填写需求说明。')
const operationNote = computed(() => task.value?.note || task.value?.operation_note || task.value?.remark || '未填写运营备注。')
const referenceFiles = computed(() => (task.value?.reference_file_refs || []) as ReferenceFile[])
const skuItems = computed(() => (aggregate.value.sku_items || aggregate.value.skuItems || []) as Array<Record<string, unknown>>)
const retouchRequirements = computed(() => (aggregate.value.retouch_requirements || aggregate.value.retouchRequirements || []) as Array<Record<string, unknown>>)
const skuModeHints = computed<Record<string, boolean>>(() => {
  const hints = Object.fromEntries(
    skuItems.value
      .filter((item) => Boolean(item.set_mode_hint ?? item.setModeHint))
      .map((item) => [String(item.sku_code ?? item.skuCode ?? ''), true])
      .filter(([sku]) => Boolean(sku)),
  )
  if (task.value?.set_mode_hint || task.value?.setModeHint) hints[''] = true
  return hints
})
const actionSet = computed(() => new Set(task.value?.allowed_actions || []))
const canManageReferences = computed(() => actionSet.value.has('task.reference.append'))
const canHandover = computed(() => actionSet.value.has('task.audit.handover'))
const supportsAuditCollaboration = computed(() => task.value?.task_status === 'PendingAudit' && !isPlanning.value && !isRetouch.value)
const canAssignDesigner = computed(() => {
  if (!task.value || isPlanning.value) return false
  if (['Completed', 'Archived', 'Cancelled'].includes(task.value.task_status)) return false
  return actionSet.value.has('task.assign')
})
const currentDesignerId = computed(() => task.value?.designer_id != null && task.value.designer_id !== '' ? String(task.value.designer_id) : null)
const isTerminal = computed(() => ['Completed', 'Archived', 'Cancelled'].includes(task.value?.task_status || ''))
const hasOwner = computed(() => Boolean(task.value?.current_handler_name || task.value?.designer_name))
const skuCount = computed(() => skuItems.value.length || (task.value?.primary_sku_code || task.value?.sku_code ? 1 : 0))
const skuScopeLabel = computed(() => skuCount.value > 1 ? `批量 SKU · ${skuCount.value} 项` : '单 SKU')
const dueAtRaw = computed(() => task.value?.due_at || task.value?.deadline_at || '')
const dueAtText = computed(() => {
  if (!dueAtRaw.value) return '未设置'
  const date = new Date(String(dueAtRaw.value))
  return Number.isNaN(date.getTime()) ? String(dueAtRaw.value) : new Intl.DateTimeFormat('zh-CN', { month: 'numeric', day: 'numeric', hour: '2-digit', minute: '2-digit' }).format(date)
})
const dueSoonOrOverdue = computed(() => {
  if (!dueAtRaw.value || ['Completed', 'Archived', 'Cancelled'].includes(task.value?.task_status || '')) return false
  const dueMs = new Date(String(dueAtRaw.value)).getTime()
  return Number.isFinite(dueMs) && dueMs - Date.now() < 24 * 60 * 60 * 1000
})
const canOperateResources = computed(() => !isPlanning.value && (actionSet.value.has('task.design.submit') || actionSet.value.has('task.audit.approve') || actionSet.value.has('task.audit.decision') || actionSet.value.has('task.reopen')))
const currentOwner = computed(() => task.value?.current_handler_name || task.value?.designer_name || (isTerminal.value ? '已结单' : '等待指派'))
const ownerOrg = computed(() => [task.value?.owner_department, task.value?.owner_org_team].filter(Boolean).join(' · ') || '未设置')
const ownerInitial = computed(() => ['等待指派', '已结单'].includes(currentOwner.value) ? (isTerminal.value ? '✓' : '待') : currentOwner.value.trim().slice(0, 1))
/** Hero 四宫格随任务类型变化：策划类无处理人/截止时间，改展示 SKU 数量与创建信息。 */
const heroFacts = computed<Array<{ label: string; value: string; tone?: 'empty' | 'danger' }>>(() => {
  if (isPlanning.value) return [
    { label: 'SKU 数量', value: skuCount.value ? `${skuCount.value} 个` : '—' },
    { label: '创建人', value: String(task.value?.creator_name || '—') },
    { label: isTerminal.value ? '完成时间' : '最近更新', value: displayDate(task.value?.updated_at) },
    { label: '归属组织', value: ownerOrg.value },
  ]
  return [
    { label: '当前处理人', value: currentOwner.value, tone: !hasOwner.value && !isTerminal.value ? 'empty' : undefined },
    { label: '截止时间', value: dueAtText.value, tone: dueSoonOrOverdue.value ? 'danger' : undefined },
    { label: '归属组织', value: ownerOrg.value },
    { label: isTerminal.value ? '完成时间' : '最近更新', value: displayDate(task.value?.updated_at) },
  ]
})
const sortedEvents = computed(() => [...events.value].sort((left, right) => {
  const timeDelta = new Date(String(right.created_at || '')).getTime() - new Date(String(left.created_at || '')).getTime()
  if (Number.isFinite(timeDelta) && timeDelta !== 0) return timeDelta
  return Number(right.id || 0) - Number(left.id || 0)
}))
const latestEvent = computed(() => sortedEvents.value[0] || null)
const latestEventTitle = computed(() => latestEvent.value ? eventTitle(latestEvent.value) : '等待首次处理')
/** 连续同类动态折叠成一条（×N），避免“任务信息已更新”刷屏、重点被淹没。 */
const groupedEvents = computed(() => {
  const groups: Array<{ key: string; title: string; actor: string; count: number; timeRange: string; note: string }> = []
  for (const item of sortedEvents.value) {
    const title = eventTitle(item)
    const actor = String(eventActor(item))
    const note = String(item.reason || item.remark || '')
    const last = groups[groups.length - 1]
    if (last && last.title === title && last.actor === actor && !note) {
      last.count += 1
      last.timeRange = `${displayDate(item.created_at)} ~ ${last.timeRange.split(' ~ ').pop()}`
      continue
    }
    groups.push({ key: eventKey(item), title, actor, count: 1, timeRange: displayDate(item.created_at), note })
  }
  return groups
})
const specSummary = computed(() => ['category_name','spec_text','size_text','material','craft_text'].map((key) => String(task.value?.[key] || '')).filter(Boolean).slice(0,4))
const businessDetailItems = computed(() => [
  { label: '优先级', value: detailValue('priority') },
  { label: '截止时间', value: dueAtText.value },
  { label: '数量', value: task.value?.quantity == null ? '—' : `${task.value.quantity}` },
  { label: 'SKU 当前成本', value: resourceCostSummary.value || formatMoney(task.value?.cost_price) },
  { label: '成本方式', value: detailValue('cost_price_mode') },
])
const contentDetailItems = computed(() => [
  { label: '产品简称', value: detailValue('product_short_name') },
  { label: '页面文案', value: detailValue('copy_content') },
  { label: '风格关键词', value: detailValue('style_keywords') },
  { label: '参考链接', value: detailValue('reference_link') },
  { label: 'ERP 建档', value: detailValue('filing_status') },
  { label: 'ERP 同步', value: task.value?.erp_sync_required === true ? taskDetailDisplayValue('erp_sync_status', task.value?.erp_sync_status, '等待同步') : task.value?.erp_sync_required === false ? '无需同步' : taskDetailDisplayValue('erp_sync_status', task.value?.erp_sync_status) },
])
const resourceSKUProfiles = computed(() => (bundle.value?.groups || []).map((group) => {
  const profile = group.sku_profile
  const area = profile?.area_trace?.area_m2
  const cost = profile?.cost_price
  return {
    groupId: group.id,
    sku: group.sku_code || '未绑定 SKU',
    product: group.product_name || profile?.product_name || '未命名产品',
    specification: profile?.size_text || profile?.spec_text || '规格待补充',
    area: typeof area === 'number' ? `${area.toFixed(3)} ㎡` : '面积待核对',
    cost: typeof cost === 'number' ? `¥${cost.toFixed(2)}` : '成本待计算',
    rule: profile?.cost_trace?.rule_name || '尚未关联规则',
    costValue: typeof cost === 'number' ? cost : null,
  }
}))
const resourceCostSummary = computed(() => {
  const values = resourceSKUProfiles.value.map((item) => item.costValue).filter((value): value is number => value != null)
  if (!values.length) return ''
  const min = Math.min(...values); const max = Math.max(...values)
  return min === max ? `¥${min.toFixed(2)}` : `¥${min.toFixed(2)} ～ ¥${max.toFixed(2)}`
})
const currentStageTitle = computed(() => ({ Draft: '等待完善', PendingAssign: '等待指派', Assigned: '准备开始', InProgress: isRetouch.value ? '修图处理中' : '设计处理中', PendingAudit: '等待审核', Completed: '任务已结单', Archived: '任务已归档', Cancelled: '任务已取消', Blocked: '任务被阻塞' }[task.value?.task_status || ''] || '任务处理中'))
const currentStageDescription = computed(() => {
  if (task.value?.task_status === 'InProgress' && !isRetouch.value) return '设计人员先确定每个 SKU 是单图还是套装，并提交一份可编辑源文件。'
  if (task.value?.task_status === 'PendingAudit') return '审核人员按设计确认的单图/套装模式上传最终成品，必要时替换源文件。'
  if (isPlanning.value) return 'SKU 与策划信息已经生成，可筛选结果或导出留档。'
  if (isRetouch.value && task.value?.task_status === 'InProgress') return '修图人员提交最终成品后，任务直接结单。'
  return '在这里查看任务全貌、资源与最新处理动态。'
})
const workflowButtonLabel = computed(() => task.value?.task_status === 'PendingAudit' ? '进入审核工作台' : isRetouch.value ? '提交修图成品' : task.value?.task_status === 'Completed' ? '查看结单资源' : '进入设计提交')
const workspaceTitle = computed(() => ({ workflow: currentStageTitle.value, resources: '任务文件总览', attachments: '参考附件', details: '完整任务信息', history: '全部任务动态', collaboration: '审核交班与接手' }[workspaceMode.value || 'details']))
const workspaceSubtitle = computed(() => {
  if (workspaceMode.value === 'workflow') return currentStageDescription.value
  if (workspaceMode.value === 'attachments') return '集中预览和下载运营提供的参考图片与文件。'
  return '看完点右上角关闭，页面还停在这张任务上。'
})

function unwrap<T>(response: { data?: unknown }): T { const body = response.data as Record<string, unknown> | undefined; return (body?.data && typeof body.data === 'object' ? body.data : body) as T }
function unwrapCollection<T>(response: { data?: unknown } | null, fallback: T[] = []): T[] {
  if (!response) return fallback
  const value = unwrap<unknown>(response)
  if (Array.isArray(value)) return value as T[]
  if (value && typeof value === 'object') {
    const envelope = value as Record<string, unknown>
    for (const key of ['items', 'events', 'handovers', 'data']) {
      if (Array.isArray(envelope[key])) return envelope[key] as T[]
    }
  }
  return fallback
}
function displayDate(value: unknown) { if (!value) return '刚刚'; const date = new Date(String(value)); return Number.isNaN(date.getTime()) ? String(value) : new Intl.DateTimeFormat('zh-CN',{ month:'numeric',day:'numeric',hour:'2-digit',minute:'2-digit' }).format(date) }
function detailValue(key: string) { return taskDetailDisplayValue(key, task.value?.[key]) }
function formatMoney(value: unknown) { const amount = Number(value); return Number.isFinite(amount) ? `¥${amount.toFixed(2)}` : '—' }
function referenceName(file: ReferenceFile) { return String(file.filename || file.file_name || file.asset_id || '参考附件') }
function referenceUrl(file: ReferenceFile) { return String(file.download_url || file.preview_url || file.url || '') }
function referencePreviewUrl(file: ReferenceFile) { return String(file.preview_url || file.url || file.download_url || '') }
function referenceKey(file: ReferenceFile,index: number) { return String(file.id || file.asset_id || `${referenceName(file)}-${index}`) }
function markReferenceBroken(file: ReferenceFile,index: number) { brokenReferences.value = new Set(brokenReferences.value).add(referenceKey(file,index)) }
function fileExtension(file: ReferenceFile) { const name = referenceName(file); const suffix = name.includes('.') ? name.split('.').pop() : 'FILE'; return String(suffix || 'FILE').slice(0,4).toUpperCase() }
function isPreviewable(file: ReferenceFile) { return String(file.mime_type || '').startsWith('image/') || /\.(png|jpe?g|webp|gif)$/i.test(referenceName(file)) }
function eventKey(item: TaskEvent) { return String(item.id || `${item.event_type}-${item.created_at}`) }
const friendlyEventTitles: Record<string, string> = {
  'task.created': '任务已创建',
  'task.batch_items_created': 'SKU 明细已生成',
  'task.assigned': '任务已完成指派',
  'task.reassigned': '任务负责人已调整',
  'task.reference.asset.finalized': '参考资料已关联',
  'task.design.submitted': '设计已提交审核',
  'task.audit.approved': '审核通过并已结单',
  'task.audit.returned_to_design': '审核已打回设计',
  'task.completed': '任务已结单',
  'task.closed': '任务已结单',
  'task.filing.triggered': '资料已安排同步到 ERP',
  'module.claimed': '处理人已领取任务',
}
function eventTitle(item: TaskEvent) {
  const type = String(item.event_type || '')
  const title = String(item.title || '').trim()
  if (friendlyEventTitles[type]) return friendlyEventTitles[type]
  if (title && title !== type && !title.includes('.')) return title
  return '任务信息已更新'
}
function eventActor(item: TaskEvent) { return item.operator_name || item.actor_name || '系统' }

async function load() {
  if (!Number.isInteger(taskId.value) || taskId.value <= 0) { error.value = '任务 ID 无效。'; return }
  loading.value = true; error.value = ''
  try {
    const [detailResponse, taskResponse] = await Promise.all([
      tasksApi.getDetail(String(taskId.value)),
      tasksApi.getById(String(taskId.value)),
    ])
    const envelope = unwrap<Record<string, unknown>>(detailResponse) || {}
    const taskContract = unwrap<Record<string, unknown>>(taskResponse) || {}
    aggregate.value = envelope
    const merged = { ...taskContract, ...mergeDetailEnvelopeIntoTaskRaw(envelope) }
    if (!Array.isArray(merged.allowed_actions) && Array.isArray(taskContract.allowed_actions)) {
      merged.allowed_actions = taskContract.allowed_actions
    }
    task.value = merged as V8Task
    const aggregateEvents = Array.isArray(envelope.events) ? envelope.events as TaskEvent[] : []
    const [nextBundle,eventResponse,handoverResponse] = await Promise.all([
      task.value.task_type === 'sku_planning' ? Promise.resolve(null) : resourceGroupsApi.taskBundle(taskId.value),
      tasksApi.listTaskEvents(String(taskId.value)).catch(() => null),
      supportsAuditCollaboration.value ? tasksApi.listAuditHandovers(String(taskId.value)).catch(() => null) : Promise.resolve(null),
    ])
    bundle.value = nextBundle
    events.value = unwrapCollection<TaskEvent>(eventResponse, aggregateEvents)
    handovers.value = unwrapCollection<AuditHandover>(handoverResponse)
  } catch (cause) { error.value = cause instanceof Error ? cause.message : '任务加载失败。' }
  finally { loading.value = false }
}
function goBack() {
  const backState = window.history.state?.back
  if (typeof backState === 'string' && backState.startsWith('/')) router.back()
  else void router.push('/tasks')
}
function openWorkspace(mode: WorkspaceMode) {
  workspaceReturnFocus.value = document.activeElement instanceof HTMLElement ? document.activeElement : null
  workspaceMode.value = mode
  if (mode === 'collaboration' && canHandover.value && !auditorCandidates.value.length) void loadAuditorCandidates()
  void nextTick(() => focusWorkspaceInitial())
}
function focusWorkspaceInitial() {
  const closeButton = workspaceDialog.value?.querySelector<HTMLElement>('.close-button')
  ;(closeButton || workspaceDialog.value)?.focus()
}
function openAssignDialog() {
  assignError.value = ''
  assignDialogOpen.value = true
  if (!assignDesigners.value.length) void loadAssignDesigners()
}
async function uploadReferenceFiles(event: Event) {
  const input = event.target as HTMLInputElement
  const files = Array.from(input.files || [])
  input.value = ''
  if (!task.value || !canManageReferences.value || !files.length || referenceUploading.value) return
  referenceUploading.value = true
  error.value = ''
  try {
    for (const file of files) {
      await uploadReferenceFileRef(file, {
        taskId: String(task.value.id),
        ownerModuleKey: 'basic_info',
        uploadPolicy: 'append_only',
      })
    }
    await load()
  } catch (cause) {
    error.value = cause instanceof Error ? cause.message : '参考附件上传失败，请稍后重试。'
  } finally {
    referenceUploading.value = false
  }
}
async function downloadPlanningResult() {
  if (!task.value || planningExporting.value) return
  planningExporting.value = true
  error.value = ''
  try {
    await planningSkuApi.downloadTask(task.value.id)
  } catch (cause) {
    error.value = cause instanceof Error ? cause.message : '策划结果导出失败，请稍后重试。'
  } finally {
    planningExporting.value = false
  }
}
async function submitAssign(payload: { mode: 'reassign' | 'clear'; assigneeId: string | null; assigneeName: string | null; reasonLabel: string; reasonNote: string }) {
  if (assignSubmitting.value || !task.value) return
  assignSubmitting.value = true
  assignError.value = ''
  try {
    await tasksApi.assign(String(taskId.value), {
      designer_id: payload.mode === 'clear' ? null : Number(payload.assigneeId),
      designer_name: payload.mode === 'clear' ? undefined : payload.assigneeName || undefined,
      remark: [payload.reasonLabel, payload.reasonNote].filter(Boolean).join('：'),
    })
    assignDialogOpen.value = false
    await load()
  } catch (cause) {
    assignError.value = cause instanceof Error ? cause.message : '指派失败，请稍后重试。'
  } finally {
    assignSubmitting.value = false
  }
}
function closeWorkspace() {
  if (workspaceMode.value === 'workflow' && workflowDirty.value && !window.confirm('当前上传区还有未提交修改，确定关闭吗？')) return
  workspaceMode.value = null
  void nextTick(() => workspaceReturnFocus.value?.focus())
}
function handleWorkspaceKeydown(event: KeyboardEvent) {
  if (event.key === 'Escape') {
    event.preventDefault()
    closeWorkspace()
    return
  }
  if (event.key !== 'Tab' || !workspaceDialog.value) return
  const focusable = Array.from(workspaceDialog.value.querySelectorAll<HTMLElement>('a[href],button:not([disabled]),input:not([disabled]),textarea:not([disabled]),select:not([disabled]),[tabindex]:not([tabindex="-1"])'))
    .filter((element) => element.getClientRects().length > 0)
  if (!focusable.length) {
    event.preventDefault()
    workspaceDialog.value.focus()
    return
  }
  const first = focusable[0]
  const last = focusable[focusable.length - 1]
  if (!focusable.includes(document.activeElement as HTMLElement)) {
    event.preventDefault()
    ;(event.shiftKey ? last : first).focus()
    return
  }
  if (event.shiftKey && document.activeElement === first) { event.preventDefault(); last.focus() }
  else if (!event.shiftKey && document.activeElement === last) { event.preventDefault(); first.focus() }
}
async function onWorkflowUpdated(next: ResourceBundle) { workflowDirty.value = false; bundle.value = next; closeWorkspace(); await load() }
async function submitHandover() { if (!handoverUserId.value || !handoverReason.value || handoverBusy.value) return; handoverBusy.value = true; error.value = ''; try { await tasksApi.auditHandover(String(taskId.value),{ to_auditor_id:handoverUserId.value,reason:handoverReason.value }); handoverReason.value=''; await load() } catch(cause) { error.value=cause instanceof Error?cause.message:'审核交班失败。' } finally { handoverBusy.value=false } }
async function takeover(handoverId:number) { if(handoverBusy.value)return; handoverBusy.value=true; error.value=''; try{await tasksApi.auditTakeover(String(taskId.value),{handover_id:handoverId});await load()}catch(cause){error.value=cause instanceof Error?cause.message:'接手任务失败。'}finally{handoverBusy.value=false} }
function confirmWorkflowLeave(){if(!workflowDirty.value)return true;return window.confirm('当前资源区还有未提交修改，确定离开吗？')}
function warnBeforeUnload(event:BeforeUnloadEvent){if(!workflowDirty.value)return;event.preventDefault();event.returnValue=''}
watch(workspaceMode, (mode) => {
  if (mode) {
    previousBodyOverflow = document.body.style.overflow
    document.body.style.overflow = 'hidden'
  } else {
    document.body.style.overflow = previousBodyOverflow
  }
})
onBeforeRouteLeave(()=>confirmWorkflowLeave())
onBeforeRouteUpdate(() => {
  if (!confirmWorkflowLeave()) return false
  workspaceMode.value = null
  return true
})
watch(() => route.params.id, () => { void load() })
onMounted(()=>{window.addEventListener('beforeunload',warnBeforeUnload);void load()})
onBeforeUnmount(()=>{window.removeEventListener('beforeunload',warnBeforeUnload);document.body.style.overflow=previousBodyOverflow})
</script>

<style scoped>
.task-page{max-width:1500px;margin:0 auto;padding:18px;display:grid;gap:12px;color:rgb(var(--yb-text))}
.task-hero{position:relative;min-height:210px;overflow:hidden;border-radius:24px;background:rgb(var(--yb-text-navy));box-shadow:0 22px 54px rgb(var(--yb-shadow)/.17)}
.hero-content{position:relative;z-index:1;min-height:210px;display:grid;grid-template-rows:auto 1fr auto;row-gap:16px;padding:22px 26px;color:rgb(var(--yb-text-inverse))}
.hero-nav,.hero-main,.hero-actions,.command-strip,.command-actions,.brief-card header,.resource-story header,.workspace-head{display:flex;align-items:center;justify-content:space-between;gap:14px}
.back-button,.hero-actions button{min-height:34px;padding:0 11px;border:1px solid rgb(var(--yb-text-inverse)/.18);border-radius:10px;background:rgb(var(--yb-text-night)/.16);color:rgb(var(--yb-text-inverse));cursor:pointer}
.hero-main{align-items:end}.hero-main h1{max-width:820px;margin:10px 0 8px;font-size:clamp(28px,3.2vw,42px);line-height:1.12;letter-spacing:-.03em}
.task-number{margin:0;color:rgb(var(--yb-text-inverse)/.78);font-family:var(--yb-font-data)}
.eyebrow{margin:0;color:rgb(var(--yb-brand));font-size:11px;letter-spacing:.14em;font-weight:900;text-transform:uppercase}.eyebrow.light{color:rgb(var(--yb-text-inverse)/.8)}
.hero-facts{display:grid;grid-template-columns:repeat(4,minmax(112px,1fr));gap:1px;margin:0;overflow:hidden;border:1px solid rgb(var(--yb-text-inverse)/.2);border-radius:13px;background:rgb(var(--yb-text-inverse)/.14)}
.hero-facts div{padding:10px 12px;background:rgb(var(--yb-text-night)/.32)}.hero-facts dt{font-size:11px;font-weight:700;color:rgb(var(--yb-text-inverse)/.82)}.hero-facts dd{margin:3px 0 0;font-weight:800;font-size:14px;color:rgb(var(--yb-text-inverse))}
.hero-facts dd.is-empty{color:rgb(var(--yb-warning-bright,255 196 87))}.hero-facts dd.is-danger{color:rgb(var(--yb-danger-bright,255 138 128))}
.hero-progress{padding-top:14px;border-top:1px solid rgb(var(--yb-text-inverse)/.16)}
.hero-progress :deep(.n-step-content-header__title){color:rgb(var(--yb-text-inverse));font-weight:700}
.hero-progress :deep(.n-step-content__description){color:rgb(var(--yb-text-inverse)/.66)}
.hero-progress :deep(.n-step-splitor){background-color:rgb(var(--yb-text-inverse)/.28)}
.hero-progress :deep(.n-step--wait-status .n-step-content-header__title){color:rgb(var(--yb-text-inverse)/.72)}
.hero-progress :deep(.n-step--wait-status .n-step-indicator){background-color:rgb(var(--yb-text-inverse)/.12);border-color:rgb(var(--yb-text-inverse)/.4)}
.hero-progress :deep(.n-step--wait-status .n-step-indicator .n-step-indicator-slot__index){color:rgb(var(--yb-text-inverse)/.88)}
.command-strip,.brief-card,.resource-story{border:1px solid rgb(var(--yb-border));border-radius:18px;background:rgb(var(--yb-surface));box-shadow:0 12px 30px rgb(var(--yb-shadow)/.05)}
.card-actions{display:flex;align-items:center;gap:8px}.card-actions .upload-button{border-color:rgb(var(--yb-brand)/.55);color:rgb(var(--yb-brand-deep));background:rgb(var(--yb-brand-soft));font-weight:800}.card-actions .upload-button:disabled{opacity:.6;cursor:progress}
.command-strip{padding:13px 16px}.stage-summary{min-width:0;display:flex;align-items:center;gap:12px}.stage-copy{min-width:250px}.stage-summary h2,.brief-card h2,.resource-story h2,.workspace-head h2{margin:3px 0}.stage-summary p:last-child,.workspace-head p:last-child{margin:0;color:rgb(var(--yb-text-muted))}
.stage-orb{flex:0 0 auto;display:grid;place-items:center;width:38px;height:38px;border-radius:13px;background:radial-gradient(circle at 35% 30%,rgb(var(--yb-text-inverse)),rgb(var(--yb-brand-bright)) 22%,rgb(var(--yb-brand-deep)) 70%);box-shadow:0 8px 20px rgb(var(--yb-brand)/.24);color:rgb(var(--yb-text-inverse));font-weight:900}.stage-orb.is-complete{background:rgb(var(--yb-success-strong));box-shadow:none}.command-strip.is-complete{border-color:rgb(var(--yb-success-strong)/.4);background:linear-gradient(180deg,rgb(var(--yb-success-soft)/.5),rgb(var(--yb-surface)) 60%)}
.stage-context{display:grid;grid-template-columns:repeat(2,minmax(120px,1fr));gap:1px;margin:0;overflow:hidden;border:1px solid rgb(var(--yb-border));border-radius:11px;background:rgb(var(--yb-border))}.stage-context div{padding:8px 10px;background:rgb(var(--yb-surface-soft))}.stage-context dt{font-size:10px;color:rgb(var(--yb-text-muted))}.stage-context dd{max-width:180px;margin:2px 0 0;overflow:hidden;font-size:12px;font-weight:750;text-overflow:ellipsis;white-space:nowrap}
.primary-button,.secondary-button,.brief-card header button,.resource-story header button,.collaboration-actions button{min-height:38px;padding:0 14px;border-radius:10px;font-weight:750;text-decoration:none;cursor:pointer}
.primary-button{border:0;background:rgb(var(--yb-brand));color:rgb(var(--yb-text-inverse));box-shadow:0 8px 18px rgb(var(--yb-brand)/.16)}
.retouch-requirement-list{display:grid;gap:10px;margin:12px 0 0;padding:0;list-style:none}.retouch-requirement-list li{display:grid;grid-template-columns:30px 1fr;gap:10px;padding:12px;border:1px solid rgb(var(--yb-border));border-radius:12px;background:rgb(var(--yb-surface-soft))}.retouch-requirement-list li>span{display:grid;width:26px;height:26px;place-items:center;border-radius:9px;background:rgb(var(--yb-brand-soft));color:rgb(var(--yb-brand));font-weight:800}.retouch-requirement-list strong{display:block}.retouch-requirement-list p{margin:4px 0 0;color:rgb(var(--yb-text-muted))}
.secondary-button,.brief-card header button,.resource-story header button,.collaboration-actions button{border:1px solid rgb(var(--yb-border));background:rgb(var(--yb-surface));color:rgb(var(--yb-text))}
.overview-grid{display:grid;grid-template-columns:minmax(0,1.55fr) minmax(250px,.8fr) minmax(285px,1fr);align-items:start;gap:12px}.brief-card{min-width:0;padding:16px;display:grid;align-content:start;gap:12px}.brief-card header{align-items:start}.brief-card header button{min-height:30px;padding:0 9px}
.mission-copy{display:grid;grid-template-columns:1.15fr .85fr;gap:10px}.mission-copy>section,.mission-copy>aside{min-width:0;padding:12px;border-radius:12px;background:rgb(var(--yb-surface-soft))}.mission-copy>aside{border-left:3px solid rgb(var(--yb-brand));background:rgb(var(--yb-brand-soft))}.section-label{display:block;margin-bottom:5px;color:rgb(var(--yb-text-muted));font-size:10px;font-weight:800;letter-spacing:.08em}
.clamped-copy{display:-webkit-box;margin:0;overflow:hidden;color:rgb(var(--yb-text-body));line-height:1.65;-webkit-box-orient:vertical;-webkit-line-clamp:3}.detail-chips,.sku-list{display:flex;gap:6px;flex-wrap:wrap}.detail-chips span,.sku-list span{display:inline-flex;align-items:center;gap:6px;padding:5px 8px;border-radius:999px;background:rgb(var(--yb-surface-muted));font-size:11px}.sku-list em{padding:2px 6px;border-radius:999px;background:rgb(var(--yb-warning-soft));color:rgb(var(--yb-warning-strong));font-size:10px;font-style:normal;font-weight:750}
.reference-preview{display:grid;grid-template-columns:repeat(2,minmax(0,1fr));gap:7px}.reference-preview a{min-width:0;display:grid;grid-template-columns:44px 1fr;align-items:center;gap:7px;padding:6px;border:1px solid rgb(var(--yb-border));border-radius:10px;color:rgb(var(--yb-text));text-decoration:none}.reference-preview img,.file-glyph{width:44px;height:42px;border-radius:8px;object-fit:cover;background:rgb(var(--yb-surface-preview))}.file-glyph{display:grid;place-items:center;color:rgb(var(--yb-brand));font:700 10px var(--yb-font-data)}.reference-preview small{overflow:hidden;text-overflow:ellipsis;white-space:nowrap}
.assignment-snapshot{display:grid;grid-template-columns:42px 1fr;align-items:center;gap:10px;padding:10px;border-radius:12px;background:rgb(var(--yb-surface-soft))}.person-avatar{width:42px;height:42px;display:grid;place-items:center;border-radius:13px;background:rgb(var(--yb-text-night));color:rgb(var(--yb-text-inverse));font-weight:850}.assignment-snapshot small,.current-event small{color:rgb(var(--yb-text-muted));font-size:10px}.assignment-snapshot strong,.current-event strong{display:block;margin-top:2px}.assignment-snapshot p,.current-event p{margin:2px 0 0;color:rgb(var(--yb-text-muted));font-size:11px}
.current-event{display:grid;grid-template-columns:13px 1fr;gap:8px;padding-top:10px;border-top:1px solid rgb(var(--yb-border))}.collaboration-actions{display:flex;gap:7px;margin-top:auto}.collaboration-actions button{min-height:32px;flex:1;padding:0 8px;font-size:12px}
.event-dot{width:9px;height:9px;margin-top:5px;border:2px solid rgb(var(--yb-surface));border-radius:50%;background:rgb(var(--yb-brand));box-shadow:0 0 0 2px rgb(var(--yb-brand-soft))}.activity-list,.full-timeline{display:grid;gap:12px;margin:0;padding:0;list-style:none}.full-timeline li{display:grid;grid-template-columns:14px 1fr;gap:9px}.full-timeline p,.full-timeline small{margin:2px 0 0;color:rgb(var(--yb-text-muted));font-size:12px}.muted-copy{margin:0;color:rgb(var(--yb-text-muted))}
.resource-story{display:grid;grid-template-columns:minmax(260px,.7fr) 1.3fr;align-items:center;gap:16px;padding:13px 16px}.resource-story header{align-items:start}.resource-steps{display:grid;grid-template-columns:1fr auto 1fr auto 1fr;align-items:center;gap:9px}.resource-steps>div{display:grid;grid-template-columns:auto 1fr;gap:1px 8px;padding:10px;border-radius:11px;background:rgb(var(--yb-surface-soft))}.resource-steps span{grid-row:1/3;font:800 17px var(--yb-font-data);color:rgb(var(--yb-brand))}.resource-steps small{color:rgb(var(--yb-text-muted));font-size:11px}.resource-steps i{color:rgb(var(--yb-text-faint));font-style:normal}
.workspace-backdrop{position:fixed;inset:0;z-index:7400;padding:20px;background:rgb(var(--yb-overlay-night)/.56);backdrop-filter:blur(8px)}.workspace-dialog{width:min(1420px,100%);height:calc(100dvh - 40px);margin:0 auto;display:grid;grid-template-rows:auto minmax(0,1fr);overflow:hidden;border:1px solid rgb(var(--yb-border));border-radius:22px;background:rgb(var(--yb-surface));box-shadow:0 32px 90px rgb(var(--yb-shadow)/.3)}.workspace-dialog:focus{outline:none}.workspace-head{padding:16px 20px;border-bottom:1px solid rgb(var(--yb-border));background:rgb(var(--yb-surface-soft))}.close-button{width:40px;height:40px;border:1px solid rgb(var(--yb-border));border-radius:11px;background:rgb(var(--yb-surface));font-size:24px;cursor:pointer}.workspace-body{min-height:0;overflow:auto;padding:18px}.workflow-body{padding:12px}
.detail-sections{display:grid;grid-template-columns:repeat(2,minmax(0,1fr));grid-auto-rows:max-content;align-content:start;gap:13px}.detail-sections>section{padding:16px;border:1px solid rgb(var(--yb-border));border-radius:15px}.detail-summary-strip,.detail-requirement{grid-column:1/-1}.detail-sections>.detail-summary-strip{display:grid;grid-template-columns:repeat(5,minmax(0,1fr));gap:1px;padding:0;overflow:hidden;background:rgb(var(--yb-border))}.detail-summary-strip div{padding:13px;background:rgb(var(--yb-surface-soft))}.detail-summary-strip span{display:block;color:rgb(var(--yb-text-muted));font-size:10px}.detail-summary-strip strong{display:block;margin-top:3px}.detail-copy-grid{display:grid;grid-template-columns:1.15fr .85fr;gap:13px}.detail-copy-grid>aside{padding:13px;border-radius:12px;background:rgb(var(--yb-brand-soft))}.detail-sections h3{margin:9px 0 5px}.long-copy{margin:0;white-space:pre-wrap;color:rgb(var(--yb-text-body));line-height:1.75}.detail-list{display:grid;grid-template-columns:repeat(2,minmax(0,1fr));gap:1px;margin:10px 0 0;overflow:hidden;border:1px solid rgb(var(--yb-border));border-radius:11px;background:rgb(var(--yb-border))}.detail-list div{padding:10px;background:rgb(var(--yb-surface))}.detail-list dt{font-size:10px;color:rgb(var(--yb-text-muted))}.detail-list dd{margin:3px 0 0}
.reference-detail-grid{display:grid;grid-template-columns:repeat(2,minmax(0,1fr));gap:8px;margin-top:10px}.reference-detail-grid a{min-width:0;display:grid;grid-template-columns:48px 1fr;align-items:center;gap:8px;padding:8px;border:1px solid rgb(var(--yb-border));border-radius:11px;color:rgb(var(--yb-text));text-decoration:none}.reference-detail-grid img{width:48px;height:48px;border-radius:8px;object-fit:cover}.reference-detail-grid strong{overflow:hidden;text-overflow:ellipsis;white-space:nowrap}
.history-body{max-width:880px;width:100%;margin:0 auto}.history-now{display:grid;grid-template-columns:18px 1fr;gap:12px;padding:16px;border-radius:15px;background:rgb(var(--yb-brand-soft))}.history-now h3,.history-now p{margin:3px 0}.history-heading{display:flex;align-items:end;justify-content:space-between;margin:22px 0 16px}.history-heading h3{margin:3px 0 0}.history-heading span{color:rgb(var(--yb-text-muted));font-size:12px}.full-timeline{gap:0}.full-timeline li{position:relative;padding:0 0 22px}.full-timeline li:not(:last-child)::before{content:"";position:absolute;left:5px;top:14px;bottom:0;width:1px;background:rgb(var(--yb-border))}
.collaboration-body{display:grid;grid-template-columns:minmax(280px,.75fr) 1.25fr;gap:16px}.handover-form,.handover-list,.handover-side{display:grid;align-content:start;gap:12px}.handover-form{padding:18px;border:1px solid rgb(var(--yb-border));border-radius:16px}.handover-form label{display:grid;gap:6px;font-weight:700;font-size:13px}.handover-form input,.handover-form textarea,.handover-form select{min-height:40px;padding:9px 11px;border:1px solid rgb(var(--yb-border));border-radius:10px;background:rgb(var(--yb-surface));color:rgb(var(--yb-text));font-weight:500}
.event-count{margin-left:6px;padding:1px 7px;border-radius:999px;background:rgb(var(--yb-surface-muted));color:rgb(var(--yb-text-muted));font-size:11px;font-style:normal;font-weight:750}.handover-list article{display:flex;align-items:center;justify-content:space-between;gap:12px;padding:13px;border-radius:12px;background:rgb(var(--yb-surface-soft))}.handover-list p{margin:3px 0 0;color:rgb(var(--yb-text-muted))}.collaboration-fab{position:fixed;right:24px;bottom:24px;z-index:20;min-height:48px;padding:0 18px;border:0;border-radius:999px;background:rgb(var(--yb-text-night));color:rgb(var(--yb-text-inverse));box-shadow:0 14px 36px rgb(var(--yb-shadow)/.24);cursor:pointer}.collaboration-fab span{margin-left:8px;padding:2px 6px;border-radius:999px;background:rgb(var(--yb-brand))}.message,.state{padding:18px;border-radius:14px;background:rgb(var(--yb-surface-muted))}.error{background:rgb(var(--yb-danger-soft));color:rgb(var(--yb-danger-text))}
@media(max-width:1160px){.hero-main{align-items:start;flex-direction:column}.hero-facts{width:100%}.command-strip{align-items:flex-start;flex-direction:column}.stage-summary{width:100%;flex-wrap:wrap}.stage-context{width:100%;grid-template-columns:repeat(2,minmax(0,1fr))}.command-actions{width:100%;justify-content:flex-start;flex-wrap:wrap}.overview-grid{grid-template-columns:repeat(2,minmax(0,1fr))}.mission-card{grid-column:1/-1}.resource-story{grid-template-columns:1fr}}
@media(max-width:760px){.task-page{padding:10px}.task-hero{min-height:248px;border-radius:19px}.hero-content{min-height:248px;padding:12px}.hero-nav,.command-strip,.workspace-head{align-items:flex-start}.hero-actions{flex-wrap:wrap}.hero-main{gap:8px}.hero-main h1{font-size:27px;line-height:1.02}.hero-facts{grid-template-columns:repeat(2,minmax(0,1fr))}.hero-facts div{min-width:0;padding:7px}.hero-facts dd{overflow:hidden;font-size:11px;line-height:1.35;text-overflow:ellipsis}.overview-grid{grid-template-columns:1fr}.mission-card{grid-column:auto}.mission-copy,.detail-copy-grid{grid-template-columns:1fr}.command-strip{align-items:flex-start;flex-direction:column}.stage-summary{width:100%;display:grid;grid-template-columns:34px 1fr;align-items:start}.stage-orb{width:34px;height:34px}.stage-copy{min-width:0}.stage-context{grid-column:1/-1;width:100%;display:grid;grid-template-columns:repeat(2,minmax(0,1fr))}.stage-context dd{max-width:none}.command-actions{width:100%;display:grid;grid-template-columns:repeat(2,minmax(0,1fr));gap:8px}.command-actions>*{min-width:0}.command-actions>.primary-button{grid-column:1/-1}.resource-story{display:block}.resource-steps{grid-template-columns:1fr;margin-top:12px}.resource-steps i{transform:rotate(90deg);justify-self:center}.workspace-backdrop{padding:0}.workspace-dialog{height:100dvh;border:0;border-radius:0}.workspace-head{padding:13px}.workspace-body{padding:12px}.detail-sections,.collaboration-body{grid-template-columns:1fr}.detail-summary-strip,.detail-requirement{grid-column:auto}.detail-summary-strip{grid-template-columns:repeat(2,minmax(0,1fr))}.detail-list,.reference-detail-grid{grid-template-columns:1fr}.reference-preview{grid-template-columns:repeat(2,minmax(0,1fr))}.collaboration-fab{right:12px;bottom:12px}.hero-progress{overflow:auto}}
@media(max-width:420px){.reference-preview{grid-template-columns:1fr}.command-actions>*{flex-basis:100%}}
@media(prefers-reduced-motion:reduce){.workspace-backdrop{backdrop-filter:none}}
</style>

<style scoped>
.detail-sku-costs{display:grid;gap:.55rem}.detail-sku-costs article{display:grid;grid-template-columns:minmax(9rem,.85fr) minmax(0,2.2fr);gap:1rem;border:1px solid rgb(var(--yb-border));border-radius:.75rem;padding:.75rem;background:rgb(var(--yb-surface-soft))}.detail-sku-costs article>div{min-width:0;display:grid;align-content:center;gap:.2rem}.detail-sku-costs article>div strong{font:800 .8rem var(--yb-font-data);color:rgb(var(--yb-brand))}.detail-sku-costs article>div span{overflow:hidden;color:rgb(var(--yb-text-muted));font-size:.72rem;text-overflow:ellipsis;white-space:nowrap}.detail-sku-costs dl{display:grid;grid-template-columns:1.2fr .8fr .8fr 1.2fr;margin:0}.detail-sku-costs dl>div{min-width:0;display:grid;gap:.25rem;padding:0 .7rem;border-left:1px solid rgb(var(--yb-border))}.detail-sku-costs dt{color:rgb(var(--yb-text-muted));font-size:.65rem}.detail-sku-costs dd{margin:0;overflow:hidden;font-size:.72rem;text-overflow:ellipsis;white-space:nowrap}@media(max-width:760px){.detail-sku-costs article{grid-template-columns:1fr}.detail-sku-costs dl{grid-template-columns:1fr 1fr}.detail-sku-costs dl>div{padding:.45rem;border-left:0}.detail-sku-costs dl>div:nth-child(n+3){border-top:1px solid rgb(var(--yb-border))}}
</style>

<style scoped>
/* Premium task workstation visual system — restrained, legible and state-led. */
.task-page{max-width:none;margin:0;padding:0 0 30px;gap:14px}
.task-hero{min-height:184px;border:1px solid rgb(var(--yb-text-inverse)/.12);border-radius:18px;background:color-mix(in srgb,rgb(var(--yb-text-navy)) 80%,rgb(var(--yb-brand-accent)) 20%);box-shadow:0 12px 30px rgb(var(--yb-shadow)/.14),inset 0 1px rgb(var(--yb-text-inverse)/.045)}
.task-hero::after{position:absolute;inset:0;z-index:0;border-radius:inherit;background:rgb(var(--yb-text-inverse)/.012);content:"";pointer-events:none}
.task-hero.is-terminal::after{background:radial-gradient(circle at 91% 18%,rgb(var(--yb-success-bright)/.06),transparent 48%)}
.hero-content{min-height:184px;row-gap:12px;padding:16px 20px 14px}
.hero-nav{align-items:center}
.back-button,.hero-actions button{display:inline-flex;align-items:center;justify-content:center;gap:7px;min-height:36px;padding:0 12px;border-color:rgb(var(--yb-text-inverse)/.17);border-radius:9px;background:rgb(var(--yb-text-inverse)/.055);box-shadow:inset 0 1px rgb(var(--yb-text-inverse)/.04);font-size:12px;font-weight:750;backdrop-filter:blur(10px)}
.back-button:hover,.hero-actions button:hover{border-color:rgb(var(--yb-text-inverse)/.3);background:rgb(var(--yb-text-inverse)/.09)}
.refresh-button:disabled svg{animation:task-refresh-spin 1s linear infinite}
.hero-main{align-items:center;gap:28px}
.hero-identity{min-width:0;display:grid;align-content:center}
.hero-identity h1{max-width:780px;margin:0;font-size:clamp(29px,2.4vw,36px);font-weight:820;line-height:1.1;letter-spacing:-.025em}
.task-number{display:flex;align-items:center;gap:9px;margin:8px 0 0;color:rgb(var(--yb-text-inverse)/.74);font-size:12px;letter-spacing:.015em}
.task-number i{width:3px;height:3px;border-radius:50%;background:rgb(var(--yb-text-inverse)/.38)}
.identity-badges{display:flex;flex-wrap:wrap;gap:7px;margin-top:12px}
.identity-badge{display:inline-flex;align-items:center;gap:6px;min-height:28px;padding:0 9px;border:1px solid rgb(var(--yb-text-inverse)/.15);border-radius:8px;background:rgb(var(--yb-text-inverse)/.055);color:rgb(var(--yb-text-inverse)/.88);font-size:11px;font-weight:750;backdrop-filter:blur(8px)}
.identity-badge.lane{border-color:rgb(var(--yb-brand-accent)/.3);background:rgb(var(--yb-brand)/.13);color:rgb(var(--yb-text-inverse))}
.identity-badge.lane.customization{border-color:rgb(var(--yb-purple-border-strong)/.44);background:rgb(var(--yb-purple)/.14)}
.hero-facts{flex:0 0 min(460px,42vw);grid-template-columns:repeat(4,minmax(92px,1fr));gap:0;border-color:rgb(var(--yb-text-inverse)/.15);border-radius:11px;background:rgb(var(--yb-text-inverse)/.085);box-shadow:inset 0 1px rgb(var(--yb-text-inverse)/.04);backdrop-filter:blur(14px)}
.hero-facts div{padding:10px;background:transparent}.hero-facts div+div{border-left:1px solid rgb(var(--yb-text-inverse)/.1)}
.hero-facts dt{font-size:10px;font-weight:650;color:rgb(var(--yb-text-inverse)/.62)}
.hero-facts dd{font-size:12px;font-weight:760;color:rgb(var(--yb-text-inverse)/.96)}
.hero-progress{padding-top:12px;border-top-color:rgb(var(--yb-text-inverse)/.11)}
.command-strip,.brief-card,.resource-story{border-radius:15px;box-shadow:0 5px 18px rgb(var(--yb-shadow)/.045)}
.command-strip{min-height:78px;padding:12px 14px}
.stage-summary{gap:11px}
.stage-orb{width:40px;height:40px;border:1px solid rgb(var(--yb-brand-border));border-radius:12px;background:rgb(var(--yb-brand-soft));box-shadow:none;color:rgb(var(--yb-brand))}
.stage-orb.is-complete{border-color:rgb(var(--yb-success-border));background:rgb(var(--yb-success-soft));color:rgb(var(--yb-success-strong))}
.stage-label{margin:0;color:rgb(var(--yb-brand));font-size:10px;font-weight:850;letter-spacing:.08em}
.stage-copy h2{margin:2px 0;font-size:17px;line-height:1.25}
.stage-copy p:last-child{font-size:12px;line-height:1.45}
.stage-context{border-radius:9px}
.stage-context div{padding:7px 9px}
.command-actions{flex-wrap:wrap}
.primary-button,.secondary-button,.brief-card header button,.resource-story header button,.collaboration-actions button{display:inline-flex;align-items:center;justify-content:center;gap:7px;min-height:36px;border-radius:9px;font-size:12px;font-weight:720}
.primary-button{box-shadow:0 4px 12px rgb(var(--yb-brand)/.16)}
.overview-grid{grid-template-columns:minmax(0,1.2fr) minmax(300px,.95fr) minmax(285px,.82fr);gap:14px}
.brief-card{gap:13px;padding:15px}
.brief-card header{align-items:center}
.brief-card header button{min-height:32px;padding:0 9px}
.card-heading{display:flex;align-items:center;gap:9px;min-width:0}
.heading-icon{display:grid;flex:0 0 auto;width:32px;height:32px;place-items:center;border:1px solid rgb(var(--yb-border));border-radius:9px;background:rgb(var(--yb-surface-soft));color:rgb(var(--yb-brand))}
.card-heading h2{margin:0;font-size:15px;line-height:1.25}
.card-heading p{margin:3px 0 0;color:rgb(var(--yb-text-muted));font-size:10px}
.mission-copy{gap:9px}
.mission-copy>section,.mission-copy>aside{padding:11px;border-radius:10px}
.reference-preview a{border-radius:9px}
.assignment-snapshot{border-radius:10px}
.collaboration-actions button{min-height:31px}
.resource-story{padding:13px 15px}
.resource-steps>div{border:1px solid rgb(var(--yb-border));border-radius:10px;background:rgb(var(--yb-surface))}
.workspace-backdrop{padding:14px;background:rgb(var(--yb-overlay-night)/.68);backdrop-filter:blur(10px)}
.workspace-dialog{width:min(1540px,100%);height:calc(100dvh - 28px);border-radius:18px;box-shadow:0 34px 90px rgb(var(--yb-shadow)/.34)}
.workspace-head{padding:13px 16px;background:rgb(var(--yb-surface))}
.workflow-body{overflow:hidden}
.attachment-body{padding:0;overflow:hidden}
.reference-summary>div{display:grid;gap:7px;border:1px solid rgb(var(--yb-border));border-radius:11px;padding:13px;background:rgb(var(--yb-surface-soft))}.reference-summary strong{font-size:14px}.reference-summary p{margin:0;color:rgb(var(--yb-text-muted));font-size:11px;line-height:1.55}.reference-summary button{justify-self:start;min-height:34px;border:1px solid rgb(var(--yb-brand));border-radius:9px;padding:0 11px;background:rgb(var(--yb-brand));color:rgb(var(--yb-text-inverse));font-size:11px;font-weight:720;cursor:pointer}
.workspace-head h2{font-size:19px}
.close-button{display:grid;width:36px;height:36px;place-items:center;border-radius:9px;font-size:21px}
@media(max-width:1160px){.hero-facts{flex-basis:auto}.overview-grid{grid-template-columns:repeat(2,minmax(0,1fr))}.mission-card{grid-column:1/-1}}
@media(max-width:760px){.task-page{padding:0 0 22px;gap:10px}.task-hero{min-height:0;border-radius:14px}.hero-content{min-height:0;padding:12px}.hero-nav{align-items:center}.back-button span,.refresh-button span{display:none}.back-button,.refresh-button{width:36px;padding:0}.hero-main{gap:12px}.hero-identity h1{font-size:26px;line-height:1.12}.identity-badges{margin-top:10px}.identity-badge{min-height:26px;padding-inline:8px}.hero-facts{grid-template-columns:repeat(2,minmax(0,1fr))}.hero-progress{padding-bottom:2px;overflow:visible}.command-strip{padding:12px}.overview-grid{grid-template-columns:1fr;gap:10px}.mission-card{grid-column:auto}.card-actions{flex-wrap:wrap;justify-content:flex-end}.workspace-backdrop{padding:0}.workspace-dialog{height:100dvh;border-radius:0}.collaboration-fab{display:none}}
@media(prefers-reduced-motion:no-preference){@keyframes task-refresh-spin{to{transform:rotate(360deg)}}}
</style>
