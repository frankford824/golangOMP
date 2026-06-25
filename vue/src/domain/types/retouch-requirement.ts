import type { ReferenceFileRef } from '@/services/api/assetsApi'
import type { BackendAsset } from '@/services/apiTypes'

/** API / domain model for one retouch_task structured demand line. */
export interface RetouchRequirement {
  id: number
  taskId: number
  description: string
  skuCode?: string
  spec?: string
  remark?: string
  sortOrder: number
  createdBy?: number | null
  updatedBy?: number | null
  createdAt?: string
  updatedAt?: string
  /** Requirement-scoped reference images (GET read model). */
  referenceFileRefs?: ReferenceFileRef[]
  /** Requirement-scoped source/material files (`asset_type=source`). */
  sourceAssets?: BackendAsset[]
}

/** Create-form draft row; not persisted until POST /v1/tasks. */
export interface RetouchRequirementDraft {
  description: string
  skuCode?: string
  spec?: string
  remark?: string
  sortOrder?: number
  /** 创建前本地暂存，POST /v1/tasks 后不发送；创建成功后按 retouch_requirement_id 上传。 */
  pendingReferenceFiles?: File[]
  /** 创建前本地暂存，创建成功后以 asset_kind=source 上传。 */
  pendingSourceFiles?: File[]
}

export function createEmptyRetouchRequirementDraft(sortOrder = 1): RetouchRequirementDraft {
  return {
    description: '',
    sortOrder,
    pendingReferenceFiles: [],
    pendingSourceFiles: [],
  }
}
