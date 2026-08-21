// @vitest-environment jsdom
import { defineComponent, h } from 'vue'
import { flushPromises, shallowMount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it, vi } from 'vitest'

const listSettlementBatches = vi.hoisted(() => vi.fn())
const reverseSettlementBatchConfirmation = vi.hoisted(() => vi.fn())

vi.mock('@aw/shared/api/assetWorkbenchApi', async (loadOriginal) => {
  const original = await loadOriginal<typeof import('@aw/shared/api/assetWorkbenchApi')>()
  return {
    ...original,
    assetWorkbenchApi: {
      ...original.assetWorkbenchApi,
      previewSettlement: vi.fn().mockResolvedValue({
        business_month: '2026-08',
        payroll_rows: [],
        totals: { item_count: 0, gross_amount: 0, deduction_amount: 0, welfare_amount: 0, supplement_amount: 0, adjustment_amount: 0, net_amount: 0, error_count: 0 },
      }),
      listSettlementBatches,
      listSettlementSupplements: vi.fn().mockResolvedValue({ items: [], total: 0, page: 1, page_size: 20 }),
      listSupplementPermissions: vi.fn().mockResolvedValue({ items: [], total: 0, page: 1, page_size: 50 }),
      listDifficultyClasses: vi.fn().mockResolvedValue([]),
      listUploadDirectories: vi.fn().mockResolvedValue([]),
      listErrorImports: vi.fn().mockResolvedValue({ items: [], total: 0 }),
      reverseSettlementBatchConfirmation,
    },
  }
})

import { useAssetWorkbenchSessionStore } from '@aw/app/session.store'
import SettlementPage from './SettlementPage.vue'

const confirmedBatch = {
  id: 8801,
  batch_no: 'AWB2026071309523139',
  business_month: '2026-07',
  status: 'confirmed',
  item_count: 1,
  gross_amount: 35.42,
  deduction_amount: 0,
  welfare_amount: 0,
  supplement_amount: 0,
  adjustment_amount: 0,
  net_amount: 35.42,
  confirmed_at: '2026-07-13T09:52:31Z',
}

const WorkbenchDataGridStub = defineComponent({
  props: { rows: { type: Array, default: () => [] } },
  setup(props, { slots }) {
    return () => h('div', { class: 'grid-stub' }, (props.rows as Array<Record<string, unknown>>).map((row) =>
      h('div', { class: 'grid-row' }, slots.cell?.({ row, column: { key: 'actions' }, value: 'actions' })),
    ))
  },
})

function textButton(wrapper: ReturnType<typeof shallowMount>, text: string) {
  const found = wrapper.findAll('button').find((button) => button.text().trim() === text)
  if (!found) throw new Error(`missing button ${text}`)
  return found
}

describe('SettlementPage confirmed batch reversal', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    listSettlementBatches.mockResolvedValue({ items: [confirmedBatch], total: 1, page: 1, page_size: 20 })
    reverseSettlementBatchConfirmation.mockResolvedValue({ batch_id: 8801, status: 'cancelled' })
  })

  it('requires reason, exact batch number, and unpaid attestation before reversing', async () => {
    const pinia = createPinia()
    setActivePinia(pinia)
    useAssetWorkbenchSessionStore().setBootstrap({
      app: 'asset_workbench', version: 'v1', timezone: 'Asia/Shanghai', oss_prefix: '', upload_session_ttl_seconds: 3600,
      is_admin: true,
      access: { membership_status: 'active', is_enabled: true, is_admin_shell: true, asset_roles: ['SuperAdmin'], role_labels: [], capabilities: ['asset.workbench.settlement'] },
      capabilities: ['asset.workbench.settlement'], settlement_item_types: [], deferred_business_items: [], architecture_guardrails: [],
    })
    const wrapper = shallowMount(SettlementPage, {
      global: {
        plugins: [pinia],
        stubs: {
          WorkbenchDataGrid: WorkbenchDataGridStub,
          SettlementHubTabs: true,
          SpreadsheetWorkbench: true,
          LedgerReadout: true,
          AsyncBoundary: true,
          PersonnelPicker: true,
        },
      },
    })
    await flushPromises()

    const batchesNav = wrapper.findAll('button').find((button) => button.text().includes('批次确认'))
    if (!batchesNav) throw new Error('missing batches navigation')
    await batchesNav.trigger('click')
    await flushPromises()
    await textButton(wrapper, '撤销确认').trigger('click')

    const dialog = wrapper.get('[role="dialog"][aria-label="撤销已确认批次"]')
    expect(dialog.text()).toContain('若工资已经实际发放，请勿撤销')
    await dialog.get('input[aria-label="撤销确认原因"]').setValue('误确认，尚未发放')
    await dialog.get('input[aria-label="确认撤销批次号"]').setValue(confirmedBatch.batch_no)
    await dialog.get('input[type="checkbox"]').setValue(true)
    await textButton(wrapper, '确认撤销').trigger('click')
    await flushPromises()

    expect(reverseSettlementBatchConfirmation).toHaveBeenCalledWith(8801, {
      reason: '误确认，尚未发放',
      expected_batch_no: confirmedBatch.batch_no,
      confirm_unpaid: true,
    })
    expect(wrapper.text()).toContain(`已撤销确认：${confirmedBatch.batch_no}`)
    expect(wrapper.find('[role="dialog"][aria-label="撤销已确认批次"]').exists()).toBe(false)
  })
})
