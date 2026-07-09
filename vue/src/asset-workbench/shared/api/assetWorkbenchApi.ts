import http from '@/services/http'
import type { BackendUser } from '@/services/apiTypes'

interface ApiEnvelope<T> {
  data?: T
  pagination?: {
    total?: number
    page?: number
    page_size?: number
  }
}

export interface AssetWorkbenchProfile {
  id: number
  user_id: number
  worker_type: string
  job_grade: string
  real_name: string
  phone?: string
  province?: string
  city?: string
  id_card?: string
  gender?: string
  alipay_account?: string
  onboarded_at?: string
  grade_hidden?: boolean
  status: string
  pii_completed: boolean
}

export interface AssetWorkbenchRegisterPayload {
  account: string
  name: string
  phone: string
  email?: string
  password: string
  worker_type?: string
  province?: string
  city?: string
  id_card?: string
  gender?: string
  alipay_account?: string
}

export interface AssetWorkbenchRegisterResult {
  auth?: {
    user?: BackendUser
    session?: {
      session_id?: string
      token?: string
      token_type?: string
      expires_at?: string
    }
  }
  profile?: AssetWorkbenchProfile
}

export interface UpsertProfilePayload {
  worker_type?: string
  job_grade?: string
  real_name?: string
  phone?: string
  province?: string
  city?: string
  id_card?: string
  gender?: string
  alipay_account?: string
  onboarded_at?: string
  grade_hidden?: boolean
  status?: string
  reason?: string
}

export interface AssetWorkbenchBootstrap {
  app: string
  version: string
  timezone: string
  oss_prefix: string
  upload_session_ttl_seconds: number
  is_admin: boolean
  access?: WorkbenchAccessState
  role_labels?: string[]
  user?: BackendUser
  profile?: AssetWorkbenchProfile
  capabilities: string[]
  settlement_item_types: string[]
  deferred_business_items: Array<{ key: string; status: string; note: string }>
  architecture_guardrails: string[]
}

export interface WorkbenchAccessState {
  membership_status: 'not_member' | 'pending' | 'active' | 'disabled' | 'merged' | string
  is_enabled: boolean
  is_admin_shell: boolean
  asset_roles: string[]
  role_labels: string[]
  capabilities: string[]
  denied_reason?: string
}

export interface WorkbenchEntryResult {
  state: 'ready' | 'not_member' | 'pending' | 'disabled' | 'merged' | string
  message: string
  access?: WorkbenchAccessState
  bootstrap?: AssetWorkbenchBootstrap
}

export interface PriceMatrixRow {
  id: number
  worker_type: string
  job_grade: string
  difficulty_class: string
  unit_price: number
  effective_from: string
  effective_to?: string
  enabled: boolean
}

export interface DeductionRuleRow {
  id: number
  worker_type: string
  job_grade: string
  difficulty_class: string
  deduction_amount: number
  effective_from: string
  effective_to?: string
  enabled: boolean
}

export interface WelfareRuleRow {
  id: number
  rule_name: string
  worker_type: string
  job_grade: string
  rule_type: string
  amount: number
  effective_from: string
  effective_to?: string
  enabled: boolean
}

export interface PromoCouponRow {
  id: number
  coupon_code: string
  coupon_name: string
  mode: string
  amount?: number
  percent?: number
  priority: number
  worker_type: string
  job_grade: string
  difficulty_class: string
  effective_from: string
  effective_to?: string
  enabled: boolean
}

export interface DifficultyClassRow {
  id: number
  code: string
  name: string
  description: string
  enabled: boolean
  sort_order: number
  created_by?: number
  updated_by?: number
  created_at?: string
  updated_at?: string
}

export interface WorkbenchGroupRow {
  id: number
  name: string
  description: string
  enabled: boolean
  created_by: number
}

export interface WorkbenchMemberRow {
  user_id: number
  username: string
  display_name: string
  real_name: string
  worker_type: string
  job_grade: string
  status: string
  pii_completed: boolean
  identity?: string
  roles?: string[]
  role_labels?: string[]
  can_edit_roles?: boolean
}

export interface WorkbenchGroupMemberRow {
  group_id: number
  user_id: number
  username?: string
  display_name?: string
  real_name?: string
  worker_type?: string
  job_grade?: string
  identity?: string
  roles?: string[]
  role_labels?: string[]
  pii_completed?: boolean
}

export interface AccountMergePreview {
  source_user_id: number
  canonical_user_id: number
  conflicts: Record<string, { field: string; source_value: string; canonical_value: string }>
  counts: Record<string, number>
  affected_months?: string[]
  settlement_note: string
}

export interface SubmissionRow {
  id: number
  submission_no: string
  submitter_user_id: number
  submitter_name?: string
  submitter_username?: string
  business_month: string
  submitted_at: string
  status: string
  item_count: number
  file_count: number
  page_count: number
  gross_total: number
}

export interface SubmissionItemRow {
  id: number
  submission_id: number
  payee_user_id: number
  order_no: string
  template_id?: number
  template_name_snapshot?: string
  category_snapshot?: string
  difficulty_class: string
  finalized: boolean
  page_count: number
  item_count: number
  business_month: string
  gross_amount: number
  pricing_status: string
  qc_status: string
  settlement_status: string
}

export interface SubmissionFileRow {
  id: number
  submission_id: number
  submission_item_id: number
  upload_directory_id?: number
  upload_directory_name?: string
  upload_directory_prefix?: string
  upload_directory_difficulty_class?: string
  upload_batch_id?: string
  relative_path?: string
  display_name?: string
  is_folder_upload?: boolean
  original_filename: string
  file_type: string
  mime_type: string
  file_size: number
  file_hash?: string
  preview_status: string
  preview_key?: string
  preview_error?: string
}

export interface SubmissionDetail {
  submission: SubmissionRow
  items: Array<{
    item: SubmissionItemRow
    files: SubmissionFileRow[]
  }>
}

export interface SettlementPreviewRow {
  payee_user_id: number
  payee_name?: string
  worker_type?: string
  item_count: number
  page_count: number
  gross_amount: number
  error_count: number
  deduction_amount: number
  welfare_amount: number
  supplement_amount: number
  net_amount: number
}

export interface SettlementPayrollRow {
  payee_user_id: number
  payee_name?: string
  worker_type?: string
  business_month: string
  row_type: 'normal_piecework' | 'supplement_piecework'
  item_count: number
  page_count: number
  gross_amount: number
  error_count: number
  deduction_amount: number
  welfare_amount: number
  supplement_amount: number
  adjustment_amount: number
  net_amount: number
}

export interface SettlementPreview {
  business_month: string
  rows: SettlementPreviewRow[]
  totals: SettlementPreviewRow
  payroll_rows: SettlementPayrollRow[]
}

export interface SettlementReportDifficultyMetric {
  difficulty_class: string
  order_count: number
  item_count: number
  page_count: number
  gross_amount: number
  error_count: number
  deduction_amount: number
  error_rate: number
  page_count_share: number
  error_count_share: number
  month_page_count_share: number
}

export interface SettlementReportRow {
  payee_user_id: number
  business_month: string
  row_type: 'normal_piecework' | 'supplement_piecework' | 'total'
  creator_name: string
  job_grade: string
  created_date: string
  order_count: number
  item_count: number
  page_count: number
  gross_amount: number
  error_count: number
  deduction_amount: number
  welfare_amount: number
  supplement_amount: number
  net_amount: number
  error_rate: number
  page_count_share: number
  error_count_share: number
  month_amount_share: number
  difficulty_metrics: SettlementReportDifficultyMetric[]
}

export interface SettlementReport {
  business_month: string
  difficulty_classes: string[]
  rows: SettlementReportRow[]
  totals: SettlementReportRow
  generated_at: string
  order_count_policy: string
  settlement_data_mode: string
}

export interface SettlementBatchRow {
  id: number
  batch_no: string
  business_month: string
  status: string
  item_count: number
  gross_amount: number
  deduction_amount: number
  welfare_amount: number
  supplement_amount: number
  adjustment_amount: number
  net_amount: number
}

export interface SettlementItemRow {
  id: number
  batch_id: number
  item_type: string
  submission_item_id?: number
  payee_user_id: number
  business_month: string
  amount: number
  quantity: number
  unit_price?: number
  direction: string
  source_ref_type: string
  source_ref_id?: number
}

