// @vitest-environment jsdom
import { defineComponent, h } from 'vue'
import { flushPromises, shallowMount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it, vi } from 'vitest'

const listErrorImports = vi.hoisted(() => vi.fn())
const getErrorImportDetail = vi.hoisted(() => vi.fn())
const deleteErrorImport = vi.hoisted(() => vi.fn())

const batchOne = {
  id: 101,
  import_no: 'QCI-101',
  business_month: '2026-08',
  uploaded_by: 1,
  original_filename: '质检第一批.xlsx',
  status: 'imported',
  total_rows: 2,
  matched_rows: 2,
  unmatched_rows: 0,
  ambiguous_rows: 0,
  error_count: 3,
  deduction_amount: 15,
  created_at: '2026-08-20T10:00:00Z',
}

const batchTwo = {
  ...batchOne,
  id: 102,
  import_no: 'QCI-102',
  original_filename: '质检第二批.xlsx',
  error_count: 2,
  deduction_amount: 11,
  created_at: '2026-08-20T11:00:00Z',
}

vi.mock('@aw/shared/api/assetWorkbenchApi', async (loadOriginal) => {
  const original = await loadOriginal<typeof import('@aw/shared/api/assetWorkbenchApi')>()
  return {
    ...original,
    assetWorkbenchApi: {
      ...original.assetWorkbenchApi,
      previewSettlement: vi.fn().mockResolvedValue({
        business_month: '2026-08', payroll_rows: [],
        totals: { item_count: 0, gross_amount: 0, deduction_amount: 26, welfare_amount: 0, supplement_amount: 0, adjustment_amount: 0, net_amount: -26, error_count: 5 },
      }),
      listSettlementBatches: vi.fn().mockResolvedValue({ items: [], total: 0 }),
      listSettlementSupplements: vi.fn().mockResolvedValue({ items: [], total: 0 }),
      listSupplementPermissions: vi.fn().mockResolvedValue({ items: [], total: 0 }),
      listDifficultyClasses: vi.fn().mockResolvedValue([]),
      listUploadDirectories: vi.fn().mockResolvedValue([]),
      listErrorImports,
      getErrorImportDetail,
      deleteErrorImport,
    },
  }
})

import SettlementPage from './SettlementPage.vue'

const WorkbenchDataGridStub = defineComponent({
  props: {
    rows: { type: Array, default: () => [] },
    columns: { type: Array, default: () => [] },
  },
  setup(props, { slots }) {
    return () => h('div', { class: 'grid-stub' }, (props.rows as Array<Record<string, unknown>>).map((row) =>
      h('div', { class: 'grid-row' }, (props.columns as Array<{ key: string }>).map((column) =>
        h('span', { class: `cell-${column.key}` }, slots.cell?.({ row, column, value: row[column.key] }) ?? String(row[column.key] ?? '')),
      )),
    ))
  },
})

const AsyncBoundaryStub = defineComponent({
  setup(_, { slots }) {
    return () => h('div', slots.default?.())
  },
})

function textButton(wrapper: ReturnType<typeof shallowMount>, text: string) {
  const found = wrapper.findAll('button').find((button) => button.text().trim() === text)
  if (!found) throw new Error(`missing button ${text}`)
  return found
}

describe('SettlementPage quality error import history', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    listErrorImports.mockResolvedValue({ items: [batchOne, batchTwo], total: 2 })
    getErrorImportDetail.mockResolvedValue({
      ...batchOne,
      records: [{
        id: 901,
        import_batch_id: 101,
        business_month: '2026-08',
        payee_user_id: 302,
        payee_name: '张三',
        order_no: 'RW-20260820-A-001',
        difficulty_class: 'A',
        occurred_date: '2026-08-19',
        error_count: 3,
        deduction_amount: 15,
        raw_payload_json: { 问题描述: '文字位置错误' },
        match_status: 'matched',
        created_at: '2026-08-20T10:00:00Z',
      }],
    })
    deleteErrorImport.mockResolvedValue(batchOne)
  })

  function mountPage() {
    const pinia = createPinia()
    setActivePinia(pinia)
    return shallowMount(SettlementPage, {
      global: {
        plugins: [pinia],
        stubs: {
          WorkbenchDataGrid: WorkbenchDataGridStub,
          AsyncBoundary: AsyncBoundaryStub,
          SettlementHubTabs: true,
          SpreadsheetWorkbench: true,
          LedgerReadout: true,
          PersonnelPicker: true,
        },
      },
    })
  }

  it('shows each imported workbook and its row-level detail independently', async () => {
    const wrapper = mountPage()
    await flushPromises()

    expect(wrapper.text()).toContain('质检第一批.xlsx')
    expect(wrapper.text()).toContain('质检第二批.xlsx')
    expect(wrapper.text()).toContain('已导入')
    expect(wrapper.text()).toContain('¥15.00')
    expect(wrapper.text()).toContain('¥11.00')

    await textButton(wrapper, '明细').trigger('click')
    await flushPromises()

    expect(getErrorImportDetail).toHaveBeenCalledWith(101)
    const detail = wrapper.get('[role="dialog"][aria-label="质检导入明细"]')
    expect(detail.text()).toContain('张三')
    expect(detail.text()).toContain('文字位置错误')
    expect(detail.text()).toContain('RW-20260820-A-001')
  })

  it('requires a reason and reloads totals after deleting one import', async () => {
    const wrapper = mountPage()
    await flushPromises()

    await textButton(wrapper, '删除').trigger('click')
    const dialog = wrapper.get('[role="dialog"][aria-label="删除质检导入"]')
    await dialog.get('input').setValue('重复导入，撤销第一批')
    await textButton(wrapper, '确认删除').trigger('click')
    await flushPromises()

    expect(deleteErrorImport).toHaveBeenCalledWith(101, '重复导入，撤销第一批')
    expect(listErrorImports).toHaveBeenCalledTimes(2)
    expect(wrapper.text()).toContain('本月工资预览已重新计算')
  })
})
