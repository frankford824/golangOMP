export interface WorkbenchResourceFile {
  task_asset_id: number
  file_name: string
  mime_type?: string
  file_size?: number | null
  download_url?: string
}

export interface WorkbenchResourceRevisionItem {
  id: number
  sort_order: number
  task_asset_id: number
  item_name?: string
  file?: WorkbenchResourceFile
}

export interface WorkbenchResourceRevision {
  id: number
  mode: 'single' | 'set'
  items: WorkbenchResourceRevisionItem[]
}

export interface WorkbenchResourceGroup {
  id: number
  task_id: number
  task_no?: string
  sku_code?: string
  scope_kind: 'task' | 'sku' | 'retouch_requirement'
  finalized_revision_id?: number | null
  finalized_revision?: WorkbenchResourceRevision | null
}

export interface WorkbenchResourceGroupList {
  items: WorkbenchResourceGroup[]
  page: number
  page_size: number
  total: number
}

export interface WorkbenchResourceDownloadManifest {
  items: Array<{
    group_id: number
    revision_id: number
    revision_item_id: number
    task_id: number
    sku_code?: string
    sort_order: number
    filename: string
    mime_type?: string
    file_size?: number | null
    download_url: string
  }>
}
