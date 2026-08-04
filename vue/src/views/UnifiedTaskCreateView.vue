<template>
  <main class="compose-page" :data-row-count="rows.length" :data-compose-intent="intent">
    <header class="compose-hero">
      <div class="hero-copy">
        <p class="eyebrow">任务中心</p>
        <h1>创建任务</h1>
        <p>先选好要做什么，再像填 Excel 一样把内容填进表格，最后一次提交就完成。</p>
      </div>
      <div class="hero-actions">
        <button class="secondary-button" type="button" :disabled="savingDraft" @click="saveDraft">
          <Save :size="16" />{{ savingDraft ? '正在保存…' : '保存草稿' }}
        </button>
        <button class="secondary-button" type="button" @click="exitCompose"><X :size="16" />返回任务中心</button>
      </div>
    </header>

    <section class="intent-grid" aria-label="选择创建意图">
      <button
        v-for="option in intentOptions"
        :key="option.value"
        type="button"
        class="intent-card"
        :class="{ 'is-active': intent === option.value }"
        :aria-pressed="intent === option.value"
        @click="selectIntent(option.value)"
      >
        <span class="intent-icon"><component :is="option.icon" :size="22" /></span>
        <span class="intent-copy"><strong>{{ option.title }}</strong><small>{{ option.summary }}</small></span>
        <span class="intent-badge">{{ option.badge }}</span>
        <CheckCircle2 v-if="intent === option.value" class="intent-check" :size="19" />
      </button>
    </section>

    <section class="common-ribbon" aria-label="任务公共信息">
      <label v-if="intent !== 'planning_sku'" class="field">
        <span class="field-name">截止时间</span>
        <input v-model="common.due_at" type="datetime-local" />
      </label>
      <label class="field">
        <span class="field-name">优先级</span>
        <select v-model="common.priority"><option value="low">低</option><option value="normal">普通</option><option value="high">高</option><option value="critical">紧急</option></select>
      </label>
      <div v-if="intent === 'new_design' || intent === 'modify_existing'" class="field">
        <span class="field-name">任务类别</span>
        <div class="lane-toggle" role="group" aria-label="选择常规或定制">
          <button type="button" :class="{ active: !common.customization_required }" :aria-pressed="!common.customization_required" @click="setLane(false)">常规</button>
          <button type="button" :class="{ active: common.customization_required }" :aria-pressed="common.customization_required" @click="setLane(true)">定制</button>
        </div>
        <small>{{ common.customization_required ? '客户定制需求，走定制流程' : '日常上新，走常规流程' }}</small>
      </div>
      <label v-if="intent === 'new_design' || intent === 'planning_sku'" class="field">
        <span class="field-name">同步到 ERP</span>
        <span class="switch-row"><input v-model="erpSync" type="checkbox" /><small>{{ erpSync ? '创建后自动同步' : '本次不同步' }}</small></span>
      </label>
      <label class="field note-field">
        <span class="field-name">备注（选填）</span>
        <input v-model.trim="common.note" maxlength="2000" placeholder="想交代给设计、审核同事的话写在这里" />
      </label>
    </section>

    <section v-if="result" class="result-board">
      <div class="result-heading">
        <div><p class="eyebrow">创建结果</p><h2>{{ resultTitle }}</h2><p>{{ resultSummary }}</p></div>
        <div class="result-heading-actions">
          <button class="secondary-button" type="button" @click="router.push('/tasks')">返回任务中心</button>
          <button class="primary-button" type="button" @click="startAnother">继续创建</button>
        </div>
      </div>
      <template v-if="planningResult">
        <div class="result-actions">
          <button class="secondary-button" @click="copyPlanningAll">复制全部 SKU</button>
          <button class="secondary-button" :disabled="!failedPlanningItems.length || resultBusy" @click="retryPlanningERP">重试失败项（{{ failedPlanningItems.length }}）</button>
          <button class="secondary-button" :disabled="!selectedPlanningIds.size || resultBusy" @click="exportPlanningSelection">导出已勾选（{{ selectedPlanningIds.size }}）</button>
          <a class="primary-button" :href="planningSkuApi.exportTaskURL(planningResult.task_id)">导出全部</a>
        </div>
        <div class="result-filter"><button :class="{ active: resultFilter === 'all' }" @click="resultFilter = 'all'">全部 {{ planningResult.items.length }}</button><button :class="{ active: resultFilter === 'failed' }" @click="resultFilter = 'failed'">失败 {{ failedPlanningItems.length }}</button></div>
        <div class="sku-result-grid">
          <label v-for="item in visiblePlanningItems" :key="item.task_sku_item_id" class="sku-result" :class="{ failed: planningFailed(item.erp_status) }">
            <input v-model="selectedPlanningIds" type="checkbox" :value="item.task_sku_item_id" />
            <span>#{{ item.sequence_no }}</span><strong>{{ item.sku_code }}</strong><small>{{ item.erp_status || '未同步' }}</small>
          </label>
        </div>
      </template>
      <div v-else class="task-result-grid">
        <article v-for="row in rows" :key="row.id" :class="['task-result', `is-${row.status || 'draft'}`]">
          <CheckCircle2 v-if="row.status === 'created'" :size="18" /><AlertTriangle v-else :size="18" />
          <div><strong>{{ row.product_name || row.description_spec || row.design_requirement || '任务明细' }}</strong><small>{{ row.status === 'created' ? `任务 #${row.result_task_id}` : row.error }}</small></div>
          <RouterLink v-if="row.result_task_id" :to="`/tasks/${row.result_task_id}`">查看</RouterLink>
        </article>
        <button v-if="failedRows.length" class="secondary-button retry-failed" type="button" @click="submit(true)">{{ intent === 'retouch' ? '只重试失败附件' : '只重试失败行' }}</button>
      </div>
    </section>

    <section v-else class="workspace-card">
      <header class="workspace-toolbar">
        <div><h2>{{ currentMeta.title }}明细</h2><p>{{ rows.length }} 行 · {{ rowCountHint }}</p></div>
        <div class="toolbar-actions">
          <label v-if="intent === 'planning_sku' || intent === 'new_design'" class="secondary-button file-button">{{ intent === 'planning_sku' ? '导入策划 Excel' : '导入新款 Excel' }}<input type="file" accept=".xlsx,.xls,.csv" @change="importComposeExcel" /></label>
          <button v-if="intent === 'modify_existing'" class="secondary-button" type="button" :disabled="batchERPResolving" @click="resolveCurrentERPRows">
            <PackageSearch :size="16" />{{ batchERPResolving ? '正在批量查询…' : '批量查询已填 SKU' }}
          </button>
          <button class="secondary-button" type="button" :disabled="rows.length >= maxRows" @click="addRow"><Plus :size="16" />添加一行</button>
          <button class="secondary-button" type="button" :disabled="!selectedRowIds.length || rows.length === 1" @click="removeSelectedRows"><Trash2 :size="16" />删除选中行<span v-if="selectedRowIds.length > 1">（{{ selectedRowIds.length }}）</span></button>
        </div>
        <p v-if="batchERPFeedback" class="toolbar-feedback" role="status">{{ batchERPFeedback }}</p>
      </header>

      <div class="workspace-layout" :class="{ 'has-drawer': Boolean(selectedRow) }">
        <div class="grid-column">
          <UnifiedTaskGrid
            ref="gridRef"
            :intent="intent"
            :rows="rows"
            :revision="gridRevision"
            :violations="violations"
            @update:rows="replaceRows"
            @select="selectRow"
            @selection="selectRows"
            @files="addFiles"
          />

          <div class="mobile-row-list" aria-label="移动端任务明细">
            <article v-for="(row, index) in rows" :key="row.id" class="mobile-row-card" :class="{ selected: selectedRowId === row.id }" data-testid="compose-row" :data-row-index="index" @click="selectRow(row.id)">
              <header><span>第 {{ index + 1 }} 行</span><strong>{{ mobileRowTitle(row) }}</strong><button type="button" :disabled="rows.length === 1" :aria-label="`删除第 ${index + 1} 行`" @click.stop="removeRow(row.id)"><Trash2 :size="15" /></button></header>
              <div class="mobile-fields">
                <label v-for="column in editableTextColumns" :key="column.key">{{ column.label }}<textarea v-if="longTextColumn(column.key)" :value="String(row[column.key as keyof ComposeRow] || '')" rows="2" @input="updateRowField(row.id, column.key, ($event.target as HTMLTextAreaElement).value)" /><input v-else :type="column.kind === 'number' ? 'number' : 'text'" :value="String(row[column.key as keyof ComposeRow] || '')" @input="updateRowField(row.id, column.key, ($event.target as HTMLInputElement).value)" /></label>
                <label v-if="showSetHint" class="mobile-switch">建议做成套装<input v-model="row.set_mode_hint" type="checkbox" /></label>
                <button class="asset-button" type="button" @click.stop="openFilePicker(row.id, 'reference_assets')"><ImagePlus :size="16" />参考图 {{ row.reference_assets.length ? `(${row.reference_assets.length})` : '' }}</button>
                <button v-if="intent === 'retouch'" class="asset-button" type="button" @click.stop="openFilePicker(row.id, 'source_assets')"><Paperclip :size="16" />待修素材 {{ row.source_assets.length ? `(${row.source_assets.length})` : '' }}</button>
              </div>
            </article>
          </div>
        </div>

        <aside v-if="selectedRow" class="row-drawer" aria-label="当前行详情">
          <header><div><p class="eyebrow">当前编辑</p><h3>第 {{ selectedRowIndex + 1 }} 行</h3></div><button type="button" aria-label="关闭行详情" @click="selectedRowId = ''"><X :size="17" /></button></header>
          <section v-if="intent === 'modify_existing'" class="drawer-section">
            <h4>ERP 商品</h4>
            <div class="erp-search"><input v-model.trim="erpSearchCode" placeholder="输入 SKU / 商品编码" @keyup.enter="searchERP" /><button type="button" :disabled="erpSearching" @click="searchERP">{{ erpSearching ? '查询中' : '查询' }}</button></div>
            <button v-for="item in erpSearchResults" :key="String(item.product_id || item.sku_code)" type="button" class="erp-result" @click="chooseERP(item)"><strong>{{ item.product_name || item.name || '未命名商品' }}</strong><span>{{ item.sku_code || item.sku || item.product_code || item.product_id }}</span></button>
            <p v-if="selectedRow.erp_sku" class="selected-erp">已选择：{{ selectedRow.product_name }} · {{ selectedRow.erp_sku }}</p>
          </section>
          <section v-if="intent === 'new_design' || (intent === 'planning_sku' && erpSync)" class="drawer-section"><IIdSelector :model-value="selectedRow.product_i_id" label="款式编码 i_id" @update:model-value="updateSelected('product_i_id', $event)" /></section>
          <section v-if="showSetHint" class="drawer-section hint-section">
            <div><h4>建议做成套装</h4><p>给设计师的参考：最终做单图还是套装，由设计师在设计时决定。</p></div>
            <input v-model="selectedRow.set_mode_hint" type="checkbox" aria-label="建议按套装设计" />
          </section>
          <section class="drawer-section"><h4>{{ intent === 'planning_sku' ? '产品图片' : '参考图' }}</h4><div class="asset-list"><article v-for="asset in selectedRow.reference_assets" :key="asset.id"><img v-if="asset.preview_url" :src="asset.preview_url" alt="" /><FileImage v-else :size="24" /><div><strong>{{ asset.name }}</strong><span>{{ assetStatusText(asset.status) }}</span></div><button type="button" aria-label="移除文件" @click="removeAsset(selectedRow.id, 'reference_assets', asset.id)"><X :size="14" /></button></article></div><button class="asset-button" type="button" @click="openFilePicker(selectedRow.id, 'reference_assets')"><ImagePlus :size="16" />添加{{ intent === 'planning_sku' ? '产品图片' : '参考图' }}</button></section>
          <section v-if="intent === 'retouch'" class="drawer-section"><h4>待修素材</h4><div class="asset-list"><article v-for="asset in selectedRow.source_assets" :key="asset.id"><FileArchive :size="24" /><div><strong>{{ asset.name }}</strong><span>{{ asset.error || assetStatusText(asset.status) }}</span></div><button type="button" @click="removeAsset(selectedRow.id, 'source_assets', asset.id)"><X :size="14" /></button></article></div><button class="asset-button" type="button" @click="openFilePicker(selectedRow.id, 'source_assets')"><Paperclip :size="16" />添加 PSD / AI / ZIP 等素材</button></section>
          <section class="drawer-section"><h4>本行提示</h4><ul v-if="selectedRowViolations.length" class="drawer-errors"><li v-for="issue in selectedRowViolations" :key="`${issue.field}-${issue.message}`">{{ issue.message }}</li></ul><p v-else class="drawer-ok"><CheckCircle2 :size="15" />本行信息已完整</p></section>
        </aside>
      </div>

      <footer class="validation-dock">
        <div class="validation-summary" :class="{ valid: !violations.length }"><CheckCircle2 v-if="!violations.length" :size="22" /><AlertTriangle v-else :size="22" /><div><strong>{{ violations.length ? `还有 ${violations.length} 处需要完善` : '内容都填好了，可以提交' }}</strong><span>{{ violations.length ? '点一下红色提示，会直接跳到要改的格子' : '提交后系统还会再整体核对一遍' }}</span></div></div>
        <div v-if="violations.length" class="validation-items"><button v-for="issue in violations.slice(0, 4)" :key="`${issue.row_id}-${issue.field}-${issue.message}`" type="button" @click="locateViolation(issue)"><span>{{ issue.row_index == null ? '公共信息' : `第 ${issue.row_index + 1} 行` }}</span>{{ issue.message }}</button></div>
        <div class="dock-actions"><p v-if="submitError" role="alert">{{ submitError }}</p><button class="primary-button" type="button" :disabled="submitting || validatingIIDs || Boolean(violations.length)" @click="submit(false)">{{ validatingIIDs ? '正在核对款式编码…' : submitting ? '正在创建…' : submitLabel }}</button></div>
      </footer>
    </section>

    <input ref="referenceInput" class="sr-only" type="file" accept="image/*" multiple aria-label="上传参考图或产品图片" @change="handleFileInput($event, 'reference_assets')" />
    <input ref="sourceInput" class="sr-only" type="file" multiple aria-label="上传待修素材文件" @change="handleFileInput($event, 'source_assets')" />

    <Teleport to="body">
      <div v-if="confirmState.open" class="compose-confirm-backdrop" @mousedown.self="resolveConfirm(false)">
        <section ref="confirmDialog" class="compose-confirm" role="alertdialog" aria-modal="true" :aria-label="confirmState.title" tabindex="-1" @keydown="handleConfirmKeydown">
          <span class="confirm-icon"><AlertTriangle :size="22" /></span>
          <h3>{{ confirmState.title }}</h3>
          <p>{{ confirmState.message }}</p>
          <footer>
            <button ref="confirmCancelButton" class="secondary-button" type="button" @click="resolveConfirm(false)">先不了</button>
            <button class="primary-button danger" type="button" @click="resolveConfirm(true)">{{ confirmState.confirmLabel }}</button>
          </footer>
        </section>
      </div>
    </Teleport>
  </main>
