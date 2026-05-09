import { describe, expect, it } from 'vitest'
import {
  formatUserRoleForDisplay,
  formatWorkflowRolesForDisplay,
  workflowRoleApiToDisplay,
} from '../user-workflow-roles'

describe('user-workflow-roles 展示', () => {
  it('V7Role PascalCase → 中文', () => {
    expect(workflowRoleApiToDisplay('Admin')).toBe('管理员')
    expect(workflowRoleApiToDisplay('SuperAdmin')).toBe('超级管理员')
    expect(workflowRoleApiToDisplay('Outsource')).toBe('外协')
  })

  it('主角色 slug snake_case → 中文', () => {
    expect(formatUserRoleForDisplay('super_admin')).toBe('超级管理员')
    expect(formatUserRoleForDisplay('designer')).toBe('设计师')
  })

  it('多角色与 GET /v1/me/org 示例数据', () => {
    expect(
      formatWorkflowRolesForDisplay(['Admin', 'Designer', 'Member', 'Ops', 'SuperAdmin']),
    ).toBe('管理员、设计师、成员、运营、超级管理员')
  })

  it('主角色空值', () => {
    expect(formatUserRoleForDisplay('')).toBe('-')
    expect(formatUserRoleForDisplay(undefined)).toBe('-')
  })
})