export interface SettlementBatchDetail {
  batch: SettlementBatchRow
  items: SettlementItemRow[]
  payroll_rows: SettlementPayrollRow[]
}

export interface MySettlementMonthRow {
  business_month: string
  item_count: number
  page_count: number
  gross_amount: number
  deduction_amount: number
  welfare_amount: number
  supplement_amount: number
  adjustment_amount: number
  net_amount: number
  confirmed: boolean
}

export interface MySettlementResult {
  current_month: string
  estimated_net_amount: number
  months: MySettlementMonthRow[]
}

export interface SettlementAdjustmentRow {
  id: number
  batch_id?: number
  payee_user_id: number
  business_month: string
  adjustment_type: string
  amount: number
  reason: string
  status: string
  payload_json?: Record<string, unknown>
  created_by: number
}

export interface SettlementSupplementRow {
  id: number
  payee_user_id: number
  business_month: string
  linked_batch_id?: number
  status: string
  order_no: string
  supplement_date?: string
  difficulty_class: string
  finalized: boolean
  page_count: number
  gross_amount: number
  duplicate_hint_json?: {
    has_duplicates?: boolean
    submission_item_ids?: number[]
    supplement_ids?: number[]
    order_no?: string
    supplement_date?: string
    business_month?: string
    payee_user_id?: number
  }
}

export interface SettlementSupplementImportResult {
  created: SettlementSupplementRow[]
  failures: Array<{
    row: number
    reason: string
  }>
}

export interface SupplementPermissionRow {
  id: number
  payee_user_id: number
  business_month: string
  enabled: boolean
  reason: string
  granted_by: number
  revoked_by?: number
  granted_at: string
  revoked_at?: string
}

export interface ErrorImportBatchRow {
  id: number
  import_no: string
  business_month: string
  uploaded_by: number
  original_filename: string
  status: string
  total_rows: number
  matched_rows: number
  unmatched_rows: number
  ambiguous_rows: number
  error_message?: string
}

export interface AssetWorkbenchEventRow {
  id: number
  actor_user_id?: number
  actor_display_name?: string
  actor_username?: string
  event_type: string
  entity_type: string
  entity_id?: number
  reason: string
  created_at: string
}

export interface NotificationRow {
  id: number
  user_id?: number
  notification_type: string
  payload?: Record<string, unknown>
  is_read: boolean
  read_at?: string
  created_at: string
}

export interface NotificationListResult {
  items: NotificationRow[]
  next_cursor?: string
}

export interface AssetWorkbenchSavedView {
  id: number
  user_id: number
  view_type: string
  view_name: string
  config_json: Record<string, unknown>
  is_default: boolean
  created_at: string
  updated_at: string
}

export interface FilePreviewMeta {
  file_id: number
  status: string
  preparing: boolean
  preview_url?: string
  download_url?: string
  expires_at?: string
  mime_type?: string
  filename?: string
  preview_available?: boolean
  error?: string
}

export interface FileDownloadMeta {
  file_id: number
  filename: string
  mime_type: string
  file_size: number
  download_url: string
  expires_at: string
}

export interface FileBatchDownloadManifest {
  items: FileDownloadMeta[]
  failures?: Array<{
    file_id: number
    reason: string
  }>
}

export interface BatchFileMutationResult {
  files?: SubmissionFileRow[]
  deleted?: number[]
  failures?: Array<{
    file_id: number
    reason: string
  }>
}

export interface UpdateSubmissionItemPayload {
  order_no?: string
  difficulty_class?: string
  finalized?: boolean
  page_count?: number
  reason?: string
}

export interface SubmissionItemQCImportResult {
  updated: SubmissionItemRow[]
  failures: Array<{
    row: number
    reason: string
  }>
}

export interface SystemAssetDownloadInfo {
  download_mode: string
  download_url?: string
  access_hint?: string
  preview_available?: boolean
  filename: string
  file_size: number
  mime_type?: string
  expires_at?: string
}

export interface SystemAssetPreviewMeta {
  asset_id: number
  source_type?: string
  source_ref?: string
  status: string
  preparing: boolean
  preview_url?: string
  download_url?: string
  expires_at?: string
  mime_type?: string
  filename?: string
  preview_available: boolean
}

export interface SystemAssetRow {
  id: number
  material_id?: number
  resource_id?: string
  source_type?: string
  source_label?: string
  asset_no?: string
  scope_sku_code?: string
  sku_code?: string
  primary_sku_code?: string
  file_name?: string
  original_filename?: string
  preview_url?: string
  download_url?: string
  mime_type?: string
  product_name?: string
  task_no?: string
  created_by_name?: string
  created_by_username?: string
  task_creator_name?: string
  task_creator_username?: string
  preview_available?: boolean
  origin_path?: string
  created_at?: string
  updated_at?: string
}

export interface SystemSearchResult {
  items: SystemAssetRow[]
  total: number
  page: number
  size: number
}

export interface MaterialFolderRow {
  path: string
  name: string
  source_type: 'system' | 'external' | string
  file_count: number
  direct_file_count?: number
}

export interface MaterialBrowseResult {
  path: string
  folders: MaterialFolderRow[]
  files: SystemAssetRow[]
  total: number
  page: number
  size: number
}

export interface OverviewSearchRow {
  source: 'system_asset' | 'client_material' | 'submission_file' | 'submission' | 'piecework_item' | string
  scope?: 'all' | 'operational' | 'files' | 'orders' | string
  source_label?: string
  id: number
  title: string
  primary_code: string
  secondary_code?: string
  order_no?: string
  creator_user_id?: number
  creator_name?: string
  business_month?: string
  status?: string
  page_count?: number
  amount?: number
  created_at: string
  updated_at?: string
  route_path?: string
  locate?: {
    source?: string
    file_id?: number
    submission_id?: number
    item_id?: number
    order_no?: string
    material_id?: number
    source_type?: string
    source_ref?: string
    resource_id?: string
    query?: string
  }
  meta_json?: Record<string, unknown>
}

export interface OverviewSearchResult {
  items: OverviewSearchRow[]
  total: number
  page: number
  size: number
}

export interface SystemAssetBatchDownloadManifest {
  items: Array<{
    material_id?: number
    asset_id: number
    source_type?: string
    source_ref?: string
    task_id?: number
    filename: string
    file_size: number
    mime_type?: string
    download_url: string
    expires_at?: string
  }>
  failures?: Array<{
    material_id?: number
    asset_id: number
    source_type?: string
    source_ref?: string
    task_id?: number
    filename?: string
    reason: string
  }>
  success_count: number
  failure_count: number
  total_size: number
  expires_at?: string
}

export interface UploadDirectoryRow {
  id: number
  name: string
  oss_prefix: string
  description: string
  difficulty_class: string
  allowed_file_types?: string[]
  enabled: boolean
  sort_order: number
  created_by: number
  updated_by?: number
  created_at?: string
  updated_at?: string
}

export interface ClientMaterialRow {
  id: number
  asset_id: number
  source_type?: string
  source_ref?: string
  resource_id?: string
  source_label?: string
  title: string
  description: string
  filename_snapshot: string
  mime_type_snapshot: string
  file_size_snapshot: number
  scope_sku_code?: string
  sku_code?: string
  primary_sku_code?: string
  preview_available?: boolean
  enabled: boolean
  sort_order: number
  published_by: number
  updated_by?: number
  published_at?: string
  created_at?: string
  updated_at?: string
}

export interface ClientMaterialSearchResult {
  items: ClientMaterialRow[]
  total: number
  page: number
  size: number
}

export type ClientMaterialBatchAction = 'publish' | 'enable' | 'disable' | 'remove'
export type MaterialFormatCategory = 'all' | 'image' | 'design' | 'pdf' | 'video' | 'archive'
export type MaterialSourceFilter = 'all' | 'system' | 'external'

export interface ClientMaterialBatchUpdatePayload {
  action: ClientMaterialBatchAction
  items?: UpsertClientMaterialPayload[]
  folders?: Array<{
    path: string
    source?: MaterialSourceFilter
    format_category?: MaterialFormatCategory
    include_children?: boolean
  }>
  query?: string
  source?: MaterialSourceFilter
  format_category?: MaterialFormatCategory
  selection_scope?: 'selected' | 'current_page' | 'current_folder' | 'current_folder_recursive' | 'current_filter'
}

