<template>
  <Teleport to="body">
    <div class="revision-drawer-backdrop" @mousedown.self="emit('close')">
      <aside ref="drawer" class="revision-drawer" role="dialog" aria-modal="true" :aria-label="`${group.sku_code || '资源组'} 历史修订`" tabindex="-1" @keydown="handleKeydown">
        <header class="drawer-head">
          <div><p>资源修订审计</p><h2>{{ group.sku_code || group.product_name || '任务资源' }} · 历史修订</h2><span>共 {{ result.total }} 个版本，当前资源面仍以 working/finalized 指针为准。</span></div>
          <button ref="closeButton" type="button" aria-label="关闭历史修订" @click="emit('close')">×</button>
        </header>

        <div v-if="loading" class="drawer-state">正在读取历史修订…</div>
        <div v-else-if="error" class="drawer-state error" role="alert"><span>{{ error }}</span><button type="button" @click="load()">重试</button></div>
        <div v-else-if="!result.items.length" class="drawer-state">暂无历史修订。</div>
        <div v-else class="revision-list">
          <article v-for="revision in result.items" :key="revision.id" class="revision-card">
            <header>
              <div><strong>第 {{ revision.revision_no }} 版</strong><span>{{ statusLabel(revision.status) }} · {{ stageLabel(revision.source_stage) }} · {{ modeLabel(revision.mode) }}</span></div>
              <div class="revision-flags">
                <span v-if="result.finalized_revision_id === revision.id" class="is-final">当前最终版</span>
                <span v-if="result.working_revision_id === revision.id" class="is-working">当前工作版</span>
              </div>
            </header>
            <dl class="revision-meta">
              <div><dt>创建时间</dt><dd>{{ formatDate(revision.created_at) }}</dd></div>
              <div><dt>提交时间</dt><dd>{{ formatDate(revision.submitted_at) }}</dd></div>
              <div><dt>定稿时间</dt><dd>{{ formatDate(revision.finalized_at) }}</dd></div>
              <div><dt>操作人</dt><dd>{{ revision.created_by_name || `用户 ${revision.created_by}` }}</dd></div>
            </dl>
            <p v-if="displayReason(revision)" class="revision-reason">{{ displayReason(revision) }}</p>

            <section v-if="revision.legacy_migration" class="migration-evidence" aria-label="历史迁移证据">
              <header><h3>历史迁移证据</h3><span>{{ evidenceConfidenceLabel(revision) }}</span></header>
              <template v-if="revision.evidence_summary">
                <dl>
                  <div><dt>确认人</dt><dd>用户 {{ revision.evidence_summary.confirmed_by }}</dd></div>
                  <div><dt>确认时间</dt><dd>{{ formatDate(revision.evidence_summary.confirmed_at) }}</dd></div>
                  <div><dt>清单摘要</dt><dd><code>{{ shortHash(revision.evidence_summary.manifest_sha256) }}</code></dd></div>
                  <div><dt>上传会话</dt><dd>{{ uploadSessionLabel(revision) }}</dd></div>
                </dl>
                <p>事件证据：{{ revision.evidence_summary.evidence_event_ids.join('、') }}</p>
              </template>
              <p v-else class="evidence-warning">迁移标记存在，但结构化证据无法安全解析，请核对原始审计记录。</p>
            </section>

            <section class="revision-files">
              <h3>源文件</h3>
              <div v-if="revision.source_file" class="file-row">
                <span class="file-kind">{{ fileType(revision.source_file.file_name) }}</span>
                <div><strong>{{ revision.source_file.file_name }}</strong><small>{{ formatBytes(revision.source_file.file_size) }}</small></div>
                <nav>
                  <button v-if="revision.source_file.preview_url" type="button" @click="previewAsset(revision.source_file.task_asset_id)">预览</button>
                  <button v-if="revision.source_file.download_url" type="button" @click="downloadAsset(revision.source_file.task_asset_id, revision.source_file.file_name)">下载</button>
                </nav>
              </div>
              <p v-else class="empty-file">本版本没有源文件。</p>
            </section>

            <section class="revision-files">
              <h3>最终成品 · {{ revision.items.length }}</h3>
              <div v-if="revision.items.length" class="file-grid">
                <article v-for="(item,index) in orderedItems(revision)" :key="item.id">
                  <button v-if="item.file?.preview_url" type="button" class="file-preview" :aria-label="`预览 ${item.file.file_name}`" @click="previewAsset(item.file.task_asset_id)">{{ fileType(item.file.file_name) }}</button>
                  <span v-else class="file-preview">{{ fileType(item.file?.file_name || item.item_name) }}</span>
                  <div><strong>{{ item.file?.file_name || item.item_name || `成品 ${index + 1}` }}</strong><small>顺序 {{ index + 1 }}</small></div>
                  <button v-if="item.file?.download_url" type="button" @click="downloadAsset(item.file.task_asset_id, item.file.file_name)">下载</button>
                </article>
              </div>
              <p v-else class="empty-file">本版本没有最终成品。</p>
            </section>

            <section v-if="revision.references.length" class="revision-files">
              <h3>参考资料 · {{ revision.references.length }}</h3>
              <div class="reference-list">
                <div v-for="reference in orderedReferences(revision)" :key="reference.id || reference.reference_file_ref_id">
                  <span>{{ reference.file_name || reference.ref_id || '参考资料' }}</span>
                  <nav>
                    <button v-if="reference.preview_url && reference.formal_task_asset_id" type="button" @click="previewAsset(reference.formal_task_asset_id)">预览</button>
                    <button v-if="reference.download_url && reference.formal_task_asset_id" type="button" @click="downloadAsset(reference.formal_task_asset_id, reference.file_name)">下载</button>
                  </nav>
                </div>
              </div>
            </section>
          </article>
        </div>

        <p v-if="actionError" class="action-error" role="alert">{{ actionError }}</p>
        <footer class="drawer-pagination">
          <span>第 {{ result.page }} / {{ totalPages }} 页</span>
          <div><button type="button" :disabled="loading || result.page <= 1" aria-label="上一页历史修订" @click="changePage(result.page - 1)">上一页</button><button type="button" :disabled="loading || result.page >= totalPages" aria-label="下一页历史修订" @click="changePage(result.page + 1)">下一页</button></div>
        </footer>
      </aside>
    </div>
  </Teleport>
