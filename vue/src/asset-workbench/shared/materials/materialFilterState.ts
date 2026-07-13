import type { MaterialBusinessLane, MaterialFormatCategory, MaterialSourceFilter } from '@aw/shared/api/assetWorkbenchApi'

export type MaterialFilterDimension = 'source' | 'business_lane' | 'format'

export interface MaterialFilterState {
  source: MaterialSourceFilter
  businessLane: MaterialBusinessLane
  format: MaterialFormatCategory
}

export function reconcileMaterialFilterState(state: MaterialFilterState, changed: MaterialFilterDimension): MaterialFilterState {
  const next = { ...state }
  if (changed === 'business_lane' && next.businessLane !== 'all') {
    next.source = 'system'
  }
  if (changed === 'source' && next.source !== 'system') {
    next.businessLane = 'all'
  }
  return next
}

export function materialBrowseRootForSource(source: MaterialSourceFilter): string {
  return source === 'system' ? '/系统资源' : ''
}

export function filtersForLocatedMaterial(
  source: MaterialSourceFilter,
  businessLane: MaterialBusinessLane | 'all',
  format: MaterialFormatCategory | 'other',
): MaterialFilterState {
  return {
    source,
    businessLane: source === 'system' && businessLane !== 'all' ? businessLane : 'all',
    format: format === 'other' ? 'all' : format,
  }
}
