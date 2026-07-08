/**
 * 组织相关 API（筛选下拉、v0.9 组织主数据变更）
 */
import http from '@/services/http'
import type { Department, Group } from '@/types'

export interface OrgDepartmentRecord {
  id: string
  name: string
  enabled?: boolean
  /** 后端 member_count:该部门当前人数(含停用账号),用于树上人数徽标与治理判断 */
  memberCount?: number
}

export interface OrgTeamRecord {
  id: string
  name: string
  departmentId: string
  departmentName?: string
  enabled?: boolean
  /** 后端 member_count:该小组当前人数(含停用账号) */
  memberCount?: number
}

export interface OrgOwnershipOptionsParsed {
  departmentOptions: Array<{ value: string; label: string }>
  /** 团队项，可选带 department 用于联动过滤 */
  teamOptions: Array<{ value: string; label: string; department?: string }>
  /** GET /v1/org/options 若返回带 id 的结构，供 Pinia 与 POST /teams 的 department_id 对齐 */
  departmentRecords: OrgDepartmentRecord[]
  teamRecords: OrgTeamRecord[]
}

export interface FetchOrgOwnershipOptionsOptions {
  signal?: AbortSignal
  includeDisabled?: boolean
  /** 默认失败时静默返回空列表;置 true 时把网络/解析错误抛给调用方显式提示 */
  throwOnError?: boolean
}

function asString(v: unknown): string | undefined {
  if (typeof v === 'string' && v.trim() !== '') return v.trim()
  return undefined
}

function recordEntityId(rec: Record<string, unknown>, keys: string[]): string | undefined {
  for (const key of keys) {
    const v = rec[key]
    if (typeof v === 'number' && Number.isFinite(v)) return String(v)
    if (typeof v === 'string' && v.trim()) return v.trim()
  }
  return undefined
}

function recordDepartmentId(rec: Record<string, unknown>): string | undefined {
  return recordEntityId(rec, ['id', 'department_id', 'departmentId'])
}

function recordTeamId(rec: Record<string, unknown>): string | undefined {
  return recordEntityId(rec, ['id', 'team_id', 'teamId'])
}

function unwrapCreatedRow(body: unknown): Record<string, unknown> {
  const root = (body && typeof body === 'object' ? body : {}) as Record<string, unknown>
  const inner = (root.data && typeof root.data === 'object' ? root.data : root) as Record<string, unknown>
  return inner
}

function isAbortSignalLike(value: unknown): value is AbortSignal {
  return !!value && typeof value === 'object' && 'aborted' in value && 'addEventListener' in value
}

/** 将 GET /v1/org/options 中带 id 的读模型同步为任务侧 / 小组管理用的 Department[]、Group[] */
export function departmentsAndGroupsFromOrgOptions(
  parsed: OrgOwnershipOptionsParsed,
): { departments: Department[]; groups: Group[] } | null {
  const dr = parsed.departmentRecords ?? []
  const tr = parsed.teamRecords ?? []
  if (!dr.length && !tr.length) return null

  const departments: Department[] =
    dr.length > 0
      ? dr.map((d) => ({ id: d.id, name: d.name }))
      : (() => {
          const seen = new Set<string>()
          const out: Department[] = []
          for (const t of tr) {
            const id = String(t.departmentId ?? '').trim()
            if (!id || seen.has(id)) continue
            seen.add(id)
            const label = parsed.teamOptions.find((x) => x.department === id)?.department
            out.push({ id, name: (typeof label === 'string' && label.trim()) || id })
          }
          return out
        })()

  const groups: Group[] = tr.map((t) => ({
    id: t.id,
    name: t.name,
    departmentId: t.departmentId,
  }))

  return { departments, groups }
}

/** POST /v1/org/departments */
export async function createOrgDepartment(payload: {
  name: string
  enabled?: boolean
}): Promise<{ id: string; name: string }> {
  const res = await http.post<unknown>('/v1/org/departments', {
    name: payload.name.trim(),
    enabled: payload.enabled ?? true,
  })
  const row = unwrapCreatedRow(res.data)
  const id = recordDepartmentId(row) ?? ''
  const name = asString(row.name) ?? payload.name.trim()
  if (!id) throw new Error('创建部门成功但未返回 id')
  return { id, name }
}

/** POST /v1/org/teams */
export async function createOrgTeam(payload: {
  name: string
  department_id: string | number
  enabled?: boolean
}): Promise<{ id: string; name: string; department_id: string }> {
  const deptId =
    typeof payload.department_id === 'number'
      ? payload.department_id
      : /^\d+$/.test(String(payload.department_id).trim())
        ? Number(String(payload.department_id).trim())
        : String(payload.department_id).trim()
  const res = await http.post<unknown>('/v1/org/teams', {
    name: payload.name.trim(),
    department_id: deptId,
    enabled: payload.enabled ?? true,
  })
  const row = unwrapCreatedRow(res.data)
  const id = recordTeamId(row) ?? ''
  const name = asString(row.name) ?? payload.name.trim()
  const did = asString(row.department_id ?? row.departmentId) ?? String(payload.department_id)
  if (!id) throw new Error('创建小组成功但未返回 id')
  return { id, name, department_id: did }
}

