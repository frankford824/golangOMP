<template>
  <section class="resource-workspace" :data-phase="phase">
    <header class="workspace-head">
      <div class="workspace-identity">
        <span class="workspace-mark" :class="`is-${phase}`"><ShieldCheck v-if="isAuditStage" :size="20" aria-hidden="true" /><FilePenLine v-else :size="20" aria-hidden="true" /></span>
        <div>
          <h2>{{ heading }}</h2>
          <p>{{ headingHint }}</p>
        </div>
      </div>
      <div class="workspace-tools">
        <span class="workspace-count"><Boxes :size="15" aria-hidden="true" />{{ rows.length }} 个资源单元</span>
        <button v-if="canBulkApplyMode && rows.length > 1" class="quiet-button" @click="applyFirstMode"><CopyCheck :size="15" aria-hidden="true" />{{ isAuditStage ? '统一按首个模式变更全部' : '将首个模式应用到全部' }}</button>
      </div>
    </header>

    <div class="stage-map" aria-label="任务资源的三个阶段">
      <article class="stage-node complete">
        <span class="stage-index">1</span>
        <div><strong>运营参考</strong><small>{{ displayReferenceCount }} 份参考资料</small></div>
      </article>
      <article class="stage-node" :class="{ active: isDesignStage, complete: !isDesignStage }">
        <span class="stage-index">2</span>
        <div><strong>设计源文件</strong><small>设计选择单图或套装，并提交源文件</small></div>
      </article>
      <article class="stage-node" :class="{ active: isAuditStage, locked: isDesignStage }">
        <span class="stage-index">3</span>
        <div><strong>{{ isRetouch ? '修图定稿' : '审核定稿' }}</strong><small>{{ finalStageHint }}</small></div>
      </article>
    </div>

    <div v-if="error" class="message error" role="alert">{{ error }}</div>
    <div v-if="success" class="message success" role="status">{{ success }}</div>

    <aside v-if="isDesignStage" class="contract-note">
      <Info :size="17" aria-hidden="true" />
      <strong>设计阶段只提交源文件</strong>
      <span>请为每个 SKU 选择单图或套装。最终成品由审核人员上传；套装在审核时至少需要 2 张有序成品。</span>
    </aside>
    <aside v-else-if="isAuditStage" class="contract-note audit-note">
      <ShieldCheck :size="17" aria-hidden="true" />
      <strong>按设计模式完成定稿</strong>
      <span>单图上传 1 张，套装至少上传 2 张并排序。未替换源文件时，默认保留设计师提交的源文件。</span>
    </aside>

    <section v-if="isAuditStage && auditReferences.length" class="audit-references" aria-label="审核参考图">
      <header><div><strong>运营参考图</strong><small>审核时可直接查看原始参考资料</small></div><span>{{ auditReferences.length }} 份</span></header>
      <div class="audit-reference-strip">
        <button v-for="(file, index) in auditReferences" :key="referenceIdentity(file, index)" type="button" @click="openAuditReference(file)">
          <AssetPreviewMedia
            v-if="referencePreviewable(file) && referencePreview(file)"
            class="audit-reference-media"
            :task-asset-id="referenceTaskAssetID(file)"
            :fallback-src="referencePreview(file)"
            :alt="referenceName(file)"
            img-class="audit-reference-shell"
            inner-img-class="audit-reference-image"
          />
          <FileText v-else :size="24" aria-hidden="true" />
          <span>{{ referenceName(file) }}</span>
        </button>
      </div>
    </section>

    <div
      v-if="showEditor"
      ref="editorViewport"
      class="editor-viewport"
      :style="{ '--editor-row-height': editorRowHeight + 'px' }"
      data-testid="resource-editor-viewport"
      @scroll="onEditorScroll"
    >
      <div class="editor-spacer" :style="{ height: editorTotalHeight + 'px' }">
        <div class="editor-window" :style="{ transform: 'translateY(' + editorWindowOffset + 'px)' }">
          <article v-for="entry in visibleEditorRows" :key="entry.row.group.id" class="sku-workbench" :data-group-index="entry.groupIndex">
            <header class="sku-head">
              <div>
                <span>SKU 资源</span>
                <strong>{{ entry.row.group.sku_code || scopeLabel(entry.row.group) }}</strong>
                <small v-if="entry.row.group.product_name || entry.row.group.sku_profile?.product_name" class="sku-product-name">{{ entry.row.group.product_name || entry.row.group.sku_profile?.product_name }}</small>
                <small v-if="skuModeHints[entry.row.group.sku_code || '']" class="operations-hint">运营建议套装 · 最终由设计判定</small>
              </div>
              <div class="mode-control" :aria-label="`${entry.row.group.sku_code || '当前资源'}的成品模式`">
                <span>{{ isAuditStage ? '审核可调整' : '设计判定' }}</span>
                <button type="button" :class="{ selected: entry.row.mode === 'single' }" :disabled="!canChooseMode" @click="setMode(entry.row, 'single')">单图</button>
                <button type="button" :class="{ selected: entry.row.mode === 'set' }" :disabled="!canChooseMode" @click="setMode(entry.row, 'set')">套装</button>
              </div>
            </header>

            <div class="resource-columns">
              <section class="source-column">
                <div class="column-title">
                  <div><strong>设计源文件</strong><small>{{ isAuditStage ? '默认保留设计提交' : sourceRequired ? '每个 SKU 必填 1 份' : '选填' }}</small></div>
                  <label v-if="isAuditStage" class="replace-toggle"><input type="checkbox" :checked="replaceSourceGroups.has(entry.row.group.id)" @change="toggleSourceReplacement(entry.row)" />我修改了源文件</label>
                </div>
                <div v-if="entry.row.source" class="file-tile source-file">
                  <span class="file-mark">{{ sourceExtension(entry.row.source.name) }}</span>
                  <div>
                    <strong>{{ entry.row.source.name }}</strong>
                    <small>{{ entry.row.source.inherited ? '设计师提交 · 将作为有效源文件' : entry.row.sourceMemberNames.length > 1 ? `本次已打包 ${entry.row.sourceMemberNames.length} 份源文件` : '本次上传' }}</small>
                    <small v-if="entry.row.sourceMemberNames.length > 1" class="bundle-members">{{ entry.row.sourceMemberNames.join('、') }}</small>
                  </div>
                  <button
                    v-if="entry.row.source.inherited"
                    type="button"
                    class="source-download"
                    :disabled="downloadingSourceID === entry.row.source.id"
                    aria-label="下载设计源文件"
                    @click="downloadSource(entry.row.source)"
                  >
                    <Download :size="16" aria-hidden="true" />{{ downloadingSourceID === entry.row.source.id ? '下载中' : '下载' }}
                  </button>
                  <CheckCircle2 :size="18" class="file-ready" aria-label="已就绪" />
                </div>
                <label v-if="canUploadSource(entry.row)" class="drop-zone source-drop">
                  <UploadCloud :size="20" aria-hidden="true" /><span>{{ entry.row.sourceMemberNames.length ? `继续添加源文件（已选 ${entry.row.sourceMemberNames.length} 份）` : entry.row.source && !replaceSourceGroups.has(entry.row.group.id) ? '替换源文件' : '选择一份或多份 PSD、AI、PSB、ZIP 等源文件' }}</span>
                  <input type="file" multiple :disabled="Boolean(entry.row.uploading)" @change="uploadSource($event, entry.row)" />
                </label>
              </section>

              <section class="final-column" :class="{ locked: isDesignStage }">
                <div class="column-title">
                  <div><strong>最终成品图</strong><small>{{ finalRequirement(entry.row) }}</small></div>
                  <span v-if="isAuditStage" class="mode-summary">{{ entry.row.mode === 'set' ? '套装' : '单图' }}</span>
                </div>
                <div v-if="isDesignStage" class="locked-final">
                  <span class="lock-symbol"><LockKeyhole :size="17" aria-hidden="true" /></span>
                  <div><strong>审核阶段上传</strong><small>设计提交后，审核人员会一眼看到当前模式。</small></div>
                </div>
                <template v-else>
                  <label class="drop-zone final-drop">
                    <Images :size="20" aria-hidden="true" /><span>{{ entry.row.finals.length && entry.row.mode === 'set' ? `继续添加成品（当前 ${entry.row.finals.length} 张）` : entry.row.finals.length ? '替换单图成品' : '上传最终成品图或成品 ZIP' }}</span>
                    <input type="file" accept="image/*,.zip,application/zip" :multiple="entry.row.mode === 'set'" :disabled="Boolean(entry.row.uploading)" @change="uploadFinals($event, entry.row)" />
                  </label>
                  <button v-if="entry.row.finals.length" type="button" class="clear-finals" :disabled="Boolean(entry.row.uploading)" @click="clearFinals(entry.row)">清空本组成品</button>
                  <ol v-if="entry.row.finals.length" class="final-order">
                    <li
                      v-for="(file, index) in entry.row.finals"
                      :key="`${file.id}-${index}`"
                      draggable="true"
                      tabindex="0"
                      @dragstart="dragStart(entry.groupIndex, index)"
                      @dragover.prevent
                      @drop="drop(entry.groupIndex, index)"
                      @keydown.alt.up.prevent="move(entry.row, index, -1)"
                      @keydown.alt.down.prevent="move(entry.row, index, 1)"
                    >
                      <span>{{ index + 1 }}</span><strong>{{ file.name }}</strong>
                      <div><button aria-label="上移" :disabled="index === 0" @click="move(entry.row,index,-1)">↑</button><button aria-label="下移" :disabled="index === entry.row.finals.length - 1" @click="move(entry.row,index,1)">↓</button><button aria-label="移除" @click="removeFinal(entry.row,index)">×</button></div>
                    </li>
                  </ol>
                </template>
              </section>
            </div>
            <div v-if="entry.row.uploading" class="uploading">正在上传 {{ entry.row.uploading }}…</div>
          </article>
        </div>
      </div>
    </div>

    <footer v-if="isDesignStage" class="command-dock">
      <div class="dock-progress"><strong>{{ workspaceProgressLabel }}</strong><span>{{ dirtyLabel }} · 离开前会提醒未提交修改。</span></div>
      <button class="primary" :disabled="busy || !validDesign" @click="submitDesign"><Send :size="16" aria-hidden="true" />{{ busy ? '提交中…' : '确认模式并提交源文件' }}</button>
    </footer>

    <footer v-if="isRetouchStage" class="command-dock">
      <div><strong>{{ dirtyLabel }}</strong><span>每项修图需求提交最终成品后，任务直接结单。</span></div>
      <button class="primary" :disabled="busy || !validRetouch" @click="submitDesign"><Send :size="16" aria-hidden="true" />{{ busy ? '提交中…' : '提交修图成品并结单' }}</button>
    </footer>

    <footer v-if="isAuditStage" class="command-dock audit-dock">
      <label><span>审核说明</span><input v-model.trim="reason" maxlength="1000" placeholder="通过时选填，打回时必填" /></label>
      <div class="dock-actions">
        <button v-if="canReturnToDesign" class="danger" :disabled="busy || !reason" @click="openConfirmation('return_to_design')"><RotateCcw :size="16" aria-hidden="true" />打回设计</button>
        <button v-if="canApprove" class="primary approve" :disabled="busy || !validAudit" @click="openConfirmation('approve')"><CircleCheckBig :size="16" aria-hidden="true" />{{ busy ? '处理中…' : '确认定稿并结单' }}</button>
      </div>
    </footer>

    <footer v-if="canReopen" class="command-dock reopen-dock">
      <label><span>修改阶段</span><select v-model="reopenTarget"><option value="audit">审核修改成品/源文件</option><option value="design">设计重新提交</option><option v-if="isRetouch" value="retouch">修图重新提交</option></select></label>
      <label><span>修改原因</span><input v-model.trim="reason" maxlength="1000" placeholder="请说明需要修改的文件和原因" /></label>
      <button class="quiet-button" :disabled="busy || !reason" @click="openConfirmation('reopen')">确认重开并修改文件</button>
    </footer>

    <div v-if="pendingAction" class="confirm-backdrop" role="presentation" @click.self="cancelConfirmation">
      <section ref="confirmDialog" class="confirm-dialog" role="dialog" aria-modal="true" aria-labelledby="workflow-confirm-title" tabindex="-1" @keydown.esc.prevent="cancelConfirmation" @keydown.tab="trapConfirmationFocus">
        <button class="dialog-close" aria-label="关闭" :disabled="busy" @click="cancelConfirmation">×</button>
        <span class="section-label">确认审核结果</span>
        <h3 id="workflow-confirm-title">{{ confirmationTitle }}</h3>
        <p>{{ confirmationImpact }}</p>
        <dl>
          <div><dt>目标状态</dt><dd>{{ confirmationTarget }}</dd></div>
          <div><dt>资源结果</dt><dd>{{ confirmationResources }}</dd></div>
          <div v-if="pendingAction === 'approve'"><dt>设计判定</dt><dd>{{ modeOverview }}</dd></div>
        </dl>
        <div class="confirm-actions">
          <button ref="confirmCancelButton" class="quiet-button" :disabled="busy" @click="cancelConfirmation">取消</button>
          <button class="primary" :disabled="busy" @click="confirmAction">{{ busy ? '处理中…' : pendingAction === 'approve' ? '确认通过并结单' : '确认执行' }}</button>
        </div>
      </section>
    </div>

    <ImagePreviewLightbox
      v-model="referenceLightboxOpen"
      :items="referenceLightboxItems"
      :initial-index="referenceLightboxIndex"
      aria-label="审核参考图预览"
      fallback-title="运营参考图"
    />
  </section>
