import http from '@/services/http'
import type {
  AssetUploadSessionCreateResponse,
  CreateAssetUploadSessionPayload,
} from '@/services/api/assetsApi'

/**
 * 仅保留「创建前无 task_id」所需的兼容入口：POST /v1/task-create/asset-center/upload-sessions。
 * 已有 task_id 的上传请统一用 {@link assetsApi.createAssetUploadSession}（/v1/assets/upload-sessions）。
 */
export const taskAssetsApi = {
  createTaskCreateUploadSession: (payload: CreateAssetUploadSessionPayload, signal?: AbortSignal) =>
    http.post<AssetUploadSessionCreateResponse>(
      '/v1/task-create/asset-center/upload-sessions',
      payload,
      { signal },
    ),
}
