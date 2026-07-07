import { beforeEach, describe, expect, it } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import { DataScopeEnum, RoleEnum } from '@/types'
import { roleCodeCompareKeys, usePermissionsStore } from '@/stores/permissions'

describe('permissions role code matching', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
  })

  function signInWithRoles(roles: string[]): ReturnType<typeof usePermissionsStore> {
    const permissions = usePermissionsStore()
    permissions.setCurrentUser({
      id: '9',
      name: 'Role User',
      role: RoleEnum.MEMBER,
      departmentId: '',
      groupId: '',
      dataScope: DataScopeEnum.GLOBAL,
      permissions: [],
    })
    permissions.roles = roles
    return permissions
  }

  it('matches backend PascalCase roles with frontend snake_case roles', () => {
    const permissions = signInWithRoles(['super_admin', 'hr_admin', 'department_admin', 'team_lead'])

    expect(permissions.hasAnyRole(['SuperAdmin'])).toBe(true)
    expect(permissions.hasAnyRole(['HRAdmin'])).toBe(true)
    expect(permissions.hasAnyRole(['DepartmentAdmin'])).toBe(true)
    expect(permissions.hasAnyRole(['TeamLead'])).toBe(true)
  })

  it('keeps historical frontend aliases for department and team leads', () => {
    const permissions = signInWithRoles(['dept_admin', 'group_leader'])

    expect(permissions.hasAnyRole(['DepartmentAdmin'])).toBe(true)
    expect(permissions.hasAnyRole(['TeamLead'])).toBe(true)
  })

  it('normalizes separators without widening unrelated roles', () => {
    expect(roleCodeCompareKeys('Audit_A')).toContain('audita')
    expect(roleCodeCompareKeys('audit_a')).toContain('audita')

    const permissions = signInWithRoles(['role_admin'])
    expect(permissions.hasAnyRole(['RoleAdmin'])).toBe(true)
    expect(permissions.hasAnyRole(['TeamLead'])).toBe(false)
  })
})
