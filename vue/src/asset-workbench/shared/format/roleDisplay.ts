const roleLabels: Record<string, string> = {
  AssetSubmitter: '交付人员',
  AssetManager: '作品管理',
  AssetTemplateAdmin: '类型与计价',
  AssetSettlement: '结算财务',
  SuperAdmin: '超级管理员',
  HRAdmin: '人员资料',
}

export const managedAssetRoles = [
  'AssetSubmitter',
  'AssetManager',
  'AssetTemplateAdmin',
  'AssetSettlement',
] as const

export function roleDisplayName(role: string) {
  return roleLabels[role] ?? role
}

export function roleDisplayList(roles: string[] = []) {
  return roles.map(roleDisplayName).filter(Boolean)
}
