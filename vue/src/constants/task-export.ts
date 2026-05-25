/** 导出全部筛选结果时，单次请求 page_size（对齐后端上限 100） */
export const TASK_EXPORT_ALL_PAGE_SIZE = 100

/** 前端同步导出条数上限，超出则提示缩小筛选或等待后台导出 */
export const TASK_EXPORT_MAX_TOTAL = 2000
