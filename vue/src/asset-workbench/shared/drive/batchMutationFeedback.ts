import { mapRawBackendMessageToZh } from '@/utils/api-message-zh'

interface BatchMutationFailure {
  file_id: number
  reason: string
}

export function batchMutationFailureMessage(action: '移动' | '删除', failures: BatchMutationFailure[] | undefined) {
  if (!failures?.length) return ''
  const firstReason = mapRawBackendMessageToZh(failures[0]?.reason || '')
  const remaining = failures.length > 1 ? `，另有 ${failures.length - 1} 个文件同样未处理` : ''
  const reason = remaining ? firstReason.replace(/[。！？；，,.!?;]+$/, '') : firstReason
  return `${action}未完成：${reason}${remaining}`
}
