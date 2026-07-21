import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import axios from 'axios'
import { useTasksStore } from '@/stores/tasks'
import { formatMultipartPartsLabel } from '@/services/api/assetsApi'
import {
  cancelPreparedTaskAssetUploadSession,
  completeWithAssetVersionRaceRetry,
  prepareTaskAssetUploadSession,
  type PreparedTaskAssetUploadSession,
} from '@/services/upload/assetUploadFlow'
import type { SubmitDesignAssetItem } from '@/services/apiTypes'
import { canSubmitAudit, canUploadDesignDelivery } from '@/domain/task-actions'
import { DesignDeliveryUploadPhase } from '@/domain/enums/upload-status'
import {
  DESIGN_UPLOAD_COPY,
  DESIGN_UPLOAD_TIMING,
  DESIGN_UPLOAD_MAX_FILE_SIZE_BYTES,
  designUploadTooLargeMessage,
} from '@/domain/copy/design-upload'
import { formatSubmitDesignFailureMessage } from '@/utils/upload-errors'
import type {
  DesignDeliveryAuditBatch,
  DesignDeliveryUploadSession,
} from '@/domain/types/design-upload'

function sleep(ms: number): Promise<void> {
  return new Promise((resolve) => setTimeout(resolve, ms))
}

let rafId = 0

function debugUploadLog(...args: unknown[]) {
  if (!import.meta.env.DEV) return
  // eslint-disable-next-line no-console
  console.info('[design-upload-debug]', ...args)
}

function stopSmoothLoop() {
  if (rafId) cancelAnimationFrame(rafId)
  rafId = 0
}

function findOversizedDesignFile(files: File[]): File | undefined {
  return files.find((file) => file.size > DESIGN_UPLOAD_MAX_FILE_SIZE_BYTES)
}

type UploadAssetKind = 'source' | 'delivery'

interface UploadedSessionFact {
  uploadSessionId: string
  assetKind: UploadAssetKind
  targetSkuCode?: string
}

function normalizeSkuCode(value: string | undefined): string | undefined {
  const t = (value ?? '').trim()
  return t || undefined
}

function readUploadDenyDetail(err: unknown, key: string): string | undefined {
  if (!axios.isAxiosError(err)) return undefined
  const data = err.response?.data as
    | {
        error?: { details?: Record<string, unknown> }
      }
    | undefined
  const details = data?.error?.details
  const v = details?.[key]
  return typeof v === 'string' && v.trim() ? v.trim() : undefined
}

function isTaskStatusNotActionableUploadError(err: unknown): boolean {
  if (!axios.isAxiosError(err)) return false
  const data = err.response?.data as
    | {
        error?: { code?: string; details?: Record<string, unknown> }
      }
    | undefined
  const code = data?.error?.code
  const denyCode = readUploadDenyDetail(err, 'deny_code')
  const action = readUploadDenyDetail(err, 'action')
  return (
    String(code ?? '').toUpperCase() === 'PERMISSION_DENIED' &&
    denyCode === 'task_status_not_actionable' &&
    (action === 'asset_upload_session_complete' || action === 'asset_upload_session_cancel')
  )
}

function appendPartialUploadHint(
  message: string,
  pendingCount: number,
): string {
  if (pendingCount <= 0) return message
  if (
    /服务端确认完成失败|complete upload session|main_complete|http 500|internal error/i.test(
      message,
    )
  ) {
    return `${message}。已有部分文件上传成功并入库，请先刷新查看已成功版本，再仅重传剩余 ${pendingCount} 个文件。`
  }
  return message
}

function toSubmitDesignAssetPayload(
  facts: UploadedSessionFact[],
  options?: { requireTargetSkuForDelivery?: boolean },
): { ok: true; assets: SubmitDesignAssetItem[] } | { ok: false; message: string } {
  const assets: SubmitDesignAssetItem[] = []
  const requireTarget = options?.requireTargetSkuForDelivery === true
  for (const item of facts) {
    const uploadSessionId = item.uploadSessionId.trim()
    if (!uploadSessionId) {
      return { ok: false, message: '上传会话缺少 upload_session_id，无法提交审核' }
    }
    const targetSkuCode = normalizeSkuCode(item.targetSkuCode)
    if (requireTarget && item.assetKind === 'delivery' && !targetSkuCode) {
      return { ok: false, message: '批量交付文件必须绑定 target_sku_code，请检查商品归属后重试' }
    }
    assets.push({
      upload_session_id: uploadSessionId,
      asset_kind: item.assetKind,
      target_sku_code: targetSkuCode,
    })
  }
  return { ok: true, assets }
}

