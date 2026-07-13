import { describe, expect, it } from 'vitest'

import { filtersForLocatedMaterial, materialBrowseRootForSource, reconcileMaterialFilterState } from './materialFilterState'

describe('material filter state', () => {
  it('clears system-only classification when switching to all or external sources', () => {
    expect(reconcileMaterialFilterState({ source: 'all', businessLane: 'customization', format: 'all' }, 'source')).toEqual({
      source: 'all',
      businessLane: 'all',
      format: 'all',
    })
    expect(reconcileMaterialFilterState({ source: 'external', businessLane: 'normal', format: 'image' }, 'source')).toEqual({
      source: 'external',
      businessLane: 'all',
      format: 'image',
    })
  })

  it('selects system source when a system classification is chosen', () => {
    expect(reconcileMaterialFilterState({ source: 'all', businessLane: 'normal', format: 'design' }, 'business_lane')).toEqual({
      source: 'system',
      businessLane: 'normal',
      format: 'design',
    })
  })

  it('opens external browsing at the shared root so p3 and quark remain visible', () => {
    expect(materialBrowseRootForSource('external')).toBe('')
    expect(materialBrowseRootForSource('all')).toBe('')
    expect(materialBrowseRootForSource('system')).toBe('/系统资源')
  })

  it('aligns all filters to a located customization PSD', () => {
    expect(filtersForLocatedMaterial('system', 'customization', 'design')).toEqual({
      source: 'system',
      businessLane: 'customization',
      format: 'design',
    })
  })
})
