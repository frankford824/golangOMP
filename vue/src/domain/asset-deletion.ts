import {
  assetReplacementUnavailableReason,
  type AssetReplacementGateInput,
} from '@/domain/asset-replacement'

const COMPLETED_ASSET_DELETE_ROLES = new Set([
  'customizationreviewer',
  'audita',
  'auditb',
  'assetmanager',
])

function normalizeRole(value: unknown): string {
  return String(value ?? '').trim().toLowerCase().replace(/[^a-z0-9]/g, '')
}

function normalizedTaskStatus(value: unknown): string {
  return String(value ?? '').trim().toLowerCase().replace(/[^a-z0-9]/g, '')
}

export function assetDeletionUnavailableReason(
  input: AssetReplacementGateInput,
  roles: readonly string[],
): string {
  const resourceReason = assetReplacementUnavailableReason(input)
  if (resourceReason) {
    return resourceReason
      .split('修改资源').join('删除资源')
      .split('修改').join('删除')
      .split('替换').join('删除')
  }

  const normalizedRoles = new Set(roles.map(normalizeRole))
  if (normalizedRoles.has('superadmin')) return ''

  const hasMaintenanceRole = [...normalizedRoles].some((role) => COMPLETED_ASSET_DELETE_ROLES.has(role))
  if (!hasMaintenanceRole) return '当前账号没有删除资源的权限'
  if (normalizedTaskStatus(input.taskStatus) !== 'completed') {
    return '定制审核、常规审核和资产管理员只能删除已结单任务的当前资源'
  }
  return ''
}

export function canDeleteAssetResource(
  input: AssetReplacementGateInput,
  roles: readonly string[],
): boolean {
  return assetDeletionUnavailableReason(input, roles) === ''
}

export function assetDeletionSuccessMessage(taskStatus: unknown): string {
  return normalizedTaskStatus(taskStatus) === 'completed'
    ? '资源已删除，列表已刷新；所属任务状态未改变'
    : '资源已删除，列表已刷新'
}
