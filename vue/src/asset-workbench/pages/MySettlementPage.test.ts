// @vitest-environment jsdom
import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'

const mocks = vi.hoisted(() => ({
  mySettlement: vi.fn(),
  listUploadDirectories: vi.fn(),
  batchDeleteSettlementSupplements: vi.fn(),
}))

vi.mock('@aw/shared/api/assetWorkbenchApi', () => ({
  assetWorkbenchApi: mocks,
}))

vi.mock('@aw/shared/preview/WorkbenchPreviewDialog.vue', () => ({
  default: { template: '<div />' },
}))

import MySettlementPage from './MySettlementPage.vue'

const supplements = [
  {
    id: 601,
    payee_user_id: 1001,
    business_month: '2026-07',
    status: 'approved',
    order_no: 'wrong-a.jpg',
    supplement_date: '2026-07-20',
    difficulty_class: 'A',
    finalized: false,
    page_count: 1,
    gross_amount: 1.14,
    files: [],
  },
  {
    id: 602,
    payee_user_id: 1001,
    business_month: '2026-07',
    status: 'draft',
    order_no: 'wrong-b.jpg',
    supplement_date: '2026-07-20',
    difficulty_class: 'A',
    finalized: false,
    page_count: 1,
    gross_amount: 2.28,
    files: [],
  },
]

describe('MySettlementPage supplement deletion', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    mocks.mySettlement.mockResolvedValue({
      estimated_net_amount: 3.42,
      months: [],
      supplement_permission: null,
      supplements,
    })
    mocks.listUploadDirectories.mockResolvedValue([])
    mocks.batchDeleteSettlementSupplements.mockResolvedValue({ deleted_ids: [601], supplements: [] })
  })

  it('opens single-record deletion from the row header and confirms without a bottom action panel', async () => {
    const wrapper = mount(MySettlementPage)
    await flushPromises()

    expect(wrapper.text()).not.toContain('删除此条补录')
    const deleteButton = wrapper.get('button[aria-label="删除补录 wrong-a.jpg"]')
    expect(deleteButton.element.closest('.aw-supplement-row__summary')).not.toBeNull()

    await deleteButton.trigger('click')
    const dialog = wrapper.get('[role="dialog"]')
    expect(dialog.text()).toContain('确认删除这条补录')
    expect(dialog.text()).toContain('¥1.14')

    await dialog.get('input').setValue('上传错文件')
    await dialog.get('.aw-secondary-button--danger').trigger('click')
    await flushPromises()

    expect(mocks.batchDeleteSettlementSupplements).toHaveBeenCalledWith([601], '上传错文件')
    expect(wrapper.find('[role="dialog"]').exists()).toBe(false)
  })

  it('keeps the batch delete action in the toolbar above the supplement list', async () => {
    mocks.batchDeleteSettlementSupplements.mockResolvedValue({ deleted_ids: [601, 602], supplements: [] })
    const wrapper = mount(MySettlementPage)
    await flushPromises()

    const toolbar = wrapper.get('.aw-supplement-delete-toolbar')
    const list = wrapper.get('.aw-simple-income__list')
    expect(toolbar.element.compareDocumentPosition(list.element) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy()

    await toolbar.get('input[type="checkbox"]').setValue(true)
    await toolbar.get('.aw-secondary-button--danger').trigger('click')
    const dialog = wrapper.get('[role="dialog"]')
    expect(dialog.text()).toContain('确认删除选中的补录')
    expect(dialog.text()).toContain('2 条')
    expect(dialog.text()).toContain('¥3.42')

    await dialog.get('input').setValue('批量误传')
    await dialog.get('.aw-secondary-button--danger').trigger('click')
    await flushPromises()

    expect(mocks.batchDeleteSettlementSupplements).toHaveBeenCalledWith([601, 602], '批量误传')
  })
})
