/**
 * Round I.g · D1 单测：`computeAllowedCreateOwnerScope` 根据 actor snapshot
 * 正确裁剪 owner_department / owner_team 白名单，防止 DA 选到其他部门。
 */
import { describe, it, expect } from 'vitest'
import { computeAllowedCreateOwnerScope } from '../src/composables/useActorOwnerScope'

const baseSnapshot = {
  isSuperAdmin: false,
  isHRAdmin: false,
  isDeptAdmin: false,
  isGroupLeader: false,
  roles: [] as readonly string[],
  managedDepartments: [] as readonly string[],
  managedTeams: [] as readonly string[],
  actorDepartment: '',
  actorTeam: '',
}

describe('computeAllowedCreateOwnerScope', () => {
  it('case 5：运营部 DA 仅可选 [运营部]，人事部 不在白名单内', () => {
    const scope = computeAllowedCreateOwnerScope({
      ...baseSnapshot,
      isDeptAdmin: true,
      roles: ['dept_admin'],
      managedDepartments: ['运营部'],
      actorDepartment: '运营部',
      actorTeam: '运营一组',
    })
    expect(scope.isOwnerScopeUnrestricted).toBe(false)
    expect(scope.hideOwnerFields).toBe(false)
    expect(scope.lockOwnerDepartment).toBe(true)
    expect([...scope.allowedCreateOwnerDepartments]).toContain('运营部')
    expect([...scope.allowedCreateOwnerDepartments]).not.toContain('人事部')
  })

  it('case 6：人事部 DA 仅可选 [人事部]', () => {
    const scope = computeAllowedCreateOwnerScope({
      ...baseSnapshot,
      isDeptAdmin: true,
      roles: ['dept_admin'],
      managedDepartments: ['人事部'],
      actorDepartment: '人事部',
    })
    expect([...scope.allowedCreateOwnerDepartments]).toEqual(['人事部'])
  })

  it('SuperAdmin / HRAdmin：isOwnerScopeUnrestricted=true，白名单为空表示不限制', () => {
    const s1 = computeAllowedCreateOwnerScope({ ...baseSnapshot, isSuperAdmin: true })
    expect(s1.isOwnerScopeUnrestricted).toBe(true)
    expect(s1.allowedCreateOwnerDepartments).toEqual([])
    const s2 = computeAllowedCreateOwnerScope({ ...baseSnapshot, isHRAdmin: true })
    expect(s2.isOwnerScopeUnrestricted).toBe(true)
  })

  it('GroupLeader：只锁定 team，不锁定 department', () => {
    const scope = computeAllowedCreateOwnerScope({
      ...baseSnapshot,
      isGroupLeader: true,
      roles: ['group_leader'],
      managedTeams: ['运营一组', '运营二组'],
      actorTeam: '运营一组',
      actorDepartment: '运营部',
    })
    expect(scope.lockOwnerTeam).toBe(true)
    expect(scope.lockOwnerDepartment).toBe(false)
    expect([...scope.allowedCreateOwnerTeams]).toEqual(
      expect.arrayContaining(['运营一组', '运营二组']),
    )
  })

  it('纯 Ops / Designer / Member：hideOwnerFields=true', () => {
    const scope = computeAllowedCreateOwnerScope({
      ...baseSnapshot,
      roles: ['ops'],
      actorTeam: '运营一组',
      actorDepartment: '运营部',
    })
    expect(scope.hideOwnerFields).toBe(true)
    expect(scope.isOwnerScopeUnrestricted).toBe(false)
  })
})
