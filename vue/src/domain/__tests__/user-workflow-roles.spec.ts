import { describe, expect, it } from 'vitest'
import {
  formatUserRoleForDisplay,
  formatWorkflowRolesForDisplay,
  workflowRoleApiToDisplay,
} from '../user-workflow-roles'

describe('user-workflow-roles 展示', () => {
  it('历史身份只显示中性停用提示', () => {
    expect(workflowRoleApiToDisplay('Admin')).toBe('超级管理员')
    expect(workflowRoleApiToDisplay('SuperAdmin')).toBe('超级管理员')
    expect(workflowRoleApiToDisplay('TeamLead')).toBe('部门管理员')
    expect(workflowRoleApiToDisplay('DesignDirector')).toBe('部门管理员')
    expect(workflowRoleApiToDisplay('ERP')).toBe('超级管理员')
    expect(workflowRoleApiToDisplay('LegacyRole')).toBe('历史身份（已停用）')
  })

  it('主角色 slug snake_case → 中文', () => {
    expect(formatUserRoleForDisplay('super_admin')).toBe('超级管理员')
    expect(formatUserRoleForDisplay('designer')).toBe('常规设计师')
  })

  it('多身份展示会合并同义审核身份', () => {
    expect(
      formatWorkflowRolesForDisplay(['Audit_A', 'Audit_B', 'CustomizationReviewer', 'Designer']),
    ).toBe('审核、常规设计师')
  })

  it('主角色空值', () => {
    expect(formatUserRoleForDisplay('')).toBe('-')
    expect(formatUserRoleForDisplay(undefined)).toBe('-')
  })
})