export interface ClientMaterialBatchUpdateResult {
  requested: number
  created: number
  updated: number
  enabled: number
  disabled: number
  removed: number
  skipped: number
  failed: number
  async_required?: boolean
  job_id?: string
  job?: AssetWorkbenchBatchJob
  message?: string
  items?: ClientMaterialRow[]
  failures?: Array<{
    index: number
    asset_id?: number
    source_type?: string
    source_ref?: string
    resource_id?: string
    reason: string
  }>
}

export type AssetWorkbenchBatchJobStatus = 'queued' | 'running' | 'succeeded' | 'failed' | 'cancelled'

export interface AssetWorkbenchBatchJob {
  id: number
  job_id: string
  job_type: string
  status: AssetWorkbenchBatchJobStatus
  action: string
  selection_scope: string
  requested_by: number
  request_payload?: Record<string, unknown>
  result_payload?: Record<string, unknown>
  total_count: number
  processed_count: number
  created_count: number
  updated_count: number
  enabled_count: number
  disabled_count: number
  removed_count: number
  skipped_count: number
  failed_count: number
  error_message?: string
  lease_owner?: string
  lease_expires_at?: string | null
  started_at?: string | null
  finished_at?: string | null
  created_at: string
  updated_at: string
}

export interface AssetWorkbenchBatchJobListResult {
  items: AssetWorkbenchBatchJob[]
  total: number
  page: number
  size: number
}

export interface MaterialGroupRow {
  group_key: string
  group_code: string
  group_type: string
  title: string
  source_type: string
  file_total: number
  preview_files?: SystemAssetRow[]
}

export interface MaterialGroupSearchResult {
  items: MaterialGroupRow[]
  total: number
  page: number
  size: number
}

export interface MaterialGroupFilesResult {
  group_key: string
  items: SystemAssetRow[]
  total: number
  page: number
  size: number
}

export interface DriveDirectoryRow {
  directory_id?: number | null
  name: string
  prefix: string
  difficulty_class: string
  allowed_file_types?: string[]
  description?: string
  enabled?: boolean
  sort_order?: number
  file_count: number
  order_count: number
}

export interface DriveOrderRow {
  order_no: string
  submission_item_id?: number
  submission_item_ids?: number[]
  file_count: number
  latest_at: string
}

export interface DriveFileRow {
  id: number
  submission_id: number
  submission_item_id: number
  submission_no: string
  owner_user_id: number
  owner_name?: string
  owner_username?: string
  upload_directory_id?: number | null
  upload_directory_name: string
  difficulty_class?: string
  order_no: string
  original_filename: string
  display_name?: string
  relative_path?: string
  upload_batch_id?: string
  is_folder_upload?: boolean
  file_type: string
  mime_type: string
  file_size: number
  preview_status: string
  qc_status?: string
  pricing_status?: string
  settlement_status?: string
  page_count?: number
  gross_amount?: number
  business_month: string
  created_at: string
  locate_page?: number
  locate_page_size?: number
}

export interface DriveFolderRow {
  name: string
  path: string
  file_count: number
  direct_file_count: number
  latest_at?: string
}

export interface DriveFolderBrowseResult {
  path: string
  folders: DriveFolderRow[]
  files: DriveFileRow[]
  total: number
  page: number
  size: number
  truncated?: boolean
}

export interface ArchiveVirtualFolder {
  name: string
  path: string
  file_count: number
}

export interface ArchiveVirtualFile {
  name: string
  path: string
  mime_type: string
  file_type: string
  file_size: number
  preview_url?: string
  download_url?: string
}

export interface ArchiveBrowseResult {
  file_id: number
  path: string
  format: string
  folders: ArchiveVirtualFolder[]
  files: ArchiveVirtualFile[]
}

export interface PaginatedResult<T> {
  items: T[]
  total: number
}

export interface CreateUploadSessionPayload {
  original_filename: string
  file_size: number
  mime_type: string
  file_hash?: string
  upload_directory_id?: number
  upload_batch_id?: string
  relative_path?: string
  is_folder_upload?: boolean
  expected_business_month?: string
}

export interface UploadSessionRow {
  id: number
  session_id: string
  status: string
  object_key: string
  upload_directory_id?: number
  upload_directory_name?: string
  upload_directory_prefix?: string
  upload_directory_difficulty_class?: string
  upload_batch_id?: string
  relative_path?: string
  is_folder_upload?: boolean
  expected_business_month?: string
  original_filename: string
  file_size: number
  mime_type: string
  file_hash?: string
  upload_id?: string
}

export interface UploadPlanPart {
  part_number: number
  upload_url: string
}

export interface UploadPlan {
  mode?: string
  upload_url?: string
  object_key?: string
  upload_id?: string
  parts?: UploadPlanPart[]
  part_size?: number
  method?: string
  required_upload_content_type?: string
}

export interface CreateUploadSessionResult {
  session: UploadSessionRow
  plan?: UploadPlan
}

export interface CompleteUploadSessionPayload {
  parts: Array<{
    part_number: number
    etag: string
  }>
}

export interface CreateSubmissionPayload {
  notes?: string
  expected_business_month?: string
  month_rollover_ack?: boolean
  business_month_override?: string
  items: Array<{
    order_no?: string
    difficulty_class?: string
    finalized: boolean
    page_count: number
    item_count?: number
    upload_session_ids: string[]
  }>
}

export interface CreateSettlementSupplementPayload {
  payee_user_id: number
  business_month: string
  order_no: string
  supplement_date?: string
  difficulty_class: string
  finalized: boolean
  page_count: number
  gross_amount: number
  status?: string
}

export interface UpsertSupplementPermissionPayload {
  payee_user_id: number
  business_month: string
  enabled: boolean
  reason?: string
}

export interface CreateSettlementAdjustmentPayload {
  payee_user_id: number
  adjustment_type?: string
  direction?: string
  amount: number
  reason: string
  payload_json?: Record<string, unknown>
}

export interface UpsertSavedViewPayload {
  view_type: string
  view_name: string
  config_json: Record<string, unknown>
  is_default?: boolean
}

export interface CreatePriceMatrixPayload {
  worker_type: string
  job_grade: string
  difficulty_class: string
  unit_price: number
  effective_from: string
  effective_to?: string
  remark?: string
}

export interface CreateDeductionRulePayload {
  worker_type: string
  job_grade: string
  difficulty_class: string
  deduction_amount: number
  effective_from: string
  effective_to?: string
  remark?: string
}

export interface CreateWelfareRulePayload {
  rule_name: string
  worker_type: string
  job_grade: string
  rule_type: string
  amount: number
  config_json?: Record<string, unknown>
  effective_from: string
  effective_to?: string
  remark?: string
}

export interface CreatePromoCouponPayload {
  coupon_code: string
  coupon_name: string
  mode: string
  amount?: number
  percent?: number
  priority: number
  worker_type: string
  job_grade: string
  difficulty_class: string
  eligible_user_ids_json?: number[]
  eligible_codes_json?: string[]
  effective_from: string
  effective_to?: string
  remark?: string
}

export interface UpsertDifficultyClassPayload {
  code?: string
  name?: string
  description?: string
  enabled?: boolean
  sort_order?: number
}

export interface SetCostRuleEnabledPayload {
  enabled: boolean
  reason?: string
}

export interface UpsertGroupPayload {
  name: string
  description?: string
  enabled?: boolean
}

export interface UpsertUploadDirectoryPayload {
  name?: string
  oss_prefix?: string
  description?: string
  difficulty_class?: string
  allowed_file_types?: string[]
  enabled?: boolean
  sort_order?: number
}

export interface UpsertClientMaterialPayload {
  asset_id?: number
  source_type?: string
  source_ref?: string
  resource_id?: string
  title?: string
  description?: string
  enabled?: boolean
  sort_order?: number
}

function unwrap<T>(payload: ApiEnvelope<T> | T): T {
  if (payload && typeof payload === 'object' && 'data' in payload) {
    const wrapped = payload as ApiEnvelope<T>
    if (wrapped.data !== undefined) return wrapped.data
  }
  return payload as T
}

