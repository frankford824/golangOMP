import type { ReferenceFileRef } from '@/services/api/assetsApi'
import type { RetouchRequirement } from '@/domain/types/retouch-requirement'
import type { ModuleSummary } from '@/services/apiTypes'
import type { TaskPriorityApi } from '@/domain/task-priority'

/** Cutover 后唯一可用于活动任务查询、渲染与动作判定的状态合同。 */
export type ActiveTaskStatus =
  | 'Draft'
  | 'PendingAssign'
  | 'Assigned'
  | 'InProgress'
  | 'PendingAudit'
  | 'Completed'
  | 'Archived'
  | 'Blocked'
  | 'Cancelled'

/** @deprecated use ActiveTaskStatus for activity queries. */
export type LegacyTaskStatus = ActiveTaskStatus
/** Read-model boundary; historical values are never valid activity-query filters. */
export type TaskStatus = ActiveTaskStatus

// ─── 主状态（PRD V2.0 主线阶段，与子状态统一使用 SCREAMING_SNAKE_CASE）─────────────
export type MainTaskStatus =
  | 'DRAFT'
  | 'PENDING_ASSIGN'
  | 'ASSIGNED'
  | 'IN_PROGRESS'
  | 'PENDING_AUDIT'
  | 'COMPLETED'
  | 'ARCHIVED'
  | 'CANCELLED'
  | 'BLOCKED'

// ─── 设计子状态：仅在需要设计的任务（原品开发 / 新品开发）中生效 ────────────────
export type DesignSubStatus =
  | 'NOT_REQUIRED'   // 修图或策划 SKU 等无需设计审核的任务
  | 'PENDING_ASSIGN' // 待指派设计师
  | 'IN_PROGRESS'    // 设计中
  | 'PENDING_AUDIT'  // 待审核
  | 'REJECTED'       // 审核打回，需修改
  | 'APPROVED'       // 审核通过（中间状态）
  | 'FINALIZED'      // 终稿确认，流程结束

// ─── 审核子状态：覆盖常规审核 / 交接复核 / 定制转派 / 交班等节点 ────────────────────────
export type AuditSubStatus =
  | 'NOT_REQUIRED'  // 修图或策划 SKU 等无需审核的任务
  | 'PENDING'       // 待审核（含常规审核 / 交接复核排队）
  | 'IN_PROGRESS'   // 审核进行中（已领取审核单）
  | 'PASSED'        // 审核通过
  | 'REJECTED'      // 审核打回
  | 'TRANSFERRED'   // 已转定制 / 转派
  | 'HANDED_OVER'   // 已交班

// ─── 任务业务分型 ─────────────────────────────────────────────────────────────
export type TaskBusinessType =
  | 'ORIGINAL_PRODUCT_DEV' // 原品开发：基于 ERP 已有单品
  | 'NEW_PRODUCT_DEV'      // 新品开发：基于外部链接/截图/文案
  | 'SKU_PLANNING'         // 策划 SKU：生成 SKU 与策划信息后直接结单
  | 'RETOUCH_TASK'         // P 图任务：图片精修，不走设计审核

export type TaskType = TaskBusinessType

// ─── 兼容旧命名别名（迁移完成后删除）────────────────────────────────────────────
/** @deprecated 使用 MainTaskStatus */
export type TaskMainStatus = MainTaskStatus

/** 活动详情只保留设计与统一审核节点。 */
export type TaskSubStatus =
  | 'PendingAssign'
  | 'PendingAudit'

// ─── 资产版本 ─────────────────────────────────────────────────────────────────
export type ProductSource = 'existing' | 'new'
export type AssetVersionType = 'reference' | 'draft' | 'revision' | 'final' | 'derivative'

export interface TaskAssetVersion {
  id: string
  type: AssetVersionType
  /** 后端版本行资产类型（delivery/source/reference...），用于视图按业务分区过滤 */
  assetKind?: string
  uploaderId: string
  uploaderName: string
  uploadedAt: string
  note?: string
  /** 后端 `asset_id`：用于 GET /v1/assets/{id}/preview|download */
  assetRootId?: string
  /** 后端 `asset_no`（AST-0001/AST-0002 等业务编号），用于按根分组/定位 */
  assetNo?: string
  /**
   * 同一资产根（同 asset_id/asset_no）内的版本序号，对应后端 `version_no`。
   * V1 = 首次上传；V2 及以上通常意味着审核拒回重传（"替换"）。
   */
  rootVersionNo?: number
  /** 若 asset_versions 行含 sku 作用域，用于批量任务按子项筛选（缺省视为任务主 SKU） */
  scopeSkuCode?: string
  /**
   * 可在浏览器内联预览的图片 URL（jpg/png/webp/gif 等），用于 img 预览。
   * PSD 等不应放入 fileRefs（浏览器无法直链 PSD）；若存在 `assetRootId`，大图区走 GET /v1/assets/{id}/preview。
   */
  fileRefs: string[]
  /** 后端版本行 `preview_available` 原值；false 时不应继续请求 preview 元数据 */
  previewAvailable?: boolean
  /** 不可 `<img>` 直链的源文件（如 PSD）；有 `assetRootId` 时仍可通过预览 API 展示栅格图 */
  nonPreviewFiles?: Array<{ label: string; url?: string }>
  /** 版本内文件总数（可预览 + 不可预览），用于「N 图」角标 */
  totalFileCount?: number
}