</template>

<script setup lang="ts">
import { computed, markRaw, nextTick, onBeforeUnmount, onMounted, reactive, ref, watch } from 'vue'
import { onBeforeRouteLeave, useRoute, useRouter } from 'vue-router'
import { AlertTriangle, Barcode, CheckCircle2, FileArchive, FileImage, ImagePlus, PackageSearch, Paintbrush, Paperclip, Plus, Save, Scissors, Trash2, X } from 'lucide-vue-next'

import UnifiedTaskGrid from '@/components/task-create/UnifiedTaskGrid.vue'
import IIdSelector from '@/components/task-create/IIdSelector.vue'
import {
  applyBackendViolations,
  buildPlanningInputs,
  buildTaskSubmissionUnits,
  COMPOSE_INTENT_META,
  composeColumns,
  createComposeRow,
  validateCompose,
  type ComposeAssetDraft,
  type ComposeColumnKey,
  type ComposeCommonInfo,
  type ComposeIntent,
  type ComposeRow,
  type ComposeViolation,
} from '@/domain/unified-task-compose'
import { useTaskDraft } from '@/composables/useTaskDraft'
import { useTasksStore } from '@/stores/tasks'
import { erpApi } from '@/services/api/erpApi'
import { planningSkuApi, type PlanningSKUCreateResult } from '@/services/api/planningSkuApi'
import { batchSkuApi, formatBatchViolationMessage, normalizeBatchPreviewRow, type BatchPreviewRow, type BatchViolation } from '@/services/api/batchSkuApi'
import { uploadReferenceFileRef } from '@/services/upload/assetUploadFlow'
import { uploadRetouchRequirementPendingAssets } from '@/services/upload/retouchRequirementUpload'
import { usePermission } from '@/composables/usePermission'
import { generateActionId } from '@/utils/uuid'

