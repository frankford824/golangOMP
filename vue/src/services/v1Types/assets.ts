export interface UploadSession {
  session_id: string
  upload_url?: string
  upload_urls?: string[]
  upload_strategy?: 'small' | 'multipart' | string
  oss_upload_id?: string
  oss_object_key?: string
  expires_at?: string
}

export interface AssetVersion {
  id: string
  version_no?: number
  file_name?: string
  storage_key?: string | null
  created_at?: string
}

export interface AssetRecord {
  id: string
  file_name?: string
  module_key?: string
  owner_team_code?: string
  latest_version?: AssetVersion
}