export interface TaskSkuItem {
  id?: number
  sequenceNo?: number
  skuCode?: string
  skuCodeType?: 'regular' | 'customization' | string
  skuStatus?: string
  productNameSnapshot?: string
  productShortName?: string
  productIId?: string
  erpProductId?: string
  categoryCode?: string
  materialMode?: string
  costPriceMode?: string
  costPrice?: number
  estimatedCost?: number
  costRuleId?: number
  costRuleName?: string
  costRuleSource?: string
  matchedRuleVersion?: number
  prefillSource?: string
  prefillAt?: string | null
  requiresManualReview?: boolean
  manualCostOverride?: boolean
  manualCostOverrideReason?: string
  overrideActor?: string
  overrideAt?: string | null
  specText?: string
  sizeText?: string
  width?: number
  height?: number
  area?: number
  quantity?: number
  baseSalePrice?: number
  variantJson?: Record<string, unknown>
  /** GET 任务读模型 sku_items[].reference_file_refs，与任务级字段解析规则一致 */
  referenceFileRefs?: ReferenceFileRef[]
  /** 批量创建时子项级 design_requirement（若后端返回） */
  designRequirement?: string
  /** 批量子项 ERP 同步投影 */
  filing_status?: string
  erp_sync_status?: string
  erp_sync_required?: boolean
  erp_sync_version?: number
  last_filed_at?: string | null
  filing_error_message?: string
}

// ─── Task 主接口 ──────────────────────────────────────────────────────────────
export interface Task {
  id: string
  taskNo: string
  sku: string | null
  productId: string | null
  productName: string
  /**
   * 原品开发：GET /v1/tasks 与 GET /v1/tasks/{id} 读模型
   * `product_selection.erp_product.image_url`（及分类等同层字段）。
   */
  productImageUrl?: string
  /**
   * `product_selection.erp_product.sku_id`（读模型归一化为字符串，便于展示）。
   */
  erpProductSkuId?: string
  /**
   * 选品来源语义：在 `product_selection` 上，与 `erp_product` 平级。
   * 例：`erp_bridge_keyword_search`。
   */
  productSelectionSourceMatchType?: string
  productSource: ProductSource
  taskType: TaskType
  businessType?: TaskType

  /**
   * @deprecated 过渡期字段，新代码禁止基于此字段做业务判断。
   * 使用 mainStatus + *SubStatus 组合替代。
   * 由 enrichTaskDomainFields 自动填充保持同步。
   */
  status: TaskStatus

  /** v8 后端动作合同；workflow_contract_version=2 时空数组表示明确禁止。 */
  allowedActions?: string[]
  workflowContractVersion?: number

  // ── 新状态模型（由 enrichTaskDomainFields 负责派生和同步）────────────────
  mainStatus?: MainTaskStatus
  designSubStatus?: DesignSubStatus
  auditSubStatus?: AuditSubStatus
  /** @deprecated 使用具体子状态字段代替 */
  subStatus?: TaskSubStatus

  // ── 责任人与组织（v0.9 actor/source 正式字段）────────────────────────────────
  /** 发起人（正式） */
  requesterId: string
  requesterName: string
  /** 创建人（可与发起人不同） */
  creatorId: string | null
  creatorName: string | null
  /** 设计师（正式） */
  designerId: string | null
  designerName: string | null
  /** 当前处理人（后端投影，部分状态可为空） */
  currentHandlerId: string | null
  currentHandlerName: string | null
  /**
   * @deprecated 与 designer_* 对齐的兼容镜像，由读模型映射填充；新展示逻辑请用 designer_*。
   */
  assigneeId: string | null
  /**
   * @deprecated 与 designer_name 对齐的兼容别名。
   */
  assigneeName: string | null
  groupId: string
  /**
   * Legacy compatibility：来自后端 `owner_team` / `group_name` 等，用于历史数据与创建侧兼容展示。
   * 正式组织归属请使用 `ownerDepartment` + `ownerOrgTeam`。
   */
  groupName: string
  /** 规范归属：部门（GET 任务读模型 `owner_department`） */
  ownerDepartment?: string
  /** 规范归属：组织树团队（GET 任务读模型 `owner_org_team`） */
  ownerOrgTeam?: string
  /** 工作流业务 lane：普通 / 定制（GET 任务读模型 `workflow_lane`） */
  workflowLane?: 'normal' | 'customization'
  /** 任务业务 lane：普通 / 定制（GET 任务读模型 `business_lane`） */
  businessLane?: 'normal' | 'customization'
  skuCodeType?: 'regular' | 'customization' | string
  /** GET /v1/tasks/{id}/detail modules projection, including backend allowed actions. */
  moduleSummaries?: ModuleSummary[]
  /** 上游来源部门（GET 任务读模型 `source_department`） */
  sourceDepartment?: string