const route = useRoute()
const router = useRouter()
const tasksStore = useTasksStore()
const { can } = usePermission()
const { save: saveTaskDraft, update: updateTaskDraft, getById: getTaskDraft, saving: savingDraft } = useTaskDraft()

const allIntentOptions = [
  { value: 'modify_existing' as const, ...COMPOSE_INTENT_META.modify_existing, icon: markRaw(PackageSearch) },
  { value: 'new_design' as const, ...COMPOSE_INTENT_META.new_design, icon: markRaw(Paintbrush) },
  { value: 'retouch' as const, ...COMPOSE_INTENT_META.retouch, icon: markRaw(Scissors) },
  { value: 'planning_sku' as const, ...COMPOSE_INTENT_META.planning_sku, icon: markRaw(Barcode) },
]
const intentOptions = computed(() => allIntentOptions.filter((option) => option.value === 'planning_sku' ? can('planning_sku.create') : can('task.create')))

function initialIntent(): ComposeIntent {
  const query = String(route.query.intent || '').trim()
  if (intentOptions.value.some((item) => item.value === query)) return query as ComposeIntent
  return intentOptions.value[0]?.value ?? 'new_design'
}

function defaultDueAt(): string {
  const date = new Date(Date.now() + 24 * 60 * 60 * 1000)
  date.setMinutes(0, 0, 0)
  return new Date(date.getTime() - date.getTimezoneOffset() * 60_000).toISOString().slice(0, 16)
}

const intent = ref<ComposeIntent>(initialIntent())
const common = reactive<ComposeCommonInfo>({ due_at: defaultDueAt(), priority: 'normal', note: '', customization_required: false, customization_source_type: undefined, erp_sync_mode: 'none' })
const rows = ref<ComposeRow[]>([createComposeRow()])
const selectedRowId = ref(rows.value[0].id)
const selectedRowIds = ref<string[]>([rows.value[0].id])
const gridRevision = ref(0)
const gridRef = ref<InstanceType<typeof UnifiedTaskGrid> | null>(null)
const referenceInput = ref<HTMLInputElement | null>(null)
const sourceInput = ref<HTMLInputElement | null>(null)
const confirmDialog = ref<HTMLElement | null>(null)
const confirmCancelButton = ref<HTMLButtonElement | null>(null)
const pendingFileRowId = ref('')
const submitting = ref(false)
const validatingIIDs = ref(false)
const remoteViolations = ref<ComposeViolation[]>([])
const submitError = ref('')
const result = ref(false)
const planningResult = ref<PlanningSKUCreateResult | null>(null)
const selectedPlanningIds = ref<Set<number>>(new Set())
const resultFilter = ref<'all' | 'failed'>('all')
const resultBusy = ref(false)
const dirty = ref(false)
const hydrating = ref(true)
const currentDraftId = ref(String(route.query.draft_id || ''))
const clientCreateId = ref<string>(generateActionId())
const erpSearchCode = ref('')
const erpSearching = ref(false)
const erpSearchResults = ref<Array<Record<string, unknown>>>([])
const batchERPResolving = ref(false)
const batchERPFeedback = ref('')

const currentMeta = computed(() => COMPOSE_INTENT_META[intent.value])
const columns = computed(() => composeColumns(intent.value))
const editableTextColumns = computed(() => columns.value.filter((column) => column.kind !== 'asset' && column.kind !== 'boolean' && column.key !== 'erp_sku'))
const showSetHint = computed(() => intent.value === 'new_design' || intent.value === 'modify_existing')
const maxRows = computed(() => intent.value === 'planning_sku' ? 200 : intent.value === 'modify_existing' ? 50 : 100)
const rowCountHint = computed(() => intent.value === 'modify_existing' ? '每一行都会变成一张单独的任务单' : intent.value === 'new_design' && rows.value.length > 1 ? '这几行会合成一张批量任务单' : intent.value === 'retouch' ? '这些修图要求会放进同一张任务单' : intent.value === 'planning_sku' ? '提交后马上拿到全部编码' : '这一行会生成一个新 SKU 的设计任务')
const erpSync = computed({ get: () => common.erp_sync_mode === 'async', set: (value: boolean) => { common.erp_sync_mode = value ? 'async' : 'none' } })
const violations = computed(() => [...validateCompose(intent.value, common, rows.value), ...remoteViolations.value])
const selectedRow = computed(() => rows.value.find((row) => row.id === selectedRowId.value))
const selectedRowIndex = computed(() => Math.max(0, rows.value.findIndex((row) => row.id === selectedRowId.value)))
const selectedRowViolations = computed(() => violations.value.filter((issue) => issue.row_id === selectedRowId.value))
const failedRows = computed(() => rows.value.filter((row) => row.status === 'failed'))
const submitLabel = computed(() => intent.value === 'planning_sku' ? `生成 ${rows.value.length} 个 SKU 并结单` : intent.value === 'modify_existing' ? `创建 ${rows.value.length} 张任务单` : '创建任务')
const failedPlanningItems = computed(() => planningResult.value?.items.filter((item) => planningFailed(item.erp_status)) ?? [])
const visiblePlanningItems = computed(() => planningResult.value?.items.filter((item) => resultFilter.value === 'all' || planningFailed(item.erp_status)) ?? [])
const resultTitle = computed(() => {
  if (planningResult.value) return planningResult.value.task_no
  if (intent.value === 'retouch' && failedRows.value.some((row) => row.result_task_id)) return '任务已创建，部分附件上传失败'
  return failedRows.value.length ? '部分任务创建失败' : '任务创建完成'
})
const resultSummary = computed(() => {
  if (planningResult.value) return `已生成 ${planningResult.value.items.length} 个 SKU，任务已经结单。`
  if (intent.value === 'retouch' && failedRows.value.some((row) => row.result_task_id)) return '任务不会重复创建；可在下方安全重试尚未上传成功的附件。'
  return `${rows.value.filter((row) => row.status === 'created').length} 行创建成功，${failedRows.value.length} 行失败。`
})

watch([intent, common, rows], () => {
  remoteViolations.value = []
  if (!hydrating.value) dirty.value = true
}, { deep: true })

onMounted(async () => {
  if (currentDraftId.value) await restoreDraft(currentDraftId.value)
  hydrating.value = false
})
onBeforeUnmount(() => rows.value.flatMap((row) => [...row.reference_assets, ...row.source_assets]).forEach((asset) => { if (asset.preview_url?.startsWith('blob:')) URL.revokeObjectURL(asset.preview_url) }))
onBeforeRouteLeave(async () => !dirty.value || await askConfirm('离开创建页？', '当前填写的内容还没有提交，也没有保存草稿，离开后会丢失。', '确定离开'))

const confirmState = reactive({ open: false, title: '', message: '', confirmLabel: '确定' })
let confirmResolver: ((value: boolean) => void) | null = null
let confirmReturnFocus: HTMLElement | null = null
function askConfirm(title: string, message: string, confirmLabel = '确定'): Promise<boolean> {
  confirmResolver?.(false)
  confirmReturnFocus = document.activeElement instanceof HTMLElement ? document.activeElement : null
  confirmState.open = true
  confirmState.title = title
  confirmState.message = message
  confirmState.confirmLabel = confirmLabel
  void nextTick(() => confirmCancelButton.value?.focus())
  return new Promise((resolve) => { confirmResolver = resolve })
}
function resolveConfirm(value: boolean) {
  confirmState.open = false
  confirmResolver?.(value)
  confirmResolver = null
  void nextTick(() => confirmReturnFocus?.focus())
}
function handleConfirmKeydown(event: KeyboardEvent) {
  if (event.key === 'Escape') {
    event.preventDefault()
    resolveConfirm(false)
    return
  }
  if (event.key !== 'Tab' || !confirmDialog.value) return
  const focusable = Array.from(confirmDialog.value.querySelectorAll<HTMLElement>('button:not([disabled]),[tabindex]:not([tabindex="-1"])'))
    .filter((element) => element.getClientRects().length > 0)
  if (!focusable.length) { event.preventDefault(); confirmDialog.value.focus(); return }
  const first = focusable[0]
  const last = focusable[focusable.length - 1]
  if (event.shiftKey && document.activeElement === first) { event.preventDefault(); last.focus() }
  else if (!event.shiftKey && document.activeElement === last) { event.preventDefault(); first.focus() }
}

