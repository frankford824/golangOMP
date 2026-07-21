// @vitest-environment jsdom
import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'

const mocks = vi.hoisted(() => ({
  listSettlementSupplements: vi.fn(),
  batchDeleteSettlementSupplements: vi.fn(),
}))

vi.mock('@aw/shared/api/assetWorkbenchApi', () => ({ assetWorkbenchApi: mocks }))
vi.mock('@aw/shared/console/SettlementHubTabs.vue', () => ({ default: { template: '<nav />' } }))
vi.mock('@aw/shared/ui/PersonnelPicker.vue', () => ({ default: { template: '<label />' } }))
vi.mock('@aw/shared/preview/WorkbenchPreviewDialog.vue', () => ({ default: { template: '<div />' } }))

import SupplementQueryPage from './SupplementQueryPage.vue'

const rows = [
  {
    id: 701,
    payee_user_id: 1001,
    business_month: '2026-07',
    status: 'approved',
    order_no: 'admin-wrong-a.jpg',
    supplement_date: '2026-07-20',
    difficulty_class: 'A',
    finalized: false,
    page_count: 1,
    gross_amount: 1.14,
    files: [],
  },
  {
    id: 702,
    payee_user_id: 1002,
    business_month: '2026-07',
    status: 'draft',
    order_no: 'admin-wrong-b.jpg',
    supplement_date: '2026-07-20',
    difficulty_class: 'A',
    finalized: false,
    page_count: 2,
    gross_amount: 2.28,
    files: [],
  },
]

describe('SupplementQueryPage deletion placement', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    mocks.listSettlementSupplements.mockResolvedValue({ items: rows, total: rows.length })
    mocks.batchDeleteSettlementSupplements.mockResolvedValue({ deleted_ids: [701, 702], supplements: [] })
  })

  it('shows batch deletion in the toolbar above the list and confirms in a dialog', async () => {
    const wrapper = mount(SupplementQueryPage)
    await flushPromises()

    const toolbar = wrapper.get('.aw-supplement-delete-toolbar')
    const table = wrapper.get('.aw-supplement-table-wrap')
    expect(toolbar.element.compareDocumentPosition(table.element) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy()
    expect(wrapper.text()).not.toContain('删除选中的补录')

    await toolbar.get('input[type="checkbox"]').setValue(true)
    await toolbar.get('.aw-secondary-button--danger').trigger('click')

    const dialog = wrapper.get('[role="dialog"]')
    expect(dialog.text()).toContain('确认删除选中的补录')
    expect(dialog.text()).toContain('2 条')
    expect(dialog.text()).toContain('¥3.42')

    await dialog.get('input').setValue('管理员批量删除错传文件')
    await dialog.get('.aw-secondary-button--danger').trigger('click')
    await flushPromises()

    expect(mocks.batchDeleteSettlementSupplements).toHaveBeenCalledWith([701, 702], '管理员批量删除错传文件')
  })

  it('opens single deletion from the row header instead of a bottom row button', async () => {
    const wrapper = mount(SupplementQueryPage)
    await flushPromises()

    expect(wrapper.text()).not.toContain('删除此条')
    const button = wrapper.get('button[aria-label="删除补录 admin-wrong-a.jpg"]')
    expect(button.element.closest('.aw-supplement-table__action')).not.toBeNull()
    await button.trigger('click')
    expect(wrapper.get('[role="dialog"]').text()).toContain('确认删除这条补录')
  })
})
