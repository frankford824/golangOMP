import type { AssignTaskPayload } from '@/services/apiTypes'

export function buildClearDesignerAssigneePayload(remark = '清空指派'): AssignTaskPayload {
  return {
    designer_id: null,
    remark,
  }
}
