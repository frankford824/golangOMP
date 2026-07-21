// @vitest-environment jsdom

import { describe, expect, it } from 'vitest'
import { router } from '@/router'

describe('active main-ops route contract', () => {
  it('keeps only current top-level business pages and removes retired page routes', () => {
    const routes = router.getRoutes()
    const dataCenter = routes.find((route) => route.name === 'DataCenter')
    expect(dataCenter?.meta.requiredPermissions).toEqual(['report.view'])
    expect(dataCenter?.meta.requiredMenuKey).toBe('report_center')

    const userManagement = routes.find((route) => route.name === 'UserManagement')
    expect(userManagement?.meta.requiredMenuKey).toBe('user_admin')

    for (const name of ['Dashboard', 'TaskList', 'AssetsIndex', 'CostRules', 'DataCenter', 'UserManagement']) {
      expect(routes.find((candidate) => candidate.name === name)).toBeDefined()
    }

    for (const name of [
      'ExportCenter',
      'LogsManagement',
      'Kpi',
      'Finance',
      'AuditLog',
      'RuleConfig',
      'ProductManagement',
      'ReportsHome',
      'AccessPolicy',
    ]) {
      expect(routes.find((candidate) => candidate.name === name)).toBeUndefined()
    }
  })
})