</template>

<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import {
  Boxes,
  CheckCircle2,
  CircleCheckBig,
  CopyCheck,
  Download,
  FilePenLine,
  FileText,
  Images,
  Info,
  LockKeyhole,
  RotateCcw,
  Send,
  ShieldCheck,
  UploadCloud,
} from 'lucide-vue-next'
import AssetPreviewMedia from '@/components/media/AssetPreviewMedia.vue'
import ImagePreviewLightbox from '@/components/media/ImagePreviewLightbox.vue'
import type { ImagePreviewLightboxItem } from '@/components/media/imagePreviewLightbox'
import { uploadTaskFileViaAssetSession } from '@/services/upload/assetUploadFlow'
import { resourceGroupsApi, type ResourceBundle, type ResourceGroup, type ResourceGroupSubmission, type ResourceMode, type ResourceReference } from '@/services/api/resourceGroupsApi'
import type { ReferenceFileRef } from '@/services/api/assetsApi'
import { tasksApi } from '@/services/api/tasksApi'
import { buildSourceBundleFile, expandFinalUploadFiles } from '@/domain/resource-workflow-files'
import { downloadAssetFileWithOriginalFilename } from '@/utils/assetFileDownload'

type UploadedFile = { id: number; name: string; inherited?: boolean }
type EditorRow = {
  group: ResourceGroup
  mode: ResourceMode
  submittedSource: UploadedFile | null
  source: UploadedFile | null
  sourceFiles: File[]
  sourceMemberNames: string[]
  finals: UploadedFile[]
  uploading: string
}

