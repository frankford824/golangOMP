export interface MockNotification {
  id: number
  notification_type: string
  payload?: Record<string, unknown>
  is_read: boolean
  created_at: string
}

export const mockNotifications: MockNotification[] = [
  {
    id: 3,
    notification_type: 'task_assigned_to_me',
    payload: {
      action: 'reassign',
      task_id: 614,
      task_no: 'RW-20260428-A-000611',
      task_type: 'original_product_development',
      module_key: 'task',
      assigned_by: 1,
      assigned_by_name: '系统管理员',
      designer_id: 198,
      previous_handler_id: 202,
      previous_designer_id: 202,
    },
    is_read: false,
    created_at: '2026-04-28T10:26:10+08:00',
  },
  {
    id: 1,
    notification_type: 'task_cancelled',
    payload: {
      task_id: 614,
      task_no: 'RW-20260428-A-000611',
      cancel_reason: '需求变更',
      cancelled_by: 1,
      cancelled_by_name: '系统管理员',
      module_key: 'task',
    },
    is_read: false,
    created_at: '2026-04-28T10:23:29+08:00',
  },
  {
    id: 4,
    notification_type: 'pool_reassigned',
    payload: {
      task_id: 615,
      task_no: 'RW-20260428-A-000612',
      module_key: 'task',
      team_code: 'TEAM-DESIGN-A',
      team_name: '设计一组',
      reassigned_by: 1,
      reassigned_by_name: '系统管理员',
    },
    is_read: true,
    created_at: '2026-04-27T14:01:20+08:00',
  },
]

export function unreadNotificationCount(): number {
  return mockNotifications.filter((item) => !item.is_read).length
}