function exitCompose() { void router.push('/tasks') }

async function selectIntent(next: ComposeIntent) {
  if (next === intent.value) return
  if (dirty.value && !(await askConfirm('切换创建类型？', '切换后当前已填写的行会被清空，可先点右上角「保存草稿」留底。', '清空并切换'))) return
  intent.value = next
  resetComposeState()
  void router.replace({ query: { ...route.query, intent: next } })
}

function resetComposeState() {
  common.due_at = defaultDueAt()
  common.priority = 'normal'
  common.note = ''
  common.customization_required = false
  common.customization_source_type = undefined
  common.erp_sync_mode = 'none'
  common.designer_id = undefined
  rows.value = [createComposeRow()]
  selectRow(rows.value[0].id)
  remoteViolations.value = []
  submitError.value = ''
  batchERPFeedback.value = ''
  erpSearchCode.value = ''
  erpSearchResults.value = []
  result.value = false
  planningResult.value = null
  selectedPlanningIds.value = new Set()
  resultFilter.value = 'all'
  clientCreateId.value = generateActionId()
  gridRevision.value += 1
  dirty.value = false
}

function syncCustomizationSource() {
  common.customization_source_type = common.customization_required ? (intent.value === 'modify_existing' ? 'existing_product' : 'new_product') : undefined
}
function setLane(customized: boolean) {
  common.customization_required = customized
  syncCustomizationSource()
}

function addRow() {
  if (rows.value.length >= maxRows.value) return
  const row = createComposeRow()
  rows.value.push(row)
  selectRow(row.id)
  gridRevision.value += 1
}
function removeRow(rowId: string) {
  if (rows.value.length === 1) return
  const index = rows.value.findIndex((row) => row.id === rowId)
  rows.value = rows.value.filter((row) => row.id !== rowId)
  const nextRowId = rows.value[Math.min(index, rows.value.length - 1)]?.id || ''
  if (nextRowId) selectRow(nextRowId)
  else {
    selectedRowId.value = ''
    selectedRowIds.value = []
  }
  gridRevision.value += 1
}
function removeSelectedRows() {
  const selected = new Set(selectedRowIds.value.length ? selectedRowIds.value : [selectedRowId.value].filter(Boolean))
  if (!selected.size || rows.value.length === 1) return
  const firstSelectedIndex = rows.value.findIndex((row) => selected.has(row.id))
  const remaining = rows.value.filter((row) => !selected.has(row.id))
  rows.value = remaining.length ? remaining : [createComposeRow()]
  selectRow(rows.value[Math.min(Math.max(0, firstSelectedIndex), rows.value.length - 1)].id)
  gridRevision.value += 1
}
function selectRows(rowIds: string[]) {
  const valid = [...new Set(rowIds)].filter((rowId) => rows.value.some((row) => row.id === rowId))
  selectedRowIds.value = valid
  selectedRowId.value = valid[valid.length - 1] || ''
}
function selectRow(rowId: string) {
  selectedRowId.value = rowId
  selectedRowIds.value = rowId ? [rowId] : []
}
function replaceRows(next: ComposeRow[]) {
  rows.value = next
  const validSelection = selectedRowIds.value.filter((rowId) => next.some((row) => row.id === rowId))
  if (validSelection.length) selectRows(validSelection)
  else if (next[0]) selectRow(next[0].id)
}
function longTextColumn(key: ComposeColumnKey) { return key === 'design_requirement' || key === 'description_spec' || key === 'note' || key === 'special_note' }
function updateRowField(rowId: string, key: ComposeColumnKey, value: string) {
  const row = rows.value.find((item) => item.id === rowId)
  if (!row) return
  if (['quantity', 'width', 'height', 'area'].includes(key)) (row as Record<string, unknown>)[key] = value.trim() ? Number(value) : undefined
  else (row as Record<string, unknown>)[key] = value
}
function updateSelected(key: ComposeColumnKey, value: unknown) {
  if (!selectedRow.value) return
  ;(selectedRow.value as Record<string, unknown>)[key] = value
  // The drawer edits the row model outside Univer. Rebuild only for this
  // explicit parent-side change so the selected i_id is visible in the grid.
  gridRevision.value += 1
}
function mobileRowTitle(row: ComposeRow) { return row.product_name || row.description_spec || row.design_requirement || row.erp_sku || '待完善' }

function openFilePicker(rowId: string, field: 'reference_assets' | 'source_assets') {
  pendingFileRowId.value = rowId
  ;(field === 'source_assets' ? sourceInput.value : referenceInput.value)?.click()
}
function handleFileInput(event: Event, field: 'reference_assets' | 'source_assets') {
  const input = event.target as HTMLInputElement
  const files = Array.from(input.files ?? [])
  if (pendingFileRowId.value && files.length) void addFiles({ rowId: pendingFileRowId.value, field, files })
  input.value = ''
}

async function addFiles(payload: { rowId: string; field: 'reference_assets' | 'source_assets'; files: File[] }) {
  const row = rows.value.find((item) => item.id === payload.rowId)
  if (!row) return
  const limit = intent.value === 'planning_sku' ? 1 : 5
  const remaining = Math.max(0, limit - row[payload.field].length)
  const drafts = payload.files.slice(0, remaining).map<ComposeAssetDraft>((file) => ({ id: generateActionId(), file, name: file.name, preview_url: file.type.startsWith('image/') ? URL.createObjectURL(file) : undefined, status: intent.value === 'retouch' && payload.field === 'source_assets' ? 'local' : 'uploading' }))
  row[payload.field].push(...drafts)
  if (intent.value === 'retouch') {
    drafts.forEach((draft) => patchAssetDraft(payload.rowId, payload.field, draft.id, { status: 'local' }))
    gridRevision.value += 1
    return
  }
  for (const draft of drafts) {
    try {
      if (!draft.file) continue
      const uploadRef = intent.value === 'planning_sku'
        ? await planningSkuApi.uploadImage(draft.file, clientCreateId.value, row.id)
        : await uploadReferenceFileRef(draft.file)
      patchAssetDraft(payload.rowId, payload.field, draft.id, { upload_ref: uploadRef, status: 'uploaded' })
    } catch (error) {
      patchAssetDraft(payload.rowId, payload.field, draft.id, {
        status: 'failed',
        error: error instanceof Error ? error.message : '上传失败',
      })
    }
  }
  gridRevision.value += 1
}

function patchAssetDraft(
  rowId: string,
  field: 'reference_assets' | 'source_assets',
  assetId: string,
  patch: Partial<ComposeAssetDraft>,
) {
  const currentRow = rows.value.find((item) => item.id === rowId)
  if (!currentRow) return
  const index = currentRow[field].findIndex((item) => item.id === assetId)
  if (index < 0) return
  currentRow[field][index] = { ...currentRow[field][index], ...patch }
}

function removeAsset(rowId: string, field: 'reference_assets' | 'source_assets', assetId: string) {
  const row = rows.value.find((item) => item.id === rowId)
  if (!row) return
  const asset = row[field].find((item) => item.id === assetId)
  if (asset?.preview_url?.startsWith('blob:')) URL.revokeObjectURL(asset.preview_url)
  row[field] = row[field].filter((item) => item.id !== assetId)
  gridRevision.value += 1
}
function assetStatusText(status: ComposeAssetDraft['status']) { return ({ local: '将在创建后上传', uploading: '正在上传', uploaded: '已上传', failed: '上传失败' })[status] }

async function searchERP() {
  if (!erpSearchCode.value) return
  erpSearching.value = true
  submitError.value = ''
  erpSearchResults.value = []
  try {
    const response = await erpApi.getProductByCode(erpSearchCode.value)
    erpSearchResults.value = normalizeERPProducts(response.data)
  } catch (error) {
    submitError.value = error instanceof Error ? error.message : 'ERP 商品查询失败'
  } finally { erpSearching.value = false }
}
function chooseERP(item: Record<string, unknown>) {
  if (!selectedRow.value) return
  applyERPProduct(selectedRow.value, item)
  erpSearchResults.value = []
  gridRevision.value += 1
}

