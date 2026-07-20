// @vitest-environment jsdom

import { describe, expect, it } from 'vitest'
import { router } from '@/router'

describe('data center route contract', () => {
  it('requires report.view and removes legacy tab redirects', () => {
    const routes = router.getRoutes()
    const dataCenter = routes.find((route) => route.name === 'DataCenter')
    expect(dataCenter?.meta.requiredPermissions).toEqual(['report.view'])

    for (const name of ['ExportCenter', 'LogsManagement', 'Kpi']) {
      const route = routes.find((candidate) => candidate.name === name)
      expect(route?.redirect).toEqual({ name: 'DataCenter' })
    }
  })
})
