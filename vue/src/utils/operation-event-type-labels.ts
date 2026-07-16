/**
 * 操作日志（任务侧）event_type → 中文说明。
 * 未知类型回退为原始字符串（便于联调）；空为「—」。
 */
export const OPERATION_EVENT_TYPE_LABELS: Record<string, string> = {
  'task.created': '任务已创建',
  'task.status.changed': '任务状态已变更',
  'task.audit.claimed': '已领取审核',
  'task.audit.approved': '审核已通过',
  'task.audit.rejected': '审核已驳回',
  'task.audit.transferred': '审核已转交',
  'task.audit.handed_over': '审核已交接',
  'task.audit.taken_over': '审核已接管',
  'task.assigned': '任务已指派',
  'task.reassigned': '任务已改派',
  'task.business_info.updated': '业务信息已更新',
  'task.cost.updated': '成本已更新',
  'task.sku_item.updated': '子项信息已更新',
  /** 与部分后端/旧 mock 的裸字段名对齐（无 task. 前缀） */
  'business_info.updated': '业务信息已更新',
  'task.design.submitted': '设计已提交',
  'task.asset.mock_uploaded': '素材已模拟上传',
  'task.asset.upload_session.created': '上传记录已创建',
  'task.asset.version.created': '素材已保存',
  'task.asset.upload_session.completed': '上传记录已完成',
  'task.asset.upload_session.cancelled': '上传记录已取消',
  'task.reference.asset.formalized': '参考图已接入任务',
  'task.reference.asset.bulk_formalized': '参考图已批量接入任务',
  'task.reference.asset.formalize_failed': '参考图接入失败',
  'task.filing.triggered': 'ERP 同步已提交',
  'task.filing.readback_confirmed': 'ERP 同步已确认',
  'task.erp_image.auto_synced': 'ERP 图片已自动同步',
  'task.erp_image.auto_sync_failed': 'ERP 图片自动同步失败',
  'task.erp_image.awaiting_upload': '待上传 ERP 商品图',
  'task.completed': '任务已结单',
  'task.reminded': '任务已提醒',
  'task.batch_assigned': '任务已批量指派',
  'task.batch_items_created': '批量子项已生成',
  'task.customization.reviewed': '定制需求已审核',
  // 模块 / mock 中可能出现的裸动词
  submitted: '已提交',
  approved: '已通过',
  rejected: '已驳回',
  received: '已接收',
  archived: '已归档',
  task_cancelled: '任务已取消',
  // 流程模块（GET /v1/tasks/.../modules/...）
  'module.enter': '进入流程模块',
  'module.claimed': '已领取环节',
  'module.reassigned': '环节已改派',
  'module.pool_reassigned': '环节池内改派',
  'module.submit': '环节已提交',
  'module.approve': '环节已通过',
  'module.reject': '环节已驳回',
  'module.update_reference_files': '参考文件已更新',
}

export function getOperationEventTypeLabel(
  eventType: string | undefined | null,
  unknownFallback: 'raw' | 'other' = 'raw',
): string {
  if (eventType == null || !String(eventType).trim()) return '—'
  const key = eventType.trim()
  const mapped = OPERATION_EVENT_TYPE_LABELS[key]
  if (mapped) return mapped
  const byCi = (Object.keys(OPERATION_EVENT_TYPE_LABELS) as (keyof typeof OPERATION_EVENT_TYPE_LABELS)[]).find(
    (k) => k.toLowerCase() === key.toLowerCase(),
  )
  if (byCi) return OPERATION_EVENT_TYPE_LABELS[byCi]!
  if (unknownFallback === 'other') return '其他事件'
  return key
}

/**
 * 任务侧事件流/动态里展示的标题：仅将 `event_type` 换为中文，不改接口/存储原值。
 * 全表未命中时用语根/「其他事件」兜底，避免直出英文技术码。
 */
export function getTaskEventDisplayTitle(eventType: string | undefined | null): string {
  if (eventType == null || !String(eventType).trim()) return '—'
  const key = String(eventType).trim()
  const fromTable = getOperationEventTypeLabel(key, 'raw')
  if (fromTable !== key) return fromTable
  const t = key.toLowerCase()
  if (t.includes('replace') || t.includes('replacement')) return '稿件替换'
  if (t.includes('submit') && t.includes('design')) return '提交设计'
  if (t.includes('audit')) return '审核'
  if (t.includes('assign')) return '指派'
  if (t.startsWith('module.')) return '流程模块'
  return '其他事件'
}
