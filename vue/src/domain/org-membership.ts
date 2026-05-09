import type { OrgTreeDepartmentNode, OrgTreeGroupNode, OrgUser } from '@/domain/types/org-membership'
import type { OrgOwnershipOptionsParsed } from '@/services/api/orgApi'

export function groupKey(departmentName: string, teamName: string): string {
  const d = (departmentName ?? '').trim()
  const t = (teamName ?? '').trim()
  return `${d}\n${t}`
}

export function parseGroupKey(key: string): { departmentName: string; teamName: string } {
  const idx = key.indexOf('\n')
  if (idx === -1) return { departmentName: key.trim(), teamName: '' }
  return { departmentName: key.slice(0, idx).trim(), teamName: key.slice(idx + 1).trim() }
}

/** 用户部门字段归一（WorkflowUser.department 可能是 string 或对象） */
export function userDepartmentLabel(u: OrgUser): string {
  const raw = u.department
  if (typeof raw === 'string') return raw.trim()
  if (raw && typeof raw === 'object' && 'name' in raw) {
    const n = (raw as { name?: unknown }).name
    return typeof n === 'string' ? n.trim() : ''
  }
  return ''
}

export function userTeamLabel(u: OrgUser): string {
  const t = u.team ?? u.group
  return typeof t === 'string' ? t.trim() : ''
}

export function userDisplayName(u: OrgUser): string {
  return (
    (typeof u.display_name === 'string' && u.display_name.trim()) ||
    (typeof u.displayName === 'string' && u.displayName.trim()) ||
    (typeof u.real_name === 'string' && u.real_name.trim()) ||
    (typeof u.username === 'string' && u.username.trim()) ||
    '—'
  )
}

export function formatUserGroupDisplay(u: OrgUser): string {
  const d = userDepartmentLabel(u)
  const t = userTeamLabel(u)
  if (d && t) return `${d} / ${t}`
  if (t) return t
  if (d) return `${d}（未分组）`
  return '未分组'
}

export function isUngroupedUser(u: OrgUser): boolean {
  return !userTeamLabel(u)
}

export function getUsersByGroupKey(users: OrgUser[], groupKeyStr: string): OrgUser[] {
  const { departmentName, teamName } = parseGroupKey(groupKeyStr)
  return users.filter((u) => userDepartmentLabel(u) === departmentName && userTeamLabel(u) === teamName)
}

export function getGroupMemberCount(groupKeyStr: string, users: OrgUser[]): number {
  return getUsersByGroupKey(users, groupKeyStr).length
}

export function getUngroupedUsers(users: OrgUser[]): OrgUser[] {
  return users.filter(isUngroupedUser)
}

/**
 * 搜索候选：用户名、姓名、部门、组、角色
 */
export function filterCandidateUsers(query: string, users: OrgUser[]): OrgUser[] {
  const q = query.trim().toLowerCase()
  if (!q) return users
  return users.filter((u) => {
    const username = (u.username ?? '').toLowerCase()
    const name = userDisplayName(u).toLowerCase()
    const dept = userDepartmentLabel(u).toLowerCase()
    const team = userTeamLabel(u).toLowerCase()
    const roles = (u.roles ?? []).join(' ').toLowerCase()
    return (
      username.includes(q) ||
      name.includes(q) ||
      dept.includes(q) ||
      team.includes(q) ||
      roles.includes(q)
    )
  })
}

function insertGroup(
  map: Map<string, OrgTreeGroupNode[]>,
  departmentName: string,
  node: OrgTreeGroupNode,
) {
  const d = departmentName.trim()
  if (!d || !node.teamName.trim()) return
  const list = map.get(d) ?? []
  if (!list.some((g) => g.key === node.key)) list.push(node)
  map.set(d, list)
}

/**
 * 合并：/v1/org/options + 权限 store 中的部门/组 + 用户实际出现的部门/组（避免树为空）
 */
export function buildOrgTreeDepartments(
  api: OrgOwnershipOptionsParsed | null,
  extraDepartments: { id: string; name: string }[],
  extraGroups: { id: string; name: string; departmentId: string }[],
  departmentIdToName: Map<string, string>,
  departmentNameToStoreId: Map<string, string>,
  users: OrgUser[],
): OrgTreeDepartmentNode[] {
  const deptToGroups = new Map<string, OrgTreeGroupNode[]>()

  if (api) {
    for (const t of api.teamOptions) {
      const teamName = (t.label || t.value).trim()
      if (!teamName) continue
      const deptName = (t.department ?? '').trim() || '未指定部门'
      const key = groupKey(deptName, teamName)
      insertGroup(deptToGroups, deptName, {
        key,
        departmentName: deptName,
        teamName,
        fromApi: true,
      })
    }
    for (const d of api.departmentOptions) {
      const name = (d.label || d.value).trim()
      if (!name) continue
      if (!deptToGroups.has(name)) deptToGroups.set(name, [])
    }
  }

  for (const g of extraGroups) {
    const deptName = departmentIdToName.get(g.departmentId) ?? ''
    if (!deptName) continue
    const key = groupKey(deptName, g.name.trim())
    insertGroup(deptToGroups, deptName, {
      key,
      departmentName: deptName,
      teamName: g.name.trim(),
      storeDepartmentId: g.departmentId,
      storeGroupId: g.id,
      fromApi: false,
    })
  }

  for (const u of users) {
    const d = userDepartmentLabel(u)
    const t = userTeamLabel(u)
    if (!d || !t) continue
    const key = groupKey(d, t)
    insertGroup(deptToGroups, d, {
      key,
      departmentName: d,
      teamName: t,
      fromApi: false,
    })
  }

  const deptNames = new Set<string>([...deptToGroups.keys()])
  for (const d of extraDepartments) {
    if (d.name.trim()) deptNames.add(d.name.trim())
  }
  if (api) {
    for (const d of api.departmentOptions) {
      const name = (d.label || d.value).trim()
      if (name) deptNames.add(name)
    }
  }

  const sortedDepts = [...deptNames].filter(Boolean).sort((a, b) => a.localeCompare(b, 'zh-CN'))

  return sortedDepts.map((name) => {
    const groups = (deptToGroups.get(name) ?? []).sort((a, b) =>
      a.teamName.localeCompare(b.teamName, 'zh-CN'),
    )
    const storeDepartmentId = departmentNameToStoreId.get(name)
    return { name, expanded: true, groups, storeDepartmentId }
  })
}
