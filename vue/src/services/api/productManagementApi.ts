import http from '@/services/http'

export type ProductImageSource =
  | 'manual'
  | 'erp_product_image'
  | 'delivery'
  | 'derived_preview'
  | 'task_reference'
  | 'auto_on_close'
  | 'missing'

export type ProductSyncStatus =
  | 'pending_sync'
  | 'queued'
  | 'syncing'
  | 'synced'
  | 'failed'
  | 'cooling_down'
  | 'waiting_image'

export interface ProductManagementRecord {
  id: number
  record_key: string
  task_id: number
  task_sku_item_id?: number
  task_no: string
  task_type?: string
  source_mode?: string
  sku_code: string
  product_i_id: string
  erp_i_id?: string
  category_name?: string
  product_family?: string
  product_name: string
  cost_price?: number | null
  creator_id: number
  creator_name: string
  task_created_at: string
  image_source: ProductImageSource
  image_source_label: string
  image_selection_mode: 'auto' | 'manual'
  image_asset_id?: number
  image_asset_version_id?: number
  image_preview_url?: string
  image_filename?: string
  image_mime_type?: string
  image_missing_reason?: string
  image_sync_source?: ProductImageSource
  erp_sync_status: ProductSyncStatus
  base_sync_status?: ProductSyncStatus
  image_sync_status?: ProductSyncStatus
  last_erp_synced_at?: string
  last_erp_checked_at?: string
  last_base_synced_at?: string
  last_image_synced_at?: string
  sync_cooldown_until?: string
  last_sync_error?: string
  base_sync_error?: string
  image_sync_error?: string
  image_required?: boolean
  can_maintain_image: boolean
  can_cross_task_select: boolean
  can_sync_erp: boolean
  can_force_override: boolean
  created_at: string
  updated_at: string
}

export interface ProductImageCandidate {
  asset_id: number
  asset_version_id: number
  task_id: number
  task_no: string
  sku_code?: string
  source: ProductImageSource
  source_label: string
  preview_url?: string
  file_name: string
  mime_type?: string
  created_at: string
}

export interface ProductManagementListParams {
  keyword?: string
  display_scope?: 'combo' | 'single' | 'all'
  image_source?: ProductImageSource | ''
  sync_status?: ProductSyncStatus | ''
  base_sync_status?: ProductSyncStatus | ''
  image_sync_status?: ProductSyncStatus | ''
  cost_status?: 'missing' | 'ready' | ''
  issue_scope?: 'attention' | 'all'
  creator_id?: number
  page?: number
  page_size?: number
}

export interface ProductManagementPagination {
  page: number
  page_size: number
  total: number
}

export interface ProductManagementListResponse {
  data: ProductManagementRecord[]
  pagination: ProductManagementPagination
}

export interface ProductManagementComboChild {
  record: ProductManagementRecord
  quantity: number
}

export interface ProductManagementComboGroup {
  group_key: string
  group_type: 'combo' | 'single'
  combo_sku_code?: string
  combo_name?: string
  combo_short_name?: string
  erp_i_id?: string
  enabled?: boolean | null
  cost_price?: number | null
  sale_price?: number | null
  modified_at?: string
  last_synced_at?: string
  children: ProductManagementComboChild[]
}

export interface ProductManagementComboSyncSummary {
  id: number
  window_begin: string
  window_end: string
  page_index: number
  page_size: number
  status: string
  last_success_at?: string
  next_retry_at?: string
  last_error?: string
  processed_items: number
}

export interface ProductManagementComboTreeResponse extends ProductManagementListResponse {
  groups: ProductManagementComboGroup[]
  combo_sync_summary?: ProductManagementComboSyncSummary
}

export const productManagementApi = {
  async list(params: ProductManagementListParams): Promise<ProductManagementListResponse> {
    const { data } = await http.get<ProductManagementListResponse>('/v1/product-management', { params })
    return data
  },

  async listComboTree(params: ProductManagementListParams): Promise<ProductManagementComboTreeResponse> {
    const { data } = await http.get<ProductManagementComboTreeResponse>('/v1/product-management/combo-tree', { params })
    return data
  },

  async listByTask(taskId: number): Promise<ProductManagementRecord[]> {
    const { data } = await http.get<{ data: ProductManagementRecord[] }>(`/v1/tasks/${taskId}/product-management`)
    return data.data ?? []
  },

  async listImageCandidates(recordId: number): Promise<ProductImageCandidate[]> {
    const { data } = await http.get<{ data: ProductImageCandidate[] }>(
      `/v1/product-management/${recordId}/image-candidates`,
    )
    return data.data ?? []
  },

  async reparseImage(recordId: number): Promise<ProductManagementRecord> {
    const { data } = await http.post<{ data: ProductManagementRecord }>(
      `/v1/product-management/${recordId}/reparse-image`,
    )
    return data.data
  },

  async setManualImage(recordId: number, assetId: number): Promise<ProductManagementRecord> {
    const { data } = await http.post<{ data: ProductManagementRecord }>(
      `/v1/product-management/${recordId}/image`,
      { asset_id: assetId },
    )
    return data.data
  },

  async requestSync(recordId: number, force = false): Promise<ProductManagementRecord> {
    const { data } = await http.post<{ data: ProductManagementRecord }>(
      `/v1/product-management/${recordId}/sync-request`,
      { force },
    )
    return data.data
  },

  async requestBaseSync(recordId: number, force = false): Promise<ProductManagementRecord> {
    const { data } = await http.post<{ data: ProductManagementRecord }>(
      `/v1/product-management/${recordId}/base-sync-request`,
      { force },
    )
    return data.data
  },

  async requestImageSync(recordId: number, force = false): Promise<ProductManagementRecord> {
    const { data } = await http.post<{ data: ProductManagementRecord }>(
      `/v1/product-management/${recordId}/image-sync-request`,
      { force },
    )
    return data.data
  },
}