type WorkflowReference = (ResourceReference | ReferenceFileRef) & { preview_url?: string | null }

const props = defineProps<{ taskId: number; taskType: string; businessLane?: string; bundle: ResourceBundle; referenceCount?: number; taskReferences?: ReferenceFileRef[]; skuModeHints?: Record<string, boolean>; allowedActions: string[] }>()
const emit = defineEmits<{ updated: [bundle: ResourceBundle]; 'dirty-change': [dirty: boolean] }>()
const skuModeHints = computed(() => props.skuModeHints ?? {})
const rows = ref<EditorRow[]>([])
const changedGroups = ref(new Set<number>())
const replaceSourceGroups = ref(new Set<number>())
const busy = ref(false)
const downloadingSourceID = ref<number | null>(null)
const error = ref('')
const success = ref('')
const reason = ref('')
const pendingAction = ref<'approve' | 'return_to_design' | 'reopen' | null>(null)
const confirmDialog = ref<HTMLElement | null>(null)
const confirmCancelButton = ref<HTMLButtonElement | null>(null)
const editorViewport = ref<HTMLElement | null>(null)
const editorScrollTop = ref(0)
const editorViewportHeight = ref(560)
const editorRowHeight = ref(330)
const editorOverscan = 2
const reopenTarget = ref<'design' | 'audit' | 'retouch'>('audit')
const referenceLightboxOpen = ref(false)
const referenceLightboxIndex = ref(0)
let confirmationTrigger: HTMLElement | null = null
let dragged: { groupIndex: number; index: number } | null = null

const actionSet = computed(() => new Set(props.allowedActions || []))
const canSubmit = computed(() => actionSet.value.has('submit_design') || actionSet.value.has('task.design.submit'))
const canReturnToDesign = computed(() => actionSet.value.has('task.audit.return_to_design') || actionSet.value.has('task.audit.decision'))
const canApprove = computed(() => actionSet.value.has('task.audit.approve') || actionSet.value.has('task.audit.decision'))
const canAudit = computed(() => canReturnToDesign.value || canApprove.value)
const canReopen = computed(() => actionSet.value.has('reopen') || actionSet.value.has('task.reopen'))
const isRetouch = computed(() => ['retouch', 'retouch_task'].includes(props.taskType.toLowerCase()))
const isCustomization = computed(() => props.businessLane === 'customization' || ['regular_customization', 'customer_customization'].includes(props.taskType.toLowerCase()))
const isDesignStage = computed(() => canSubmit.value && !isRetouch.value)
const isRetouchStage = computed(() => canSubmit.value && isRetouch.value && !canReopen.value)
const isAuditStage = computed(() => canAudit.value)
const phase = computed(() => isAuditStage.value ? 'audit' : isDesignStage.value ? 'design' : isRetouchStage.value ? 'retouch' : 'read')
const showEditor = computed(() => isDesignStage.value || isAuditStage.value || isRetouchStage.value)
const canChooseMode = computed(() => isDesignStage.value || isAuditStage.value || isRetouchStage.value)
const canBulkApplyMode = computed(() => isDesignStage.value || isAuditStage.value)
const sourceRequired = computed(() => !isRetouch.value)
const heading = computed(() => isAuditStage.value ? '审核定稿' : isDesignStage.value ? '设计提交' : canReopen.value ? '修改已结单文件' : isRetouchStage.value ? '提交修图成品' : '任务资源')
const headingHint = computed(() => isAuditStage.value ? '审核人员依据设计判定和运营参考图上传最终成品，必要时替换源文件。' : isDesignStage.value ? '先确定每个 SKU 的单图或套装模式，再提交一份可编辑源文件。' : canReopen.value ? '先重开到需要修改的阶段；现有文件继续保留，重新定稿后形成可追溯的新版本。' : '参考图、有效源文件与最终成品保持在同一资源链中。')
const finalStageHint = computed(() => isDesignStage.value ? '审核人员上传最终成品' : isAuditStage.value ? '按设计判定上传定稿' : '最终资源')
const displayReferenceCount = computed(() => {
  if (Number.isSafeInteger(props.referenceCount) && Number(props.referenceCount) >= 0) return Number(props.referenceCount)
  return props.bundle.groups.reduce((total, group) => total + ((group.working_revision || group.finalized_revision)?.references?.length || 0), 0)
})
const auditReferences = computed<WorkflowReference[]>(() => {
  const seen = new Set<string>()
  const result: WorkflowReference[] = []
  const append = (file: WorkflowReference, index: number) => {
    const identity = referenceIdentity(file, index)
    if (seen.has(identity)) return
    seen.add(identity)
    result.push(file)
  }
  ;(props.taskReferences || []).forEach((file, index) => append(file, index))
  props.bundle.groups.forEach((group, groupIndex) => {
    const revision = group.working_revision || group.finalized_revision
    ;(revision?.references || []).forEach((file, index) => append(file, (groupIndex + 1) * 1000 + index))
  })
  return result
})
const previewableAuditReferences = computed(() => auditReferences.value.filter((file) => referencePreviewable(file) && referencePreview(file)))
const referenceLightboxItems = computed<ImagePreviewLightboxItem[]>(() => previewableAuditReferences.value.map((file) => ({
  src: referencePreview(file),
  title: referenceName(file),
  alt: referenceName(file),
  downloadUrl: String(file.download_url || '') || undefined,
  fallbackAssetId: referenceTaskAssetID(file) || undefined,
})))
const editorTotalHeight = computed(() => rows.value.length * editorRowHeight.value)
const editorVisibleStart = computed(() => Math.max(0, Math.floor(editorScrollTop.value / editorRowHeight.value) - editorOverscan))
const editorVisibleCount = computed(() => Math.ceil(editorViewportHeight.value / editorRowHeight.value) + editorOverscan * 2)
const visibleEditorRows = computed(() => rows.value.slice(editorVisibleStart.value, editorVisibleStart.value + editorVisibleCount.value).map((row, offset) => ({ row, groupIndex: editorVisibleStart.value + offset })))
const editorWindowOffset = computed(() => editorVisibleStart.value * editorRowHeight.value)
const isDirty = computed(() => changedGroups.value.size > 0 || rows.value.some((row) => Boolean(row.uploading)))
const dirtyLabel = computed(() => {
  if (isDirty.value) return '当前修改尚未提交'
  if (isDesignStage.value) {
    const missing = rows.value.filter((row) => !row.source).length
    return missing ? `还需上传 ${missing} 份设计源文件` : '设计源文件与模式已就绪'
  }
  if (isAuditStage.value) {
    const missingSources = rows.value.filter((row) => !row.source).length
    if (missingSources) return `还需补充 ${missingSources} 份有效源文件`
    const unfinished = rows.value.filter((row) => !validFinals(row)).length
    return unfinished ? `还需完成 ${unfinished} 个 SKU 的定稿` : '源文件与最终成品已就绪'
  }
  if (isRetouchStage.value) {
    const unfinished = rows.value.filter((row) => !validFinals(row)).length
    return unfinished ? `还需完成 ${unfinished} 项修图定稿` : '修图成品已就绪'
  }
  return '任务资源已就绪'
})
const modeOverview = computed(() => `${rows.value.filter((row) => row.mode === 'single').length} 个单图，${rows.value.filter((row) => row.mode === 'set').length} 个套装`)
const readyCount = computed(() => rows.value.filter((row) => {
  if (isDesignStage.value) return Boolean(row.source) && ['single', 'set'].includes(row.mode)
  if (isAuditStage.value) return Boolean(row.source) && validFinals(row)
  if (isRetouchStage.value) return validFinals(row)
  return true
}).length)
const workspaceProgressLabel = computed(() => `${readyCount.value} / ${rows.value.length} 个 SKU 已就绪`)

