import {
  normalizeRetouchRequirementDraftsWithPending,
  sortRetouchRequirementDraftsBySortOrder,
  sortRetouchRequirementsBySortOrder,
} from '@/domain/retouch-requirements'
import type { RetouchRequirement, RetouchRequirementDraft } from '@/domain/types/retouch-requirement'
import { uploadReferenceFileRef, uploadTaskFileViaAssetSession } from '@/services/upload/assetUploadFlow'
import { formatUploadFailureMessage } from '@/utils/upload-errors'

export type RetouchRequirementUploadFailureKind = 'requirement_id' | 'reference' | 'source' | 'alignment'

export interface RetouchRequirementUploadFailure {
  requirementIndex: number
  kind: RetouchRequirementUploadFailureKind
  fileName?: string
  message: string
}

export interface RetouchRequirementUploadResult {
  failures: RetouchRequirementUploadFailure[]
  referenceUploaded: number
  sourceUploaded: number
}

export const RETOUCH_REQUIREMENT_UPLOAD_PARTIAL_FAILURE_MESSAGE =
  '任务已创建，但部分需求附件上传失败，请进入详情页补传或重新上传。'

export interface UploadRetouchRequirementPendingAssetsOptions {
  onStatusMessage?: (message: string) => void
  signal?: AbortSignal
}

export async function uploadRetouchRequirementPendingAssets(
  taskId: string,
  createdRequirements: RetouchRequirement[],
  drafts: RetouchRequirementDraft[],
  options?: UploadRetouchRequirementPendingAssetsOptions,
): Promise<RetouchRequirementUploadResult> {
  const failures: RetouchRequirementUploadFailure[] = []
  let referenceUploaded = 0
  let sourceUploaded = 0

  const normalizedDrafts = sortRetouchRequirementDraftsBySortOrder(
    normalizeRetouchRequirementDraftsWithPending(drafts),
  )
  const sortedRequirements = sortRetouchRequirementsBySortOrder(
    createdRequirements.filter((row) => row.id > 0),
  )

  if (sortedRequirements.length < normalizedDrafts.length) {
    for (let i = sortedRequirements.length; i < normalizedDrafts.length; i++) {
      failures.push({
        requirementIndex: i,
        kind: 'alignment',
        message: `需求 ${i + 1} 未返回有效 ID，无法上传本条附件`,
      })
    }
  }

  const pairCount = Math.min(sortedRequirements.length, normalizedDrafts.length)
  for (let index = 0; index < pairCount; index++) {
    const requirement = sortedRequirements[index]
    const draft = normalizedDrafts[index]
    const requirementId = requirement?.id
    if (!requirementId || requirementId <= 0) {
      failures.push({
        requirementIndex: index,
        kind: 'requirement_id',
        message: `需求 ${index + 1} 缺少 retouch_requirement_id`,
      })
      continue
    }

    const refFiles = draft.pendingReferenceFiles ?? []
    for (const file of refFiles) {
      options?.onStatusMessage?.(`正在上传需求 ${index + 1} 参考图：${file.name}`)
      try {
        await uploadReferenceFileRef(file, {
          taskId,
          retouchRequirementId: requirementId,
          ownerModuleKey: 'basic_info',
          uploadPolicy: 'append_only',
          signal: options?.signal,
        })
        referenceUploaded += 1
      } catch (error) {
        failures.push({
          requirementIndex: index,
          kind: 'reference',
          fileName: file.name,
          message: formatUploadFailureMessage('reference_upload', error),
        })
      }
    }

    const sourceFiles = draft.pendingSourceFiles ?? []
    for (const file of sourceFiles) {
      options?.onStatusMessage?.(`正在上传需求 ${index + 1} 素材：${file.name}`)
      try {
        await uploadTaskFileViaAssetSession(
          taskId,
          file,
          { asset_kind: 'source', remark: file.name },
          { retouchRequirementId: requirementId, signal: options?.signal },
        )
        sourceUploaded += 1
      } catch (error) {
        failures.push({
          requirementIndex: index,
          kind: 'source',
          fileName: file.name,
          message: formatUploadFailureMessage('main_complete', error),
        })
      }
    }
  }

  return { failures, referenceUploaded, sourceUploaded }
}

export function hasRetouchRequirementPendingUploads(drafts: RetouchRequirementDraft[] | undefined): boolean {
  return normalizeRetouchRequirementDraftsWithPending(drafts).some(
    (row) => (row.pendingReferenceFiles?.length ?? 0) > 0 || (row.pendingSourceFiles?.length ?? 0) > 0,
  )
}
