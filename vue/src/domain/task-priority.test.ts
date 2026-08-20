import { describe, expect, it } from 'vitest'
import { normalizePriorityForApi, TASK_PRIORITY_OPTIONS } from './task-priority'

describe('task priority options', () => {
  it('keeps drawing as a canonical selectable and queryable priority', () => {
    expect(TASK_PRIORITY_OPTIONS).toContainEqual({ label: '出单画图', value: 'drawing' })
    expect(normalizePriorityForApi('drawing')).toBe('drawing')
  })
})
