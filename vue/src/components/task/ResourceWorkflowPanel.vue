<template>
  <section class="workflow-panel">
    <header>
      <div>
        <p class="eyebrow">任务处理</p>
        <h2>{{ heading }}</h2>
        <p>这里只显示你当前可以执行的操作；若任务已被他人更新，页面会提示刷新后继续。</p>
      </div>
      <button v-if="canSubmit" class="secondary" @click="applyFirstMode">将首行模式应用到全部 SKU</button>
    </header>

    <div v-if="error" class="message error" role="alert">{{ error }}</div>
    <div v-if="success" class="message success" role="status">{{ success }}</div>

    <div
      v-if="canSubmit || editingAudit"
      ref="editorViewport"
      class="group-editor-viewport"
      :style="{ '--resource-editor-row-height': editorRowHeight + 'px' }"
      data-testid="resource-editor-viewport"
      @scroll="onEditorScroll"
    >
      <div class="group-editor-spacer" :style="{ height: editorTotalHeight + 'px' }">
        <div class="group-editor-window" :style="{ transform: 'translateY(' + editorWindowOffset + 'px)' }">
      <article v-for="entry in visibleEditorRows" :key="entry.row.group.id" class="edit-card" :data-group-index="entry.groupIndex">
        <div class="edit-head">
          <div><strong>{{ entry.row.group.sku_code || scopeLabel(entry.row.group) }}</strong><small>{{ revisionStatus(entry.row) }}</small></div>
          <label>成品模式
            <select v-model="entry.row.mode" @change="markChanged(entry.row.group.id)"><option value="single">单图</option><option value="set">套装</option></select>
          </label>
        </div>
        <div class="upload-grid">
          <label class="upload-box">
            <span>设计源文件{{ sourceRequired ? '（必填）' : '（选填）' }}</span>
            <strong>{{ entry.row.source?.name || inheritedSourceName(entry.row) || '选择 PSD 或其他源文件' }}</strong>
            <input type="file" :disabled="Boolean(entry.row.uploading)" @change="uploadSource($event, entry.row)" />
          </label>
          <label class="upload-box">
            <span>最终成品图</span>
            <strong>{{ entry.row.finals.length ? `已选择 ${entry.row.finals.length} 张` : '选择一张或多张图片' }}</strong>
            <input type="file" accept="image/*" multiple :disabled="Boolean(entry.row.uploading)" @change="uploadFinals($event, entry.row)" />
          </label>
        </div>
        <div v-if="entry.row.uploading" class="uploading">正在上传 {{ entry.row.uploading }}…</div>
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
      </article>
        </div>
      </div>
    </div>

    <footer v-if="canSubmit" class="action-bar">
      <span>single 必须 1 张成品；set 至少 2 张。任务内所有资源组原子提交。</span>
      <button class="primary" :disabled="busy || !validDesign" @click="submitDesign">{{ busy ? '提交中…' : submitLabel }}</button>
    </footer>

    <footer v-if="canReturnToDesign || canApprove || canUploadReplacement" class="audit-bar">
      <label><span>打回或修改说明</span><input v-model.trim="reason" maxlength="1000" placeholder="打回设计时必填" /></label>
      <div>
        <button v-if="canReturnToDesign" class="secondary" :disabled="busy || !reason" @click="openConfirmation('return_to_design')">打回设计</button>
        <button v-if="canUploadReplacement" class="secondary" :disabled="busy" @click="editingAudit = !editingAudit">{{ editingAudit ? '取消上传修改' : '上传修改后通过' }}</button>
        <button v-if="canApprove || (editingAudit && canUploadReplacement)" class="primary" :disabled="busy || (editingAudit && !validAudit)" @click="openConfirmation('approve')">{{ busy ? '处理中…' : '通过并结单' }}</button>
      </div>
    </footer>

    <footer v-if="canReopen" class="reopen-bar">
      <label><span>重开目标</span><select v-model="reopenTarget"><option value="design">设计</option><option value="audit">审核</option><option v-if="isRetouch" value="retouch">修图</option></select></label>
      <label><span>重开原因</span><input v-model.trim="reason" maxlength="1000" placeholder="必填" /></label>
      <button class="secondary" :disabled="busy || !reason" @click="openConfirmation('reopen')">重开任务</button>
    </footer>

    <div v-if="pendingAction" class="confirm-backdrop" role="presentation" @click.self="cancelConfirmation">
      <section ref="confirmDialog" class="confirm-dialog" role="dialog" aria-modal="true" aria-labelledby="workflow-confirm-title" tabindex="-1" @keydown.esc.prevent="cancelConfirmation" @keydown.tab="trapConfirmationFocus">
        <p class="eyebrow">请确认操作</p>
        <h3 id="workflow-confirm-title">{{ confirmationTitle }}</h3>
        <p>{{ confirmationImpact }}</p>
        <dl><div><dt>目标状态</dt><dd>{{ confirmationTarget }}</dd></div><div><dt>资源影响</dt><dd>{{ confirmationResources }}</dd></div></dl>
        <div class="confirm-actions">
          <button ref="confirmCancelButton" class="secondary" :disabled="busy" @click="cancelConfirmation">取消</button>
          <button class="primary" :disabled="busy" @click="confirmAction">{{ busy ? '处理中…' : '确认执行' }}</button>
        </div>
      </section>
    </div>
  </section>
