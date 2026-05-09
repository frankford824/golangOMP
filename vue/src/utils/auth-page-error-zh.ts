/**
 * 登录 / 注册页：将后端错误体映射为中文展示文案。
 * 实现已收敛至 @/utils/api-message-zh，此处保留导出路径以免各处 import 断裂。
 */

import { resolveApiUserMessage } from '@/utils/api-message-zh'

/**
 * 将登录/注册相关错误转为中文说明；保留 trace_id 便于用户反馈（不改变接口）。
 */
export function formatAuthPageError(err: unknown): string {
  return resolveApiUserMessage(err, {
    includeTrace: true,
    fallback: '操作失败，请稍后重试',
  })
}