watch(() => props.bundle, buildRows, { immediate: true, deep: true })
watch(isDirty, (dirty) => emit('dirty-change', dirty), { immediate: true })

function buildRows() {
  rows.value = props.bundle.groups.map((group) => {
    const revision = group.working_revision || group.finalized_revision
    const preserveFinals = !isAuditStage.value || revision?.source_stage === 'audit'
    const submittedSource = revision?.source_file ? { id: revision.source_file.task_asset_id, name: revision.source_file.file_name, inherited: true } : null
    return {
      group,
      mode: revision?.mode || 'single',
      submittedSource,
      source: submittedSource ? { ...submittedSource } : null,
      sourceFiles: [],
      sourceMemberNames: [],
      finals: preserveFinals ? [...(revision?.items || [])].sort((a,b) => a.sort_order - b.sort_order).map((item) => ({ id: item.task_asset_id, name: item.file?.file_name || item.item_name || `文件 ${item.task_asset_id}`, inherited: true })) : [],
      uploading: '',
    }
  })
  changedGroups.value = new Set()
  replaceSourceGroups.value = new Set()
  editorScrollTop.value = 0
  if (editorViewport.value) editorViewport.value.scrollTop = 0
}

function refreshEditorMetrics() {
  editorRowHeight.value = typeof window.matchMedia === 'function' && window.matchMedia('(max-width: 780px)').matches ? 570 : 330
  editorViewportHeight.value = editorViewport.value?.clientHeight || Math.min(window.innerHeight * 0.56, 620)
}
function onEditorScroll() { editorScrollTop.value = editorViewport.value?.scrollTop || 0 }
function scopeLabel(group: ResourceGroup) { return group.scope_kind === 'retouch_requirement' ? `修图需求 ${group.retouch_requirement_id}` : '任务资源' }
function referenceName(file: WorkflowReference) { return String(file.file_name || ('filename' in file ? file.filename : '') || file.ref_id || '参考附件') }
function referencePreview(file: WorkflowReference) { return String(file.preview_url || file.download_url || '') }
function referenceTaskAssetID(file: WorkflowReference) { return 'formal_task_asset_id' in file && file.formal_task_asset_id ? String(file.formal_task_asset_id) : null }
function referencePreviewable(file: WorkflowReference) { return String(file.mime_type || '').startsWith('image/') || /\.(png|jpe?g|webp|gif)$/i.test(referenceName(file)) }
function referenceIdentity(file: WorkflowReference, index: number) {
  if (file.ref_id) return `ref:${file.ref_id}`
  if ('asset_id' in file && file.asset_id) return `asset:${file.asset_id}`
  if ('reference_file_ref_id' in file && file.reference_file_ref_id != null) return `reference-file-ref:${file.reference_file_ref_id}`
  if ('formal_task_asset_id' in file && file.formal_task_asset_id != null) return `task-asset:${file.formal_task_asset_id}`
  if (file.id != null) return `reference:${file.id}`
  return `fallback:${index}:${referenceName(file)}`
}
function openAuditReference(file: WorkflowReference) {
  const index = previewableAuditReferences.value.indexOf(file)
  if (index >= 0) {
    referenceLightboxIndex.value = index
    referenceLightboxOpen.value = true
    return
  }
  const url = String(file.download_url || '')
  if (url) window.open(url, '_blank', 'noopener')
}
function sourceExtension(name: string) { return name.split('.').pop()?.slice(0, 4).toUpperCase() || 'FILE' }
function markChanged(groupId: number) { changedGroups.value = new Set(changedGroups.value).add(groupId) }
function setMode(row: EditorRow, mode: ResourceMode) { if (!canChooseMode.value || row.mode === mode) return; row.mode = mode; markChanged(row.group.id) }
function applyFirstMode() { const mode = rows.value[0]?.mode; if (!mode) return; rows.value.forEach((row) => { row.mode = mode; markChanged(row.group.id) }) }
function toggleSourceReplacement(row: EditorRow) {
  const next = new Set(replaceSourceGroups.value)
  if (next.has(row.group.id)) {
    next.delete(row.group.id)
    row.source = row.submittedSource ? { ...row.submittedSource } : null
    row.sourceFiles = []
    row.sourceMemberNames = []
  } else {
    next.add(row.group.id)
    row.source = null
    row.sourceFiles = []
    row.sourceMemberNames = []
  }
  replaceSourceGroups.value = next
  markChanged(row.group.id)
}
function canUploadSource(row: EditorRow) { return isDesignStage.value || isRetouchStage.value || replaceSourceGroups.value.has(row.group.id) }
async function downloadSource(file: UploadedFile) {
  if (downloadingSourceID.value) return
  downloadingSourceID.value = file.id
  error.value = ''
  try {
    const result = await downloadAssetFileWithOriginalFilename({
      taskAssetId: String(file.id),
      preferredFilename: file.name,
    })
    if (!result.ok) error.value = result.message || '源文件下载失败。'
  } catch (cause) {
    error.value = cause instanceof Error ? cause.message : '源文件下载失败。'
  } finally {
    downloadingSourceID.value = null
  }
}
function finalRequirement(row: EditorRow) { return row.mode === 'set' ? '套装至少 2 张，可拖拽排序' : '单图恰好 1 张' }
function assetVersionId(uploaded: Awaited<ReturnType<typeof uploadTaskFileViaAssetSession>>): number {
  const raw = uploaded.version?.id || uploaded.version?.version_id
  const id = Number(raw)
  if (!Number.isSafeInteger(id) || id <= 0) throw new Error('上传完成但未返回任务资产 ID。')
  return id
}
function uploadOptions(row: EditorRow) { return row.group.retouch_requirement_id ? { retouchRequirementId: row.group.retouch_requirement_id } : undefined }
async function uploadSource(event: Event, row: EditorRow) {
  const input = event.target as HTMLInputElement
  const selected = [...(input.files || [])]
  if (!selected.length) return
  const nextSourceFiles = [...row.sourceFiles, ...selected]
  row.uploading = selected.length > 1 ? `${selected.length} 份源文件` : selected[0].name
  error.value = ''
  try {
    const uploadFile = await buildSourceBundleFile(
      nextSourceFiles,
      `${row.group.sku_code || `任务-${props.taskId}`}-设计源文件-${nextSourceFiles.length}份`,
    )
    row.uploading = uploadFile.name
    const uploaded = await uploadTaskFileViaAssetSession(String(props.taskId), uploadFile, { asset_kind: 'source', target_sku_code: row.group.sku_code || undefined, remark: nextSourceFiles.map((file) => file.name).join('、') }, uploadOptions(row))
    row.sourceFiles = nextSourceFiles
    row.sourceMemberNames = nextSourceFiles.map((file) => file.name)
    row.source = { id: assetVersionId(uploaded), name: uploadFile.name }
    markChanged(row.group.id)
  } catch (cause) { error.value = cause instanceof Error ? cause.message : '源文件上传失败。' }
  finally { row.uploading = ''; input.value = '' }
}
async function uploadFinals(event: Event, row: EditorRow) {
  const input = event.target as HTMLInputElement
  const selected = [...(input.files || [])]
  if (!selected.length) return
  error.value = ''
  try {
    row.uploading = selected.length > 1 ? `${selected.length} 个成品文件` : selected[0].name
    const files = await expandFinalUploadFiles(selected)
    if (row.mode === 'single' && files.length !== 1) {
      throw new Error('单图模式恰好需要 1 张成品；如需多张，请让设计阶段选择套装。')
    }
    const uploadedFiles: UploadedFile[] = []
    for (const file of files) {
      row.uploading = file.name
      const uploaded = await uploadTaskFileViaAssetSession(String(props.taskId), file, { asset_kind: 'delivery', target_sku_code: row.group.sku_code || undefined, remark: file.name }, uploadOptions(row))
      uploadedFiles.push({ id: assetVersionId(uploaded), name: file.name })
    }
    row.finals = row.mode === 'set' ? [...row.finals, ...uploadedFiles] : uploadedFiles
    markChanged(row.group.id)
  } catch (cause) { error.value = cause instanceof Error ? cause.message : '成品图上传失败，可重新选择该组文件。' }
  finally { row.uploading = ''; input.value = '' }
}
function move(row: EditorRow, index: number, delta: number) { const next = index + delta; if (next < 0 || next >= row.finals.length) return; const [item] = row.finals.splice(index,1); row.finals.splice(next,0,item); markChanged(row.group.id) }
function removeFinal(row: EditorRow,index:number) { row.finals.splice(index,1); markChanged(row.group.id) }
function clearFinals(row: EditorRow) { row.finals = []; markChanged(row.group.id) }
function dragStart(groupIndex:number,index:number) { dragged = { groupIndex,index } }
function drop(groupIndex:number,index:number) { if (!dragged || dragged.groupIndex !== groupIndex) return; const row = rows.value[groupIndex]; const [item] = row.finals.splice(dragged.index,1); row.finals.splice(index,0,item); markChanged(row.group.id); dragged = null }
function validFinals(row: EditorRow) { return row.mode === 'single' ? row.finals.length === 1 : row.finals.length >= 2 }
const validDesign = computed(() => rows.value.length > 0 && rows.value.every((row) => Boolean(row.source) && ['single','set'].includes(row.mode)) && !rows.value.some((row) => row.uploading))
const validAudit = computed(() => rows.value.length > 0 && rows.value.every((row) => Boolean(row.source) && validFinals(row)) && !rows.value.some((row) => row.uploading))
const validRetouch = computed(() => rows.value.length > 0 && rows.value.every((row) => validFinals(row)) && !rows.value.some((row) => row.uploading))
function submission(row: EditorRow): ResourceGroupSubmission {
  const includeFinals = isAuditStage.value || isRetouchStage.value
  const result: ResourceGroupSubmission = { group_id: row.group.id, expected_group_lock_version: row.group.lock_version, mode: row.mode, final_task_asset_ids: includeFinals ? row.finals.map((file) => file.id) : [] }
  if (row.source && (!isAuditStage.value || replaceSourceGroups.value.has(row.group.id))) result.source_task_asset_id = row.source.id
  return result
}
async function run(action: () => Promise<ResourceBundle>, message: string) { busy.value = true; error.value = ''; success.value = ''; try { const bundle = await action(); success.value = message; emit('updated', bundle) } catch (cause) { error.value = cause instanceof Error ? cause.message : '操作失败，请刷新后重试。' } finally { busy.value = false } }
async function submitDesignRequest() {
  const groups = rows.value.map(submission)
  try {
    return await resourceGroupsApi.submitDesign(props.taskId, props.bundle, groups)
  } catch (cause) {
    const needsCustomizationPreparation = isCustomization.value
      && cause instanceof Error
      && cause.message.includes('定制任务尚未完成设计准备')
    if (!needsCustomizationPreparation) throw cause
    await tasksApi.triggerModuleAction(String(props.taskId), 'customization', 'submit')
    return resourceGroupsApi.submitDesign(props.taskId, props.bundle, groups)
  }
}
async function submitDesign() { if (isRetouch.value ? !validRetouch.value : !validDesign.value) return; await run(submitDesignRequest, isRetouch.value ? '修图任务已结单。' : '设计源文件与模式已提交审核。') }
async function returnToDesign() { if (!reason.value) return; await run(() => resourceGroupsApi.auditDecision(props.taskId, props.bundle, 'return_to_design', reason.value), '已打回设计。') }
async function approve() { if (!validAudit.value) return; await run(() => resourceGroupsApi.auditDecision(props.taskId, props.bundle, 'approve', reason.value, rows.value.map(submission)), '审核通过，任务已结单。') }
async function reopen() { if (!reason.value) return; await run(() => resourceGroupsApi.reopen(props.taskId, props.bundle, reopenTarget.value, reason.value), '任务已重开。') }
const confirmationTitle = computed(() => pendingAction.value === 'approve' ? '确认审核结果' : pendingAction.value === 'return_to_design' ? '将任务打回设计？' : '重开已结单任务？')
const confirmationImpact = computed(() => pendingAction.value === 'approve' ? '确认后任务立即结单，本次定稿成为最终成品。' : pendingAction.value === 'return_to_design' ? '任务回到设计处理中，不会生成最终成品。' : '任务恢复为可处理状态，原最终资源继续有效直至再次通过。')
const confirmationTarget = computed(() => pendingAction.value === 'approve' ? '已结单' : pendingAction.value === 'return_to_design' ? '设计处理中' : reopenTarget.value === 'audit' ? '待审核' : reopenTarget.value === 'retouch' ? '修图处理中' : '设计处理中')
const confirmationResources = computed(() => pendingAction.value === 'approve' ? `保留运营参考、有效源文件与 ${rows.value.reduce((sum,row) => sum + row.finals.length,0)} 张最终成品。` : pendingAction.value === 'return_to_design' ? '保留参考资料，设计人员重新提交源文件与模式。' : '当前最终资源保持可见。')
function openConfirmation(action: 'approve' | 'return_to_design' | 'reopen') { if (busy.value) return; if ((action === 'return_to_design' || action === 'reopen') && !reason.value) { error.value = '请先填写操作原因。'; return }; confirmationTrigger = document.activeElement instanceof HTMLElement ? document.activeElement : null; pendingAction.value = action; void nextTick(() => confirmCancelButton.value?.focus()) }
function closeConfirmation() { pendingAction.value = null; void nextTick(() => confirmationTrigger?.focus()) }
function cancelConfirmation() { if (!busy.value) closeConfirmation() }
function trapConfirmationFocus(event: KeyboardEvent) { const controls = [...(confirmDialog.value?.querySelectorAll<HTMLElement>('button:not([disabled]), [href], input:not([disabled]), select:not([disabled]), [tabindex]:not([tabindex="-1"])') || [])]; if (!controls.length) return; const first = controls[0]; const last = controls[controls.length - 1]; if (event.shiftKey && document.activeElement === first) { event.preventDefault(); last.focus() } else if (!event.shiftKey && document.activeElement === last) { event.preventDefault(); first.focus() } }
async function confirmAction() { if (!pendingAction.value || busy.value) return; const action = pendingAction.value; if (action === 'approve') await approve(); else if (action === 'return_to_design') await returnToDesign(); else await reopen(); if (!error.value) closeConfirmation() }
onMounted(() => { refreshEditorMetrics(); window.addEventListener('resize', refreshEditorMetrics) })
onBeforeUnmount(() => { window.removeEventListener('resize', refreshEditorMetrics); emit('dirty-change', false) })
</script>

