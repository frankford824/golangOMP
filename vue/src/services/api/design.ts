/**
 * 设计稿交付（delivery）与任务内参考图补充上传：统一走 POST /v1/assets/upload-sessions，
 * 由后端返回的 upload_strategy 驱动远端上传，再 complete。
 */
import type { AssetUploadProgress } from '@/services/api/assetsApi'
import {
  formatMultipartPartsLabel,
} from '@/services/api/assetsApi'
import { uploadTaskFileViaAssetSession } from '@/services/upload/assetUploadFlow'
import axios from 'axios'
import type { HttpAppError } from '@/services/http'
import { formatUploadFailureMessage } from '@/utils/upload-errors'

export interface DeliveryUploadProgressPayload {
  loaded: number
  total: number
  percent: number
  multipartLabel: string
}

function isTargetSkuBelongError(err: unknown): boolean {
  let data: Record<string, unknown> | undefined
  if (axios.isAxiosError(err)) {
    data = err.response?.data as Record<string, unknown> | undefined
  } else {
    const appErr = err as HttpAppError
    const rd = appErr?.responseData
    if (rd && typeof rd === 'object') data = rd as Record<string, unknown>
  }
  if (!data) return false
  const e = (data?.error ?? {}) as Record<string, unknown>
  const msg = String(e.message ?? '').toLowerCase()
  const code = String(e.code ?? '').toUpperCase()
  return code === 'INVALID_REQUEST' && msg.includes('target_sku_code must belong to the task')
}

/**
 * 单文件上传至 delivery 资产会话，进度经 onProgress。
 */
export async function uploadDeliveryFileViaAssetSession(
  taskId: string,
  file: File,
  onProgress: (p: DeliveryUploadProgressPayload) => void,
  options?: { remarkSuffix?: string; targetSkuCode?: string },
): Promise<void> {
  const remarkSuffix = options?.remarkSuffix ?? ''
  const targetSkuCode = options?.targetSkuCode?.trim()

  await uploadTaskFileViaAssetSession(
    taskId,
    file,
    {
      asset_kind: 'delivery',
      target_sku_code: targetSkuCode || undefined,
      remark: file.name,
    },
    {
      remarkSuffix,
      onProgress: (progress: AssetUploadProgress) => {
        onProgress({
          loaded: progress.loaded,
          total: progress.total,
          percent: progress.percent,
          multipartLabel: formatMultipartPartsLabel(progress),
        })
      },
    },
  )
}

/**
 * 任务详情内补充参考图：与 delivery 相同 canonical 上传链，asset_kind=reference。
 */
export async function uploadTaskReferenceFileViaAssetSession(
  taskId: string,
  file: File,
  options?: { targetSkuCode?: string; remarkSuffix?: string },
): Promise<void> {
  const remarkSuffix = options?.remarkSuffix ?? ''
  const targetTrim = options?.targetSkuCode?.trim() || undefined
  try {
    await uploadTaskFileViaAssetSession(
      taskId,
      file,
      {
        asset_kind: 'reference',
        target_sku_code: targetTrim,
        remark: file.name,
      },
      { remarkSuffix, onProgress: () => {} },
    )
  } catch (err) {
    if (targetTrim && isTargetSkuBelongError(err)) {
      throw new Error('target_sku_code 与任务商品不匹配，请先修正 SKU 绑定后重试')
    }
    throw err instanceof Error ? err : new Error(formatUploadFailureMessage('reference_upload', err))
  }
}
