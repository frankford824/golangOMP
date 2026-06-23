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

  it('hides reference formalization ids in task detail activity summaries', () => {
    const event = mapTaskEventRowToRecentEvent(
      {
        id: 4,
        event_type: 'task.reference.asset.formalized',
        created_at: '2026-06-23T09:54:52+08:00',
        operator_name: '王亚琳',
        payload: {
          ref_id: '71992380-8e4b-40d7-9f77-44d0966882b8',
          task_asset_id: 8215,
          design_asset_id: 8243,
          owner_module_key: 'basic_info',
        },
      },
      '1707',
    )

    expect(event.title).toBe('参考图已接入任务')
    expect(event.summary).toBe('王亚琳 已将创建任务时上传的参考图接入任务。')
    expect(event.summary).not.toContain('71992380')
    expect(event.summary).not.toContain('task_asset_id')
    expect(event.summary).not.toContain('design_asset_id')
  })

  it('keeps live upload payload details out of user-facing activity copy', () => {
    const event = mapTaskEventRowToRecentEvent(
      {
        id: 5,
        event_type: 'task.asset.upload_session.completed',
        created_at: '2026-06-23T10:55:33+08:00',
        operator_name: '芹菲',
        payload: {
          remark: '芹菲-常规海报-毕业横幅-拉个横幅告诉你我毕业了-300-50cm.psd',
          asset_id: 8245,
          asset_type: 'delivery',
          storage_key: 'tasks/RW-20260623-A-001702/assets/AST-0003/v1/delivery/1782183332515365113_80e758b5.psd',
          remote_upload_id: 'b19d2e399bc370f22ecf0896e3d524d3',
          upload_session_id: '975a4ca2-2494-4709-b373-4d09cd34c508',
        },
      },
      '1705',
    )

    expect(event.title).toBe('上传记录已完成')
    expect(event.summary).toBe('芹菲 完成了最终成品图上传记录。')
    expect(event.summary).not.toContain('上传会话')
    expect(event.summary).not.toContain('975a4ca2')
    expect(event.summary).not.toContain('storage_key')
    expect(event.summary).not.toContain('remote_upload_id')
  })

  it('uses business copy for warehouse and close events without handler ids', () => {
    const closeEvent = mapTaskEventRowToRecentEvent(
      {
        id: 6,
        event_type: 'task.closed',
        created_at: '2026-06-23T10:20:02+08:00',
        operator_name: 'session_actor #1',
        payload: {
          remark: '系统自动结单：仓库 30 分钟未处理',
          sku_code: 'CGK000128',
          from_handler_id: null,
          to_handler_id: null,
          from_task_status: 'PendingClose',
          to_task_status: 'Completed',
        },
      },
      '1001',
    )

    expect(closeEvent.summary).toBe('系统 已结单，SKU CGK000128，系统自动结单：仓库 30 分钟未处理。')
    expect(closeEvent.summary).not.toContain('handler')
    expect(closeEvent.summary).not.toContain('PendingClose')
    expect(closeEvent.summary).not.toContain('Completed')
    expect(closeEvent.summary).not.toContain('session_actor')
  })

  it('does not expose raw event type for unknown task activity', () => {
    const event = mapTaskEventRowToRecentEvent(
      {
        id: 7,
        event_type: 'task.some_internal_debug_event',
        created_at: '2026-06-23T10:20:02+08:00',
        operator_name: '用户228',
      },
      '1001',
    )

    expect(event.title).toBe('其他事件')
    expect(event.summary).toBe('系统 记录了其他事件。')
    expect(event.summary).not.toContain('task.some_internal_debug_event')
    expect(event.summary).not.toContain('用户228')
  })

  it('uses concise copy for system generated preview assets', () => {
    const event = mapTaskEventRowToRecentEvent(
      {
        id: 8,
        event_type: 'task.asset.version.created',
        created_at: '2026-06-23T10:55:37+08:00',
        operator_name: '李晓雨',
        payload: {
          asset_type: 'preview',
          storage_key: 'tasks/RW-1/assets/AST-3/v1/preview/1782183336566646834_4e38d400.webp',
          derived_async: true,
          derivation_reason: 'source_non_direct_preview',
        },
      },
      '1705',
    )

    expect(event.summary).toBe('系统生成了预览图。')
    expect(event.summary).not.toContain('预览辅助预览')
    expect(event.summary).not.toContain('storage_key')
  })

  it('translates backend status codes in task detail activity summaries', () => {
    const event = mapTaskEventRowToRecentEvent(
      {
        id: 9,
        event_type: 'task.filing.readback_confirmed',
        created_at: '2026-06-23T11:20:00+08:00',
        actor_username: '系统',
        message: 'ERP readback: filing_status changed to not_filed, image_sync_status pending_sync.',
      },
      '1705',
    )

    expect(event.summary).toContain('未同步')
    expect(event.summary).toContain('待同步')
    expect(event.summary).not.toContain('not_filed')
    expect(event.summary).not.toContain('pending_sync')
    expect(event.summary).not.toContain('filing_status')
  })
})