</template>

<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { uploadTaskFileViaAssetSession } from '@/services/upload/assetUploadFlow'
import { resourceGroupsApi, type ResourceBundle, type ResourceGroup, type ResourceGroupSubmission, type ResourceMode } from '@/services/api/resourceGroupsApi'

type UploadedFile = { id: number; name: string; inherited?: boolean }
type EditorRow = { group: ResourceGroup; mode: ResourceMode; source: UploadedFile | null; finals: UploadedFile[]; uploading: string }

const props = defineProps<{ taskId: number; taskType: string; bundle: ResourceBundle; allowedActions: string[] }>()
const emit = defineEmits<{ updated: [bundle: ResourceBundle]; 'dirty-change': [dirty: boolean] }>()
const rows = ref<EditorRow[]>([])
const changedGroups = ref(new Set<number>())
const busy = ref(false)
const error = ref('')
const success = ref('')
const reason = ref('')
const editingAudit = ref(false)
const pendingAction = ref<'approve' | 'return_to_design' | 'reopen' | null>(null)
const confirmDialog = ref<HTMLElement | null>(null)
const confirmCancelButton = ref<HTMLButtonElement | null>(null)
const editorViewport = ref<HTMLElement | null>(null)
const editorScrollTop = ref(0)
const editorViewportHeight = ref(720)
const editorRowHeight = ref(390)
const editorOverscan = 2
let confirmationTrigger: HTMLElement | null = null
const reopenTarget = ref<'design' | 'audit' | 'retouch'>('design')
let dragged: { groupIndex: number; index: number } | null = null

const actionSet = computed(() => new Set(props.allowedActions || []))
const canSubmit = computed(() => actionSet.value.has('submit_design') || actionSet.value.has('task.design.submit'))
const canReturnToDesign = computed(() => actionSet.value.has('task.audit.return_to_design') || actionSet.value.has('task.audit.decision'))
const canApprove = computed(() => actionSet.value.has('task.audit.approve') || actionSet.value.has('task.audit.decision'))
const canUploadReplacement = computed(() => canApprove.value)
const canAudit = computed(() => canReturnToDesign.value || canApprove.value || canUploadReplacement.value)
const canReopen = computed(() => actionSet.value.has('reopen') || actionSet.value.has('task.reopen'))
const isRetouch = computed(() => ['retouch', 'retouch_task'].includes(props.taskType.toLowerCase()))
const sourceRequired = computed(() => !isRetouch.value)
const heading = computed(() => canAudit.value ? '统一审核' : canSubmit.value ? (isRetouch.value ? '提交修图成品' : '设计提交') : canReopen.value ? '重开任务' : '当前无可执行动作')
const submitLabel = computed(() => isRetouch.value ? '提交成品并结单' : '提交审核')
const editorTotalHeight = computed(() => rows.value.length * editorRowHeight.value)
const editorVisibleStart = computed(() => Math.max(0, Math.floor(editorScrollTop.value / editorRowHeight.value) - editorOverscan))
const editorVisibleCount = computed(() => Math.ceil(editorViewportHeight.value / editorRowHeight.value) + editorOverscan * 2)
const visibleEditorRows = computed(() => rows.value.slice(editorVisibleStart.value, editorVisibleStart.value + editorVisibleCount.value).map((row, offset) => ({ row, groupIndex: editorVisibleStart.value + offset })))
const editorWindowOffset = computed(() => editorVisibleStart.value * editorRowHeight.value)
const isDirty = computed(() => changedGroups.value.size > 0 || rows.value.some((row) => Boolean(row.uploading)))

