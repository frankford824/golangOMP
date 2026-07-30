<template>
  <section class="resource-rail" aria-label="任务资源链路">
    <header>
      <div><p>任务资源链路</p><h2>参考图、有效源文件与最终成品</h2></div>
      <button type="button" @click="$emit('openResources')">查看全部文件 <ArrowRight :size="15" aria-hidden="true" /></button>
    </header>
    <div class="rail-grid">
      <article class="rail-column references">
        <div class="column-head"><span>01</span><div><strong>运营参考图</strong><small>{{ resourceReferences.length }} 个附件</small></div><button type="button" @click="openAllReferences">预览全部</button></div>
        <div v-if="resourceReferences.length" class="media-strip">
          <button v-for="(file,index) in resourceReferences.slice(0,4)" :key="referenceKey(file,index)" type="button" @click="openReference(file)">
            <AssetPreviewMedia
              v-if="referencePreviewable(file) && referencePreview(file)"
              class="resource-preview"
              :task-asset-id="taskAssetID(file)"
              :fallback-src="referencePreview(file)"
              :alt="referenceName(file)"
              img-class="resource-preview-shell"
              inner-img-class="resource-preview-img"
            />
            <FileText v-else :size="23" aria-hidden="true" />
            <span>{{ referenceName(file) }}</span>
          </button>
        </div>
        <p v-else class="empty-copy">暂无参考附件</p>
      </article>

      <ArrowRight class="rail-arrow" :size="20" aria-hidden="true" />

      <article class="rail-column sources">
        <div class="column-head"><span>02</span><div><strong>当前有效源文件</strong><small>{{ sourceSummary }}</small></div></div>
        <div v-if="sources.length" class="file-strip">
          <div v-for="source in sources.slice(0,4)" :key="source.task_asset_id"><FileArchive :size="23" aria-hidden="true" /><span><strong>{{ source.file_name }}</strong><small>{{ sourceStageLabel(source.stage) }}</small></span></div>
        </div>
        <p v-else class="empty-copy">{{ sourceEmptyCopy }}</p>
      </article>

      <ArrowRight class="rail-arrow" :size="20" aria-hidden="true" />

      <article class="rail-column finals" :class="{ locked: finalLocked }">
        <div class="column-head"><span>03</span><div><strong>最终成品图</strong><small>{{ finalCount }} 张 · {{ setCount ? `${setCount} 个套装` : '单图成品' }}</small></div><LockKeyhole v-if="finalLocked" :size="16" aria-label="设计阶段暂不可上传" /></div>
        <div v-if="finals.length" class="media-strip">
          <button v-for="item in finals.slice(0,4)" :key="item.id" type="button" @click="$emit('openResources')">
            <AssetPreviewMedia
              v-if="finalPreviewable(item) && finalPreview(item)"
              class="resource-preview"
              :task-asset-id="String(item.task_asset_id)"
              :fallback-src="finalPreview(item)"
              :alt="item.item_name || item.file?.file_name || '最终成品'"
              img-class="resource-preview-shell"
              inner-img-class="resource-preview-img"
            />
            <Images v-else :size="23" aria-hidden="true" />
            <span>{{ item.item_name || item.file?.file_name || `成品 ${item.sort_order + 1}` }}</span>
          </button>
        </div>
        <div v-else-if="finalLocked" class="locked-copy"><LockKeyhole :size="22" aria-hidden="true" /><span><strong>设计阶段不上传成品</strong><small>审核人员会按设计判定的单图或套装模式完成定稿。</small></span></div>
        <p v-else class="empty-copy">等待审核人员上传最终成品</p>
      </article>
    </div>
    <footer v-if="canOperate"><span>{{ actionHint }}</span><button type="button" @click="$emit('openWorkflow')">{{ actionLabel }} <ArrowRight :size="15" aria-hidden="true" /></button></footer>
  </section>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { ArrowRight, FileArchive, FileText, Images, LockKeyhole } from 'lucide-vue-next'
import AssetPreviewMedia from '@/components/media/AssetPreviewMedia.vue'
import type { ResourceBundle, ResourceFile, ResourceReference, ResourceRevisionItem } from '@/services/api/resourceGroupsApi'
import type { ReferenceFileRef } from '@/services/api/assetsApi'

