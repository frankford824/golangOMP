import { describe, expect, it } from 'vitest'
import {
  buildKpiOperationTraceEvent,
  kpiOperationActorDisplayName,
  kpiOperationActorId,
  shouldContinueUserDirectoryLoad,
  type KpiUserDirectoryEntry,
} from '@/domain/data-center-kpi'
import type { OperationLogEntry } from '@/services/apiTypes'

function operation(overrides: Partial<OperationLogEntry>): OperationLogEntry {
  return {
    source: 'task_event',
    log_id: '1',
    reference_type: 'task',
    reference_id: '100',
    event_type: 'task.assigned',
    summary: 'Task workflow event',
    actor_id: 253,
    actor_username: '王岩',
    actor_type: 'session_actor',
    payload: { designer_id: 228 },
    created_at: '2026-06-10T06:00:00.000Z',
    ...overrides,
  }
}

describe('data-center kpi operation parsing', () => {
  it('attributes assignment workload to the target designer id, not the ops actor', () => {
    const entry = operation({ actor_id: 253, actor_username: '王岩', payload: { designer_id: 228 } })

    expect(kpiOperationActorId(entry)).toBe(228)
    expect(kpiOperationActorDisplayName(entry, 228, undefined)).toBe('人员#228')
  })

  it('uses directory metadata when the target designer exists', () => {
    const designer: KpiUserDirectoryEntry = {
      id: '228',
      username: 'designer228',
      name: '王亚琳',
      realName: '王亚琳',
      department: '设计研发部',
      team: '默认组',
    }

    const trace = buildKpiOperationTraceEvent(operation({}), {
      rangeStartMs: Date.parse('2026-06-10T00:00:00.000Z'),
      rangeEndMs: Date.parse('2026-06-10T23:59:59.999Z'),
      resolveUserById: (id) => (id === 228 ? designer : undefined),
    })

    expect(trace?.actor_id).toBe(228)
    expect(trace?.actor_username).toBe('王亚琳')
    expect(trace?.actor_department).toBe('设计研发部')
  })

  it('allows self-claim assignment to keep the actor name', () => {
    const entry = operation({ actor_id: 228, actor_username: '王亚琳', payload: { designer_id: 228 } })

    expect(kpiOperationActorDisplayName(entry, 228, undefined)).toBe('王亚琳')
  })

  it('continues loading the user directory when backend page size is capped but total is not reached', () => {
    expect(
      shouldContinueUserDirectoryLoad({
        receivedCount: 20,
        requestedPageSize: 500,
        totalLoaded: 20,
        total: 131,
      }),
    ).toBe(true)
  })

  it('stops loading the user directory only when the reported total is reached', () => {
    expect(
      shouldContinueUserDirectoryLoad({
        receivedCount: 31,
        requestedPageSize: 100,
        totalLoaded: 131,
        total: 131,
      }),
    ).toBe(false)
  })
})
