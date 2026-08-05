import { mockAssets } from './db/assets'
import { mockNotifications } from './db/notifications'
import { mockTaskDrafts } from './db/taskDrafts'
import { mockTaskModules } from './db/taskModules'
import { mockTasks } from './db/tasks'
import { pushTaskEvent } from './db/events'
import { addMillisecondsToNowISO } from '@/utils/date'

function isoNowMinus(minutes: number): string {
  return addMillisecondsToNowISO(-minutes * 60_000)
}

let seeded = false

export function ensureMockSeed(): void {
  if (seeded) return
  seeded = true

  mockTasks.push(
    {
      id: 'task_1001',
      task_no: 'T-20260423-1001',
      task_type: 'original_product_development',
      title: '春季新款原款开发',
      priority: 'normal',
      status: 'pending_claim',
      created_by: 'ops_demo',
      created_at: isoNowMinus(90),
      updated_at: isoNowMinus(45),
    },
    {
      id: 'task_1002',
      task_no: 'T-20260423-1002',
      task_type: 'regular_customization',
      title: '常规定制补图',
      priority: 'high',
      status: 'in_progress',
      created_by: 'ops_demo',
      designer_id: '2',
      designer_name: '李四',
      assignee_id: '2',
      assignee_name: '李四',
      created_at: isoNowMinus(120),
      updated_at: isoNowMinus(15),
    },
    {
      id: 'task_1003',
      task_no: 'T-20260423-1003',
      task_type: 'retouch_task',
      title: '主图精修任务',
      priority: 'normal',
      status: 'pending_claim',
      created_by: 'ops_demo',
      created_at: isoNowMinus(50),
      updated_at: isoNowMinus(20),
    },
  )

  mockTaskModules.push(
    {
      id: 'tm_1',
      task_id: 'task_1001',
      module_key: 'design',
      state: 'pending_claim',
      claimed_by: null,
      allowed_actions: ['claim', 'start'],
      updated_at: isoNowMinus(44),
    },
    {
      id: 'tm_2',
      task_id: 'task_1002',
      module_key: 'design',
      state: 'in_progress',
      claimed_by: '2',
      allowed_actions: ['reassign'],
      updated_at: isoNowMinus(16),
    },
    {
      id: 'tm_2_audit',
      task_id: 'task_1002',
      module_key: 'audit',
      state: 'in_progress',
      claimed_by: 'auditor_demo',
      allowed_actions: ['approve', 'reject'],
      updated_at: isoNowMinus(14),
    },
    {
      id: 'tm_3',
      task_id: 'task_1003',
      module_key: 'retouch',
      state: 'pending_claim',
      claimed_by: null,
      allowed_actions: ['claim', 'submit'],
      updated_at: isoNowMinus(19),
    },
  )

  mockTaskDrafts.push({
    id: 'draft_1001',
    task_type: 'original_product_development',
    payload: {
      product_name: '待补充',
      task_type: 'original_product_development',
    },
    created_by: 'ops_demo',
    created_at: isoNowMinus(30),
    updated_at: isoNowMinus(10),
    expires_at: isoNowMinus(-6 * 24 * 60),
  })

  mockNotifications.push({
    id: 1001,
    notification_type: 'task_assigned_to_me',
    payload: {
      task_id: 1002,
      task_no: 'T-20260423-1002',
      action: 'assign',
    },
    is_read: false,
    created_at: isoNowMinus(8),
  })

  mockAssets.push({
    id: 'asset_1001',
    task_id: 'task_1002',
    file_name: 'design-v1.psd',
    file_role: 'source',
    created_at: isoNowMinus(20),
  })

  pushTaskEvent({
    task_id: 'task_1002',
    module_key: 'audit',
    event_type: 'module.enter',
    payload: {
      state: 'in_progress',
      operator_name: 'ops_demo',
      creator_id: 1,
      creator_name: 'ops_demo',
    },
  })
}