<style scoped>
.resource-workspace{display:grid;grid-template-rows:auto auto auto minmax(0,1fr) auto;min-height:0;border:1px solid rgb(var(--yb-border));border-radius:18px;background:rgb(var(--yb-surface));overflow:hidden;box-shadow:0 18px 44px rgb(var(--yb-shadow)/.05)}
.workspace-head{display:flex;align-items:flex-start;justify-content:space-between;gap:18px;padding:18px 20px 14px}.workspace-head h2{margin:2px 0 4px;font-size:22px}.workspace-head p{margin:0;color:rgb(var(--yb-text-muted));font-size:13px}.section-label{font-size:11px;font-weight:800;letter-spacing:.08em;color:rgb(var(--yb-brand))}
.stage-map{display:grid;grid-template-columns:repeat(3,1fr);border-block:1px solid rgb(var(--yb-border));background:linear-gradient(90deg,rgb(var(--yb-brand-soft)/.45),rgb(var(--yb-surface)) 48%)}.stage-node{position:relative;display:flex;align-items:center;gap:10px;min-height:66px;padding:10px 18px;color:rgb(var(--yb-text-muted))}.stage-node:not(:last-child)::after{content:"";position:absolute;right:-8px;width:16px;height:1px;background:rgb(var(--yb-border-strong))}.stage-node.active{color:rgb(var(--yb-text));background:rgb(var(--yb-brand-soft)/.45)}.stage-node.complete{color:rgb(var(--yb-text))}.stage-index{display:grid;place-items:center;width:28px;height:28px;border:1px solid rgb(var(--yb-border-strong));border-radius:50%;font-weight:800}.active .stage-index{border-color:rgb(var(--yb-brand));background:rgb(var(--yb-brand));color:rgb(var(--yb-text-inverse))}.complete .stage-index{border-color:rgb(var(--yb-brand));color:rgb(var(--yb-brand))}.stage-node div{display:grid;gap:3px}.stage-node small{font-size:11px;color:rgb(var(--yb-text-muted))}
.contract-note{display:flex;gap:10px;align-items:center;margin:12px 18px 0;padding:10px 12px;border:1px solid rgb(var(--yb-brand-border));border-radius:11px;background:rgb(var(--yb-brand-soft)/.45);font-size:12px}.contract-note span{color:rgb(var(--yb-text-muted))}.audit-note{border-color:rgb(var(--yb-success-border));background:rgb(var(--yb-success-soft))}
.editor-viewport{min-height:260px;overflow:auto;overscroll-behavior:contain;margin:12px 18px;background:rgb(var(--yb-surface-soft));border-radius:14px}.editor-spacer{position:relative}.editor-window{position:absolute;inset:0 0 auto}.sku-workbench{box-sizing:border-box;height:calc(var(--editor-row-height) - 10px);margin:5px;padding:14px;border:1px solid rgb(var(--yb-border));border-radius:13px;background:rgb(var(--yb-surface));overflow:auto}.sku-head,.column-title{display:flex;align-items:center;justify-content:space-between;gap:12px}.sku-head>div:first-child,.column-title>div{display:grid;gap:3px}.sku-head span,.column-title small{font-size:11px;color:rgb(var(--yb-text-muted))}.sku-head .operations-hint{width:max-content;padding:3px 7px;border-radius:999px;background:rgb(var(--yb-warning-soft));color:rgb(var(--yb-warning-strong));font-weight:750}.mode-control{display:flex;align-items:center;gap:4px}.mode-control>span{margin-right:4px}.mode-control button{min-height:32px;padding:0 11px;border:1px solid rgb(var(--yb-border));background:rgb(var(--yb-surface));color:rgb(var(--yb-text-muted));cursor:pointer}.mode-control button:first-of-type{border-radius:8px 0 0 8px}.mode-control button:last-of-type{margin-left:-5px;border-radius:0 8px 8px 0}.mode-control button.selected{position:relative;z-index:1;border-color:rgb(var(--yb-brand));background:rgb(var(--yb-brand-soft));color:rgb(var(--yb-brand-strong));font-weight:800}.mode-control button:disabled{cursor:default;opacity:1}
.resource-columns{display:grid;grid-template-columns:1fr 1.08fr;gap:10px;margin-top:12px}.source-column,.final-column{display:grid;align-content:start;gap:10px;min-height:185px;padding:12px;border:1px solid rgb(var(--yb-border));border-radius:11px}.final-column.locked{background:rgb(var(--yb-surface-soft))}.replace-toggle{display:flex;align-items:center;gap:6px;font-size:11px;color:rgb(var(--yb-text-muted))}.file-tile{display:flex;align-items:center;gap:10px;padding:11px;border-radius:10px;background:rgb(var(--yb-surface-muted))}.file-tile div{display:grid;gap:3px;min-width:0;flex:1}.file-tile strong{overflow:hidden;text-overflow:ellipsis;white-space:nowrap}.file-tile small{font-size:11px;color:rgb(var(--yb-text-muted))}.source-download{display:inline-flex;align-items:center;gap:4px;padding:7px 9px;border:1px solid rgb(var(--yb-border));border-radius:8px;background:rgb(var(--yb-surface));color:rgb(var(--yb-brand-strong));font-size:11px;font-weight:750;cursor:pointer}.source-download:disabled{opacity:.55;cursor:wait}.file-mark{display:grid;place-items:center;min-width:42px;height:42px;border-radius:9px;background:rgb(var(--yb-surface-neutral-inverse-deep));color:rgb(var(--yb-text-inverse));font-size:10px;font-weight:900}.drop-zone{display:grid;place-items:center;min-height:56px;padding:10px;border:1px dashed rgb(var(--yb-brand-border));border-radius:10px;color:rgb(var(--yb-brand-strong));font-size:12px;cursor:pointer}.drop-zone input{position:absolute;width:1px;height:1px;opacity:0}.locked-final{display:flex;align-items:center;gap:10px;min-height:92px;padding:12px;border-radius:10px;background:rgb(var(--yb-surface-muted))}.locked-final div{display:grid;gap:4px}.locked-final small{color:rgb(var(--yb-text-muted))}.lock-symbol{display:grid;place-items:center;width:36px;height:36px;border-radius:50%;background:rgb(var(--yb-brand-soft));color:rgb(var(--yb-brand))}.mode-summary{padding:4px 8px;border-radius:8px;background:rgb(var(--yb-brand-soft));color:rgb(var(--yb-brand-strong));font-size:11px;font-weight:800}.final-order{display:grid;gap:5px;margin:0;padding:0;list-style:none}.final-order li{display:grid;grid-template-columns:24px minmax(0,1fr) auto;align-items:center;gap:7px;padding:7px;border-radius:8px;background:rgb(var(--yb-surface-muted))}.final-order li>span{display:grid;place-items:center;width:22px;height:22px;border-radius:7px;background:rgb(var(--yb-brand-soft));color:rgb(var(--yb-brand-strong));font-size:11px}.final-order strong{overflow:hidden;text-overflow:ellipsis;white-space:nowrap;font-size:12px}.final-order button{border:0;background:transparent;cursor:pointer}.uploading{margin-top:8px;color:rgb(var(--yb-brand));font-size:12px}
.command-dock{display:flex;align-items:center;justify-content:space-between;gap:14px;padding:12px 18px;border-top:1px solid rgb(var(--yb-border));background:rgb(var(--yb-surface))}.command-dock>div:first-child{display:grid;gap:2px}.command-dock span{font-size:11px;color:rgb(var(--yb-text-muted))}.command-dock label{display:grid;gap:4px;flex:1}.command-dock input,.command-dock select{min-height:39px;border:1px solid rgb(var(--yb-border));border-radius:9px;padding:0 10px;background:rgb(var(--yb-surface));color:rgb(var(--yb-text))}.dock-actions{display:flex;gap:8px}.primary,.quiet-button,.danger{min-height:42px;padding:0 17px;border-radius:10px;font-weight:750;cursor:pointer}.primary{border:0;background:rgb(var(--yb-brand));color:rgb(var(--yb-text-inverse));box-shadow:0 8px 18px rgb(var(--yb-brand)/.16)}.quiet-button{border:1px solid rgb(var(--yb-border));background:rgb(var(--yb-surface));color:rgb(var(--yb-text))}.danger{border:1px solid rgb(var(--yb-danger-border));background:rgb(var(--yb-surface));color:rgb(var(--yb-danger-text))}.primary:disabled,.quiet-button:disabled,.danger:disabled{opacity:.45;cursor:not-allowed}.message{margin:10px 18px 0;padding:10px 12px;border-radius:10px}.error{background:rgb(var(--yb-danger-soft));color:rgb(var(--yb-danger-text))}.success{background:rgb(var(--yb-success-soft));color:rgb(var(--yb-success-strong))}
.confirm-backdrop{position:fixed;inset:0;z-index:90;display:grid;place-items:center;padding:20px;background:rgb(var(--yb-overlay-night)/.48)}.confirm-dialog{position:relative;width:min(610px,100%);display:grid;gap:13px;padding:24px;border:1px solid rgb(var(--yb-border));border-radius:18px;background:rgb(var(--yb-surface));box-shadow:0 26px 70px rgb(var(--yb-shadow)/.24)}.confirm-dialog h3{margin:0;font-size:24px}.confirm-dialog>p{margin:0;color:rgb(var(--yb-text-muted))}.dialog-close{position:absolute;top:14px;right:14px;width:34px;height:34px;border:0;border-radius:9px;background:rgb(var(--yb-surface-muted));cursor:pointer}.confirm-dialog dl{display:grid;gap:7px;margin:0;padding:12px;border-radius:11px;background:rgb(var(--yb-surface-soft))}.confirm-dialog dl div{display:grid;grid-template-columns:80px 1fr;gap:12px}.confirm-dialog dt{color:rgb(var(--yb-text-muted))}.confirm-dialog dd{margin:0}.confirm-actions{display:flex;justify-content:flex-end;gap:8px}
@media(max-width:780px){.resource-workspace{border-radius:14px}.workspace-head{padding:15px}.stage-map{grid-template-columns:1fr}.stage-node{min-height:54px}.stage-node:not(:last-child)::after{left:31px;right:auto;bottom:-9px;width:1px;height:18px}.contract-note{align-items:flex-start;flex-direction:column}.editor-viewport{margin:10px;height:min(57vh,620px)}.resource-columns{grid-template-columns:1fr}.sku-head{align-items:flex-start;flex-direction:column}.mode-control{width:100%}.mode-control button{flex:1}.command-dock,.audit-dock,.reopen-dock{align-items:stretch;flex-direction:column}.dock-actions{display:grid;grid-template-columns:1fr 1.5fr}.dock-actions button,.command-dock>.primary{width:100%}.confirm-backdrop{align-items:end;padding:0}.confirm-dialog{max-height:92vh;overflow:auto;border-radius:20px 20px 0 0}.stage-node small{max-width:32ch}}
@media(prefers-reduced-motion:no-preference){.stage-node.active .stage-index{animation:stage-pulse 2.8s ease-in-out infinite}@keyframes stage-pulse{50%{box-shadow:0 0 0 7px rgb(var(--yb-brand-soft)/.8)}}}
</style>