function unwrapPaginated<T>(payload: ApiEnvelope<T[]> | T[]): PaginatedResult<T> {
  if (payload && typeof payload === 'object' && 'data' in payload) {
    const wrapped = payload as ApiEnvelope<T[]>
    return {
      items: wrapped.data ?? [],
      total: wrapped.pagination?.total ?? wrapped.data?.length ?? 0,
    }
  }
  if (payload && typeof payload === 'object' && 'items' in payload) {
    const wrapped = payload as { items?: T[]; total?: number }
    const items = Array.isArray(wrapped.items) ? wrapped.items : []
    return { items, total: Number.isFinite(Number(wrapped.total)) ? Number(wrapped.total) : items.length }
  }
  const items = Array.isArray(payload) ? payload : []
  return { items, total: items.length }
}

export const assetWorkbenchApi = {
  async register(payload: AssetWorkbenchRegisterPayload, signal?: AbortSignal): Promise<AssetWorkbenchRegisterResult> {
    const res = await http.post<ApiEnvelope<AssetWorkbenchRegisterResult>>('/v1/asset-workbench/register', payload, { signal })
    return unwrap(res.data)
  },

  async bootstrap(signal?: AbortSignal): Promise<AssetWorkbenchBootstrap> {
    const res = await http.get<ApiEnvelope<AssetWorkbenchBootstrap>>('/v1/asset-workbench/bootstrap', { signal })
    return unwrap(res.data)
  },

  async entry(signal?: AbortSignal): Promise<WorkbenchEntryResult> {
    const res = await http.get<ApiEnvelope<WorkbenchEntryResult>>('/v1/asset-workbench/entry', { signal })
    return unwrap(res.data)
  },

  async listDifficultyClasses(signal?: AbortSignal): Promise<DifficultyClassRow[]> {
    const res = await http.get<ApiEnvelope<DifficultyClassRow[]>>('/v1/asset-workbench/difficulty-classes', { signal })
    return unwrap(res.data)
  },

  async listDifficultyClassesAdmin(signal?: AbortSignal): Promise<DifficultyClassRow[]> {
    const res = await http.get<ApiEnvelope<DifficultyClassRow[]>>('/v1/asset-workbench/difficulty-classes/admin', { signal })
    return unwrap(res.data)
  },

  async createDifficultyClass(payload: UpsertDifficultyClassPayload, signal?: AbortSignal): Promise<DifficultyClassRow> {
    const res = await http.post<ApiEnvelope<DifficultyClassRow>>('/v1/asset-workbench/difficulty-classes', payload, { signal })
    return unwrap(res.data)
  },

  async updateDifficultyClass(code: string, payload: UpsertDifficultyClassPayload, signal?: AbortSignal): Promise<DifficultyClassRow> {
    const res = await http.patch<ApiEnvelope<DifficultyClassRow>>(`/v1/asset-workbench/difficulty-classes/${encodeURIComponent(code)}`, payload, { signal })
    return unwrap(res.data)
  },

  async listPriceMatrix(params: Record<string, unknown> = {}, signal?: AbortSignal): Promise<PaginatedResult<PriceMatrixRow>> {
    const res = await http.get<ApiEnvelope<PriceMatrixRow[]>>('/v1/asset-workbench/price-matrix', { params, signal })
    return unwrapPaginated(res.data)
  },

  async createPriceMatrix(payload: CreatePriceMatrixPayload, signal?: AbortSignal): Promise<PriceMatrixRow> {
    const res = await http.post<ApiEnvelope<PriceMatrixRow>>('/v1/asset-workbench/price-matrix', payload, { signal })
    return unwrap(res.data)
  },

  async updatePriceMatrix(ruleId: number, payload: SetCostRuleEnabledPayload, signal?: AbortSignal): Promise<PriceMatrixRow> {
    const res = await http.patch<ApiEnvelope<PriceMatrixRow>>(`/v1/asset-workbench/price-matrix/${ruleId}`, payload, { signal })
    return unwrap(res.data)
  },

  async supersedePriceMatrix(ruleId: number, payload: CreatePriceMatrixPayload, signal?: AbortSignal): Promise<PriceMatrixRow> {
    const res = await http.post<ApiEnvelope<PriceMatrixRow>>(`/v1/asset-workbench/price-matrix/${ruleId}/supersede`, payload, { signal })
    return unwrap(res.data)
  },

  async upsertMyProfile(payload: UpsertProfilePayload, signal?: AbortSignal): Promise<AssetWorkbenchProfile> {
    const res = await http.patch<ApiEnvelope<AssetWorkbenchProfile>>('/v1/asset-workbench/profile', payload, { signal })
    return unwrap(res.data)
  },

  async listProfiles(params: Record<string, unknown> = {}, signal?: AbortSignal): Promise<PaginatedResult<AssetWorkbenchProfile>> {
    const res = await http.get<ApiEnvelope<AssetWorkbenchProfile[]>>('/v1/asset-workbench/profiles', { params, signal })
    return unwrapPaginated(res.data)
  },

  async upsertProfile(userId: number, payload: UpsertProfilePayload, signal?: AbortSignal): Promise<AssetWorkbenchProfile> {
    const res = await http.patch<ApiEnvelope<AssetWorkbenchProfile>>(`/v1/asset-workbench/profiles/${userId}`, payload, { signal })
    return unwrap(res.data)
  },

  async listMembers(params: Record<string, unknown> = {}, signal?: AbortSignal): Promise<PaginatedResult<WorkbenchMemberRow>> {
    const res = await http.get<ApiEnvelope<WorkbenchMemberRow[]>>('/v1/asset-workbench/members', { params, signal })
    return unwrapPaginated(res.data)
  },

  async searchPeople(params: Record<string, unknown> = {}, signal?: AbortSignal): Promise<PaginatedResult<WorkbenchMemberRow>> {
    const res = await http.get<ApiEnvelope<WorkbenchMemberRow[]>>('/v1/asset-workbench/people-lookup', { params, signal })
    return unwrapPaginated(res.data)
  },

  async updateMemberIdentity(userId: number, identity: 'admin' | 'normal', reason?: string, signal?: AbortSignal): Promise<WorkbenchMemberRow> {
    const res = await http.patch<ApiEnvelope<WorkbenchMemberRow>>(
      `/v1/asset-workbench/members/${userId}/identity`,
      { identity, reason },
      { signal },
    )
    return unwrap(res.data)
  },

  async updateMemberRoles(userId: number, roles: string[], reason?: string, signal?: AbortSignal): Promise<WorkbenchMemberRow> {
    const res = await http.patch<ApiEnvelope<WorkbenchMemberRow>>(
      `/v1/asset-workbench/members/${userId}/roles`,
      { roles, reason },
      { signal },
    )
    return unwrap(res.data)
  },

  async openAccess(payload: { user_id: number; roles?: string[]; identity_type?: string; reason?: string }, signal?: AbortSignal) {
    const res = await http.post<ApiEnvelope<unknown>>('/v1/asset-workbench/access/open', payload, { signal })
    return unwrap(res.data)
  },

  async disableAccess(payload: { user_id: number; reason: string }, signal?: AbortSignal) {
    const res = await http.post<ApiEnvelope<unknown>>('/v1/asset-workbench/access/disable', payload, { signal })
    return unwrap(res.data)
  },

  async previewAccountMerge(payload: { source_user_id: number; canonical_user_id: number }, signal?: AbortSignal): Promise<AccountMergePreview> {
    const res = await http.post<ApiEnvelope<AccountMergePreview>>('/v1/asset-workbench/accounts/merge/preview', payload, { signal })
    return unwrap(res.data)
  },

  async mergeAccounts(payload: { source_user_id: number; canonical_user_id: number; profile_choices: Record<string, string>; reason?: string }, signal?: AbortSignal): Promise<AccountMergePreview> {
    const res = await http.post<ApiEnvelope<AccountMergePreview>>('/v1/asset-workbench/accounts/merge', payload, { signal })
    return unwrap(res.data)
  },

  async listDeductionRules(params: Record<string, unknown> = {}, signal?: AbortSignal): Promise<PaginatedResult<DeductionRuleRow>> {
    const res = await http.get<ApiEnvelope<DeductionRuleRow[]>>('/v1/asset-workbench/deduction-rules', { params, signal })
    return unwrapPaginated(res.data)
  },

  async createDeductionRule(payload: CreateDeductionRulePayload, signal?: AbortSignal): Promise<DeductionRuleRow> {
    const res = await http.post<ApiEnvelope<DeductionRuleRow>>('/v1/asset-workbench/deduction-rules', payload, { signal })
    return unwrap(res.data)
  },

  async updateDeductionRule(ruleId: number, payload: SetCostRuleEnabledPayload, signal?: AbortSignal): Promise<DeductionRuleRow> {
    const res = await http.patch<ApiEnvelope<DeductionRuleRow>>(`/v1/asset-workbench/deduction-rules/${ruleId}`, payload, { signal })
    return unwrap(res.data)
  },

  async supersedeDeductionRule(ruleId: number, payload: CreateDeductionRulePayload, signal?: AbortSignal): Promise<DeductionRuleRow> {
    const res = await http.post<ApiEnvelope<DeductionRuleRow>>(`/v1/asset-workbench/deduction-rules/${ruleId}/supersede`, payload, { signal })
    return unwrap(res.data)
  },

  async listWelfareRules(params: Record<string, unknown> = {}, signal?: AbortSignal): Promise<PaginatedResult<WelfareRuleRow>> {
    const res = await http.get<ApiEnvelope<WelfareRuleRow[]>>('/v1/asset-workbench/welfare-rules', { params, signal })
    return unwrapPaginated(res.data)
  },

  async createWelfareRule(payload: CreateWelfareRulePayload, signal?: AbortSignal): Promise<WelfareRuleRow> {
    const res = await http.post<ApiEnvelope<WelfareRuleRow>>('/v1/asset-workbench/welfare-rules', payload, { signal })
    return unwrap(res.data)
  },

  async updateWelfareRule(ruleId: number, payload: SetCostRuleEnabledPayload, signal?: AbortSignal): Promise<WelfareRuleRow> {
    const res = await http.patch<ApiEnvelope<WelfareRuleRow>>(`/v1/asset-workbench/welfare-rules/${ruleId}`, payload, { signal })
    return unwrap(res.data)
  },

  async supersedeWelfareRule(ruleId: number, payload: CreateWelfareRulePayload, signal?: AbortSignal): Promise<WelfareRuleRow> {
    const res = await http.post<ApiEnvelope<WelfareRuleRow>>(`/v1/asset-workbench/welfare-rules/${ruleId}/supersede`, payload, { signal })
    return unwrap(res.data)
  },

  async listPromoCoupons(params: Record<string, unknown> = {}, signal?: AbortSignal): Promise<PaginatedResult<PromoCouponRow>> {
    const res = await http.get<ApiEnvelope<PromoCouponRow[]>>('/v1/asset-workbench/promo-coupons', { params, signal })
    return unwrapPaginated(res.data)
  },

  async createPromoCoupon(payload: CreatePromoCouponPayload, signal?: AbortSignal): Promise<PromoCouponRow> {
    const res = await http.post<ApiEnvelope<PromoCouponRow>>('/v1/asset-workbench/promo-coupons', payload, { signal })
    return unwrap(res.data)
  },

  async updatePromoCoupon(ruleId: number, payload: SetCostRuleEnabledPayload, signal?: AbortSignal): Promise<PromoCouponRow> {
    const res = await http.patch<ApiEnvelope<PromoCouponRow>>(`/v1/asset-workbench/promo-coupons/${ruleId}`, payload, { signal })
    return unwrap(res.data)
  },

  async supersedePromoCoupon(ruleId: number, payload: CreatePromoCouponPayload, signal?: AbortSignal): Promise<PromoCouponRow> {
    const res = await http.post<ApiEnvelope<PromoCouponRow>>(`/v1/asset-workbench/promo-coupons/${ruleId}/supersede`, payload, { signal })
    return unwrap(res.data)
  },

  async listGroups(params: Record<string, unknown> = {}, signal?: AbortSignal): Promise<PaginatedResult<WorkbenchGroupRow>> {
    const res = await http.get<ApiEnvelope<WorkbenchGroupRow[]>>('/v1/asset-workbench/groups', { params, signal })
    return unwrapPaginated(res.data)
  },

  async createGroup(payload: UpsertGroupPayload, signal?: AbortSignal): Promise<WorkbenchGroupRow> {
    const res = await http.post<ApiEnvelope<WorkbenchGroupRow>>('/v1/asset-workbench/groups', payload, { signal })
    return unwrap(res.data)
  },

  async updateGroup(groupId: number, payload: UpsertGroupPayload, signal?: AbortSignal): Promise<WorkbenchGroupRow> {
    const res = await http.patch<ApiEnvelope<WorkbenchGroupRow>>(`/v1/asset-workbench/groups/${groupId}`, payload, { signal })
    return unwrap(res.data)
  },

  async addGroupMembers(groupId: number, userIds: number[], signal?: AbortSignal): Promise<unknown> {
    const res = await http.put<ApiEnvelope<unknown>>(`/v1/asset-workbench/groups/${groupId}/members`, { user_ids: userIds }, { signal })
    return unwrap(res.data)
  },

  async listGroupMembers(groupId: number, signal?: AbortSignal): Promise<WorkbenchGroupMemberRow[]> {
    const res = await http.get<ApiEnvelope<WorkbenchGroupMemberRow[]>>(`/v1/asset-workbench/groups/${groupId}/members`, { signal })
    return unwrap(res.data)
  },

  async removeGroupMembers(groupId: number, userIds: number[], signal?: AbortSignal): Promise<unknown> {
    const res = await http.delete<ApiEnvelope<unknown>>(`/v1/asset-workbench/groups/${groupId}/members`, {
      data: { user_ids: userIds },
      signal,
    })
    return unwrap(res.data)
  },

  async deleteGroup(groupId: number, signal?: AbortSignal): Promise<WorkbenchGroupRow> {
    const res = await http.delete<ApiEnvelope<WorkbenchGroupRow>>(`/v1/asset-workbench/groups/${groupId}`, { signal })
    return unwrap(res.data)
  },

  async listUploadDirectories(signal?: AbortSignal): Promise<UploadDirectoryRow[]> {
    const res = await http.get<ApiEnvelope<UploadDirectoryRow[]>>('/v1/asset-workbench/upload-directories', { signal })
    return unwrap(res.data)
  },

  async listUploadDirectoriesAdmin(signal?: AbortSignal): Promise<UploadDirectoryRow[]> {
    const res = await http.get<ApiEnvelope<UploadDirectoryRow[]>>('/v1/asset-workbench/upload-directories/admin', { signal })
    return unwrap(res.data)
  },

  async createUploadDirectory(payload: UpsertUploadDirectoryPayload, signal?: AbortSignal): Promise<UploadDirectoryRow> {
    const res = await http.post<ApiEnvelope<UploadDirectoryRow>>('/v1/asset-workbench/upload-directories', payload, { signal })
    return unwrap(res.data)
  },

  async updateUploadDirectory(directoryId: number, payload: UpsertUploadDirectoryPayload, signal?: AbortSignal): Promise<UploadDirectoryRow> {
    const res = await http.patch<ApiEnvelope<UploadDirectoryRow>>(`/v1/asset-workbench/upload-directories/${directoryId}`, payload, { signal })
    return unwrap(res.data)
  },

  async listSubmissions(params: Record<string, unknown> = {}, signal?: AbortSignal): Promise<PaginatedResult<SubmissionRow>> {
    const res = await http.get<ApiEnvelope<SubmissionRow[]>>('/v1/asset-workbench/submissions', { params, signal })
    return unwrapPaginated(res.data)
  },

  async getSubmissionDetail(submissionId: number, signal?: AbortSignal): Promise<SubmissionDetail> {
    const res = await http.get<ApiEnvelope<SubmissionDetail>>(`/v1/asset-workbench/submissions/${submissionId}`, { signal })
    return unwrap(res.data)
  },

  async voidSubmission(submissionId: number, reason: string, signal?: AbortSignal): Promise<SubmissionRow> {
    const res = await http.post<ApiEnvelope<SubmissionRow>>(`/v1/asset-workbench/submissions/${submissionId}/void`, { reason }, { signal })
    return unwrap(res.data)
  },

  async previewSettlement(businessMonth: string, signal?: AbortSignal): Promise<SettlementPreview> {
    const res = await http.get<ApiEnvelope<SettlementPreview>>('/v1/asset-workbench/settlement/preview', {
      params: { business_month: businessMonth },
      signal,
    })
    return unwrap(res.data)
  },

  async settlementReport(businessMonth: string, signal?: AbortSignal): Promise<SettlementReport> {
    const res = await http.get<ApiEnvelope<SettlementReport>>('/v1/asset-workbench/settlement/report', {
      params: { business_month: businessMonth },
      signal,
    })
    return unwrap(res.data)
  },

  async mySettlement(signal?: AbortSignal): Promise<MySettlementResult> {
    const res = await http.get<ApiEnvelope<MySettlementResult>>('/v1/asset-workbench/settlement/my', { signal })
    return unwrap(res.data)
  },

  async listSettlementBatches(params: Record<string, unknown> = {}, signal?: AbortSignal): Promise<PaginatedResult<SettlementBatchRow>> {
    const res = await http.get<ApiEnvelope<SettlementBatchRow[]>>('/v1/asset-workbench/settlement/batches', { params, signal })
    return unwrapPaginated(res.data)
  },

  async getSettlementBatchDetail(batchId: number, signal?: AbortSignal): Promise<SettlementBatchDetail> {
    const res = await http.get<ApiEnvelope<SettlementBatchDetail>>(`/v1/asset-workbench/settlement/batches/${batchId}`, { signal })
    return unwrap(res.data)
  },

  async listSettlementSupplements(params: Record<string, unknown> = {}, signal?: AbortSignal): Promise<PaginatedResult<SettlementSupplementRow>> {
    const res = await http.get<ApiEnvelope<SettlementSupplementRow[]>>('/v1/asset-workbench/settlement/supplements', { params, signal })
    return unwrapPaginated(res.data)
  },

  async listSupplementPermissions(params: Record<string, unknown> = {}, signal?: AbortSignal): Promise<PaginatedResult<SupplementPermissionRow>> {
    const res = await http.get<ApiEnvelope<SupplementPermissionRow[]>>('/v1/asset-workbench/settlement/supplement-permissions', { params, signal })
    return unwrapPaginated(res.data)
  },

  async listSupplementEligibleMonths(payeeUserId: number, signal?: AbortSignal): Promise<string[]> {
    const res = await http.get<ApiEnvelope<{ months: string[] }>>('/v1/asset-workbench/settlement/supplement-eligible-months', {
      params: { payee_user_id: payeeUserId },
      signal,
    })
    return unwrap(res.data).months ?? []
  },

  async upsertSupplementPermission(payload: UpsertSupplementPermissionPayload, signal?: AbortSignal): Promise<SupplementPermissionRow> {
    const res = await http.put<ApiEnvelope<SupplementPermissionRow>>('/v1/asset-workbench/settlement/supplement-permissions', payload, { signal })
    return unwrap(res.data)
  },

  async generateSettlementBatch(businessMonth: string, signal?: AbortSignal): Promise<SettlementBatchRow> {
    const res = await http.post<ApiEnvelope<SettlementBatchRow>>(
      '/v1/asset-workbench/settlement/batches',
      { business_month: businessMonth },
      { signal },
    )
    return unwrap(res.data)
  },

  async confirmSettlementBatch(batchId: number, signal?: AbortSignal): Promise<unknown> {
    const res = await http.post<ApiEnvelope<unknown>>(`/v1/asset-workbench/settlement/batches/${batchId}/confirm`, undefined, { signal })
    return unwrap(res.data)
  },

  async cancelSettlementBatch(batchId: number, reason: string, signal?: AbortSignal): Promise<unknown> {
    const res = await http.post<ApiEnvelope<unknown>>(`/v1/asset-workbench/settlement/batches/${batchId}/cancel`, { reason }, { signal })
    return unwrap(res.data)
  },

  async createSettlementAdjustment(batchId: number, payload: CreateSettlementAdjustmentPayload, signal?: AbortSignal): Promise<SettlementAdjustmentRow> {
    const res = await http.post<ApiEnvelope<SettlementAdjustmentRow>>(`/v1/asset-workbench/settlement/batches/${batchId}/adjustments`, payload, { signal })
    return unwrap(res.data)
  },

  async createSettlementSupplement(payload: CreateSettlementSupplementPayload, signal?: AbortSignal): Promise<SettlementSupplementRow> {
    const res = await http.post<ApiEnvelope<SettlementSupplementRow>>('/v1/asset-workbench/settlement/supplements', payload, { signal })
    return unwrap(res.data)
  },

  async importSettlementSupplementsExcel(businessMonth: string, file: File, signal?: AbortSignal): Promise<SettlementSupplementImportResult> {
    const form = new FormData()
    form.set('business_month', businessMonth)
    form.set('file', file)
    const res = await http.post<ApiEnvelope<SettlementSupplementImportResult>>('/v1/asset-workbench/settlement/supplements/excel', form, { signal })
    return unwrap(res.data)
  },

  async deleteSettlementSupplement(supplementId: number, reason: string, signal?: AbortSignal): Promise<SettlementSupplementRow> {
    const res = await http.delete<ApiEnvelope<SettlementSupplementRow>>(`/v1/asset-workbench/settlement/supplements/${supplementId}`, {
      data: { reason },
      signal,
    })
    return unwrap(res.data)
  },

  async importErrorExcel(businessMonth: string, file: File, signal?: AbortSignal): Promise<ErrorImportBatchRow> {
    const form = new FormData()
    form.set('business_month', businessMonth)
    form.set('file', file)
    const res = await http.post<ApiEnvelope<ErrorImportBatchRow>>('/v1/asset-workbench/error-imports/excel', form, { signal })
    return unwrap(res.data)
  },

  async createUploadSession(payload: CreateUploadSessionPayload, signal?: AbortSignal): Promise<CreateUploadSessionResult> {
    const res = await http.post<ApiEnvelope<CreateUploadSessionResult>>('/v1/asset-workbench/upload-sessions', payload, { signal })
    return unwrap(res.data)
  },

  async completeUploadSession(sessionId: string, payload: CompleteUploadSessionPayload, signal?: AbortSignal): Promise<unknown> {
    const res = await http.post<ApiEnvelope<unknown>>(`/v1/asset-workbench/upload-sessions/${encodeURIComponent(sessionId)}/complete`, payload, { signal })
    return unwrap(res.data)
  },

  async cancelUploadSession(sessionId: string, signal?: AbortSignal): Promise<unknown> {
    const res = await http.post<ApiEnvelope<unknown>>(`/v1/asset-workbench/upload-sessions/${encodeURIComponent(sessionId)}/cancel`, undefined, { signal })
    return unwrap(res.data)
  },

  async createSubmission(payload: CreateSubmissionPayload, signal?: AbortSignal): Promise<SubmissionDetail> {
    const res = await http.post<ApiEnvelope<SubmissionDetail>>('/v1/asset-workbench/submissions', payload, { signal })
    return unwrap(res.data)
  },

  async getFilePreview(fileId: number, signal?: AbortSignal): Promise<FilePreviewMeta> {
    const res = await http.get<ApiEnvelope<FilePreviewMeta>>(`/v1/asset-workbench/files/${fileId}/preview`, { signal })
    return unwrap(res.data)
  },

  async getFileDownload(fileId: number, signal?: AbortSignal): Promise<FileDownloadMeta> {
    const res = await http.get<ApiEnvelope<FileDownloadMeta>>(`/v1/asset-workbench/files/${fileId}/download`, { signal })
    return unwrap(res.data)
  },

  async browseArchiveFile(fileId: number, path = '', signal?: AbortSignal): Promise<ArchiveBrowseResult> {
    const res = await http.get<ApiEnvelope<ArchiveBrowseResult>>(`/v1/asset-workbench/files/${fileId}/archive`, {
      params: path ? { path } : undefined,
      signal,
    })
    return unwrap(res.data)
  },

  async getArchiveEntryBlob(fileId: number, path: string, disposition: 'inline' | 'attachment' = 'inline', signal?: AbortSignal): Promise<Blob> {
    const res = await http.get<Blob>(`/v1/asset-workbench/files/${fileId}/archive/entry`, {
      params: { path, disposition },
      responseType: 'blob',
      signal,
    })
    return res.data
  },

  async batchDownloadFiles(fileIds: number[], signal?: AbortSignal): Promise<FileBatchDownloadManifest> {
    const res = await http.post<ApiEnvelope<FileBatchDownloadManifest>>('/v1/asset-workbench/files/batch-download', { file_ids: fileIds }, { signal })
    return unwrap(res.data)
  },

  async batchMoveFiles(fileIds: number[], uploadDirectoryId: number, reason = '', signal?: AbortSignal): Promise<BatchFileMutationResult> {
    const res = await http.post<ApiEnvelope<BatchFileMutationResult>>(
      '/v1/asset-workbench/files/batch-move',
      { file_ids: fileIds, upload_directory_id: uploadDirectoryId, reason },
      { signal },
    )
    return unwrap(res.data)
  },

  async batchDeleteFiles(fileIds: number[], reason: string, signal?: AbortSignal): Promise<BatchFileMutationResult> {
    const res = await http.post<ApiEnvelope<BatchFileMutationResult>>('/v1/asset-workbench/files/batch-delete', { file_ids: fileIds, reason }, { signal })
    return unwrap(res.data)
  },

  async updateSubmissionItem(itemId: number, payload: UpdateSubmissionItemPayload, signal?: AbortSignal): Promise<SubmissionItemRow> {
    const res = await http.patch<ApiEnvelope<SubmissionItemRow>>(`/v1/asset-workbench/items/${itemId}`, payload, { signal })
    return unwrap(res.data)
  },

  async updateSubmissionItemQC(itemId: number, payload: { qc_status: string; reason?: string }, signal?: AbortSignal): Promise<SubmissionItemRow> {
    const res = await http.patch<ApiEnvelope<SubmissionItemRow>>(`/v1/asset-workbench/items/${itemId}/qc`, payload, { signal })
    return unwrap(res.data)
  },

  async importSubmissionItemQCExcel(businessMonth: string, file: File, signal?: AbortSignal): Promise<SubmissionItemQCImportResult> {
    const form = new FormData()
    if (businessMonth) form.set('business_month', businessMonth)
    form.set('file', file)
    const res = await http.post<ApiEnvelope<SubmissionItemQCImportResult>>('/v1/asset-workbench/items/qc/excel', form, { signal })
    return unwrap(res.data)
  },

  async voidSubmissionItem(itemId: number, reason: string, signal?: AbortSignal): Promise<SubmissionItemRow> {
    const res = await http.post<ApiEnvelope<SubmissionItemRow>>(`/v1/asset-workbench/items/${itemId}/void`, { reason }, { signal })
    return unwrap(res.data)
  },

  async repriceSubmissionItem(itemId: number, reason = '', signal?: AbortSignal): Promise<SubmissionItemRow> {
    const res = await http.post<ApiEnvelope<SubmissionItemRow>>(`/v1/asset-workbench/items/${itemId}/reprice`, { reason }, { signal })
    return unwrap(res.data)
  },

  async systemSearch(params: { q?: string; source?: MaterialSourceFilter; format_category?: MaterialFormatCategory; limit?: number; page?: number; page_size?: number } = {}, signal?: AbortSignal): Promise<SystemSearchResult> {
    const res = await http.get<ApiEnvelope<SystemSearchResult>>('/v1/asset-workbench/system-search', { params, signal })
    return unwrap(res.data)
  },

  async browseMaterials(params: { path?: string; source?: MaterialSourceFilter; format_category?: MaterialFormatCategory; limit?: number; page?: number; page_size?: number } = {}, signal?: AbortSignal): Promise<MaterialBrowseResult> {
    const res = await http.get<ApiEnvelope<MaterialBrowseResult>>('/v1/asset-workbench/materials/browse', { params, signal })
    return unwrap(res.data)
  },

  async downloadMaterialAsset(asset: SystemAssetRow, signal?: AbortSignal): Promise<SystemAssetDownloadInfo> {
    if (asset.source_type === 'external') {
      const resourceId = asset.resource_id || `ext-${asset.id}`
      const res = await http.get<ApiEnvelope<SystemAssetDownloadInfo>>(`/v1/assets/${encodeURIComponent(resourceId)}/download`, { signal })
      return unwrap(res.data)
    }
    return this.downloadSystemAsset(asset.id, signal)
  },

  async previewMaterialAsset(asset: SystemAssetRow, signal?: AbortSignal): Promise<SystemAssetPreviewMeta> {
    if (asset.source_type === 'external') {
      const resourceId = asset.resource_id || `ext-${asset.id}`
      const res = await http.get<ApiEnvelope<SystemAssetDownloadInfo>>(`/v1/assets/${encodeURIComponent(resourceId)}/preview`, { signal })
      const info = unwrap(res.data)
      const downloadUrl = info.download_url
      const ready = Boolean(downloadUrl && info.preview_available)
      return {
        asset_id: asset.id,
        source_type: 'external',
        source_ref: resourceId,
        status: ready ? 'ready' : info.download_url ? 'not_applicable' : 'pending',
        preparing: !info.download_url,
        preview_url: ready ? downloadUrl : undefined,
        download_url: downloadUrl,
        expires_at: info.expires_at,
        mime_type: info.mime_type || asset.mime_type,
        filename: info.filename || asset.original_filename || asset.file_name,
        preview_available: ready,
      }
    }
    return this.previewSystemAsset(asset.id, signal)
  },

  async overviewSearch(params: Record<string, unknown> = {}, signal?: AbortSignal): Promise<OverviewSearchResult> {
    const res = await http.get<ApiEnvelope<OverviewSearchResult>>('/v1/asset-workbench/overview-search', { params, signal })
    return unwrap(res.data)
  },

  async driveDirectories(signal?: AbortSignal): Promise<DriveDirectoryRow[]> {
    const res = await http.get<ApiEnvelope<DriveDirectoryRow[]>>('/v1/asset-workbench/drive/directories', { signal })
    return unwrap(res.data)
  },

  async driveOrders(params: { dir_id?: number; unassigned?: boolean } = {}, signal?: AbortSignal): Promise<DriveOrderRow[]> {
    const query: Record<string, unknown> = {}
    if (params.unassigned) query.unassigned = 1
    else if (params.dir_id) query.dir_id = params.dir_id
    const res = await http.get<ApiEnvelope<DriveOrderRow[]>>('/v1/asset-workbench/drive/orders', { params: query, signal })
    return unwrap(res.data)
  },

  async driveFiles(
    params: {
      dir_id?: number
      unassigned?: boolean
      order_no?: string
      q?: string
      owner?: string
      created_from?: string
      created_to?: string
      sort_by?: string
      sort_dir?: string
      page?: number
      page_size?: number
    },
    signal?: AbortSignal,
  ): Promise<PaginatedResult<DriveFileRow>> {
    const query: Record<string, unknown> = {}
    if (params.order_no) query.order_no = params.order_no
    if (params.q) query.q = params.q
    if (params.owner) query.owner = params.owner
    if (params.created_from) query.created_from = params.created_from
    if (params.created_to) query.created_to = params.created_to
    if (params.sort_by) query.sort_by = params.sort_by
    if (params.sort_dir) query.sort_dir = params.sort_dir
    if (params.unassigned) query.unassigned = 1
    else if (params.dir_id) query.dir_id = params.dir_id
    if (params.page) query.page = params.page
    if (params.page_size) query.page_size = params.page_size
    const res = await http.get<ApiEnvelope<DriveFileRow[]>>('/v1/asset-workbench/drive/files', { params: query, signal })
    return unwrapPaginated(res.data)
  },

  async updateSubmissionFile(fileId: number, payload: { display_name: string }, signal?: AbortSignal): Promise<SubmissionFileRow> {
    const res = await http.patch<ApiEnvelope<SubmissionFileRow>>(`/v1/asset-workbench/files/${fileId}`, payload, { signal })
    return unwrap(res.data)
  },

  async driveFolder(
    params: { dir_id?: number; unassigned?: boolean; path?: string; page?: number; page_size?: number },
    signal?: AbortSignal,
  ): Promise<DriveFolderBrowseResult> {
    const query: Record<string, unknown> = {}
    if (params.unassigned) query.unassigned = 1
    else if (params.dir_id) query.dir_id = params.dir_id
    if (params.path) query.path = params.path
    if (params.page) query.page = params.page
    if (params.page_size) query.page_size = params.page_size
    const res = await http.get<ApiEnvelope<DriveFolderBrowseResult>>('/v1/asset-workbench/drive/folder', { params: query, signal })
    return unwrap(res.data)
  },

  async driveSearch(params: { q: string; page?: number; page_size?: number }, signal?: AbortSignal): Promise<PaginatedResult<DriveFileRow>> {
    const res = await http.get<ApiEnvelope<DriveFileRow[]>>('/v1/asset-workbench/drive/search', { params, signal })
    return unwrapPaginated(res.data)
  },

  async driveLocate(fileId: number, signal?: AbortSignal): Promise<DriveFileRow> {
    const res = await http.get<ApiEnvelope<DriveFileRow>>('/v1/asset-workbench/drive/locate', { params: { file_id: fileId }, signal })
    return unwrap(res.data)
  },

  async downloadSystemAsset(assetId: number, signal?: AbortSignal): Promise<SystemAssetDownloadInfo> {
    const res = await http.get<ApiEnvelope<SystemAssetDownloadInfo>>(`/v1/asset-workbench/system-assets/${assetId}/download`, { signal })
    return unwrap(res.data)
  },

  async previewSystemAsset(assetId: number, signal?: AbortSignal): Promise<SystemAssetPreviewMeta> {
    const res = await http.get<ApiEnvelope<SystemAssetPreviewMeta>>(`/v1/asset-workbench/system-assets/${assetId}/preview`, { signal })
    return unwrap(res.data)
  },

  async batchDownloadSystemAssets(assetIds: number[], namingMode = 'business', signal?: AbortSignal): Promise<SystemAssetBatchDownloadManifest> {
    const res = await http.post<ApiEnvelope<SystemAssetBatchDownloadManifest>>('/v1/asset-workbench/system-assets/batch-download', {
      asset_ids: assetIds,
      naming_mode: namingMode,
    }, { signal })
    return unwrap(res.data)
  },

  async listClientMaterials(admin = false, signal?: AbortSignal): Promise<ClientMaterialRow[]> {
    const res = await http.get<ApiEnvelope<ClientMaterialRow[]>>('/v1/asset-workbench/client-materials', {
      params: admin ? { admin: 1 } : undefined,
      signal,
    })
    return unwrap(res.data)
  },

  async searchClientMaterials(params: { q?: string; sku?: string; creator?: string; admin?: boolean; page?: number; page_size?: number } = {}, signal?: AbortSignal): Promise<ClientMaterialSearchResult> {
    const res = await http.get<ApiEnvelope<ClientMaterialSearchResult>>('/v1/asset-workbench/client-materials/search', {
      params: { ...params, admin: params.admin ? 1 : undefined },
      signal,
    })
    return unwrap(res.data)
  },

  async createClientMaterial(payload: UpsertClientMaterialPayload, signal?: AbortSignal): Promise<ClientMaterialRow> {
    const res = await http.post<ApiEnvelope<ClientMaterialRow>>('/v1/asset-workbench/client-materials', payload, { signal })
    return unwrap(res.data)
  },

  async updateClientMaterial(materialId: number, payload: UpsertClientMaterialPayload, signal?: AbortSignal): Promise<ClientMaterialRow> {
    const res = await http.patch<ApiEnvelope<ClientMaterialRow>>(`/v1/asset-workbench/client-materials/${materialId}`, payload, { signal })
    return unwrap(res.data)
  },

  async deleteClientMaterial(materialId: number, signal?: AbortSignal): Promise<unknown> {
    const res = await http.delete<ApiEnvelope<unknown>>(`/v1/asset-workbench/client-materials/${materialId}`, { signal })
    return unwrap(res.data)
  },

  async batchUpdateClientMaterials(payload: ClientMaterialBatchUpdatePayload, signal?: AbortSignal): Promise<ClientMaterialBatchUpdateResult> {
    const res = await http.post<ApiEnvelope<ClientMaterialBatchUpdateResult>>('/v1/asset-workbench/client-materials/batch-update', payload, { signal })
    return unwrap(res.data)
  },

  async listBatchJobs(params: { status?: AssetWorkbenchBatchJobStatus | ''; page?: number; page_size?: number } = {}, signal?: AbortSignal): Promise<AssetWorkbenchBatchJobListResult> {
    const res = await http.get<ApiEnvelope<AssetWorkbenchBatchJobListResult>>('/v1/asset-workbench/batch-jobs', { params, signal })
    return unwrap(res.data)
  },

  async getBatchJob(jobId: string, signal?: AbortSignal): Promise<AssetWorkbenchBatchJob> {
    const res = await http.get<ApiEnvelope<AssetWorkbenchBatchJob>>(`/v1/asset-workbench/batch-jobs/${encodeURIComponent(jobId)}`, { signal })
    return unwrap(res.data)
  },

  async downloadClientMaterial(materialId: number, signal?: AbortSignal): Promise<SystemAssetDownloadInfo> {
    const res = await http.get<ApiEnvelope<SystemAssetDownloadInfo>>(`/v1/asset-workbench/client-materials/${materialId}/download`, { signal })
    return unwrap(res.data)
  },

  async previewClientMaterial(materialId: number, signal?: AbortSignal): Promise<SystemAssetPreviewMeta> {
    const res = await http.get<ApiEnvelope<SystemAssetPreviewMeta>>(`/v1/asset-workbench/client-materials/${materialId}/preview`, { signal })
    return unwrap(res.data)
  },

  async batchDownloadClientMaterials(materialIds: number[], namingMode = 'business', signal?: AbortSignal): Promise<SystemAssetBatchDownloadManifest> {
    const res = await http.post<ApiEnvelope<SystemAssetBatchDownloadManifest>>('/v1/asset-workbench/client-materials/batch-download', {
      material_ids: materialIds,
      naming_mode: namingMode,
    }, { signal })
    return unwrap(res.data)
  },

  async materialGroups(params: { q?: string; source?: MaterialSourceFilter; format_category?: MaterialFormatCategory; page?: number; page_size?: number } = {}, signal?: AbortSignal): Promise<MaterialGroupSearchResult> {
    const res = await http.get<ApiEnvelope<MaterialGroupSearchResult>>('/v1/asset-workbench/materials/groups', { params, signal })
    return unwrap(res.data)
  },

  async materialGroupFiles(params: { group_key: string; page?: number; page_size?: number }, signal?: AbortSignal): Promise<MaterialGroupFilesResult> {
    const res = await http.get<ApiEnvelope<MaterialGroupFilesResult>>('/v1/asset-workbench/materials/group-files', { params, signal })
    return unwrap(res.data)
  },

  async listEvents(params: Record<string, unknown> = {}, signal?: AbortSignal): Promise<PaginatedResult<AssetWorkbenchEventRow>> {
    const res = await http.get<ApiEnvelope<AssetWorkbenchEventRow[]>>('/v1/asset-workbench/events', { params, signal })
    return unwrapPaginated(res.data)
  },

  async listNotifications(params: { is_read?: boolean; limit?: number; cursor?: string } = {}, signal?: AbortSignal): Promise<NotificationListResult> {
    const res = await http.get<ApiEnvelope<NotificationRow[]> & { next_cursor?: string }>('/v1/me/notifications', { params, signal })
    return { items: unwrap(res.data), next_cursor: res.data.next_cursor }
  },

  async markNotificationRead(id: number, signal?: AbortSignal): Promise<void> {
    await http.post(`/v1/me/notifications/${encodeURIComponent(id)}/read`, {}, { signal })
  },

  async markAllNotificationsRead(signal?: AbortSignal): Promise<void> {
    await http.post('/v1/me/notifications/read-all', {}, { signal })
  },

  async listSavedViews(viewType: string, signal?: AbortSignal): Promise<AssetWorkbenchSavedView[]> {
    const res = await http.get<ApiEnvelope<AssetWorkbenchSavedView[]>>('/v1/asset-workbench/saved-views', {
      params: { view_type: viewType },
      signal,
    })
    return unwrap(res.data)
  },

  async upsertSavedView(payload: UpsertSavedViewPayload, signal?: AbortSignal): Promise<AssetWorkbenchSavedView> {
    const res = await http.put<ApiEnvelope<AssetWorkbenchSavedView>>('/v1/asset-workbench/saved-views', payload, { signal })
    return unwrap(res.data)
  },

  async deleteSavedView(viewId: number, signal?: AbortSignal): Promise<unknown> {
    const res = await http.delete<ApiEnvelope<unknown>>(`/v1/asset-workbench/saved-views/${viewId}`, { signal })
    return unwrap(res.data)
  },
}