function normalizeERPProducts(raw: unknown): Array<Record<string, unknown>> {
  const root = raw && typeof raw === 'object' ? raw as Record<string, unknown> : {}
  const data = root.data && typeof root.data === 'object' ? root.data : root
  if (Array.isArray(data)) return data as Array<Record<string, unknown>>
  if (!data || typeof data !== 'object') return []
  const record = data as Record<string, unknown>
  if (Array.isArray(record.items)) return record.items as Array<Record<string, unknown>>
  const snapshot = record.snapshot && typeof record.snapshot === 'object'
    ? record.snapshot as Record<string, unknown>
    : {}
  const product = {
    ...snapshot,
    product_id: snapshot.product_id ?? record.product_id ?? record.code,
    sku_code: snapshot.sku_code ?? record.sku_code ?? record.code,
    product_name: record.product_name ?? snapshot.product_name,
  }
  return product.product_id || product.sku_code ? [product] : []
}

function erpProductCode(item: Record<string, unknown>): string {
  return String(item.sku_code ?? item.sku ?? item.product_code ?? item.product_id ?? item.id ?? '').trim()
}

function applyERPProduct(row: ComposeRow, item: Record<string, unknown>) {
  row.erp_product_id = String(item.product_id ?? item.id ?? item.sku_code ?? item.product_code ?? '')
  row.erp_sku = erpProductCode(item)
  row.product_name = String(item.product_name ?? item.name ?? '')
  row.erp_product_snapshot = item
}

async function lookupERPProduct(code: string): Promise<Record<string, unknown> | undefined> {
  const response = await erpApi.getProductByCode(code)
  const products = normalizeERPProducts(response.data)
  return products.find((item) => erpProductCode(item).toLowerCase() === code.toLowerCase()) ?? products[0]
}

async function resolveERPBindings(candidates: ComposeRow[]): Promise<ComposeViolation[]> {
  const unresolved = candidates.filter((row) => row.erp_sku?.trim() && !row.erp_product_id)
  if (!unresolved.length) return []
  const codes = [...new Set(unresolved.map((row) => row.erp_sku!.trim()))]
  const results = new Map<string, Record<string, unknown> | Error>()
  let cursor = 0
  const worker = async () => {
    while (cursor < codes.length) {
      const code = codes[cursor++]
      try {
        const product = await lookupERPProduct(code)
        results.set(code, product ?? new Error(`ERP 中未找到商品 ${code}`))
      } catch (error) {
        results.set(code, error instanceof Error ? error : new Error(`ERP 商品 ${code} 查询失败`))
      }
    }
  }
  await Promise.all(Array.from({ length: Math.min(6, codes.length) }, () => worker()))
  const issues: ComposeViolation[] = []
  for (const row of unresolved) {
    const code = row.erp_sku!.trim()
    const product = results.get(code)
    if (product instanceof Error || !product) {
      issues.push({
        row_id: row.id,
        row_index: rows.value.indexOf(row),
        field: 'erp_sku',
        message: product?.message || `ERP 商品 ${code} 查询失败`,
      })
      continue
    }
    applyERPProduct(row, product)
  }
  gridRevision.value += 1
  return issues
}

async function resolveCurrentERPRows() {
  gridRef.value?.readRowsFromWorkbook?.()
  await Promise.resolve()
  batchERPResolving.value = true
  batchERPFeedback.value = ''
  submitError.value = ''
  remoteViolations.value = []
  try {
    const pendingCount = rows.value.filter((row) => row.erp_sku?.trim() && !row.erp_product_id).length
    if (!pendingCount) {
      batchERPFeedback.value = '没有需要查询的 SKU；可先从 Excel / WPS 粘贴多行编码。'
      return
    }
    remoteViolations.value = await resolveERPBindings(rows.value)
    const resolvedCount = pendingCount - remoteViolations.value.length
    batchERPFeedback.value = remoteViolations.value.length
      ? `已匹配 ${resolvedCount} 行，${remoteViolations.value.length} 行需要检查。`
      : `已匹配并回填 ${resolvedCount} 行 ERP 商品。`
    if (remoteViolations.value[0]) locateViolation(remoteViolations.value[0])
  } finally {
    batchERPResolving.value = false
  }
}

function locateViolation(issue: ComposeViolation) {
  if (issue.row_id) selectRow(issue.row_id)
  if (issue.row_index != null) {
    gridRef.value?.focusCell(issue.row_index, String(issue.field))
    return
  }
  document.querySelector('.common-ribbon')?.scrollIntoView({ behavior: 'smooth', block: 'center' })
}

async function submit(retryOnly: boolean) {
  gridRef.value?.readRowsFromWorkbook?.()
  await Promise.resolve()
  submitError.value = ''
  const candidates = retryOnly ? rows.value.filter((row) => row.status === 'failed') : rows.value
  remoteViolations.value = []
  if (intent.value === 'modify_existing') {
    batchERPResolving.value = true
    try {
      remoteViolations.value = await resolveERPBindings(candidates)
    } finally {
      batchERPResolving.value = false
    }
    if (remoteViolations.value.length) { locateViolation(remoteViolations.value[0]); return }
  }
  const currentViolations = validateCompose(intent.value, common, candidates)
  if (currentViolations.length) { locateViolation(currentViolations[0]); return }
  validatingIIDs.value = true
  try {
    remoteViolations.value = await validateIIDsBeforeSubmit(candidates)
  } finally {
    validatingIIDs.value = false
  }
  if (remoteViolations.value.length) { locateViolation(remoteViolations.value[0]); return }
  submitting.value = true
  try {
    if (intent.value === 'planning_sku') {
      try {
        planningResult.value = await planningSkuApi.create(buildPlanningInputs(candidates), common.erp_sync_mode, clientCreateId.value)
        selectedPlanningIds.value = new Set(planningResult.value.items.map((item) => item.task_sku_item_id))
        candidates.forEach((row, index) => { row.status = 'created'; row.result_task_id = String(planningResult.value?.task_id || ''); row.result_sku_code = planningResult.value?.items[index]?.sku_code })
        result.value = true
        dirty.value = false
      } catch (error) {
        submitError.value = error instanceof Error ? error.message : 'SKU 编码生成失败'
      }
      return
    }
    if (intent.value === 'retouch' && retryOnly && candidates.some((row) => row.result_task_id)) {
      await retryRetouchUploads(candidates)
      result.value = true
      dirty.value = false
      return
    }
    const units = buildTaskSubmissionUnits(intent.value, common, candidates)
    for (const unit of units) {
      const unitRows = rows.value.filter((row) => unit.row_ids.includes(row.id))
      unitRows.forEach((row) => { row.status = 'submitting'; row.error = '' })
      try {
        const created = await tasksStore.addTask(unit.task, `compose:${clientCreateId.value}:${unit.row_ids.join(',')}`)
        unitRows.forEach((row) => { row.status = 'created'; row.result_task_id = created.id })
        if (intent.value === 'retouch') {
          const loaded = tasksStore.getById(created.id) ?? created
          const drafts = unit.task.retouchRequirements ?? []
          const uploadResult = await uploadRetouchRequirementPendingAssets(created.id, loaded.retouchRequirements ?? [], drafts)
          applyRetouchUploadResult(unitRows, uploadResult)
          if (uploadResult.failures.length) {
            unitRows.forEach((row) => { row.status = 'failed'; row.error = uploadResult.failures.map((failure) => failure.message).join('；') })
          }
        }
      } catch (error) {
        const response = (error as { response?: { data?: unknown } })?.response?.data
        const mapped = applyBackendViolations(unitRows, response)
        unitRows.forEach((row) => { row.status = 'failed'; row.error = mapped.filter((issue) => !issue.row_id || issue.row_id === row.id).map((issue) => issue.message).join('；') || (error instanceof Error ? error.message : '创建失败') })
      }
    }
    result.value = true
    dirty.value = false
  } finally { submitting.value = false }
}