  // ── 需求描述 ───────────────────────────────────────────────────────────────
  /** retouch_task structured demand lines (Phase 1A text only). */
  retouchRequirements?: RetouchRequirement[]
  designRequirement?: string
  copyContent?: string
  styleKeywords?: string
  /**
   * 任务详情顶层参考图：GET 稳定返回 `reference_file_refs`（可来自 canonical asset session 或 pre-task fallback），非 reference_images。
   * v1.18+：后端返回 presigned URL，带 download_url_expires_at（15 min TTL）。
   */
  referenceFileRefs: ReferenceFileRef[]
  dueAt: string | null
  /** v1.21 四态；读 API 时由 store 将 urgent/medium 归一 */
  priority: TaskPriorityApi
  customizationRequired?: boolean
  customizationSourceType?: 'new_product' | 'existing_product' | string | null
  lastCustomizationOperatorId?: string | null
  note?: string

  // ── 资产版本（GET /v1/tasks/{id} 主读模型；与 upload-session 落库一致，不依赖 submit-design）──
  assetVersions: TaskAssetVersion[]

  // ── 成本与采购 ─────────────────────────────────────────────────────────────
  costPrice?: {
    amount: number
    currency: string
  }
  /** 新品创建时录入的基础售价。 */
  basePriceAmount?: number
  /** 新品成本单价来源。 */
  costPriceMode?: 'manual' | 'template'
  /** 系统规则估算成本；为空且 requiresManualReview=true 时，需补尺寸或人工维护 */
  estimatedCost?: number
  costRuleId?: number
  costRuleName?: string
  costRuleSource?: string
  requiresManualReview?: boolean
  manualCostOverride?: boolean
  manualCostOverrideReason?: string
  /** 成本人工覆盖摘要：GET /v1/tasks/{id} override_summary */
  costOverrideSummary?: Record<string, unknown>
  /** 成本治理审计摘要：GET /v1/tasks/{id} governance_audit_summary */
  governanceAuditSummary?: Record<string, unknown>
  /** 成本覆盖审批/财务边界：GET /v1/tasks/{id} override_governance_boundary */
  costOverrideBoundary?: Record<string, unknown>

  /**
   * 与创建任务 3-in-1 表单对齐的只读字段（GET task 归一化）。
   * 仅用于详情展示，不替代 workflow / 成本治理等运行态数据。
   */
  /** 原品：ERP 选品分类 */
  erpIId?: string
  erpCategoryCode?: string
  erpCategoryName?: string
  /** business-info / 详情子表返回的展示类目值 */
  categoryName?: string
  category?: string
  /** 新品：产品分类编码、材质、简称、参考链接与创建时数量/成本单价 */
  newProductCategoryCode?: string
  newProductMaterial?: string
  newProductMaterialOther?: string
  productNameSnapshot?: string
  productShortName?: string
  productReferenceUrl?: string
  newProductQuantity?: number
  newProductCostUnitPrice?: number
  productChannel?: string
  /**
   * GET 任务读模型 `spec_text` / `size_text`：创建时「规格尺寸」等主档快照，供后续环节只读展示。
   */
  specText?: string
  sizeText?: string

  // ── 其他标志位 ─────────────────────────────────────────────────────────────
  requiresAssetVersions?: boolean
  createdAt: string
  updatedAt: string

  // ── Step 87：建档 / ERP 同步状态（后端可选返回，老数据可能无）────────────────
  filing_status?: string
  filing_error_message?: string
  missing_fields?: string[]
  missing_fields_summary_cn?: string
  last_filed_at?: string | null
  erp_sync_required?: boolean
  filing_trigger_source?: string
  last_filing_attempt_at?: string | null
  erp_sync_version?: number

  // ── 批量 SKU 能力（后端可选返回，老数据可能无）───────────────────────────────
  isBatchTask?: boolean
  batchItemCount?: number
  batchMode?: string
  primarySkuCode?: string
  skuGenerationStatus?: string
  skuItems?: TaskSkuItem[]
}