export interface UpdateOrgDepartmentPayload {
  name?: string
  enabled?: boolean
}

export interface UpdateOrgTeamPayload {
  name?: string
  enabled?: boolean
}

/** PUT /v1/org/departments/{id} */
export async function updateOrgDepartment(
  id: string | number,
  payload: UpdateOrgDepartmentPayload | boolean,
): Promise<void> {
  await http.put(`/v1/org/departments/${id}`, typeof payload === 'boolean' ? { enabled: payload } : payload)
}

/** PUT /v1/org/teams/{id} */
export async function updateOrgTeam(
  id: string | number,
  payload: UpdateOrgTeamPayload | boolean,
): Promise<void> {
  await http.put(`/v1/org/teams/${id}`, typeof payload === 'boolean' ? { enabled: payload } : payload)
}

/** POST /v1/org/departments/{id}/merge — 把 source 部门的用户/小组并入 target 后停用 source */
export async function mergeOrgDepartment(
  sourceId: string | number,
  targetDepartmentId: string | number,
): Promise<void> {
  await http.post(`/v1/org/departments/${sourceId}/merge`, {
    target_department_id: Number(targetDepartmentId),
  })
}

/** POST /v1/org/teams/{id}/merge — 把 source 小组的用户并入 target 小组后停用 source */
export async function mergeOrgTeam(
  sourceId: string | number,
  targetTeamId: string | number,
): Promise<void> {
  await http.post(`/v1/org/teams/${sourceId}/merge`, {
    target_team_id: Number(targetTeamId),
  })
}

/** DELETE /v1/org/departments/{id} — 仅允许删除已停用且无成员的部门(硬删除,含其小组) */
export async function deleteOrgDepartment(id: string | number): Promise<void> {
  await http.delete(`/v1/org/departments/${id}`)
}

/** DELETE /v1/org/teams/{id} — 仅允许删除已停用且无成员的小组(硬删除) */
export async function deleteOrgTeam(id: string | number): Promise<void> {
  await http.delete(`/v1/org/teams/${id}`)
}

export const orgMoveRequestsApi = {
  list: (params?: Record<string, unknown>, signal?: AbortSignal) =>
    http.get('/v1/org-move-requests', { params, signal }),

  /**
   * POST /v1/departments/{departmentId}/org-move-requests
   */
  create: (departmentId: string | number, payload: Record<string, unknown>, signal?: AbortSignal) =>
    http.post(`/v1/departments/${encodeURIComponent(String(departmentId))}/org-move-requests`, payload, { signal }),

  approve: (id: string, payload: Record<string, unknown> = {}, signal?: AbortSignal) =>
    http.post(`/v1/org-move-requests/${encodeURIComponent(id)}/approve`, payload, { signal }),

  reject: (id: string, payload: Record<string, unknown>, signal?: AbortSignal) =>
    http.post(`/v1/org-move-requests/${encodeURIComponent(id)}/reject`, payload, { signal }),
}

/**
 * GET /v1/org/options
 * 响应结构兼容多种后端形态；失败时返回空列表，不抛错。
 */