watch(() => props.bundle, buildRows, { immediate: true, deep: true })
watch(isDirty, (dirty) => emit('dirty-change', dirty), { immediate: true })

function buildRows() {
  rows.value = props.bundle.groups.map((group) => {
    const revision = group.working_revision || group.finalized_revision
    return {
      group,
      mode: revision?.mode || 'single',
      source: revision?.source_file ? { id: revision.source_file.task_asset_id, name: revision.source_file.file_name, inherited: true } : null,
      finals: [...(revision?.items || [])].sort((a,b) => a.sort_order - b.sort_order).map((item) => ({ id: item.task_asset_id, name: item.file?.file_name || item.item_name || `文件 ${item.task_asset_id}`, inherited: true })),
      uploading: '',
    }
  })
  changedGroups.value = new Set()
  editorScrollTop.value = 0
  if (editorViewport.value) editorViewport.value.scrollTop = 0
}

function refreshEditorMetrics() {
  editorRowHeight.value = typeof window.matchMedia === 'function' && window.matchMedia('(max-width: 780px)').matches ? 620 : 390
  editorViewportHeight.value = editorViewport.value?.clientHeight || Math.min(window.innerHeight * 0.68, 760)
}
function onEditorScroll() { editorScrollTop.value = editorViewport.value?.scrollTop || 0 }