type SourceWithStage = ResourceFile & { stage: string }
type RailReference = (ResourceReference | ReferenceFileRef) & { source?: 'task' | 'resource'; preview_url?: string | null }
const props = defineProps<{ bundle: ResourceBundle; taskStatus: string; taskType: string; taskReferences?: ReferenceFileRef[]; canOperate?: boolean; actionLabel?: string }>()
const emit = defineEmits<{ openResources: []; openAttachments: []; openWorkflow: [] }>()
const groups = computed(() => props.bundle.groups || [])
const resourceReferences = computed<RailReference[]>(() => {
  const seen = new Set<string>()
  const references: RailReference[] = []
  ;(props.taskReferences || []).forEach((reference, referenceIndex) => {
    const normalized = { ...reference, source: 'task' as const }
    const identity = referenceIdentity(normalized, -1, referenceIndex)
    if (seen.has(identity)) return
    seen.add(identity)
    references.push(normalized)
  })
  groups.value.forEach((group, groupIndex) => {
    const revision = group.finalized_revision || group.working_revision
    const ordered = [...(revision?.references || [])].sort((left, right) =>
      Number(left.sort_order || 0) - Number(right.sort_order || 0)
      || Number(left.id || 0) - Number(right.id || 0),
    )
    ordered.forEach((reference, referenceIndex) => {
      const normalized = { ...reference, source: 'resource' as const }
      const identity = referenceIdentity(normalized, groupIndex, referenceIndex)
      if (seen.has(identity)) return
      seen.add(identity)
      references.push(normalized)
    })
  })
  return references
})
const sources = computed<SourceWithStage[]>(() => groups.value.flatMap((group) => {
  const revision = group.finalized_revision || group.working_revision
  return revision?.source_file ? [{ ...revision.source_file, stage: revision.source_stage }] : []
}))
const isRetouch = computed(() => ['retouch', 'retouch_task'].includes(props.taskType))
const sourceSummary = computed(() => isRetouch.value
  ? `${sources.value.length} 个源文件 · ${groups.value.length} 个修图范围`
  : `${sources.value.length} / ${groups.value.length} 个 SKU 已提交`)
const sourceEmptyCopy = computed(() => isRetouch.value
  ? '修图任务无需独立源文件，以参考图与最终成品为准'
  : '设计人员尚未提交源文件')
const finals = computed<ResourceRevisionItem[]>(() => groups.value.flatMap((group) => group.finalized_revision?.items || []))
const finalCount = computed(() => finals.value.length)
const setCount = computed(() => groups.value.filter((group) => (group.finalized_revision || group.working_revision)?.mode === 'set').length)
const finalLocked = computed(() => props.taskStatus === 'InProgress' && !finalCount.value)
const actionHint = computed(() => {
  if (props.taskStatus === 'PendingAudit') return '审核人员在这里确认模式、上传成品并决定是否替换源文件。'
  if (isRetouch.value) return '修图任务以参考图为输入，按修图范围提交最终成品；独立源文件可选。'
  return '设计人员先判定单图或套装，再为每个 SKU 提交可编辑源文件。'
})

function referenceIdentity(file: RailReference, groupIndex: number, referenceIndex: number) {
  if (file.ref_id) return `ref:${file.ref_id}`
  if ('asset_id' in file && file.asset_id) return `asset:${file.asset_id}`
  if (file.reference_file_ref_id != null) return `reference-file-ref:${file.reference_file_ref_id}`
  if (file.formal_task_asset_id != null) return `task-asset:${file.formal_task_asset_id}`
  if (file.id != null) return `revision-reference:${file.id}`
  if ('storage_key' in file && file.storage_key) return `storage:${file.storage_key}`
  return `fallback:${groupIndex}:${referenceIndex}:${referenceName(file)}`
}
function referenceName(file: RailReference) { return String(file.file_name || ('filename' in file ? file.filename : '') || file.ref_id || '参考附件') }
function referencePreview(file: RailReference) { return String(file.preview_url || file.download_url || '') }
function taskAssetID(file: RailReference) { return file.formal_task_asset_id ? String(file.formal_task_asset_id) : null }
function referenceKey(file: RailReference, index: number) { return referenceIdentity(file, 0, index) }
function referencePreviewable(file: RailReference) { return String(file.mime_type || '').startsWith('image/') || /\.(png|jpe?g|webp|gif)$/i.test(referenceName(file)) }
function openReference(file: RailReference) {
  if (file.source === 'task') emit('openAttachments')
  else emit('openResources')
}
function openAllReferences() {
  if (resourceReferences.value.some((file) => file.source === 'resource')) emit('openResources')
  else emit('openAttachments')
}
function finalPreview(item: ResourceRevisionItem) { return String(item.file?.preview_url || item.file?.download_url || '') }
function finalPreviewable(item: ResourceRevisionItem) { return String(item.file?.mime_type || '').startsWith('image/') || /\.(png|jpe?g|webp|gif)$/i.test(item.file?.file_name || item.item_name || '') }
function sourceStageLabel(stage: string) { return stage === 'audit' ? '审核确认的源文件' : stage === 'retouch' ? '修图源文件' : '设计师提交的源文件' }
</script>