<style scoped>
.resource-workspace{display:flex;height:100%;flex-direction:column;border:0;border-radius:13px;background:rgb(var(--yb-surface-soft));box-shadow:none}
.workspace-head{align-items:center;padding:14px 16px;border-bottom:1px solid rgb(var(--yb-border));background:rgb(var(--yb-surface))}
.workspace-identity,.workspace-tools{display:flex;align-items:center;gap:11px}
.workspace-identity>div{display:grid;gap:3px}
.workspace-mark{display:grid;width:38px;height:38px;place-items:center;border:1px solid rgb(var(--yb-brand-border));border-radius:10px;background:rgb(var(--yb-brand-soft));color:rgb(var(--yb-brand))}
.workspace-mark.is-audit{border-color:rgb(var(--yb-success-border));background:rgb(var(--yb-success-soft));color:rgb(var(--yb-success-strong))}
.workspace-head h2{margin:0;font-size:19px;line-height:1.2}.workspace-head p{font-size:12px}.workspace-count{display:inline-flex;align-items:center;gap:6px;color:rgb(var(--yb-text-muted));font-size:11px;font-weight:750}
.workspace-tools .quiet-button{display:inline-flex;align-items:center;gap:6px;min-height:34px;padding-inline:11px}
.stage-map{border-top:0;background:rgb(var(--yb-surface));padding-inline:8px}
.stage-node{min-height:60px;padding:9px 15px}.stage-node:not(:last-child)::after{right:0;width:1px;height:34px;background:rgb(var(--yb-border))}.stage-node.active{background:rgb(var(--yb-brand-soft)/.58)}.stage-node.active::before{content:"";position:absolute;right:12px;bottom:0;left:12px;height:2px;border-radius:999px;background:rgb(var(--yb-brand))}.stage-node.complete .stage-index{border-color:rgb(var(--yb-success-border));background:rgb(var(--yb-success-soft));color:rgb(var(--yb-success-strong))}.stage-node.locked{opacity:.58}.stage-node strong{font-size:12px}
.contract-note{display:grid;grid-template-columns:auto auto minmax(0,1fr);margin:10px 12px 0;padding:9px 11px;border-radius:9px;background:rgb(var(--yb-brand-soft));line-height:1.45}.contract-note>svg{color:rgb(var(--yb-brand))}.contract-note strong{font-size:12px}.contract-note span{font-size:11px}.audit-note>svg{color:rgb(var(--yb-success-strong))}
.audit-references{display:grid;gap:8px;margin:10px 12px 0;padding:10px 11px;border:1px solid rgb(var(--yb-border));border-radius:10px;background:rgb(var(--yb-surface))}.audit-references>header{display:flex;align-items:center;justify-content:space-between;gap:12px}.audit-references>header>div{display:grid;gap:2px}.audit-references>header strong{font-size:12px}.audit-references>header small,.audit-references>header>span{color:rgb(var(--yb-text-muted));font-size:10px}.audit-reference-strip{display:flex;gap:8px;overflow-x:auto;padding-bottom:2px}.audit-reference-strip>button{display:grid;grid-template-rows:64px auto;gap:4px;flex:0 0 104px;min-width:0;border:1px solid rgb(var(--yb-border));border-radius:8px;padding:5px;background:rgb(var(--yb-surface-soft));color:rgb(var(--yb-text));text-align:left;cursor:pointer}.audit-reference-strip>button>svg{box-sizing:border-box;width:100%;height:64px;padding:18px;color:rgb(var(--yb-text-muted))}.audit-reference-strip>button>span{overflow:hidden;text-overflow:ellipsis;white-space:nowrap;font-size:9px}.audit-reference-media{pointer-events:none}.audit-reference-strip :deep(.audit-reference-shell){width:100%;height:64px;border-radius:6px;object-fit:cover}.audit-reference-strip :deep(.audit-reference-image){width:100%;height:100%;border-radius:inherit;object-fit:cover}
.editor-viewport{height:auto;min-height:0;flex:1 1 auto;margin:10px 12px;border:1px solid rgb(var(--yb-border));border-radius:12px;background:rgb(var(--yb-surface-soft));box-shadow:inset 0 1px 0 rgb(var(--yb-surface))}
.sku-workbench{margin:6px;height:calc(var(--editor-row-height) - 12px);padding:14px 15px;border-radius:11px;box-shadow:0 3px 12px rgb(var(--yb-shadow)/.04)}
.sku-head{padding-bottom:11px;border-bottom:1px solid rgb(var(--yb-border))}.sku-head>div:first-child>span{font-size:10px;font-weight:750;letter-spacing:.06em}.sku-head strong{font:800 14px var(--yb-font-data)}.sku-head .sku-product-name{max-width:48rem;overflow:hidden;text-overflow:ellipsis;white-space:nowrap;color:rgb(var(--yb-text));font-size:12px;font-weight:700}.sku-head .operations-hint{border-radius:7px}
.mode-control>span{display:inline-flex;align-items:center;gap:5px;font-size:10px}.mode-control button{min-width:68px;min-height:34px}.mode-control button.selected{box-shadow:0 1px 4px rgb(var(--yb-brand)/.12)}
.resource-columns{gap:12px}.source-column,.final-column{min-height:190px;border-radius:10px;background:rgb(var(--yb-surface))}.final-column.locked{border-style:dashed;background:rgb(var(--yb-surface-soft))}.column-title strong{font-size:13px}.replace-toggle{font-weight:650}
.file-tile{border:1px solid rgb(var(--yb-border));background:rgb(var(--yb-surface-soft))}.file-tile>div{min-width:0}.bundle-members{display:block;max-width:34rem;overflow:hidden;text-overflow:ellipsis;white-space:nowrap}.file-ready{flex:0 0 auto;margin-left:auto;color:rgb(var(--yb-success-strong))}.drop-zone{grid-template-columns:auto auto;gap:8px;min-height:62px;background:rgb(var(--yb-surface));font-weight:700}.drop-zone:hover{border-color:rgb(var(--yb-brand));background:rgb(var(--yb-brand-soft)/.34)}.clear-finals{justify-self:end;margin-top:7px;border:0;background:transparent;color:rgb(var(--yb-danger-text));font-size:11px;font-weight:700;cursor:pointer}.clear-finals:disabled{opacity:.5;cursor:not-allowed}.locked-final{min-height:104px;border:1px dashed rgb(var(--yb-border));background:rgb(var(--yb-surface-muted))}.locked-final strong{font-size:13px}.lock-symbol{border-radius:10px}
.final-order li{min-height:38px;border:1px solid rgb(var(--yb-border));background:rgb(var(--yb-surface))}
.command-dock{position:relative;z-index:2;min-height:72px;padding:11px 16px;border-top:1px solid rgb(var(--yb-border-strong));background:rgb(var(--yb-surface));box-shadow:0 -8px 22px rgb(var(--yb-shadow)/.05)}.dock-progress strong{font:850 14px var(--yb-font-data)}.command-dock span{font-size:10px}.primary,.quiet-button,.danger{display:inline-flex;align-items:center;justify-content:center;gap:7px;min-height:40px;border-radius:9px;font-size:12px}.primary.approve{background:rgb(var(--yb-success-strong));box-shadow:0 6px 15px rgb(var(--yb-success-strong)/.18)}.danger{border-color:rgb(var(--yb-danger-border));background:rgb(var(--yb-danger-soft)/.18);font-weight:760}
.audit-dock>label{max-width:560px}.audit-dock input{background:rgb(var(--yb-surface-soft))}
@media(max-width:780px){.workspace-head,.workspace-tools{align-items:flex-start}.workspace-head{flex-direction:column}.workspace-tools{width:100%;justify-content:space-between}.contract-note{grid-template-columns:auto 1fr}.contract-note span{grid-column:1/-1}.editor-viewport{height:auto}.command-dock{min-height:0}.dock-progress{display:grid}.dock-actions{grid-template-columns:1fr 1.45fr}}
</style>
