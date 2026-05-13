import type { PurchaseInfo } from './purchase'
import type { ReferenceFileRef } from '@/services/api/assetsApi'
import type { ModuleSummary } from '@/services/apiTypes'
import type { TaskPriorityApi } from '@/domain/task-priority'

/**
 * @deprecated 旧扁平状态枚举，仅用于过渡期桥接。
 * 所有新代码改用 MainTaskStatus + DesignSubStatus / AuditSubStatus / WarehouseSubStatus / PurchaseSubStatus 组合。
 * 迁移完成后此类型将被删除。
 */
export type LegacyTaskStatus =
  | 'Draft'
  | 'PendingAssign'
  | 'InProgress'
  | 'PendingAuditA'
  | 'RejectedByAuditA'
  | 'PendingAuditB'
  | 'RejectedByAuditB'
  | 'PendingOutsource'
  | 'Outsourcing'
  | 'PendingOutsourceReview'
  | 'PendingCustomizationReview'
  | 'PendingCustomizationProduction'
  | 'PendingEffectReview'
  | 'PendingEffectRevision'
  | 'PendingProductionTransfer'
  | 'PendingWarehouseQC'
  | 'RejectedByWarehouse'
  | 'PendingWarehouseReceive'
  | 'PendingClose'
  | 'Completed'
  | 'Archived'
  | 'Blocked'
  | 'Cancelled'

/** @deprecated 使用 LegacyTaskStatus 或新的主/子状态类型，迁移完成后移除此别名 */
export type TaskStatus = LegacyTaskStatus

// ─── 主状态（PRD V2.0 主线阶段，与子状态统一使用 SCREAMING_SNAKE_CASE）─────────────
export type MainTaskStatus =
  | 'DRAFT'
  | 'CREATED'
  | 'CODE_GENERATED'
  | 'ERP_REGISTERED'
  | 'INFO_PENDING'
  | 'WAREHOUSE_PENDING'
  | 'WAREHOUSE_PROCESSING'
  | 'READY_TO_CLOSE'
  | 'CLOSED'
  | 'BLOCKED'

// ─── 设计子状态：仅在需要设计的任务（原品开发 / 新品开发）中生效 ────────────────
export type DesignSubStatus =
  | 'NOT_REQUIRED'   // 采购任务默认；不需要设计
  | 'PENDING_ASSIGN' // 待指派设计师
  | 'IN_PROGRESS'    // 设计中
  | 'PENDING_AUDIT'  // 待审核
  | 'REJECTED'       // 审核打回，需修改
  | 'APPROVED'       // 审核通过（中间状态）
  | 'FINALIZED'      // 终稿确认，流程结束

// ─── 审核子状态：覆盖初审 / 复审 / 定制转派 / 交班等节点 ────────────────────────
export type AuditSubStatus =
  | 'NOT_REQUIRED'  // 采购任务默认；无需审核
  | 'PENDING'       // 待审核（含初审 / 复审排队）
  | 'IN_PROGRESS'   // 审核进行中（已领取审核单）
  | 'PASSED'        // 审核通过
  | 'REJECTED'      // 审核打回
  | 'TRANSFERRED'   // 已转定制 / 转派
  | 'HANDED_OVER'   // 已交班

// ─── 仓库子状态：统一描述仓库节点的处理进度 ────────────────────────────────────
export type WarehouseSubStatus =
  | 'NOT_REQUIRED'   // 尚未到达仓库节点
  | 'PENDING_RECEIVE' // 待仓库接收
  | 'RECEIVED'        // 已接收
  | 'RETURNED'        // 已退回
  | 'PACKING'         // 打包中
  | 'DONE'            // 仓库节点完成

// ─── 采购子状态：采购任务中为主支线，其他任务按需触发 ───────────────────────────
export type PurchaseSubStatus =
  | 'NOT_REQUIRED' // 非采购任务默认
  | 'PENDING'      // 待采购
  | 'IN_PROGRESS'  // 采购中（已下单 / 跟进中）
  | 'PURCHASED'    // 已采购，待入仓
  | 'INBOUND_DONE' // 已入仓 / 到货完成

// ─── 结单状态 ───────────────────────────────────────────────────────────────────
export type CloseStatus =
  | 'NOT_READY' // 结单条件未满足
  | 'READY'     // 所有条件已满足，可结单
  | 'CLOSED'    // 已结单

// ─── 任务业务分型 ─────────────────────────────────────────────────────────────
export type TaskBusinessType =
  | 'ORIGINAL_PRODUCT_DEV' // 原品开发：基于 ERP 已有单品
  | 'NEW_PRODUCT_DEV'      // 新品开发：基于外部链接/截图/文案
  | 'PURCHASE_TASK'        // 采购任务：不经设计/审核，直达仓库
  | 'RETOUCH_TASK'         // P 图任务：图片精修，不走设计审核

export type TaskType = TaskBusinessType

// ─── 兼容旧命名别名（迁移完成后删除）────────────────────────────────────────────
/** @deprecated 使用 MainTaskStatus */
export type TaskMainStatus = MainTaskStatus