<style scoped>
.resource-rail{overflow:hidden;border:1px solid rgb(var(--yb-border));border-radius:15px;background:rgb(var(--yb-surface));box-shadow:0 5px 18px rgb(var(--yb-shadow)/.045)}.resource-rail>header{display:flex;align-items:center;justify-content:space-between;gap:14px;padding:13px 15px;border-bottom:1px solid rgb(var(--yb-border))}.resource-rail header p{margin:0;color:rgb(var(--yb-brand));font-size:10px;font-weight:850;letter-spacing:.08em}.resource-rail h2{margin:3px 0 0;font-size:16px}.resource-rail header button,.resource-rail>footer button,.column-head button{display:inline-flex;align-items:center;justify-content:center;gap:6px;min-height:34px;border:1px solid rgb(var(--yb-border));border-radius:9px;padding:0 10px;background:rgb(var(--yb-surface));color:rgb(var(--yb-text));font-size:11px;font-weight:720;cursor:pointer}.rail-grid{display:grid;grid-template-columns:minmax(0,1fr) auto minmax(0,1fr) auto minmax(0,1fr);align-items:stretch}.rail-column{min-width:0;padding:12px 14px}.rail-column+.rail-arrow+.rail-column{border-left:1px solid rgb(var(--yb-border))}.rail-arrow{align-self:center;color:rgb(var(--yb-text-faint))}.column-head{min-height:37px;display:flex;align-items:center;gap:9px}.column-head>span:first-child{display:grid;flex:0 0 auto;width:28px;height:28px;place-items:center;border-radius:8px;background:rgb(var(--yb-brand-soft));color:rgb(var(--yb-brand));font:800 11px var(--yb-font-data)}.column-head>div{min-width:0;display:grid}.column-head strong{font-size:12px}.column-head small{color:rgb(var(--yb-text-muted));font-size:10px}.column-head button,.column-head>svg{margin-left:auto}.media-strip{display:grid;grid-template-columns:repeat(4,minmax(0,1fr));gap:6px;margin-top:9px}.media-strip button{min-width:0;display:grid;grid-template-rows:58px auto;gap:5px;border:0;padding:0;background:transparent;color:rgb(var(--yb-text));text-align:left;cursor:pointer}.media-strip :deep(.resource-preview-shell),.media-strip button>svg{width:100%;height:58px;border-radius:7px;background:rgb(var(--yb-surface-muted));object-fit:cover}.media-strip :deep(.resource-preview-img){width:100%;height:100%;border-radius:inherit;object-fit:cover}.resource-preview{pointer-events:none}.media-strip button>svg{padding:16px;color:rgb(var(--yb-text-muted))}.media-strip span{overflow:hidden;text-overflow:ellipsis;white-space:nowrap;font-size:9px}.file-strip{display:grid;gap:6px;margin-top:9px}.file-strip>div{min-width:0;display:flex;align-items:center;gap:8px;border:1px solid rgb(var(--yb-border));border-radius:8px;padding:7px;background:rgb(var(--yb-surface-soft));color:rgb(var(--yb-brand))}.file-strip>div>span{min-width:0;display:grid}.file-strip strong{overflow:hidden;text-overflow:ellipsis;white-space:nowrap;color:rgb(var(--yb-text));font-size:10px}.file-strip small{color:rgb(var(--yb-text-muted));font-size:9px}.locked{background:rgb(var(--yb-surface-soft))}.locked-copy{display:flex;align-items:center;gap:9px;margin-top:9px;border:1px dashed rgb(var(--yb-border-strong));border-radius:8px;padding:11px;color:rgb(var(--yb-text-muted))}.locked-copy>span{display:grid}.locked-copy strong{color:rgb(var(--yb-text));font-size:11px}.locked-copy small{font-size:9px;line-height:1.45}.empty-copy{margin:13px 0 0;color:rgb(var(--yb-text-muted));font-size:10px}.resource-rail>footer{display:flex;align-items:center;justify-content:space-between;gap:12px;padding:10px 15px;border-top:1px solid rgb(var(--yb-border));background:rgb(var(--yb-brand-soft)/.36);color:rgb(var(--yb-text-muted));font-size:11px}.resource-rail>footer button{border-color:rgb(var(--yb-brand));background:rgb(var(--yb-brand));color:rgb(var(--yb-text-inverse))}@media(max-width:1000px){.rail-grid{grid-template-columns:1fr}.rail-arrow{transform:rotate(90deg);justify-self:center}.rail-column+.rail-arrow+.rail-column{border-top:1px solid rgb(var(--yb-border));border-left:0}.media-strip button{grid-template-rows:72px auto}.media-strip :deep(.resource-preview-shell),.media-strip button>svg{height:72px}}@media(max-width:620px){.resource-rail>header,.resource-rail>footer{align-items:flex-start;flex-direction:column}.resource-rail>header button,.resource-rail>footer button{width:100%}.media-strip{grid-template-columns:repeat(2,minmax(0,1fr))}.media-strip button{grid-template-rows:86px auto}.media-strip :deep(.resource-preview-shell),.media-strip button>svg{height:86px}}
</style>