function scopeLabel(group: ResourceGroup) { return group.scope_kind === 'retouch_requirement' ? `修图需求 ${group.retouch_requirement_id}` : '任务资源' }
function revisionStatus(row: EditorRow) { return row.group.finalized_revision ? '当前已结单资源' : row.group.working_revision ? '当前待处理资源' : '尚未提交资源' }
function inheritedSourceName(row: EditorRow) { return row.source?.inherited ? row.source.name : '' }
function markChanged(groupId: number) { changedGroups.value = new Set(changedGroups.value).add(groupId) }
function applyFirstMode() { const mode = rows.value[0]?.mode; if (!mode) return; rows.value.forEach((row) => { row.mode = mode; markChanged(row.group.id) }) }
function assetVersionId(uploaded: Awaited<ReturnType<typeof uploadTaskFileViaAssetSession>>): number {
  const raw = uploaded.version?.id || uploaded.version?.version_id
  const id = Number(raw)
  if (!Number.isSafeInteger(id) || id <= 0) throw new Error('上传完成但未返回任务资产 ID。')
  return id
}
function uploadOptions(row: EditorRow) {
  return row.group.retouch_requirement_id ? { retouchRequirementId: row.group.retouch_requirement_id } : undefined
}
async function uploadSource(event: Event, row: EditorRow) {
  const input = event.target as HTMLInputElement; const file = input.files?.[0]; if (!file) return
  row.uploading = file.name; error.value = ''
  try {
    const uploaded = await uploadTaskFileViaAssetSession(String(props.taskId), file, { asset_kind: 'source', target_sku_code: row.group.sku_code || undefined, remark: file.name }, uploadOptions(row))
    row.source = { id: assetVersionId(uploaded), name: file.name }; markChanged(row.group.id)
  } catch (cause) { error.value = cause instanceof Error ? cause.message : '源文件上传失败。' }
  finally { row.uploading = ''; input.value = '' }
}
async function uploadFinals(event: Event, row: EditorRow) {
  const input = event.target as HTMLInputElement; const files = [...(input.files || [])]; if (!files.length) return
  error.value = ''
  try {
    const uploadedFiles: UploadedFile[] = []
    for (const file of files) {
      row.uploading = file.name
      const uploaded = await uploadTaskFileViaAssetSession(String(props.taskId), file, { asset_kind: 'delivery', target_sku_code: row.group.sku_code || undefined, remark: file.name }, uploadOptions(row))
      uploadedFiles.push({ id: assetVersionId(uploaded), name: file.name })
    }
    row.finals = uploadedFiles; markChanged(row.group.id)
  } catch (cause) { error.value = cause instanceof Error ? cause.message : '成品图上传失败，可重新选择该组文件。' }
  finally { row.uploading = ''; input.value = '' }
}
function move(row: EditorRow, index: number, delta: number) { const next = index + delta; if (next < 0 || next >= row.finals.length) return; const [item] = row.finals.splice(index,1); row.finals.splice(next,0,item); markChanged(row.group.id) }
function removeFinal(row: EditorRow,index:number) { row.finals.splice(index,1); markChanged(row.group.id) }
function dragStart(groupIndex:number,index:number) { dragged = { groupIndex,index } }
function drop(groupIndex:number,index:number) { if (!dragged || dragged.groupIndex !== groupIndex) return; const row = rows.value[groupIndex]; const [item] = row.finals.splice(dragged.index,1); row.finals.splice(index,0,item); markChanged(row.group.id); dragged = null }

