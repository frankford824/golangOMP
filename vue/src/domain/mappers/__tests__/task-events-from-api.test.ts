import { describe, expect, it } from 'vitest'
import { mapTaskEventRowToRecentEvent } from '@/domain/mappers/task-events-from-api'

describe('mapTaskEventRowToRecentEvent', () => {
  it('hides upload session ids in task detail activity summaries', () => {
    const event = mapTaskEventRowToRecentEvent(
      {
        id: 1,
        event_type: 'task.asset.upload_session.completed',
        created_at: '2026-06-01T14:41:00+08:00',
        actor_username: '露',
        payload: {
          asset_type: 'reference',
          upload_session_id: '0a6840b2-127a-4ba4-806f-59733af93eb7',
        },
      },
      '1024',
    )

    expect(event.summary).toBe('露 完成了运营参考图上传记录。')
    expect(event.summary).not.toContain('0a6840b2')
    expect(event.summary).not.toContain('上传会话')
  })

  it('sanitizes backend-provided technical summaries before display', () => {
    const event = mapTaskEventRowToRecentEvent(
      {
        id: 2,
        event_type: 'task.asset.upload_session.created',
        created_at: '2026-06-01T14:41:00+08:00',
        message: '露 创建了运营参考图上传会话（aa327068-bae7-41e5-b981-1b9cf7bb1515）。',
      },
      '1024',
    )

    expect(event.summary).toBe('露 创建了运营参考图上传记录。')
    expect(event.summary).not.toContain('aa327068')
    expect(event.summary).not.toContain('上传会话')
  })

  it('uses a business placeholder when assignee account name is absent', () => {
    const event = mapTaskEventRowToRecentEvent(
      {
        id: 3,
        event_type: 'task.assigned',
        created_at: '2026-06-01T14:42:00+08:00',
        actor_username: '露',
        payload: {
          assignee_id: 99,
          to_task_status: 'Designing',
          result: 'normal',
        },
      },
      '1024',
    )

    expect(event.summary).toContain('待确认人员')
    expect(event.summary).not.toContain('未知用户')
  })
})