/**
 * @deprecated 使用具体的 DesignSubStatus / AuditSubStatus / WarehouseSubStatus 代替。
 * 此联合类型是过渡期遗留，新代码禁止引用。
 */
export type TaskSubStatus =
  | 'PendingAssign'
  | 'PendingAuditA'
  | 'RejectedByAuditA'
  | 'PendingAuditB'
  | 'RejectedByAuditB'
  | 'PendingOutsource'
  | 'Outsourcing'
  | 'PendingOutsourceReview'
  | 'PendingCustomizationReview'
  | 'PendingCustomizationProduction'
  | 'PendingEffectReview'
  | 'PendingEffectRevision'
  | 'PendingProductionTransfer'
  | 'PendingWarehouseQC'
  | 'RejectedByWarehouse'
  | 'PendingWarehouseReceive'

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
  skuStatus?: string
  productNameSnapshot?: string
  productShortName?: string
  productIId?: string
  erpProductId?: string
  categoryCode?: string
  materialMode?: string
  costPriceMode?: string
  quantity?: number
  baseSalePrice?: number
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
  businessType?: TaskBusinessType

  /**
   * @deprecated 过渡期字段，新代码禁止基于此字段做业务判断。
   * 使用 mainStatus + *SubStatus 组合替代。
   * 由 enrichTaskDomainFields 自动填充保持同步。
   */
  status: LegacyTaskStatus

  // ── 新状态模型（由 enrichTaskDomainFields 负责派生和同步）────────────────
  mainStatus?: MainTaskStatus
  designSubStatus?: DesignSubStatus
  auditSubStatus?: AuditSubStatus
  warehouseSubStatus?: WarehouseSubStatus
  purchaseSubStatus?: PurchaseSubStatus
  closeStatus?: CloseStatus

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
  /** GET /v1/tasks/{id}/detail modules projection, including backend allowed actions. */
  moduleSummaries?: ModuleSummary[]
  /** 上游来源部门（GET 任务读模型 `source_department`） */
  sourceDepartment?: string

  // ── 需求描述 ───────────────────────────────────────────────────────────────
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
  /**
   * 后端 `need_outsource`：已持久化的外协/定制协作意图标志（筛选与展示）。
   * 不代表已存在 customization job；POST /tasks/{id}/outsource 也不会自动创建 job。
   */
  needOutsource: boolean
  customizationRequired?: boolean
  customizationSourceType?: 'new_product' | 'existing_product' | string | null
  lastCustomizationOperatorId?: string | null
  warehouseRejectReason?: string | null
  warehouseRejectCategory?: string | null
  note?: string

  // ── 资产版本（GET /v1/tasks/{id} 主读模型；与 upload-session 落库一致，不依赖 submit-design）──
  assetVersions: TaskAssetVersion[]

  // ── 成本与采购 ─────────────────────────────────────────────────────────────
  costPrice?: {
    amount: number
    currency: string
  }
  purchaseInfo?: PurchaseInfo
  /** 采购任务：创建时录入的基本售价（GET 任务常返回 base_sale_price） */
  basePriceAmount?: number
  /** 采购任务：成本单价来源 */
  costPriceMode?: 'manual' | 'template'
  /** 成本治理读模型：GET /v1/tasks/{id} 的采购摘要，用于详情页展示与仓库/财务判断 */
  procurementSummary?: Record<string, unknown>
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
  /** 采购：产品渠道 */
  productChannel?: string
  /**
   * GET 任务读模型 `spec_text` / `size_text`：创建时「规格尺寸」等主档快照，供后续环节只读展示。
   */
  specText?: string
  sizeText?: string

  // ── 仓库 ───────────────────────────────────────────────────────────────────
  /** @deprecated 使用 warehouseSubStatus 代替 */
  warehouseReceiveStatus?: 'pending' | 'received' | 'returned' | 'archived'

  /** 后端 workflow.can_prepare_warehouse：是否允许进入仓库准备/交接 */
  canPrepareWarehouse?: boolean
  /** 后端 workflow.warehouse_blocking_reasons：未满足时仓库节点不触发 */
  warehouseBlockingReasons?: Array<{ code: string; message: string }>
  /** procurement_summary：是否满足仓库准备条件（读模型） */
  warehousePrepareReady?: boolean
  /** procurement_summary：是否可进入仓库接收（读模型） */
  warehouseReceiveReady?: boolean

  /** 后端 procurement 记录原始 status（如 draft/completed），供 PATCH /procurement 幂等更新 */
  procurementApiStatus?: string

  /** 后端 workflow.main_status（小写，如 closed / pending_close），用于主状态徽章与终态判断 */
  workflowMainStatus?: string
  /** 后端 workflow.can_close / workflow.closable：有则结单按钮与后端门禁一致 */
  workflowCanClose?: boolean
  /** 后端 workflow.cannot_close_reasons */
  cannotCloseReasons?: Array<{ code: string; message: string }>

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