function validRow(row: EditorRow, requireSource: boolean) { return (!requireSource || !!row.source) && (row.mode === 'single' ? row.finals.length === 1 : row.finals.length >= 2) }
const validDesign = computed(() => rows.value.length > 0 && rows.value.every((row) => validRow(row, sourceRequired.value)) && !rows.value.some((row) => row.uploading))
const validAudit = computed(() => [...changedGroups.value].length > 0 && rows.value.filter((row) => changedGroups.value.has(row.group.id)).every((row) => validRow(row, true)))
function submission(row: EditorRow): ResourceGroupSubmission { return { group_id: row.group.id, expected_group_lock_version: row.group.lock_version, mode: row.mode, source_task_asset_id: row.source?.id, final_task_asset_ids: row.finals.map((file) => file.id) } }
async function run(action: () => Promise<ResourceBundle>, message: string) { busy.value = true; error.value = ''; success.value = ''; try { const bundle = await action(); success.value = message; emit('updated', bundle) } catch (cause) { error.value = cause instanceof Error ? cause.message : '操作失败，请刷新后重试。' } finally { busy.value = false } }
async function submitDesign() { if (!validDesign.value) return; await run(() => resourceGroupsApi.submitDesign(props.taskId, props.bundle, rows.value.map(submission)), isRetouch.value ? '修图任务已结单。' : '已提交审核。') }
async function returnToDesign() { if (!reason.value) return; await run(() => resourceGroupsApi.auditDecision(props.taskId, props.bundle, 'return_to_design', reason.value), '已打回设计。') }
async function approve() { const groups = editingAudit.value ? rows.value.filter((row) => changedGroups.value.has(row.group.id)).map(submission) : []; await run(() => resourceGroupsApi.auditDecision(props.taskId, props.bundle, 'approve', reason.value, groups), '审核通过，任务已结单。') }
async function reopen() { if (!reason.value) return; await run(() => resourceGroupsApi.reopen(props.taskId, props.bundle, reopenTarget.value, reason.value), '任务已重开。') }
const confirmationTitle = computed(() => pendingAction.value === 'approve' ? '通过审核并立即结单？' : pendingAction.value === 'return_to_design' ? '将任务打回设计？' : '重开已结单任务？')
const confirmationImpact = computed(() => pendingAction.value === 'approve' ? '确认后任务立即变为已结单，当前审核资源成为最终资源。' : pendingAction.value === 'return_to_design' ? '任务将回到设计处理中，原负责人可以按说明重新提交。' : '任务将恢复为可处理状态，现有已结单资源在再次审核通过前保持对外有效。')
const confirmationTarget = computed(() => pendingAction.value === 'approve' ? '已结单' : pendingAction.value === 'return_to_design' ? '设计处理中' : reopenTarget.value === 'audit' ? '待审核' : reopenTarget.value === 'retouch' ? '修图处理中' : '设计处理中')
const confirmationResources = computed(() => pendingAction.value === 'approve' ? (editingAudit.value ? '审核上传的源文件和成品将替换当前提交。' : '沿用设计师提交的源文件和成品。') : pendingAction.value === 'return_to_design' ? '本次审核修改不会成为最终资源。' : '当前最终资源继续可见，直到新的审核结果生效。')
function openConfirmation(action: 'approve' | 'return_to_design' | 'reopen') {
  if (busy.value) return
  if ((action === 'return_to_design' || action === 'reopen') && !reason.value) { error.value = '请先填写操作原因。'; return }
  confirmationTrigger = document.activeElement instanceof HTMLElement ? document.activeElement : null
  pendingAction.value = action
  void nextTick(() => confirmCancelButton.value?.focus())
}
function closeConfirmation() { pendingAction.value = null; void nextTick(() => confirmationTrigger?.focus()) }
function cancelConfirmation() { if (!busy.value) closeConfirmation() }
function trapConfirmationFocus(event: KeyboardEvent) {
  const controls = [...(confirmDialog.value?.querySelectorAll<HTMLElement>('button:not([disabled]), [href], input:not([disabled]), select:not([disabled]), [tabindex]:not([tabindex="-1"])') || [])]
  if (!controls.length) return
  const first = controls[0]; const last = controls[controls.length - 1]
  if (event.shiftKey && document.activeElement === first) { event.preventDefault(); last.focus() }
  else if (!event.shiftKey && document.activeElement === last) { event.preventDefault(); first.focus() }
}
async function confirmAction() {
  if (!pendingAction.value || busy.value) return
  const action = pendingAction.value
  if (action === 'approve') await approve()
  else if (action === 'return_to_design') await returnToDesign()
  else await reopen()
  if (!error.value) closeConfirmation()
}
onMounted(() => { refreshEditorMetrics(); window.addEventListener('resize', refreshEditorMetrics) })
onBeforeUnmount(() => { window.removeEventListener('resize', refreshEditorMetrics); emit('dirty-change', false) })
</script>