export const useDesignStore = defineStore('design', () => {
  const session = ref<DesignDeliveryUploadSession | null>(null)

  const isSubmitting = computed(
    () => session.value?.phase === DesignDeliveryUploadPhase.Uploading,
  )
  const isDeliveryUploading = computed(
    () => session.value?.phase === DesignDeliveryUploadPhase.Uploading,
  )

  function startSmoothLoop() {
    const tick = () => {
      const s = session.value
      if (!s || s.phase !== DesignDeliveryUploadPhase.Uploading) {
        stopSmoothLoop()
        return
      }
      const diff = s.targetPercent - s.displayPercent
      if (Math.abs(diff) < 0.15) {
        s.displayPercent = s.targetPercent
      } else {
        s.displayPercent += diff * 0.2
      }
      rafId = requestAnimationFrame(tick)
    }
    stopSmoothLoop()
    rafId = requestAnimationFrame(tick)
  }

  async function submitDeliveryAuditSequence(
    taskId: string,
    files: File[],
    opts?: { remarkSuffix?: string; targetSkuCode?: string },
  ): Promise<{ ok: true } | { ok: false; message: string }> {
    const oversized = findOversizedDesignFile(files)
    if (oversized) {
      return { ok: false, message: designUploadTooLargeMessage(oversized.name) }
    }
    if (files.length === 0) {
      return { ok: false, message: DESIGN_UPLOAD_COPY.submitHintNeedFiles }
    }
    /** 后端不区分 source/delivery；统一走 delivery 上传会话策略 */
    const orderedFiles = [...files]
    const tasksStore = useTasksStore()
    try {
      await tasksStore.loadTaskById(taskId)
    } catch {
      return { ok: false, message: '提交前刷新任务失败，请重试' }
    }
    const liveTask = tasksStore.getById(taskId)
    if (!liveTask || !canUploadDesignDelivery(liveTask)) {
      await tasksStore.forceRefreshList()
      return { ok: false, message: '任务已进入审核或不可上传状态，无法继续上传交付文件' }
    }

    const remarkSuffix = opts?.remarkSuffix ?? ''
    const targetSkuCode = opts?.targetSkuCode
    const totalBytes = orderedFiles.reduce((a, f) => a + f.size, 0)
    let uploadedBytes = 0

    session.value = {
      taskId,
      phase: DesignDeliveryUploadPhase.Uploading,
      displayPercent: 0,
      targetPercent: 0,
      currentFileIndex: 0,
      totalFiles: orderedFiles.length,
      fileName: '',
      fileSizeBytes: 0,
      multipartLabel: '',
      errorMessage: '',
      pendingFiles: [...orderedFiles],
      pendingBatches: undefined,
    }
    startSmoothLoop()

    const cancellableSessionIds = new Set<string>()
    const uploadedFacts: UploadedSessionFact[] = []
    try {
      const preparedList: PreparedTaskAssetUploadSession[] = []
      debugUploadLog('sequence:start', {
        taskId,
        files: orderedFiles.map((f) => f.name),
        total: orderedFiles.length,
      })
      for (let i = 0; i < orderedFiles.length; i++) {
        const file = orderedFiles[i]
        debugUploadLog('sequence:prepare:create', { index: i, file: file.name, size: file.size })
        const prepared = await prepareTaskAssetUploadSession(
          taskId,
          file,
          {
            asset_kind: 'delivery',
            target_sku_code: targetSkuCode || undefined,
            remark: file.name,
          },
          { remarkSuffix },
        )
        preparedList.push(prepared)
        cancellableSessionIds.add(prepared.sessionId)
        debugUploadLog('sequence:prepare:ok', { index: i, file: file.name, sessionId: prepared.sessionId })
      }
      debugUploadLog('sequence:prepare:done', { preparedCount: preparedList.length })

      for (let i = 0; i < orderedFiles.length; i++) {
        const file = orderedFiles[i]
        if (!session.value) throw new Error(DESIGN_UPLOAD_COPY.submitErrorFallback)
        session.value.currentFileIndex = i
        session.value.fileName = file.name
        session.value.fileSizeBytes = file.size

        const prepared = preparedList[i]
        if (!prepared) throw new Error('上传会话准备失败，请重试')
        debugUploadLog('sequence:complete:start', { index: i, file: file.name, sessionId: prepared.sessionId })
        // v1.8 409 asset_version_race_retry：严格单次自动重试；retry 预算由
        // completeWithAssetVersionRaceRetry 自行持有，外层循环不重复计数，防止双重预算。
        const { prepared: finalPrepared } = await completeWithAssetVersionRaceRetry(
          taskId,
          file,
          prepared,
          {
            asset_kind: prepared.assetKind,
            target_sku_code: prepared.targetSkuCode,
            remark: file.name,
            file_hash: prepared.fileHash,
            mime_type: prepared.sessionMime,
          },
          {
            remarkSuffix,
            onProgress: (p) => {
              if (!session.value) return
              const aggregateLoaded = uploadedBytes + p.loaded
              session.value.targetPercent =
                totalBytes > 0 ? Math.min(100, Math.round((aggregateLoaded / totalBytes) * 100)) : 0
              session.value.multipartLabel = formatMultipartPartsLabel(p)
            },
            onRetryPrepared: (next) => {
              cancellableSessionIds.delete(prepared.sessionId)
              cancellableSessionIds.add(next.sessionId)
              preparedList[i] = next
              debugUploadLog('sequence:complete:race-retry', {
                index: i,
                file: file.name,
                oldSessionId: prepared.sessionId,
                newSessionId: next.sessionId,
              })
            },
          },
        )
        const sessionKind: UploadAssetKind = finalPrepared.assetKind === 'delivery' ? 'delivery' : 'source'
        if (sessionKind !== 'delivery') {
          throw new Error('上传会话的 asset_kind 与预期不一致，请重新上传')
        }
        const sessionTargetSku = normalizeSkuCode(finalPrepared.targetSkuCode)
        const submitTargetSku = normalizeSkuCode(targetSkuCode)
        if ((sessionTargetSku ?? '') !== (submitTargetSku ?? '')) {
          throw new Error('target_sku_code 与上传会话不一致，请重新上传对应商品文件')
        }
        uploadedFacts.push({
          uploadSessionId: finalPrepared.sessionId,
          assetKind: 'delivery',
          targetSkuCode: submitTargetSku,
        })
        cancellableSessionIds.delete(finalPrepared.sessionId)
        debugUploadLog('sequence:complete:ok', { index: i, file: file.name, sessionId: finalPrepared.sessionId })
        uploadedBytes += file.size
        if (session.value) {
          // 仅保留未完成文件，避免部分成功后重试重复上传已成功项
          session.value.pendingFiles = orderedFiles.slice(i + 1)
        }
        if (session.value) {
          session.value.targetPercent =
            totalBytes > 0 ? Math.min(100, Math.round((uploadedBytes / totalBytes) * 100)) : 100
        }
      }

      await tasksStore.loadTaskById(taskId)
      const refreshed = tasksStore.getById(taskId)
      // 上传完成只保存暂存文件；全部 SKU 资源清单必须通过 submit-design 原子进入统一待审核状态。
      if (refreshed && canSubmitAudit(refreshed)) {
        const payload = toSubmitDesignAssetPayload(uploadedFacts, {
          requireTargetSkuForDelivery: Boolean(normalizeSkuCode(targetSkuCode)),
        })
        if (!payload.ok) throw new Error(payload.message)
        debugUploadLog('sequence:submit-design:start', {
          taskId,
          assetCount: payload.assets.length,
          deliveryCount: payload.assets.filter((a) => a.asset_kind === 'delivery').length,
        })
        try {
          await tasksStore.submitDesign(taskId, {
            assets: payload.assets,
            remark: orderedFiles.map((f) => f.name).join('、') + remarkSuffix,
          })
        } catch (err) {
          throw new Error(formatSubmitDesignFailureMessage(err))
        }
        debugUploadLog('sequence:submit-design:ok', { taskId })
      }
      await tasksStore.forceRefreshList()

      if (session.value) {
        session.value.displayPercent = 100
        session.value.targetPercent = 100
        session.value.phase = DesignDeliveryUploadPhase.Success
      }
      stopSmoothLoop()
      await sleep(DESIGN_UPLOAD_TIMING.successDisplayMs)
      session.value = null
      return { ok: true }
    } catch (e) {
      debugUploadLog('sequence:error', e)
      stopSmoothLoop()
      const statusLocked = isTaskStatusNotActionableUploadError(e)
      if (!statusLocked && cancellableSessionIds.size > 0) {
        await Promise.allSettled(
          Array.from(cancellableSessionIds).map((id) => cancelPreparedTaskAssetUploadSession(id)),
        )
      }
      try {
        // 即使序列中途失败，也回拉详情同步已成功上传的版本与状态
        await tasksStore.loadTaskById(taskId)
      } catch {
        // 不覆盖主错误
      }
      const pendingCount = session.value?.pendingFiles?.length ?? 0
      const baseMessage =
        e instanceof Error ? e.message : DESIGN_UPLOAD_COPY.submitErrorFallback
      const lockedStatus = readUploadDenyDetail(e, 'task_status')
      const message = statusLocked
        ? `上传会话处理失败：任务状态已切换为 ${lockedStatus || '不可上传'}。请刷新后核对已入库文件，并仅重传剩余 ${Math.max(0, pendingCount)} 个文件。`
        : appendPartialUploadHint(baseMessage, pendingCount)
      if (session.value) {
        session.value.phase = DesignDeliveryUploadPhase.Error
        session.value.errorMessage = message
        session.value.displayPercent = 0
        session.value.targetPercent = 0
      }
      return { ok: false, message }
    }
  }

  /**
   * 批量 SKU：按资源组串行上传（每组须带 target_sku_code）；全部完成后刷新任务并统一提交设计。
   */
  async function submitDeliveryAuditBatches(
    taskId: string,
    batches: DesignDeliveryAuditBatch[],
  ): Promise<{ ok: true } | { ok: false; message: string }> {
    const tasksStore = useTasksStore()
    try {
      await tasksStore.loadTaskById(taskId)
    } catch {
      return { ok: false, message: '提交前刷新任务失败，请重试' }
    }
    const liveTask = tasksStore.getById(taskId)
    if (!liveTask || !canUploadDesignDelivery(liveTask)) {
      await tasksStore.forceRefreshList()
      return { ok: false, message: '任务已进入审核或不可上传状态，无法继续上传交付文件' }
    }

    const nonEmpty = batches.filter((b) => b.files.length > 0)
    if (!nonEmpty.length) {
      return { ok: false, message: DESIGN_UPLOAD_COPY.submitErrorFallback }
    }

    const normalized = nonEmpty.map((b) => ({
      files: [...b.files],
      remarkSuffix: b.remarkSuffix,
      targetSkuCode: b.targetSkuCode,
    }))
    const allFiles = normalized.flatMap((b) => b.files)
    const oversized = findOversizedDesignFile(allFiles)
    if (oversized) {
      return { ok: false, message: designUploadTooLargeMessage(oversized.name) }
    }
    const totalBytes = allFiles.reduce((a, f) => a + f.size, 0)
    let uploadedBytes = 0
    let fileOrdinal = 0

    session.value = {
      taskId,
      phase: DesignDeliveryUploadPhase.Uploading,
      displayPercent: 0,
      targetPercent: 0,
      currentFileIndex: 0,
      totalFiles: allFiles.length,
      fileName: '',
      fileSizeBytes: 0,
      multipartLabel: '',
      errorMessage: '',
      pendingFiles: [...allFiles],
      pendingBatches: normalized.map((b) => ({
        files: [...b.files],
        remarkSuffix: b.remarkSuffix,
        targetSkuCode: b.targetSkuCode,
      })),
    }
    startSmoothLoop()

    const cancellableSessionIds = new Set<string>()
    const uploadedFacts: UploadedSessionFact[] = []
    try {
      const preparedBatches: PreparedTaskAssetUploadSession[][] = []
      debugUploadLog('batches:start', {
        taskId,
        bucketCount: normalized.length,
        totalFiles: allFiles.length,
      })
      for (const batch of normalized) {
        const remarkSuffix = batch.remarkSuffix ?? ''
        const targetSkuCode = batch.targetSkuCode
        const preparedRow: PreparedTaskAssetUploadSession[] = []
        for (const file of batch.files) {
          debugUploadLog('batches:prepare:create', {
            targetSkuCode,
            file: file.name,
            size: file.size,
          })
          const prepared = await prepareTaskAssetUploadSession(
            taskId,
            file,
            {
              asset_kind: 'delivery',
              target_sku_code: targetSkuCode || undefined,
              remark: file.name,
            },
            { remarkSuffix },
          )
          preparedRow.push(prepared)
          cancellableSessionIds.add(prepared.sessionId)
          debugUploadLog('batches:prepare:ok', {
            targetSkuCode,
            file: file.name,
            sessionId: prepared.sessionId,
          })
        }
        preparedBatches.push(preparedRow)
      }
      debugUploadLog('batches:prepare:done', {
        bucketCount: preparedBatches.length,
        preparedCount: preparedBatches.flatMap((row) => row).length,
      })

      for (let bi = 0; bi < normalized.length; bi++) {
        const batch = normalized[bi]!
        const preparedRow = preparedBatches[bi] ?? []
        for (let fi = 0; fi < batch.files.length; fi++) {
          const file = batch.files[fi]!
          if (!session.value) throw new Error(DESIGN_UPLOAD_COPY.submitErrorFallback)
          session.value.currentFileIndex = fileOrdinal
          session.value.fileName = file.name
          session.value.fileSizeBytes = file.size

          const prepared = preparedRow[fi]
          if (!prepared) throw new Error('上传会话准备失败，请重试')
          debugUploadLog('batches:complete:start', {
            bucketIndex: bi,
            fileIndex: fi,
            file: file.name,
            sessionId: prepared.sessionId,
          })
          // v1.8 409 asset_version_race_retry：按文件严格单次自动重试；
          // 预算持有在 completeWithAssetVersionRaceRetry 内部，batch 主循环不二次计数。
          const { prepared: finalPrepared } = await completeWithAssetVersionRaceRetry(
            taskId,
            file,
            prepared,
            {
              asset_kind: prepared.assetKind,
              target_sku_code: prepared.targetSkuCode,
              remark: file.name,
              file_hash: prepared.fileHash,
              mime_type: prepared.sessionMime,
            },
            {
              remarkSuffix: batch.remarkSuffix,
              onProgress: (p) => {
                if (!session.value) return
                const aggregateLoaded = uploadedBytes + p.loaded
                session.value.targetPercent =
                  totalBytes > 0 ? Math.min(100, Math.round((aggregateLoaded / totalBytes) * 100)) : 0
                session.value.multipartLabel = formatMultipartPartsLabel(p)
              },
              onRetryPrepared: (next) => {
                cancellableSessionIds.delete(prepared.sessionId)
                cancellableSessionIds.add(next.sessionId)
                preparedRow[fi] = next
                debugUploadLog('batches:complete:race-retry', {
                  bucketIndex: bi,
                  fileIndex: fi,
                  file: file.name,
                  oldSessionId: prepared.sessionId,
                  newSessionId: next.sessionId,
                })
              },
            },
          )
          const sessionKind: UploadAssetKind = finalPrepared.assetKind === 'delivery' ? 'delivery' : 'source'
          if (sessionKind !== 'delivery') {
            throw new Error('上传会话的 asset_kind 与预期不一致，请重新上传')
          }
          const submitTargetSku = normalizeSkuCode(batch.targetSkuCode)
          const sessionTargetSku = normalizeSkuCode(finalPrepared.targetSkuCode)
          if ((sessionTargetSku ?? '') !== (submitTargetSku ?? '')) {
            throw new Error('target_sku_code 与上传会话不一致，请重新上传对应商品文件')
          }
          uploadedFacts.push({
            uploadSessionId: finalPrepared.sessionId,
            assetKind: 'delivery',
            targetSkuCode: submitTargetSku,
          })
          cancellableSessionIds.delete(finalPrepared.sessionId)
          debugUploadLog('batches:complete:ok', {
            bucketIndex: bi,
            file: file.name,
            sessionId: finalPrepared.sessionId,
          })
          uploadedBytes += file.size
          fileOrdinal += 1
          if (session.value?.pendingBatches?.length) {
            const pb = session.value.pendingBatches
            const row = pb[bi]
            if (row?.files?.length) {
              const idx = row.files.findIndex((f) => f === file)
              if (idx >= 0) row.files.splice(idx, 1)
            }
            session.value.pendingFiles = pb.flatMap((b) => b.files)
          }
          if (session.value) {
            session.value.targetPercent =
              totalBytes > 0 ? Math.min(100, Math.round((uploadedBytes / totalBytes) * 100)) : 100
          }
        }
      }

      await tasksStore.loadTaskById(taskId)
      const refreshed = tasksStore.getById(taskId)
      const remark = normalized
        .map((b) => b.files.map((f) => f.name).join('、') + (b.remarkSuffix ?? ''))
        .join('；')
      if (refreshed && canSubmitAudit(refreshed)) {
        const payload = toSubmitDesignAssetPayload(uploadedFacts, {
          requireTargetSkuForDelivery: true,
        })
        if (!payload.ok) throw new Error(payload.message)
        debugUploadLog('batches:submit-design:start', {
          taskId,
          assetCount: payload.assets.length,
          deliveryCount: payload.assets.filter((a) => a.asset_kind === 'delivery').length,
        })
        try {
          await tasksStore.submitDesign(taskId, {
            assets: payload.assets,
            remark,
          })
        } catch (err) {
          throw new Error(formatSubmitDesignFailureMessage(err))
        }
        debugUploadLog('batches:submit-design:ok', { taskId })
      }
      await tasksStore.forceRefreshList()

      if (session.value) {
        session.value.displayPercent = 100
        session.value.targetPercent = 100
        session.value.phase = DesignDeliveryUploadPhase.Success
      }
      stopSmoothLoop()
      await sleep(DESIGN_UPLOAD_TIMING.successDisplayMs)
      session.value = null
      return { ok: true }
    } catch (e) {
      debugUploadLog('batches:error', e)
      stopSmoothLoop()
      const statusLocked = isTaskStatusNotActionableUploadError(e)
      if (!statusLocked && cancellableSessionIds.size > 0) {
        await Promise.allSettled(
          Array.from(cancellableSessionIds).map((id) => cancelPreparedTaskAssetUploadSession(id)),
        )
      }
      try {
        // 即使序列中途失败，也回拉详情同步已成功上传的版本与状态
        await tasksStore.loadTaskById(taskId)
      } catch {
        // 不覆盖主错误
      }
      const pendingCount = session.value?.pendingFiles?.length ?? 0
      const baseMessage =
        e instanceof Error ? e.message : DESIGN_UPLOAD_COPY.submitErrorFallback
      const lockedStatus = readUploadDenyDetail(e, 'task_status')
      const message = statusLocked
        ? `上传会话处理失败：任务状态已切换为 ${lockedStatus || '不可上传'}。请刷新后核对已入库文件，并仅重传剩余 ${Math.max(0, pendingCount)} 个文件。`
        : appendPartialUploadHint(baseMessage, pendingCount)
      if (session.value) {
        session.value.phase = DesignDeliveryUploadPhase.Error
        session.value.errorMessage = message
        session.value.displayPercent = 0
        session.value.targetPercent = 0
      }
      return { ok: false, message }
    }
  }

  async function retryDeliveryAudit(
    taskId: string,
    opts?: { remarkSuffix?: string; targetSkuCode?: string },
  ) {
    const s = session.value
    if (!s || s.phase !== DesignDeliveryUploadPhase.Error) {
      return { ok: false as const, message: DESIGN_UPLOAD_COPY.submitErrorFallback }
    }
    if (s.pendingBatches?.length) {
      return submitDeliveryAuditBatches(taskId, s.pendingBatches)
    }
    return submitDeliveryAuditSequence(taskId, s.pendingFiles, opts)
  }

  function clearSession() {
    stopSmoothLoop()
    session.value = null
  }

  function clearSubmitState() {
    clearSession()
  }

  return {
    session,
    isSubmitting,
    isDeliveryUploading,
    submitDeliveryAuditSequence,
    submitDeliveryAuditBatches,
    retryDeliveryAudit,
    clearSession,
    clearSubmitState,
  }
})
