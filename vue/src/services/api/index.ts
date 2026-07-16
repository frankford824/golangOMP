/**
 * API 服务统一入口
 * 按业务模块分类导出所有 API 函数集合
 */

// 认证
export { authApi } from './authApi'

// 任务（含向后兼容的旧函数式导出）
export * from './tasksApi'

// ERP 数据
export { erpApi } from './erpApi'

// 用户管理
export { usersApi } from './usersApi'
export * from './orgApi'

// 日志
export { logsApi } from './logsApi'

// 任务资产（reference/delivery/source 上传与列表）
export { assetsApi } from './assetsApi'
export { productManagementApi } from './productManagementApi'
export type {
  ProductImageCandidate,
  ProductManagementListParams,
  ProductManagementRecord,
  ProductSyncStatus,
} from './productManagementApi'

// 设计稿交付 / 任务内参考图上传（POST /v1/assets/upload-sessions，供 design.store 使用）
export {
  uploadDeliveryFileViaAssetSession,
  uploadTaskReferenceFileViaAssetSession,
} from './design'
export type { DeliveryUploadProgressPayload } from './design'

// 分类与品类映射（产品分类、材质选项）
export { categoriesApi } from './categoriesApi'

// 以下为其他业务模块（含各自内部 mock/real 实现）
export * from './productsApi'
export * from './customizationApi'
export * from './designApi'
export * from './financeApi'
export * from './kpiApi'
export * from './adminApi'
