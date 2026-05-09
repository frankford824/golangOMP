import { mockTasks } from '../db/tasks'
import { listTaskModules } from '../db/taskModules'
import type { MockHandler } from './types'

export const reportsHandler: MockHandler = (request) => {
  if (request.method !== 'GET') return null

  if (request.path === '/v1/reports/l1/throughput') {
    return {
      status: 200,
      data: {
        data: [
          {
            date: '2026-04-20',
            created: mockTasks.length,
            completed: 1,
            archived: 0,
          },
        ],
      },
    }
  }

  if (request.path === '/v1/reports/l1/module-dwell') {
    return {
      status: 200,
      data: {
        data: [
          {
            module_key: 'design',
            avg_dwell_seconds: 3600,
            p95_dwell_seconds: 7200,
            samples: mockTasks.length,
          },
        ],
      },
    }
  }

  if (request.path !== '/v1/reports/l1/cards') return null

  const designInProgress = mockTasks.filter((task) =>
    listTaskModules(task.id).some((module) => module.module_key === 'design' && module.state === 'in_progress'),
  ).length
  const auditPending = mockTasks.filter((task) =>
    listTaskModules(task.id).some((module) => module.module_key === 'audit' && module.state === 'pending_claim'),
  ).length
  const archivedToday = mockTasks.filter((task) => task.status === 'closed').length
  const createdToday = mockTasks.length
  const poolCount = mockTasks.filter((task) => task.status === 'pending_claim').length

  return {
    status: 200,
    data: {
      data: [
        { key: 'pool', title: '待接单', value: poolCount, unit: '' },
        { key: 'design_in_progress', title: '设计进行中', value: designInProgress, unit: '' },
        { key: 'audit_pending', title: '待审核', value: auditPending, unit: '' },
        { key: 'archived_today', title: '今日归档', value: archivedToday, unit: '' },
        { key: 'created_today', title: '今日新建', value: createdToday, unit: '' },
      ],
    },
  }
}
