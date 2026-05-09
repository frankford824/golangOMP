import { describe, expect, it } from 'vitest'
import { buildClearDesignerAssigneePayload } from '../task-assignment-payload'

describe('buildClearDesignerAssigneePayload', () => {
  it('builds null designer payload with default remark', () => {
    expect(buildClearDesignerAssigneePayload()).toEqual({
      designer_id: null,
      remark: '清空指派',
    })
  })

  it('allows custom remark', () => {
    expect(buildClearDesignerAssigneePayload('退回待指派')).toEqual({
      designer_id: null,
      remark: '退回待指派',
    })
  })
})
