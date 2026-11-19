export function isUnassignedDesignDepartment(actor: { actorDepartmentId: number | null; actorDepartment: string }): boolean {
  return !actor.actorDepartmentId || actor.actorDepartment.trim() === '未分配'
}
