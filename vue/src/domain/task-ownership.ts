/**
 * Canonical task ownership 展示与回退逻辑（与 legacy owner_team 区分）。
 * v0.9：以 owner_department + owner_org_team 为规范读模型；groupName/owner_team 仅作兼容展示，不作组织树真相源。
 * 列表/详情统一使用，避免各处散落判断。
 */

export interface TaskOwnershipDisplay {
  /** 主展示文案（canonical 优先，否则 legacy 回退） */
  primary: string
  /** 是否为 legacy 回退主展示 */
  usesFallback: boolean
  /** 正式部门（有则展示详情区块） */
  ownerDepartment?: string
  /** 正式团队（有则展示详情区块） */
  ownerOrgTeam?: string
  /** legacy owner_team 原始值（兼容 tooltip / 副文案） */
  legacyOwnerTeam?: string
}

/** 从任务读模型提取归属展示（安全处理空值） */
export function getTaskOwnershipDisplay(task: {
  ownerDepartment?: string
  ownerOrgTeam?: string
  groupName?: string
}): TaskOwnershipDisplay {
  const dept = typeof task.ownerDepartment === 'string' ? task.ownerDepartment.trim() : ''
  const team = typeof task.ownerOrgTeam === 'string' ? task.ownerOrgTeam.trim() : ''
  const legacy =
    typeof task.groupName === 'string' && task.groupName.trim() !== ''
      ? task.groupName.trim()
      : undefined

  if (dept && team) {
    return {
      primary: `${dept} / ${team}`,
      usesFallback: false,
      ownerDepartment: dept,
      ownerOrgTeam: team,
      legacyOwnerTeam: legacy,
    }
  }
  if (team) {
    return {
      primary: team,
      usesFallback: false,
      ownerOrgTeam: team,
      legacyOwnerTeam: legacy,
    }
  }
  if (dept) {
    return {
      primary: dept,
      usesFallback: false,
      ownerDepartment: dept,
      legacyOwnerTeam: legacy,
    }
  }
  if (legacy) {
    return {
      primary: `${legacy}（兼容）`,
      usesFallback: true,
      legacyOwnerTeam: legacy,
    }
  }
  return {
    primary: '未设置',
    usesFallback: false,
  }
}

/** 列表简短一行（与 primary 一致，便于模板直接调用） */
export function formatTaskOwnership(task: Parameters<typeof getTaskOwnershipDisplay>[0]): string {
  return getTaskOwnershipDisplay(task).primary
}