function applyRetouchUploadResult(
  unitRows: ComposeRow[],
  uploadResult: Awaited<ReturnType<typeof uploadRetouchRequirementPendingAssets>>,
) {
  unitRows.forEach((row, requirementIndex) => {
    const failedReferenceNames = new Set(uploadResult.failures
      .filter((failure) => failure.requirementIndex === requirementIndex && failure.kind === 'reference')
      .map((failure) => failure.fileName))
    const failedSourceNames = new Set(uploadResult.failures
      .filter((failure) => failure.requirementIndex === requirementIndex && failure.kind === 'source')
      .map((failure) => failure.fileName))
    row.reference_assets = row.reference_assets.map((asset) => asset.file
      ? failedReferenceNames.has(asset.name)
        ? { ...asset, status: 'local', error: undefined }
        : { ...asset, status: 'uploaded', file: undefined, error: undefined }
      : asset)
    row.source_assets = row.source_assets.map((asset) => asset.file
      ? failedSourceNames.has(asset.name)
        ? { ...asset, status: 'local', error: undefined }
        : { ...asset, status: 'uploaded', file: undefined, error: undefined }
      : asset)
  })
}

async function retryRetouchUploads(candidates: ComposeRow[]) {
  const byTask = new Map<string, ComposeRow[]>()
  for (const row of candidates) {
    if (!row.result_task_id) continue
    byTask.set(row.result_task_id, [...(byTask.get(row.result_task_id) ?? []), row])
  }
  for (const [taskId, taskRows] of byTask) {
    taskRows.forEach((row) => { row.status = 'submitting'; row.error = '' })
    try {
      const loaded = tasksStore.getById(taskId)
      if (!loaded) throw new Error('任务已创建，但本地未找到任务详情，请进入详情页补传附件')
      const unit = buildTaskSubmissionUnits('retouch', common, taskRows)[0]
      const drafts = unit.task.retouchRequirements ?? []
      const uploadResult = await uploadRetouchRequirementPendingAssets(taskId, loaded.retouchRequirements ?? [], drafts)
      applyRetouchUploadResult(taskRows, uploadResult)
      if (uploadResult.failures.length) {
        const message = uploadResult.failures.map((failure) => failure.message).join('；')
        taskRows.forEach((row) => { row.status = 'failed'; row.error = message })
      } else {
        taskRows.forEach((row) => { row.status = 'created'; row.error = '' })
      }
    } catch (error) {
      taskRows.forEach((row) => {
        row.status = 'failed'
        row.error = error instanceof Error ? error.message : '附件重试失败'
      })
    }
  }
  gridRevision.value += 1
}

function unwrapBatchParsePayload(raw: unknown): { preview: BatchPreviewRow[]; violations: BatchViolation[] } {
  const root = (raw ?? {}) as Record<string, unknown>
  const nested = root.data && typeof root.data === 'object' ? root.data as Record<string, unknown> : root
  return {
    preview: Array.isArray(nested.preview) ? nested.preview as BatchPreviewRow[] : [],
    violations: Array.isArray(nested.violations) ? nested.violations as BatchViolation[] : [],
  }
}

function assetDraftFromReference(ref: Record<string, unknown>): ComposeAssetDraft {
  return {
    id: generateActionId(),
    name: String(ref.filename ?? ref.file_name ?? 'Excel 参考图'),
    preview_url: String(ref.download_url ?? ref.preview_url ?? '') || undefined,
    upload_ref: ref,
    status: 'uploaded',
  }
}

async function importComposeExcel(event: Event) {
  const input = event.target as HTMLInputElement
  const file = input.files?.[0]
  if (!file) return
  try {
    submitError.value = ''
    if (intent.value === 'new_design') {
      const response = await batchSkuApi.parseExcel(file)
      const parsed = unwrapBatchParsePayload(response.data)
      const normalized = parsed.preview.map(normalizeBatchPreviewRow)
      if (normalized.length) {
        rows.value = normalized.slice(0, maxRows.value).map((item) => {
          const variant = item.variant_json ?? {}
          return createComposeRow({
            product_i_id: item.product_i_id,
            product_name: item.product_name,
            design_requirement: item.design_requirement,
            width: Number.isFinite(Number(variant.width)) ? Number(variant.width) : undefined,
            height: Number.isFinite(Number(variant.height)) ? Number(variant.height) : undefined,
            area: Number.isFinite(Number(variant.area)) ? Number(variant.area) : undefined,
            special_note: typeof variant.special_note === 'string' ? variant.special_note : undefined,
            set_mode_hint: variant.set_mode_hint === true,
            reference_assets: (item.reference_file_refs ?? []).map((ref) => assetDraftFromReference(ref as unknown as Record<string, unknown>)),
          })
        })
        for (const issue of parsed.violations) {
          const sourceIndex = normalized.findIndex((item) => item.source_row === issue.row)
          const row = rows.value[sourceIndex >= 0 ? sourceIndex : Math.max(0, issue.row - 2)]
          if (row && issue.code === 'invalid_i_id') row.product_i_id = ''
        }
        if (rows.value[0]) selectRow(rows.value[0].id)
        gridRevision.value += 1
      }
      if (parsed.violations.length) submitError.value = parsed.violations.slice(0, 3).map((item) => `第 ${item.row} 行：${formatBatchViolationMessage(item)}`).join('；')
      if (!normalized.length && !parsed.violations.length) submitError.value = 'Excel 中没有可创建的新款明细'
      return
    }
    const parsed = await planningSkuApi.parseExcel(file, erpSync.value)
    if (parsed.errors.length) submitError.value = parsed.errors.slice(0, 3).map((item) => `第 ${item.row} 行 ${item.field}：${item.reason}`).join('；')
    if (parsed.planning_sku_items.length) {
      rows.value = parsed.planning_sku_items.slice(0, 200).map((item) => createComposeRow({ id: item.client_item_id || generateActionId(), description_spec: item.description_spec, quantity: item.quantity, target_price: item.target_price, note: item.note, reference_url: item.reference_url, product_i_id: item.erp_product_i_id, product_name: item.erp_product_name, reference_assets: item.image_upload_ref ? [{ id: generateActionId(), name: 'Excel 产品图片', upload_ref: item.image_upload_ref, status: 'uploaded' }] : [] }))
      if (rows.value[0]) selectRow(rows.value[0].id)
      gridRevision.value += 1
    }
  } catch (error) { submitError.value = error instanceof Error ? error.message : 'Excel 解析失败' }
  finally { input.value = '' }
}

function unwrapIIDRows(raw: unknown): Array<Record<string, unknown>> {
  const root = (raw ?? {}) as Record<string, unknown>
  if (Array.isArray(root.data)) return root.data as Array<Record<string, unknown>>
  const nested = root.data && typeof root.data === 'object' ? root.data as Record<string, unknown> : root
  return Array.isArray(nested.data) ? nested.data as Array<Record<string, unknown>> : Array.isArray(nested.items) ? nested.items as Array<Record<string, unknown>> : []
}

async function validateIIDsBeforeSubmit(candidates: ComposeRow[]): Promise<ComposeViolation[]> {
  const needsIID = intent.value === 'new_design' || (intent.value === 'planning_sku' && common.erp_sync_mode === 'async')
  if (!needsIID) return []
  const byIID = new Map<string, ComposeRow[]>()
  for (const row of candidates) {
    const value = row.product_i_id?.trim() ?? ''
    if (!value) continue
    byIID.set(value, [...(byIID.get(value) ?? []), row])
  }
  const checks = await Promise.all([...byIID].map(async ([iid, matchedRows]) => {
    try {
      const response = await erpApi.getIids({ q: iid, page: 1, page_size: 200 })
      const found = unwrapIIDRows(response.data).some((item) => String(item.i_id ?? item.label ?? '').trim().toLowerCase() === iid.toLowerCase())
      return found ? [] : matchedRows.map((row) => ({ row_id: row.id, row_index: candidates.indexOf(row), field: 'product_i_id' as const, message: `款式编码 ${iid} 未匹配到 ERP 可选项` }))
    } catch {
      return matchedRows.map((row) => ({ row_id: row.id, row_index: candidates.indexOf(row), field: 'product_i_id' as const, message: `暂时无法核对款式编码 ${iid}，请稍后重试` }))
    }
  }))
  return checks.flat()
}

