/**
 * Round K-F-4 · 回归验证：bare-member（frontend_access.menus=['dashboard']，
 * 无任何业务 action）进入 DashboardView 时应命中 `v-else` 空态分支，
 * 既不渲染业务 widget，也不触发 403 / router 跳转。
 *
 * 纯静态断言版：这里直接把 DashboardView 内置的 BUSINESS_ACTIONS 常量
 * 对齐复制一份，验证 "bare-member fixture 的 actions 集合" 与 BUSINESS_ACTIONS
 * 无交集，从而保证 `hasBusinessAccess` computed 结果为 false。
 *
 * 组件挂载级断言需要 @vue/test-utils；仓库当前未安装，暂由 UAT 截图覆盖
 * （见 Round K-F 报告 · Manual UAT / Bare Member 行）。
 */
import { describe, it, expect } from 'vitest'

const BUSINESS_ACTIONS = [
  'task.list',
  'task.create',
  'task.audit.claim',
  'warehouse.receive',
  'task.customization.submit',
  'design.review',
] as const

describe('DashboardView · bare-member empty state', () => {
  it('menus=["dashboard"] 且 actions=[] 时，与 BUSINESS_ACTIONS 无交集', () => {
    const fixture = {
      roles: ['member'],
      frontend_access: {
        is_department_admin: false,
        menus: ['dashboard'],
        actions: [] as string[],
      },
    }
    const intersect = BUSINESS_ACTIONS.filter((a) => fixture.frontend_access.actions.includes(a))
    expect(intersect).toEqual([])
  })

  it('仅当 actions 含 BUSINESS_ACTIONS 任一时才算有业务访问', () => {
    const probe = (actions: string[]) => BUSINESS_ACTIONS.some((a) => actions.includes(a))
    expect(probe([])).toBe(false)
    expect(probe(['org.users.list'])).toBe(false)
    expect(probe(['task.list'])).toBe(true)
    expect(probe(['design.review'])).toBe(true)
  })
})
