import http from '@/services/http'
import type {
  AssetUploadSessionCreateResponse,
  CompleteAssetUploadSessionPayload,
  CreateAssetUploadSessionPayload,
  ReferenceFileRef,
} from '@/services/api/assetsApi'

export interface TaskCreateReferenceUploadCompleteResponse {
  data?: {
    ref_object?: ReferenceFileRef
  }
}

export const taskAssetsApi = {
  createTaskCreateUploadSession: (payload: CreateAssetUploadSessionPayload, signal?: AbortSignal) =>
    http.post<AssetUploadSessionCreateResponse>(
      '/v1/tasks/reference-upload-sessions',
      {
        filename: payload.file_name,
        expected_size: payload.expected_size,
        mime_type: payload.mime_type,
        file_hash: payload.file_hash,
        remark: payload.remark,
      },
      { signal },
    ),

  completeTaskCreateUploadSession: (
    sessionId: string,
    payload: CompleteAssetUploadSessionPayload,
    signal?: AbortSignal,
  ) =>
    http.post<TaskCreateReferenceUploadCompleteResponse>(
      `/v1/tasks/reference-upload-sessions/${sessionId}/complete`,
      payload,
      { signal },
    ),

  abortTaskCreateUploadSession: (
    sessionId: string,
    payload: { oss_object_key?: string; oss_upload_id?: string } = {},
    signal?: AbortSignal,
  ) => http.post(`/v1/tasks/reference-upload-sessions/${sessionId}/abort`, payload, { signal }),
}