function serializableRows() {
  return rows.value.map((row) => ({ ...row, reference_assets: row.reference_assets.map(({ file: _file, preview_url: _preview, ...asset }) => asset), source_assets: row.source_assets.map(({ file: _file, preview_url: _preview, ...asset }) => asset) }))
}
async function saveDraft() {
  const payload = { task_type: 'unified_task_compose', payload: { version: 1, intent: intent.value, common: { ...common }, rows: serializableRows(), client_create_id: clientCreateId.value } }
  const saved = currentDraftId.value ? await updateTaskDraft(currentDraftId.value, payload) : await saveTaskDraft(payload)
  currentDraftId.value = saved.id
  dirty.value = false
  await router.replace({ query: { ...route.query, draft_id: saved.id, intent: intent.value } })
}
async function restoreDraft(id: string) {
  try {
    const draft = await getTaskDraft(id)
    const payload = draft.payload as Record<string, unknown>
    if (intentOptions.value.some((item) => item.value === payload.intent)) intent.value = payload.intent as ComposeIntent
    if (payload.common && typeof payload.common === 'object') Object.assign(common, payload.common)
    if (Array.isArray(payload.rows)) {
      rows.value = payload.rows.map((rawRow) => {
        const row = createComposeRow(rawRow as Partial<ComposeRow>)
        row.source_assets = row.source_assets.map((asset) => asset.file || asset.upload_ref ? asset : {
          ...asset,
          status: 'failed',
          error: '本地素材不会保存在草稿中，请重新选择该文件',
        })
        return row
      })
    }
    if (typeof payload.client_create_id === 'string') clientCreateId.value = payload.client_create_id
    if (rows.value[0]) selectRow(rows.value[0].id)
    gridRevision.value += 1
  } catch (error) { submitError.value = error instanceof Error ? error.message : '草稿读取失败' }
}

function planningFailed(status?: string) { return ['failed', 'filing_failed', 'sync_failed'].includes(String(status || '').toLowerCase()) }
async function copyPlanningAll() { if (planningResult.value) await navigator.clipboard.writeText(planningResult.value.items.map((item) => item.sku_code).join('\n')) }
async function retryPlanningERP() {
  if (!planningResult.value || !failedPlanningItems.value.length) return
  resultBusy.value = true
  try { await planningSkuApi.retryFailedERP(planningResult.value.task_id); failedPlanningItems.value.forEach((item) => { item.erp_status = 'pending' }); resultFilter.value = 'all' }
  finally { resultBusy.value = false }
}
async function exportPlanningSelection() { resultBusy.value = true; try { await planningSkuApi.exportSelection([...selectedPlanningIds.value]) } finally { resultBusy.value = false } }
function startAnother() { resetComposeState() }
</script>

