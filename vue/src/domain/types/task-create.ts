import type { TaskPriorityApi } from '@/domain/task-priority'
import type { RetouchRequirementDraft } from '@/domain/types/retouch-requirement'

export type TaskKind = 'ORIGINAL_PRODUCT_DEV' | 'NEW_PRODUCT_DEV' | 'RETOUCH_TASK'
export type TaskCreateSkuMode = 'single' | 'multiple'
export type TaskSkuCodeType = 'regular' | 'customization'

/**
 * 批量商品行：完整业务字段（由录入模板复制并可编辑），提交时进入 batch_items。
 * clientKey 仅前端使用，不参与提交。
 */
export interface TaskBatchItem {
  clientKey: string
  productName: string
  productShortName?: string
  categoryCode?: string
  /** 聚水潭款式编码 i_id（parse-excel 解析结果透传） */
  productIId?: string
  /** 可选自由文本材质（与单表单一致） */
  material?: string
  /** 遗留：旧版「其他」补充；新流程可仅用 material */
  materialOther?: string
  designRequirement?: string
  newSku?: string
  skuCodeType?: TaskSkuCodeType
  /** 预生成 SKU 所用规则 id，与 SkuRulePreviewCard 选择同步 */
  skuRuleId?: string | null
  costPriceMode?: 'manual' | 'template'
  costUnitPrice?: number
  costPriceAmount?: number
  quantity?: number
  basePriceAmount?: number
  baseSalePrice?: number
  productReferenceUrl?: string
  productChannel?: string
  referenceFileRefs?: (Record<string, unknown> | string)[]
  variantJson?: Record<string, unknown>
  /** 运营创建时的套装建议；最终 single/set 仍由设计阶段决定。 */
  setModeHint?: boolean
  /** 相对模板是否已编辑（摘要区展示「已修改」） */
  _editedFromTemplate?: boolean
}

/**
 * 批量录入模板：与商品字段同源，仅作默认值来源；不单独作为一条商品提交。
 */
export type TaskBatchTemplateValues = Omit<TaskBatchItem, 'clientKey' | '_editedFromTemplate'>

/**
 * 任务创建表单模型。
 *
 * 注意：
 * - 字段设计需与 `TaskCreateView` 等创建入口保持同步；
 * - 仅包含前端在创建阶段需要收集的信息，不等同于 Task 完整结构。
 */
export interface TaskCreateFormModel {
  // ── 产品与编码 ───────────────────────────────────────────────────────────────
  productId: string | null
  productName: string
  sku: string | null
  /** ERP 产品主图，可为空 */
  productImageUrl?: string
  /** ERP 分类名称，可为空 */
  productCategoryName?: string
  /** ERP 分类编码，可为空 */
  productCategoryCode?: string
  /** ERP 产品快照，用于 product_selection.erp_product（product_id 为 SKU 等非数字时后端需此解析） */
  erpProductSnapshot?: Record<string, unknown>

  // 原品 / 新品 业务来源
  groupId: string
  groupName: string

  // 责任人
  assigneeId: string | null
  assigneeName: string | null

  // ── 设计与文案 ───────────────────────────────────────────────────────────────
  /** 订单号（定制管理创建入口） */
  orderNumber?: string
  /**
   * 设计需求 / 修改说明（原品：修改要求；新品：设计需求说明；P 图：可选任务总述）
   */
  designRequirement: string
  /** P 图任务结构化需求明细（仅 RETOUCH_TASK 创建时使用） */
  retouchRequirements?: RetouchRequirementDraft[]
  /** 文案内容（仅当业务需要时保留，文档 3-in-1 未展示则可不提交） */
  copyContent?: string
  /** 风格关键词（文档 3-in-1 未展示则可不提交） */
  styleKeywords?: string
  /** 参考图引用对象；有 task_id 时来自 canonical asset session，无 task_id 时来自 pre-task fallback。 */
  referenceFileRefs: (Record<string, unknown> | string)[]

  // ── 截止时间与优先级 ───────────────────────────────────────────────────────
  dueAt: string | null
  /** v1.21：low | normal | high | critical */
  priority: TaskPriorityApi
  /** 创建后是否立即同步 ERP，默认 true */
  syncErpOnCreate?: boolean
  /** 定制任务开关：true 时进入定制主线，false/不传走普通设计主线（本页仅创建普通任务时请保持 false） */
  customizationRequired: boolean
  /** 定制来源类型：当 customizationRequired=true 时必填 */
  customizationSourceType?: 'new_product' | 'existing_product'
  /** 自动生成 SKU 类型：regular=CG+类目1位+6位序号；customization=DZ+类目1位+6位序号 */
  skuCodeType?: TaskSkuCodeType
  note: string
  /** 成本价格模式（manual=手动录入，template=按模板/系统计算） */
  costPriceMode?: 'manual' | 'template'

  // ── 新品开发专属字段（与《创建任务-三个分型取消》文档对齐）────────────────────
  /** 聚水潭款式编码 i_id（来自 GET /v1/erp/iids，本地兜底 56 项） */
  category?: string
  /** 产品材质（可选，自由文本） */
  material?: string
  /** 遗留：与旧版枚举 OTHER 配套；新流程可留空 */
  materialOther?: string
  /** 产品简称 */
  productShortName?: string
  /** 产品参考链接 */
  productReferenceUrl?: string
  /** 成本单价（非必填） */
  costUnitPrice?: number
  /** 数量（非必填） */
  quantity?: number
  /** 基本售价（非必填） */
  basePriceAmount?: number
  inspirationNote?: string

  // ── 批量 SKU 创建（新品设计任务）────────────────────────────────────────────
  skuMode?: TaskCreateSkuMode
  batchItems?: TaskBatchItem[]
  batchTemplate?: TaskBatchTemplateValues
  /** 批量：用户已在「填写模板」弹窗中保存，模板才参与生成商品；未保存前不可新增商品 */
  batchTemplateSaved?: boolean

  // ── 创建阶段前置结单/建档主档信息（仅白名单分型展示，均可选）───────────────────
  prefillSpecText?: string
}
