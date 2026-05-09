/**
 * 系统管理接口预留（P2 阶段实现）
 * 占位文件：保持模块边界，接入后端时填充具体实现
 */

export interface User {
  id: string
  name: string
  role: string
  groupId?: string
  departmentId?: string
}

export interface CreateUserPayload {
  name: string
  role: string
  groupId?: string
  departmentId?: string
  action_id: string
}

/** 查询用户列表（占位，P2 阶段实现） */
export async function fetchUsers(): Promise<User[]> {
  return Promise.resolve([])
}

/** 创建用户（占位，P2 阶段实现） */
export async function createUser(_payload: CreateUserPayload): Promise<User | null> {
  return Promise.resolve(null)
}

/** 查询操作审计日志（占位，P2 阶段实现） */
export async function fetchAuditLog(_params: {
  page: number
  pageSize: number
  operatorId?: string
  actionType?: string
  dateFrom?: string
  dateTo?: string
}): Promise<{ items: unknown[]; total: number }> {
  return Promise.resolve({ items: [], total: 0 })
}