<style scoped>
.compose-page{max-width:1600px;margin:0 auto;padding:1.5rem;display:grid;gap:1rem}.compose-hero,.workspace-toolbar,.result-heading,.result-actions,.toolbar-actions,.hero-actions{display:flex;align-items:center;justify-content:space-between;gap:1rem}.compose-hero h1{margin:.1rem 0;font-size:clamp(2rem,4vw,3.2rem);letter-spacing:-.045em}.compose-hero p,.workspace-toolbar p,.result-heading p{margin:0;color:rgb(var(--yb-text-secondary))}.eyebrow{margin:0;color:rgb(var(--yb-brand));font-size:.68rem;font-weight:850;letter-spacing:.15em;text-transform:uppercase}.hero-actions,.toolbar-actions,.result-actions{justify-content:flex-end;flex-wrap:wrap}.primary-button,.secondary-button{display:inline-flex;align-items:center;justify-content:center;gap:.45rem;min-height:2.65rem;padding:0 .95rem;border-radius:.78rem;font-weight:760;text-decoration:none;cursor:pointer}.primary-button{border:1px solid rgb(var(--yb-brand));background:rgb(var(--yb-brand));color:rgb(var(--yb-text-inverse))}.secondary-button{border:1px solid rgb(var(--yb-border-context));background:rgb(var(--yb-surface));color:rgb(var(--yb-text-primary))}.primary-button:disabled,.secondary-button:disabled{opacity:.48;cursor:not-allowed}.intent-grid{display:grid;grid-template-columns:repeat(4,minmax(0,1fr));gap:.8rem}.intent-card{position:relative;display:grid;grid-template-columns:auto 1fr;gap:.75rem;text-align:left;min-height:7.5rem;padding:1.15rem;border:1px solid rgb(var(--yb-border-context));border-radius:1rem;background:rgb(var(--yb-surface));color:rgb(var(--yb-text-primary));box-shadow:0 .7rem 1.8rem rgb(var(--yb-shadow)/.045);cursor:pointer}.intent-card:hover,.intent-card.is-active{border-color:rgb(var(--yb-brand));box-shadow:0 1rem 2.4rem rgb(var(--yb-shadow)/.08)}.intent-icon{display:grid;place-items:center;width:2.65rem;height:2.65rem;border-radius:.75rem;background:rgb(var(--yb-brand-soft));color:rgb(var(--yb-brand-deep))}.intent-copy{display:grid;align-content:start;gap:.35rem}.intent-copy strong{font-size:1.05rem}.intent-copy small{line-height:1.45;color:rgb(var(--yb-text-secondary))}.intent-badge{grid-column:1/-1;align-self:end;width:max-content;padding:.25rem .5rem;border-radius:999px;background:rgb(var(--yb-surface-app-muted));color:rgb(var(--yb-text-secondary));font-size:.7rem}.intent-check{position:absolute;top:.85rem;right:.85rem;color:rgb(var(--yb-brand))}.common-ribbon{display:flex;flex-wrap:wrap;gap:.85rem 1.4rem;align-items:flex-start;padding:1rem 1.15rem;border:1px solid rgb(var(--yb-border-context));border-radius:1rem;background:rgb(var(--yb-surface));box-shadow:0 .8rem 2rem rgb(var(--yb-shadow)/.04)}.common-ribbon .field{display:grid;gap:.4rem;align-content:start}.field-name{font-size:.74rem;font-weight:780;color:rgb(var(--yb-text-secondary))}.common-ribbon .field>small{font-size:.7rem;font-weight:500;color:rgb(var(--yb-text-secondary))}.common-ribbon input:not([type=checkbox]),.common-ribbon select,.erp-search input{min-width:0;height:2.45rem;padding:0 .7rem;border:1px solid rgb(var(--yb-border-context));border-radius:.65rem;background:rgb(var(--yb-surface));color:rgb(var(--yb-text-primary))}.common-ribbon input[type=datetime-local]{width:13rem}.common-ribbon select{width:8.5rem}.note-field{flex:1 1 260px;min-width:260px}.lane-toggle{display:inline-flex;gap:.2rem;padding:.22rem;border:1px solid rgb(var(--yb-border-context));border-radius:.72rem;background:rgb(var(--yb-surface-app-muted))}.lane-toggle button{min-width:4.2rem;min-height:1.95rem;border:0;border-radius:.52rem;background:transparent;color:rgb(var(--yb-text-secondary));font-weight:760;cursor:pointer}.lane-toggle button.active{background:rgb(var(--yb-brand));color:rgb(var(--yb-text-inverse));box-shadow:0 .25rem .7rem rgb(var(--yb-brand)/.28)}.switch-row{display:flex;align-items:center;gap:.5rem;min-height:2.45rem}.switch-row input{width:1.15rem;height:1.15rem;accent-color:rgb(var(--yb-brand))}.switch-row small{font-weight:500;color:rgb(var(--yb-text-secondary))}.workspace-card,.result-board{border:1px solid rgb(var(--yb-border-context));border-radius:1.15rem;background:rgb(var(--yb-surface));box-shadow:0 1.2rem 3rem rgb(var(--yb-shadow)/.065);overflow:hidden}.workspace-toolbar{padding:1rem 1.15rem;border-bottom:1px solid rgb(var(--yb-border-context))}.workspace-toolbar h2,.result-heading h2{margin:0}.file-button input{display:none}.workspace-layout{display:grid;grid-template-columns:minmax(0,1fr)}.workspace-layout.has-drawer{grid-template-columns:minmax(0,1fr) 22rem}.grid-column{min-width:0;padding:1rem}.row-drawer{border-left:1px solid rgb(var(--yb-border-context));background:rgb(var(--yb-surface-app-muted));min-width:0}.row-drawer>header{display:flex;justify-content:space-between;align-items:center;padding:1rem;border-bottom:1px solid rgb(var(--yb-border-context))}.row-drawer h3,.row-drawer h4{margin:0}.row-drawer header button,.asset-list button,.mobile-row-card header button{border:0;background:transparent;color:rgb(var(--yb-text-secondary));cursor:pointer}.drawer-section{display:grid;gap:.7rem;padding:1rem;border-bottom:1px solid rgb(var(--yb-border-context))}.drawer-section p{margin:0;color:rgb(var(--yb-text-secondary));font-size:.78rem;line-height:1.5}.hint-section{grid-template-columns:1fr auto;align-items:center}.hint-section input{width:2.5rem;height:1.4rem}.erp-search{display:grid;grid-template-columns:1fr auto;gap:.4rem}.erp-search button,.erp-result,.asset-button{min-height:2.45rem;border:1px solid rgb(var(--yb-border-context));border-radius:.65rem;background:rgb(var(--yb-surface));color:rgb(var(--yb-text-primary));cursor:pointer}.erp-result{display:grid;text-align:left;padding:.65rem}.erp-result span,.selected-erp{font-size:.75rem;color:rgb(var(--yb-text-secondary))}.asset-list{display:grid;gap:.45rem}.asset-list article{display:grid;grid-template-columns:2.8rem 1fr auto;gap:.6rem;align-items:center;padding:.5rem;border:1px solid rgb(var(--yb-border-context));border-radius:.65rem;background:rgb(var(--yb-surface))}.asset-list img{width:2.8rem;height:2.8rem;object-fit:cover;border-radius:.45rem}.asset-list div{display:grid;min-width:0}.asset-list strong{overflow:hidden;text-overflow:ellipsis;white-space:nowrap;font-size:.78rem}.asset-list span{font-size:.68rem;color:rgb(var(--yb-text-secondary))}.asset-button{display:flex;align-items:center;justify-content:center;gap:.35rem;padding:.55rem}.drawer-errors{margin:0;padding-left:1rem;color:rgb(var(--yb-danger));font-size:.76rem}.drawer-section .drawer-ok{display:flex;align-items:center;gap:.35rem;color:rgb(var(--yb-success))}.validation-dock{position:sticky;bottom:0;z-index:3;display:grid;grid-template-columns:minmax(240px,.7fr) minmax(0,1.4fr) auto;gap:1rem;align-items:center;padding:.9rem 1rem;border-top:1px solid rgb(var(--yb-border-context));background:color-mix(in srgb,rgb(var(--yb-surface)) 94%,transparent);backdrop-filter:blur(18px)}.validation-summary{display:flex;align-items:center;gap:.65rem;color:rgb(var(--yb-danger))}.validation-summary.valid{color:rgb(var(--yb-success))}.validation-summary div{display:grid}.validation-summary span{font-size:.72rem;color:rgb(var(--yb-text-secondary))}.validation-items{display:flex;gap:.45rem;overflow:auto}.validation-items button{display:grid;min-width:12rem;text-align:left;padding:.5rem .7rem;border:1px solid rgb(var(--yb-danger-border));border-radius:.65rem;background:rgb(var(--yb-danger-soft));color:rgb(var(--yb-danger));font-size:.72rem;cursor:pointer}.validation-items span{font-weight:800}.dock-actions{display:flex;align-items:center;gap:.7rem}.dock-actions p{max-width:25rem;margin:0;color:rgb(var(--yb-danger));font-size:.76rem}.result-board{padding:1.2rem;display:grid;gap:1rem}.task-result-grid,.sku-result-grid{display:grid;grid-template-columns:repeat(auto-fill,minmax(260px,1fr));gap:.65rem}.task-result,.sku-result{display:grid;grid-template-columns:auto 1fr auto;gap:.6rem;align-items:center;padding:.8rem;border:1px solid rgb(var(--yb-border-context));border-radius:.75rem}.task-result>div{display:grid}.task-result small,.sku-result small{color:rgb(var(--yb-text-secondary))}.task-result.is-created{color:rgb(var(--yb-success))}.task-result.is-failed,.sku-result.failed{color:rgb(var(--yb-danger));border-color:rgb(var(--yb-danger-border));background:rgb(var(--yb-danger-soft))}.result-filter{display:flex;gap:.5rem}.result-filter button{padding:.45rem .75rem;border:1px solid rgb(var(--yb-border-context));border-radius:999px;background:rgb(var(--yb-surface));cursor:pointer}.result-filter button.active{border-color:rgb(var(--yb-brand));color:rgb(var(--yb-brand-deep));background:rgb(var(--yb-brand-soft))}.sku-result{grid-template-columns:auto auto 1fr auto}.retry-failed{grid-column:1/-1}.mobile-row-list{display:none}.sr-only{position:absolute;width:1px;height:1px;overflow:hidden;clip:rect(0,0,0,0)}
.hero-copy p{max-width:56rem}.compose-hero h1{margin:.35rem 0 .55rem}.result-heading-actions{display:flex;gap:.6rem;flex-wrap:wrap}
.workspace-toolbar{position:relative}.toolbar-feedback{position:absolute;right:1.15rem;bottom:.18rem;font-size:.7rem;color:rgb(var(--yb-text-secondary))}
.compose-confirm-backdrop{position:fixed;inset:0;z-index:8600;display:grid;place-items:center;padding:1.2rem;background:rgb(var(--yb-overlay-night)/.5);backdrop-filter:blur(6px)}
.compose-confirm{width:min(26rem,100%);display:grid;gap:.55rem;padding:1.4rem;border:1px solid rgb(var(--yb-border-context));border-radius:1.1rem;background:rgb(var(--yb-surface));box-shadow:0 1.6rem 4rem rgb(var(--yb-shadow)/.28)}
.compose-confirm .confirm-icon{display:grid;place-items:center;width:2.6rem;height:2.6rem;border-radius:.8rem;background:rgb(var(--yb-warning-soft));color:rgb(var(--yb-warning-strong))}
.compose-confirm h3{margin:.2rem 0 0;font-size:1.1rem}.compose-confirm p{margin:0;color:rgb(var(--yb-text-secondary));line-height:1.6}
.compose-confirm footer{display:flex;justify-content:flex-end;gap:.6rem;margin-top:.6rem}
.compose-confirm .primary-button.danger{border-color:rgb(var(--yb-danger));background:rgb(var(--yb-danger))}
@media(max-width:1180px){.intent-grid{grid-template-columns:repeat(2,minmax(0,1fr))}.note-field{flex-basis:100%}.workspace-layout.has-drawer{grid-template-columns:minmax(0,1fr) 19rem}.validation-dock{grid-template-columns:1fr auto}.validation-items{grid-column:1/-1;grid-row:2}.dock-actions{grid-column:2;grid-row:1}}
@media(max-width:760px){.compose-page{padding:.75rem}.compose-hero,.workspace-toolbar,.result-heading{align-items:flex-start;flex-direction:column}.hero-actions,.toolbar-actions,.result-actions{width:100%;justify-content:flex-start}.intent-grid{grid-template-columns:1fr}.intent-card{min-height:6.5rem}.common-ribbon{flex-direction:column;align-items:stretch}.common-ribbon input[type=datetime-local],.common-ribbon select{width:100%}.workspace-layout.has-drawer{grid-template-columns:1fr}.grid-column{padding:.65rem}.row-drawer{border-left:0;border-top:1px solid rgb(var(--yb-border-context))}.mobile-row-list{display:grid;gap:.65rem}.mobile-row-card{display:grid;gap:.65rem;padding:.8rem;border:1px solid rgb(var(--yb-border-context));border-radius:.8rem;background:rgb(var(--yb-surface))}.mobile-row-card.selected{border-color:rgb(var(--yb-brand))}.mobile-row-card header{display:grid;grid-template-columns:auto 1fr auto;gap:.55rem;align-items:center}.mobile-row-card header span{font-size:.7rem;color:rgb(var(--yb-text-secondary))}.mobile-fields{display:grid;gap:.55rem}.mobile-fields label{display:grid;gap:.3rem;font-size:.72rem;font-weight:700;color:rgb(var(--yb-text-secondary))}.mobile-fields input,.mobile-fields textarea{width:100%;box-sizing:border-box;padding:.6rem;border:1px solid rgb(var(--yb-border-context));border-radius:.6rem;background:rgb(var(--yb-surface));color:rgb(var(--yb-text-primary))}.mobile-fields .mobile-switch{grid-template-columns:1fr auto}.mobile-switch input{width:2.4rem}.validation-dock{position:static;grid-template-columns:1fr}.validation-items,.dock-actions{grid-column:auto;grid-row:auto}.dock-actions{display:grid}.dock-actions .primary-button{width:100%}.task-result-grid,.sku-result-grid{grid-template-columns:1fr}}
</style>