export async function fetchOrgOwnershipOptions(
  signalOrOptions?: AbortSignal | FetchOrgOwnershipOptionsOptions,
): Promise<OrgOwnershipOptionsParsed> {
  const empty: OrgOwnershipOptionsParsed = {
    departmentOptions: [],
    teamOptions: [],
    departmentRecords: [],
    teamRecords: [],
  }
  const requestOptions: FetchOrgOwnershipOptionsOptions = isAbortSignalLike(signalOrOptions)
    ? { signal: signalOrOptions }
    : (signalOrOptions ?? {})
  try {
    const res = await http.get<unknown>('/v1/org/options', {
      signal: requestOptions.signal,
      params: requestOptions.includeDisabled ? { include_disabled: true } : undefined,
    })
    const body = (res.data as { data?: unknown })?.data ?? res.data
    if (!body || typeof body !== 'object') return empty
    const o = body as Record<string, unknown>

    let departmentOptions: Array<{ value: string; label: string }> = []
    const rawDepts = o.departments ?? o.department_list
    if (Array.isArray(rawDepts)) {
      departmentOptions = rawDepts
        .map((d) => {
          if (typeof d === 'string') return { value: d, label: d }
          if (d && typeof d === 'object') {
            const rec = d as Record<string, unknown>
            const name = asString(rec.name ?? rec.label ?? rec.title)
            if (name) return { value: name, label: name }
          }
          return undefined
        })
        .filter((x): x is { value: string; label: string } => x != null)
    }

    const teamOptions: Array<{ value: string; label: string; department?: string }> = []
    const teamKeySet = new Set<string>()

    function pushTeam(nameRaw: unknown, departmentRaw?: unknown) {
      const name = asString(nameRaw)
      if (!name) return
      const dept = asString(departmentRaw)
      const key = `${name}@@${dept ?? ''}`
      if (teamKeySet.has(key)) return
      teamKeySet.add(key)
      teamOptions.push({ value: name, label: name, department: dept })
    }

    // 形态 B（v1.8 规范形态，canonical）：departments: [{ name, teams: [...] }]
    // v1.8 起后端 `GET /v1/org/options` 的规范响应在 departments[] 上内联 teams；
    // 另两种形态仅作为过渡期降级来源，命中时会发一次 console.warn 供回归观测。
    let hydratedFromCanonical = false
    if (Array.isArray(rawDepts)) {
      for (const d of rawDepts) {
        if (!d || typeof d !== 'object') continue
        const rec = d as Record<string, unknown>
        const deptName = asString(rec.name ?? rec.label ?? rec.title)
        const teams = Array.isArray(rec.team_items) ? rec.team_items : rec.teams
        if (!Array.isArray(teams)) continue
        for (const t of teams) {
          if (typeof t === 'string') {
            pushTeam(t, deptName)
            hydratedFromCanonical = true
          } else if (t && typeof t === 'object') {
            const tr = t as Record<string, unknown>
            pushTeam(tr.name ?? tr.label ?? tr.team ?? tr.value, deptName ?? tr.department)
            hydratedFromCanonical = true
          }
        }
      }
    }

    // 形态 A（已弃用）：顶层 teams / org_teams
    // 后端自 v1.8 起标注 `Deprecation: version="v1.8"`；仅当 canonical 分支未产出数据时才采纳，
    // 同时发 warn 以便 telemetry 识别残留依赖。
    const rawTeams = o.teams ?? o.org_teams
    if (!hydratedFromCanonical && Array.isArray(rawTeams) && rawTeams.length > 0) {
      // eslint-disable-next-line no-console
      console.warn(
        '[orgApi] /v1/org/options fell back to deprecated top-level `teams` shape. ' +
          'Backend v1.8 canonical is `departments[].teams`; server may still ship compat payload.',
      )
      for (const t of rawTeams) {
        if (typeof t === 'string') {
          pushTeam(t)
          continue
        }
        if (t && typeof t === 'object') {
          const rec = t as Record<string, unknown>
          pushTeam(
            rec.name ?? rec.label ?? rec.team ?? rec.value,
            rec.department ?? rec.department_name ?? rec.owner_department,
          )
        }
      }
    }

    // 形态 C（已弃用）：teams_by_department: { "运营部": ["运营一组", ...] }
    const byDept = o.teams_by_department
    if (
      !hydratedFromCanonical &&
      byDept &&
      typeof byDept === 'object' &&
      !Array.isArray(byDept)
    ) {
      // eslint-disable-next-line no-console
      console.warn(
        '[orgApi] /v1/org/options fell back to deprecated `teams_by_department` shape. ' +
          'Backend v1.8 canonical is `departments[].teams`.',
      )
      for (const [deptNameRaw, teams] of Object.entries(byDept as Record<string, unknown>)) {
        const deptName = asString(deptNameRaw)
        if (!deptName || !Array.isArray(teams)) continue
        for (const t of teams) {
          if (typeof t === 'string') pushTeam(t, deptName)
          else if (t && typeof t === 'object') {
            const tr = t as Record<string, unknown>
            pushTeam(tr.name ?? tr.label ?? tr.team ?? tr.value, deptName)
          }
        }
      }
    }

    const deptSet = new Set(departmentOptions.map((d) => d.value))
    for (const t of teamOptions) {
      if (t.department && !deptSet.has(t.department)) {
        departmentOptions.push({ value: t.department, label: t.department })
        deptSet.add(t.department)
      }
    }

    const departmentRecords: OrgDepartmentRecord[] = []
    const teamRecords: OrgTeamRecord[] = []
    if (Array.isArray(rawDepts)) {
      for (const d of rawDepts) {
        if (typeof d !== 'object' || !d) continue
        const rec = d as Record<string, unknown>
        const did = recordDepartmentId(rec)
        const dname = asString(rec.name ?? rec.label ?? rec.title)
        if (did && dname) {
          departmentRecords.push({
            id: did,
            name: dname,
            enabled: rec.enabled === false ? false : true,
            memberCount: typeof rec.member_count === 'number' ? rec.member_count : undefined,
          })
        }
        const teams = Array.isArray(rec.team_items) ? rec.team_items : rec.teams
        if (!Array.isArray(teams) || !did) continue
        for (const t of teams) {
          if (typeof t !== 'object' || !t) continue
          const tr = t as Record<string, unknown>
          const tid = recordTeamId(tr)
          const tname = asString(tr.name ?? tr.label ?? tr.team ?? tr.value)
          if (!tid || !tname) continue
          teamRecords.push({
            id: tid,
            name: tname,
            departmentId: did,
            departmentName: dname,
            enabled: tr.enabled === false ? false : true,
            memberCount: typeof tr.member_count === 'number' ? tr.member_count : undefined,
          })
        }
      }
    }

    return { departmentOptions, teamOptions, departmentRecords, teamRecords }
  } catch (error) {
    if (requestOptions.throwOnError) throw error
    return empty
  }
}
