// @vitest-environment jsdom
import { flushPromises, shallowMount } from '@vue/test-utils'
import { createPinia } from 'pinia'
import { describe, expect, it, vi } from 'vitest'

const mocks = vi.hoisted(() => ({
  previewSettlement: vi.fn().mockResolvedValue({ payroll_rows: [], totals: {} }),
  listSettlementBatches: vi.fn().mockResolvedValue({ items: [] }),
  listSettlementSupplements: vi.fn().mockResolvedValue({ items: [], total: 0 }),
  listSupplementPermissions: vi.fn().mockResolvedValue({ items: [] }),
  listDifficultyClasses: vi.fn().mockResolvedValue([]),
  listUploadDirectories: vi.fn().mockResolvedValue([]),
  listErrorImports: vi.fn().mockResolvedValue({ items: [] }),
  listSupplementEligibleMonths: vi.fn(),
  upsertSupplementPermission: vi.fn(),
}))
vi.mock('@aw/shared/api/assetWorkbenchApi', () => ({ assetWorkbenchApi: mocks }))
import SettlementPage from './SettlementPage.vue'

describe('SettlementPage supplement months', () => {
  it('uses backend eligible months for historical permission saves and handles no unlocked months', async () => {
    mocks.listSupplementEligibleMonths.mockResolvedValue(['2026-11', '2026-10'])
    mocks.upsertSupplementPermission.mockResolvedValue({ payee_user_id: 366, business_month: '2026-10', enabled: true })
    const wrapper = shallowMount(SettlementPage, { global: { plugins: [createPinia()] } })
    await flushPromises()
    await wrapper.findAll('button').find((button) => button.text().includes('补录管理'))!.trigger('click')
    const picker = wrapper.findAllComponents({ name: 'PersonnelPicker' })[0]!
    picker.vm.$emit('update:modelValue', 366)
    picker.vm.$emit('selected', { id: 366 })
    await flushPromises()
    const select = wrapper.get('select[aria-label="开放补录结算月"]')
    expect(wrapper.text()).not.toContain('补录工资统一计入当前自然月')
    expect(wrapper.text()).toContain('补录工资计入所选结算月')
    expect(select.findAll('option').map((option) => option.text())).toEqual(['2026-11', '2026-10'])
    await select.setValue('2026-10')
    await wrapper.findAll('button').find((button) => button.text() === '保存开放设置')!.trigger('click')
    await flushPromises()
    expect(mocks.upsertSupplementPermission).toHaveBeenCalledWith(expect.objectContaining({ payee_user_id: 366, business_month: '2026-10', enabled: true }))
    mocks.listSupplementEligibleMonths.mockResolvedValue([])
    await wrapper.findAll('button').find((button) => button.text() === '读取可开放月份')!.trigger('click')
    await flushPromises()
    expect(select.attributes('disabled')).toBeDefined()
    expect(wrapper.findAll('button').find((button) => button.text() === '保存开放设置')!.attributes('disabled')).toBeDefined()
    wrapper.unmount()
  })
})