</template>

<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { fetchAssetPreviewMeta } from '@/domain/asset-access'
import { materializePreviewImageUrl, revokeMaterializedPreviewImage } from '@/domain/asset-preview-image'
import { resourceGroupsApi, type ResourceGroup, type ResourceRevision, type ResourceRevisionListResult } from '@/services/api/resourceGroupsApi'
import { downloadAssetFileWithOriginalFilename } from '@/utils/assetFileDownload'

const props = defineProps<{ group: ResourceGroup }>()
const emit = defineEmits<{ close: [] }>()
const pageSize = 20
const loading = ref(false)
const error = ref('')
const actionError = ref('')
const drawer = ref<HTMLElement | null>(null)
const closeButton = ref<HTMLButtonElement | null>(null)
const result = ref<ResourceRevisionListResult>({ items: [], working_revision_id: props.group.working_revision_id, finalized_revision_id: props.group.finalized_revision_id, page: 1, page_size: pageSize, total: 0 })
const totalPages = computed(() => Math.max(1, Math.ceil(result.value.total / result.value.page_size)))
let previousBodyOverflow = ''
let returnFocus: HTMLElement | null = null

async function load(page = result.value.page || 1) {
  loading.value = true
  error.value = ''
  try { result.value = await resourceGroupsApi.revisions(props.group.id, { page, page_size: pageSize }) }
  catch (cause) { error.value = cause instanceof Error ? cause.message : '历史修订加载失败。' }
  finally { loading.value = false }
}
function changePage(page: number) { if (page >= 1 && page <= totalPages.value) void load(page) }
function orderedItems(revision: ResourceRevision) { return [...revision.items].sort((left, right) => left.sort_order - right.sort_order || left.id - right.id) }
function orderedReferences(revision: ResourceRevision) { return [...revision.references].sort((left, right) => Number(left.sort_order || 0) - Number(right.sort_order || 0) || Number(left.id || 0) - Number(right.id || 0)) }
function statusLabel(status: ResourceRevision['status']) { return ({ draft: '草稿', submitted: '已提交', finalized: '已定稿', rejected: '已退回', superseded: '已被替代' }[status]) }
function stageLabel(stage: ResourceRevision['source_stage']) { return ({ design: '设计提交', audit: '审核确认', retouch: '修图提交', migration: '历史迁移', reopen: '重开提交' }[stage]) }
function modeLabel(mode: ResourceRevision['mode']) { return mode === 'set' ? '套装' : '单图' }
function displayReason(revision: ResourceRevision) {
  if (revision.evidence_summary?.business_reason) return revision.evidence_summary.business_reason
  const reason = revision.reason || ''
  const markerIndex = reason.lastIndexOf('[migration_v2 ')
  return markerIndex >= 0 ? reason.slice(0, markerIndex).trim() : reason
}
function evidenceConfidenceLabel(revision: ResourceRevision) { return ({ confirmed_auto: '已确认迁移', proposed_review: '待人工复核', hard_blocked: '迁移阻断' }[revision.evidence_summary?.confidence || 'hard_blocked']) }
function shortHash(value: string) { return value.length > 16 ? `${value.slice(0, 8)}…${value.slice(-8)}` : value }
function uploadSessionLabel(revision: ResourceRevision) {
  const evidence = revision.evidence_summary
  if (!evidence?.upload_sessions_known) return '旧证据未记录'
  return evidence.upload_session_ids.length ? evidence.upload_session_ids.join('、') : '已确认无关联会话'
}
function fileType(name?: string) { return (name?.split('.').pop() || 'FILE').slice(0, 5).toUpperCase() }
function focusableElements() {
  if (!drawer.value) return []
  return Array.from(drawer.value.querySelectorAll<HTMLElement>('button:not(:disabled),a[href],input:not(:disabled),select:not(:disabled),textarea:not(:disabled),[tabindex]:not([tabindex="-1"])')).filter((element) => !element.hasAttribute('hidden'))
}
function handleKeydown(event: KeyboardEvent) {
  if (event.key === 'Escape') { event.preventDefault(); emit('close'); return }
  if (event.key !== 'Tab') return
  const focusable = focusableElements()
  if (!focusable.length) { event.preventDefault(); drawer.value?.focus(); return }
  const first = focusable[0]
  const last = focusable[focusable.length - 1]
  if (event.shiftKey && document.activeElement === first) { event.preventDefault(); last.focus() }
  else if (!event.shiftKey && document.activeElement === last) { event.preventDefault(); first.focus() }
}
async function previewAsset(assetId: number) {
  actionError.value = ''
  const meta = await fetchAssetPreviewMeta(String(assetId))
  if (meta.status !== 'ok' || !meta.displayUrl) { actionError.value = meta.message || '预览不可用。'; return }
  const image = await materializePreviewImageUrl(meta.displayUrl, assetId)
  if (!image) { actionError.value = '预览内容不可用。'; return }
  window.open(image.displaySrc, '_blank', 'noopener,noreferrer')
  window.setTimeout(() => revokeMaterializedPreviewImage(image), 60_000)
}
async function downloadAsset(assetId: number, fileName?: string) {
  actionError.value = ''
  const outcome = await downloadAssetFileWithOriginalFilename({ assetId: String(assetId), preferredFilename: fileName })
  if (!outcome.ok) actionError.value = outcome.message || '下载失败。'
}
function formatBytes(value?: number | null) { if (!value) return '文件大小未知'; if (value < 1024 * 1024) return `${Math.max(1, Math.round(value / 1024))} KB`; return `${(value / 1024 / 1024).toFixed(1)} MB` }
function formatDate(value?: string | null) { if (!value) return '—'; const date = new Date(value); return Number.isNaN(date.getTime()) ? value : new Intl.DateTimeFormat('zh-CN', { year: 'numeric', month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit' }).format(date) }

watch(() => props.group.id, () => { result.value = { items: [], working_revision_id: props.group.working_revision_id, finalized_revision_id: props.group.finalized_revision_id, page: 1, page_size: pageSize, total: 0 }; void load(1) })
onMounted(() => {
	returnFocus = document.activeElement instanceof HTMLElement ? document.activeElement : null
  previousBodyOverflow = document.body.style.overflow
  document.body.style.overflow = 'hidden'
  void load(1)
  void nextTick(() => closeButton.value?.focus())
})
onBeforeUnmount(() => { document.body.style.overflow = previousBodyOverflow; returnFocus?.focus() })
</script>

<style scoped>
.revision-drawer-backdrop{position:fixed;inset:0;z-index:180;display:flex;justify-content:flex-end;background:rgb(var(--yb-overlay-night)/.58)}.revision-drawer{width:min(760px,94vw);height:100dvh;display:grid;grid-template-rows:auto minmax(0,1fr) auto;overflow:hidden;border-left:1px solid rgb(var(--yb-border));background:rgb(var(--yb-surface));box-shadow:-20px 0 50px rgb(var(--yb-shadow)/.2)}.drawer-head{display:flex;align-items:flex-start;justify-content:space-between;gap:16px;padding:18px 20px;border-bottom:1px solid rgb(var(--yb-border))}.drawer-head p{margin:0;color:rgb(var(--yb-brand));font-size:10px;font-weight:850;letter-spacing:.1em}.drawer-head h2{margin:4px 0;font-size:20px}.drawer-head span{color:rgb(var(--yb-text-muted));font-size:11px}.drawer-head button{width:36px;height:36px;border:1px solid rgb(var(--yb-border));border-radius:9px;background:rgb(var(--yb-surface));color:rgb(var(--yb-text));font-size:23px;cursor:pointer}.revision-list{overflow:auto;display:grid;align-content:start;gap:12px;padding:16px}.revision-card{display:grid;gap:13px;padding:15px;border:1px solid rgb(var(--yb-border));border-radius:14px;background:rgb(var(--yb-surface-soft))}.revision-card>header{display:flex;align-items:flex-start;justify-content:space-between;gap:12px}.revision-card>header>div:first-child{display:grid;gap:3px}.revision-card>header strong{font-size:15px}.revision-card>header span,.revision-meta dt,.revision-meta dd,.file-row small,.file-grid small{color:rgb(var(--yb-text-muted));font-size:10px}.revision-flags{display:flex;flex-wrap:wrap;justify-content:flex-end;gap:5px}.revision-flags span{padding:4px 7px;border-radius:999px;font-weight:780}.revision-flags .is-final{background:rgb(var(--yb-success-soft));color:rgb(var(--yb-success-text))}.revision-flags .is-working{background:rgb(var(--yb-brand-soft));color:rgb(var(--yb-brand))}.revision-meta{display:grid;grid-template-columns:repeat(4,minmax(0,1fr));margin:0;overflow:hidden;border:1px solid rgb(var(--yb-border));border-radius:10px;background:rgb(var(--yb-surface))}.revision-meta div{min-width:0;display:grid;gap:3px;padding:9px}.revision-meta div+div{border-left:1px solid rgb(var(--yb-border))}.revision-meta dd{margin:0;color:rgb(var(--yb-text));font-weight:650}.revision-reason{margin:0;padding:9px 11px;border-radius:9px;background:rgb(var(--yb-warning-soft));color:rgb(var(--yb-warning-text));font-size:11px}.migration-evidence{display:grid;gap:8px;padding:10px 11px;border:1px solid rgb(var(--yb-brand)/.18);border-radius:10px;background:rgb(var(--yb-brand-soft)/.42)}.migration-evidence>header{display:flex;align-items:center;justify-content:space-between;gap:8px}.migration-evidence h3{margin:0;font-size:11px}.migration-evidence>header span{color:rgb(var(--yb-brand));font-size:10px;font-weight:760}.migration-evidence dl{display:grid;grid-template-columns:repeat(2,minmax(0,1fr));gap:7px;margin:0}.migration-evidence dl div{min-width:0}.migration-evidence dt{color:rgb(var(--yb-text-muted));font-size:9px}.migration-evidence dd{overflow-wrap:anywhere;margin:2px 0 0;font-size:10px}.migration-evidence p{overflow-wrap:anywhere;margin:0;color:rgb(var(--yb-text-muted));font-size:10px}.migration-evidence .evidence-warning{color:rgb(var(--yb-warning-text))}.revision-files{display:grid;gap:8px}.revision-files h3{margin:0;font-size:11px}.file-row{display:grid;grid-template-columns:auto minmax(0,1fr) auto;align-items:center;gap:9px;padding:9px;border:1px solid rgb(var(--yb-border));border-radius:10px;background:rgb(var(--yb-surface))}.file-kind,.file-preview{display:grid;place-items:center;width:46px;height:42px;border-radius:8px;background:rgb(var(--yb-surface-muted));color:rgb(var(--yb-brand));font-size:9px;font-weight:850;text-decoration:none}.file-row>div{min-width:0;display:grid}.file-row strong,.file-grid strong{overflow:hidden;text-overflow:ellipsis;white-space:nowrap;font-size:11px}.file-row nav,.reference-list nav{display:flex;gap:7px}.file-row a,.file-grid>article>a:last-child,.reference-list a{color:rgb(var(--yb-brand));font-size:10px;text-decoration:none}.file-grid{display:grid;grid-template-columns:repeat(3,minmax(0,1fr));gap:7px}.file-grid>article{min-width:0;display:grid;grid-template-columns:auto minmax(0,1fr);gap:7px;align-items:center;padding:7px;border:1px solid rgb(var(--yb-border));border-radius:10px;background:rgb(var(--yb-surface))}.file-grid>article>div{min-width:0;display:grid}.file-grid>article>a:last-child{grid-column:2}.file-preview img{width:100%;height:100%;border-radius:inherit;object-fit:cover}.reference-list{display:grid;gap:6px}.reference-list>div{display:flex;align-items:center;justify-content:space-between;gap:10px;padding:8px 10px;border:1px solid rgb(var(--yb-border));border-radius:9px;background:rgb(var(--yb-surface));font-size:10px}.empty-file{margin:0;padding:10px;border:1px dashed rgb(var(--yb-border));border-radius:9px;color:rgb(var(--yb-text-muted));font-size:10px}.drawer-state{align-self:start;margin:20px;padding:24px;border:1px dashed rgb(var(--yb-border));border-radius:12px;color:rgb(var(--yb-text-muted));text-align:center}.drawer-state.error{display:flex;align-items:center;justify-content:space-between;gap:12px;background:rgb(var(--yb-danger-soft));color:rgb(var(--yb-danger-text))}.drawer-state button,.drawer-pagination button{min-height:32px;padding:0 10px;border:1px solid rgb(var(--yb-border));border-radius:8px;background:rgb(var(--yb-surface));color:rgb(var(--yb-text));cursor:pointer}.drawer-pagination{display:flex;align-items:center;justify-content:space-between;gap:12px;padding:12px 18px;border-top:1px solid rgb(var(--yb-border));color:rgb(var(--yb-text-muted));font-size:11px}.drawer-pagination div{display:flex;gap:7px}.drawer-pagination button:disabled{cursor:not-allowed;opacity:.45}@media(max-width:640px){.revision-drawer{width:100vw}.revision-meta,.migration-evidence dl{grid-template-columns:1fr 1fr}.revision-meta div:nth-child(3){border-left:0}.revision-meta div:nth-child(n+3){border-top:1px solid rgb(var(--yb-border))}.file-grid{grid-template-columns:1fr}.drawer-head{padding:14px}.revision-list{padding:10px}}
</style>