<style scoped>
.workflow-panel{display:grid;gap:16px;border:1px solid rgb(var(--yb-border));border-radius:18px;background:rgb(var(--yb-surface));overflow:hidden}.workflow-panel>header{display:flex;align-items:flex-start;justify-content:space-between;gap:20px;padding:20px 22px;border-bottom:1px solid rgb(var(--yb-border))}.workflow-panel h2{margin:3px 0;font-size:21px}.workflow-panel p{margin:0;color:rgb(var(--yb-text-muted))}.eyebrow{font-size:11px;letter-spacing:.13em;font-weight:900;color:rgb(var(--yb-brand))}.group-editor{display:grid;gap:14px;padding:0 20px}.edit-card{border:1px solid rgb(var(--yb-border));border-radius:14px;overflow:hidden}.edit-head{display:flex;justify-content:space-between;align-items:center;padding:13px 15px;background:rgb(var(--yb-surface-soft))}.edit-head>div{display:grid;gap:3px}.edit-head small{color:rgb(var(--yb-text-muted))}.edit-head label{display:flex;gap:8px;align-items:center;font-size:12px}.edit-head select,.reopen-bar select,.audit-bar input,.reopen-bar input{min-height:36px;border:1px solid rgb(var(--yb-border));border-radius:9px;padding:0 10px;background:rgb(var(--yb-surface));color:rgb(var(--yb-text))}.upload-grid{display:grid;grid-template-columns:1fr 1fr;gap:12px;padding:14px}.upload-box{display:grid;gap:5px;padding:14px;border:1px dashed rgb(var(--yb-border));border-radius:11px;cursor:pointer}.upload-box span{font-size:12px;color:rgb(var(--yb-text-muted))}.upload-box input{margin-top:5px}.uploading{padding:0 14px 12px;color:rgb(var(--yb-brand))}.final-order{display:grid;gap:6px;padding:0 14px 14px;margin:0;list-style:none}.final-order li{display:grid;grid-template-columns:25px 1fr auto;align-items:center;gap:9px;padding:8px;border-radius:9px;background:rgb(var(--yb-surface-muted))}.final-order li>span{display:grid;place-items:center;width:24px;height:24px;border-radius:7px;background:rgb(var(--yb-brand-soft))}.final-order button{border:0;background:transparent;cursor:pointer}.action-bar,.audit-bar,.reopen-bar{display:flex;align-items:flex-end;justify-content:space-between;gap:15px;padding:16px 20px;border-top:1px solid rgb(var(--yb-border));background:rgb(var(--yb-surface-soft))}.action-bar span{color:rgb(var(--yb-text-muted));font-size:12px}.audit-bar label,.reopen-bar label{display:grid;gap:5px;flex:1}.audit-bar label span,.reopen-bar label span{font-size:12px;color:rgb(var(--yb-text-muted))}.audit-bar>div{display:flex;gap:8px}.primary,.secondary{min-height:40px;padding:0 15px;border-radius:10px;font-weight:750;cursor:pointer}.primary{border:0;background:rgb(var(--yb-brand));color:rgb(var(--yb-text-inverse))}.secondary{border:1px solid rgb(var(--yb-border));background:rgb(var(--yb-surface));color:rgb(var(--yb-text))}.primary:disabled,.secondary:disabled{opacity:.45;cursor:not-allowed}.message{margin:0 20px;padding:11px 13px;border-radius:10px}.error{background:rgb(var(--yb-danger-soft));color:rgb(var(--yb-danger-text))}.success{background:rgb(var(--yb-success-soft));color:rgb(var(--yb-success-strong))}.confirm-backdrop{position:fixed;inset:0;z-index:80;display:grid;place-items:center;padding:20px;background:rgb(var(--yb-overlay-night) / .48)}.confirm-dialog{width:min(520px,100%);display:grid;gap:14px;padding:22px;border:1px solid rgb(var(--yb-border));border-radius:18px;background:rgb(var(--yb-surface));box-shadow:0 20px 55px rgb(var(--yb-shadow) / .22)}.confirm-dialog h3{margin:0}.confirm-dialog dl{display:grid;gap:8px;margin:0}.confirm-dialog dl div{display:grid;grid-template-columns:90px 1fr;gap:12px}.confirm-dialog dt{color:rgb(var(--yb-text-muted))}.confirm-dialog dd{margin:0}.confirm-actions{display:flex;justify-content:flex-end;gap:8px}@media(max-width:780px){.workflow-panel>header,.action-bar,.audit-bar,.reopen-bar{align-items:stretch;flex-direction:column}.upload-grid{grid-template-columns:1fr}.audit-bar>div{flex-wrap:wrap}}
.group-editor-viewport {
  height: min(68vh, 760px);
  overflow: auto;
  overscroll-behavior: contain;
  margin: 0 20px;
  background: rgb(var(--yb-surface-soft));
  border-radius: 14px;
}
.group-editor-spacer { position: relative; }
.group-editor-window { position: absolute; inset: 0 0 auto; }
.group-editor-window .edit-card {
  box-sizing: border-box;
  height: calc(var(--resource-editor-row-height) - 12px);
  margin: 6px 0;
  overflow: auto;
  background: rgb(var(--yb-surface));
}
</style>
