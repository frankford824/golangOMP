// @vitest-environment jsdom
import { flushPromises, mount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

const mocks = vi.hoisted(() => ({
  route: { query: { intent: 'planning_sku' } as Record<string, string> },
  push: vi.fn(), replace: vi.fn(),
  create: vi.fn(), addTask: vi.fn(), getTaskById: vi.fn(), parseBatch: vi.fn(), getProductByCode: vi.fn(), getIids: vi.fn(), getDraft: vi.fn(),
  permissions: new Set(['task.create', 'planning_sku.create']),
  uploadReferenceFileRef: vi.fn(),
  uploadRetouchRequirementPendingAssets: vi.fn(), downloadPlanning: vi.fn(),
}))

vi.mock('vue-router', () => ({
  useRoute: () => mocks.route,
  useRouter: () => ({ push: mocks.push, replace: mocks.replace }),
  onBeforeRouteLeave: vi.fn(),
}))
vi.mock('@/composables/useTaskDraft', () => ({
  useTaskDraft: () => ({ save: vi.fn(), update: vi.fn(), getById: mocks.getDraft, saving: false }),
}))
vi.mock('@/stores/tasks', () => ({ useTasksStore: () => ({ addTask: mocks.addTask, getById: mocks.getTaskById }) }))
vi.mock('@/services/api/planningSkuApi', () => ({
  planningSkuApi: {
    create: mocks.create,
    parseExcel: vi.fn(), uploadImage: vi.fn(), retryFailedERP: vi.fn(), exportSelection: vi.fn(),
    downloadTask: mocks.downloadPlanning,
  },
}))
vi.mock('@/services/api/batchSkuApi', () => ({
  batchSkuApi: { parseExcel: mocks.parseBatch },
  normalizeBatchPreviewRow: (row: unknown) => row,
  formatBatchViolationMessage: (issue: { message?: string; code?: string }) => issue.message || issue.code || '请检查这一行',
}))
vi.mock('@/services/api/erpApi', () => ({ erpApi: { getProductByCode: mocks.getProductByCode, getIids: mocks.getIids } }))
vi.mock('@/services/upload/assetUploadFlow', () => ({ uploadReferenceFileRef: mocks.uploadReferenceFileRef }))
vi.mock('@/services/upload/retouchRequirementUpload', () => ({ uploadRetouchRequirementPendingAssets: mocks.uploadRetouchRequirementPendingAssets }))
vi.mock('@/composables/usePermission', () => ({ usePermission: () => ({ can: (permission: string) => mocks.permissions.has(permission) }) }))

import UnifiedTaskCreateView from './UnifiedTaskCreateView.vue'

describe('UnifiedTaskCreateView', () => {
  afterEach(() => {
    vi.unstubAllGlobals()
  })

  beforeEach(() => {
    vi.clearAllMocks()
    mocks.permissions = new Set(['task.create', 'planning_sku.create'])
    mocks.route.query = { intent: 'planning_sku' }
    mocks.create.mockResolvedValue({
      task_id: 88,
      task_no: 'RW-088',
      items: [{ task_sku_item_id: 1, sequence_no: 1, sku_code: 'CGH000021', erp_status: 'not_filed' }],
    })
    mocks.getIids.mockResolvedValue({ data: { data: [{ i_id: 'KT_STANDARD' }] } })
    mocks.uploadReferenceFileRef.mockResolvedValue({ asset_id: 'reference-default' })
    mocks.addTask.mockResolvedValue({ id: 'task-default', retouchRequirements: [] })
    mocks.getTaskById.mockReturnValue({ id: 'task-default', retouchRequirements: [] })
    mocks.uploadRetouchRequirementPendingAssets.mockResolvedValue({ failures: [], referenceUploaded: 0, sourceUploaded: 0 })
    mocks.downloadPlanning.mockResolvedValue(undefined)
  })

  it('renders the by-code ERP snapshot and clears an earlier lookup error', async () => {
    mocks.route.query = { intent: 'modify_existing' }
    mocks.getProductByCode
      .mockRejectedValueOnce(new Error('服务暂时不可用，请稍后重试'))
      .mockResolvedValueOnce({
        data: {
          data: {
            code: 'CGH000018',
            product_name: '露陈铝膜气球',
            snapshot: {
              product_id: 'CGH000018',
              sku_code: 'CGH000018',
              product_name: '露陈铝膜气球',
            },
          },
        },
      })
    const wrapper = mount(UnifiedTaskCreateView, {
      global: { stubs: { UnifiedTaskGrid: true, IIdSelector: true, RouterLink: true } },
    })
    await wrapper.get('.erp-search input').setValue('CGH000018')
    await wrapper.get('.erp-search button').trigger('click')
    await flushPromises()
    expect(wrapper.get('[role="alert"]').text()).toContain('服务暂时不可用')

    await wrapper.get('.erp-search button').trigger('click')
    await flushPromises()

    expect(wrapper.find('[role="alert"]').exists()).toBe(false)
    expect(wrapper.get('.erp-result').text()).toContain('露陈铝膜气球')
    expect(wrapper.get('.erp-result').text()).toContain('CGH000018')
    wrapper.unmount()
  })

  it('renders the create workbench when crypto.randomUUID is unavailable on HTTP', () => {
    vi.stubGlobal('crypto', {})
    const wrapper = mount(UnifiedTaskCreateView, {
      global: { stubs: { UnifiedTaskGrid: true, IIdSelector: true, RouterLink: true } },
    })

    expect(wrapper.get('.compose-page').attributes('data-compose-intent')).toBe('planning_sku')
    expect(wrapper.text()).toContain('创建任务')
    wrapper.unmount()
  })

  it('uses the planning redirect intent, validates rows, and renders the executable result flow', async () => {
    const wrapper = mount(UnifiedTaskCreateView, {
      global: {
        stubs: {
          UnifiedTaskGrid: { template: '<div class="grid-stub" />', methods: { readRowsFromWorkbook() {} } },
          IIdSelector: { template: '<div class="iid-stub" />' },
          RouterLink: { template: '<a><slot /></a>' },
        },
      },
    })
    expect(wrapper.get('.compose-page').attributes('data-compose-intent')).toBe('planning_sku')
    expect(wrapper.text()).toContain('只要 SKU 编码明细')
    expect(wrapper.text()).toContain('还有 3 处需要完善')

    await wrapper.get('[data-row-index="0"] input[type="text"]').setValue('HZS')
    await wrapper.get('[data-row-index="0"] textarea').setValue('亚克力立牌 20cm')
    await wrapper.get('[data-row-index="0"] input[type="number"]').setValue('2')
    await flushPromises()
    const submit = wrapper.get('.validation-dock .primary-button')
    expect(submit.attributes('disabled')).toBeUndefined()
    await submit.trigger('click')
    await flushPromises()

    expect(mocks.create).toHaveBeenCalledWith([
      expect.objectContaining({ category_code: 'HZS', sku_code_type: 'regular', description_spec: '亚克力立牌 20cm', quantity: 2 }),
    ], 'none', expect.any(String))
    expect(wrapper.text()).toContain('任务 RW-088 已结单')
    expect(wrapper.text()).toContain('CGH000021')
    expect(wrapper.text()).toContain('以下编号已正式占用')
    await wrapper.findAll('button').find((button) => button.text() === '导出全部')?.trigger('click')
    await flushPromises()
    expect(mocks.downloadPlanning).toHaveBeenCalledWith(88)
    wrapper.unmount()
  })

  it('states ERP sync behavior explicitly and exposes per-row read-only cost preview', async () => {
    const wrapper = mount(UnifiedTaskCreateView, {
      global: {
        stubs: {
          UnifiedTaskGrid: { template: '<div class="grid-stub" />' },
          IIdSelector: true,
          RouterLink: true,
          CostExplanationPanel: {
            props: ['title', 'seed', 'open'],
            template: '<div class="cost-preview-stub">{{ title }} · {{ seed.categoryCode }} · {{ seed.quantity }}</div>',
          },
        },
      },
    })

    expect(wrapper.text()).toContain('创建后自动同步 ERP')
    expect(wrapper.text()).toContain('未开启：本次只创建任务与 SKU')
    const sync = wrapper.get('input[aria-label="创建成功后自动同步 ERP"]')
    await sync.setValue(true)
    expect(wrapper.text()).toContain('已开启：创建成功后自动同步')

    await wrapper.get('[data-row-index="0"] input[type="text"]').setValue('HZS')
    await wrapper.get('[data-row-index="0"] textarea').setValue('亚克力立牌 20cm')
    await wrapper.get('[data-row-index="0"] input[type="number"]').setValue('2')
    await flushPromises()
    expect(wrapper.get('.cost-preview-stub').text()).toContain('第 1 行 SKU 预估成本')
    expect(wrapper.get('.cost-preview-stub').text()).toContain('HZS')
    expect(wrapper.get('.cost-preview-stub').text()).toContain('2')
    wrapper.unmount()
  })

  it('surfaces a planning rule configuration error without leaving the workbench', async () => {
    mocks.create.mockRejectedValueOnce(new Error('唯一启用的策划 SKU 编号规则尚未配置'))
    const wrapper = mount(UnifiedTaskCreateView, {
      global: {
        stubs: {
          UnifiedTaskGrid: { template: '<div class="grid-stub" />', methods: { readRowsFromWorkbook() {} } },
          IIdSelector: { template: '<div class="iid-stub" />' },
          RouterLink: true,
        },
      },
    })
    await wrapper.get('[data-row-index="0"] input[type="text"]').setValue('HZS')
    await wrapper.get('[data-row-index="0"] textarea').setValue('亚克力立牌 20cm')
    await wrapper.get('[data-row-index="0"] input[type="number"]').setValue('1')
    await wrapper.get('.validation-dock .primary-button').trigger('click')
    await flushPromises()

    expect(wrapper.get('[role="alert"]').text()).toContain('唯一启用的策划 SKU 编号规则尚未配置')
    expect(wrapper.get('.compose-page').attributes('data-compose-intent')).toBe('planning_sku')
    expect(wrapper.text()).not.toContain('创建成功')
    wrapper.unmount()
  })

  it('keeps the legacy planning URL as one workbench intent instead of a second page', () => {
    const wrapper = mount(UnifiedTaskCreateView, {
      global: { stubs: { UnifiedTaskGrid: true, IIdSelector: true, RouterLink: true } },
    })
    expect(wrapper.find('.planning-page').exists()).toBe(false)
    expect(wrapper.findAll('.intent-card')).toHaveLength(4)
    expect(wrapper.get('.intent-card[aria-pressed="true"]').text()).toContain('只要 SKU 编码')
  })

  it('shows only planning to a planning-only creator', () => {
    mocks.permissions = new Set(['planning_sku.create'])
    const wrapper = mount(UnifiedTaskCreateView, {
      global: { stubs: { UnifiedTaskGrid: true, IIdSelector: true, RouterLink: true } },
    })
    expect(wrapper.findAll('.intent-card')).toHaveLength(1)
    expect(wrapper.get('.intent-card').text()).toContain('只要 SKU 编码')
    wrapper.unmount()
  })

  it('does not expose planning to a task-only creator', () => {
    mocks.permissions = new Set(['task.create'])
    mocks.route.query = { intent: 'planning_sku' }
    const wrapper = mount(UnifiedTaskCreateView, {
      global: { stubs: { UnifiedTaskGrid: true, IIdSelector: true, RouterLink: true } },
    })
    expect(wrapper.findAll('.intent-card')).toHaveLength(3)
    expect(wrapper.text()).not.toContain('只要 SKU 编码')
    expect(wrapper.get('.compose-page').attributes('data-compose-intent')).toBe('modify_existing')
    wrapper.unmount()
  })

  it('imports the existing new-design workbook into the same Univer row model', async () => {
    mocks.route.query = { intent: 'new_design' }
    mocks.parseBatch.mockResolvedValue({
      data: { data: { preview: [{ source_row: 2, product_name: '导入款', design_requirement: '白底排版', product_i_id: 'KT_STANDARD', variant_json: { width: 1.2 } }], violations: [] } },
    })
    const wrapper = mount(UnifiedTaskCreateView, {
      global: { stubs: { UnifiedTaskGrid: true, IIdSelector: true, RouterLink: true } },
    })
    const input = wrapper.get('.file-button input').element as HTMLInputElement
    Object.defineProperty(input, 'files', { configurable: true, value: [new File(['xlsx'], 'new-design.xlsx')] })
    await wrapper.get('.file-button input').trigger('change')
    await flushPromises()

    expect(mocks.parseBatch).toHaveBeenCalledWith(expect.any(File))
    expect(wrapper.get('.compose-page').attributes('data-row-count')).toBe('1')
    expect(wrapper.text()).toContain('导入款')
  })

  it('projects an i_id selected in the row drawer back into the grid model', async () => {
    mocks.route.query = { intent: 'new_design' }
    const wrapper = mount(UnifiedTaskCreateView, {
      global: {
        stubs: {
          UnifiedTaskGrid: {
            props: ['rows', 'revision'],
            template: '<div class="grid-stub" :data-revision="revision">{{ rows[0].product_i_id }}</div>',
          },
          IIdSelector: {
            emits: ['update:modelValue'],
            template: '<button class="iid-select-stub" @click="$emit(\'update:modelValue\', \'A4_PRINT\')">选择款式</button>',
          },
          RouterLink: true,
        },
      },
    })

    expect(wrapper.get('.grid-stub').text()).toBe('')
    expect(wrapper.get('.grid-stub').attributes('data-revision')).toBe('0')
    await wrapper.get('.iid-select-stub').trigger('click')
    await flushPromises()

    expect(wrapper.get('.grid-stub').text()).toBe('A4_PRINT')
    expect(wrapper.get('.grid-stub').attributes('data-revision')).toBe('1')
    wrapper.unmount()
  })

  it('batch resolves ERP bindings pasted into multiple rows and projects product names into the grid', async () => {
    mocks.route.query = { intent: 'modify_existing' }
    mocks.getProductByCode.mockImplementation(async (code: string) => ({
      data: {
        data: {
          code,
          product_name: `商品-${code}`,
          snapshot: { product_id: `product-${code}`, sku_code: code },
        },
      },
    }))
    const wrapper = mount(UnifiedTaskCreateView, {
      global: {
        stubs: {
          UnifiedTaskGrid: {
            props: ['rows', 'revision'],
            emits: ['update:rows'],
            template: `
              <div class="grid-stub">
                <button class="paste-two" @click="$emit('update:rows', rows.map((row, index) => ({ ...row, erp_sku: 'SKU-00' + (index + 1), product_name: '', design_requirement: '换图' })))">粘贴</button>
                <span class="grid-values">{{ rows.map((row) => row.product_name).join('|') }}</span>
              </div>
            `,
          },
          IIdSelector: true,
          RouterLink: true,
        },
      },
    })
    const addButton = wrapper.findAll('button').find((button) => button.text().includes('添加一行'))
    await addButton?.trigger('click')
    await wrapper.get('.paste-two').trigger('click')
    await wrapper.findAll('button').find((button) => button.text().includes('批量查询已填 SKU'))?.trigger('click')
    await flushPromises()

    expect(mocks.getProductByCode).toHaveBeenCalledTimes(2)
    expect(wrapper.get('.grid-values').text()).toBe('商品-SKU-001|商品-SKU-002')
    expect(wrapper.get('[role="status"]').text()).toContain('已匹配并回填 2 行')
    wrapper.unmount()
  })

  it('deletes a multi-row Univer selection in one operation and keeps one editable row', async () => {
    const wrapper = mount(UnifiedTaskCreateView, {
      global: {
        stubs: {
          UnifiedTaskGrid: {
            props: ['rows'],
            emits: ['selection'],
            template: '<button class="select-two-rows" @click="$emit(\'selection\', rows.slice(0, 2).map((row) => row.id))">选择两行</button>',
          },
          IIdSelector: true,
          RouterLink: true,
        },
      },
    })
    const addButton = wrapper.findAll('button').find((button) => button.text().includes('添加一行'))
    await addButton?.trigger('click')
    await addButton?.trigger('click')
    expect(wrapper.get('.compose-page').attributes('data-row-count')).toBe('3')

    await wrapper.get('.select-two-rows').trigger('click')
    const deleteButton = wrapper.findAll('button').find((button) => button.text().includes('删除选中行'))
    expect(deleteButton?.text()).toContain('（2）')
    await deleteButton?.trigger('click')

    expect(wrapper.get('.compose-page').attributes('data-row-count')).toBe('1')
    expect(deleteButton?.attributes('disabled')).toBeDefined()
    wrapper.unmount()
  })

  it('retries failed retouch attachments without creating a duplicate task', async () => {
    mocks.route.query = { intent: 'retouch' }
    const created = {
      id: 'task-retouch',
      retouchRequirements: [{ id: 41, taskId: 1, description: '清理背景', sortOrder: 1 }],
    }
    mocks.addTask.mockResolvedValue(created)
    mocks.getTaskById.mockReturnValue(created)
    mocks.uploadRetouchRequirementPendingAssets
      .mockResolvedValueOnce({
        failures: [{ requirementIndex: 0, kind: 'source', fileName: 'source.psd', message: '素材上传失败' }],
        referenceUploaded: 0,
        sourceUploaded: 0,
      })
      .mockResolvedValueOnce({ failures: [], referenceUploaded: 0, sourceUploaded: 1 })
    const wrapper = mount(UnifiedTaskCreateView, {
      global: {
        stubs: {
          UnifiedTaskGrid: { template: '<div />', methods: { readRowsFromWorkbook() {} } },
          IIdSelector: true,
          RouterLink: { template: '<a><slot /></a>' },
        },
      },
    })
    await wrapper.get('[data-row-index="0"] textarea').setValue('清理背景')
    await wrapper.get('[data-row-index="0"] .asset-button').trigger('click')
    const input = wrapper.get('input[aria-label="上传待修素材文件"]')
    Object.defineProperty(input.element, 'files', {
      configurable: true,
      value: [new File(['psd'], 'source.psd', { type: 'application/octet-stream' })],
    })
    await input.trigger('change')
    await flushPromises()
    await wrapper.get('.validation-dock .primary-button').trigger('click')
    await flushPromises()

    expect(wrapper.text()).toContain('任务已创建，部分附件上传失败')
    expect(wrapper.text()).toContain('只重试失败附件')
    await wrapper.get('.retry-failed').trigger('click')
    await flushPromises()

    expect(mocks.addTask).toHaveBeenCalledTimes(1)
    expect(mocks.uploadRetouchRequirementPendingAssets).toHaveBeenCalledTimes(2)
    expect(wrapper.text()).toContain('任务创建完成')
    wrapper.unmount()
  })

  it('clears common and transient fields after confirming an intent switch', async () => {
    const wrapper = mount(UnifiedTaskCreateView, {
      attachTo: document.body,
      global: { stubs: { UnifiedTaskGrid: true, IIdSelector: true, RouterLink: true } },
    })
    await wrapper.get('.common-ribbon select').setValue('critical')
    await wrapper.get('.note-field input').setValue('不应带到下一个流程')
    const retouch = wrapper.findAll('.intent-card').find((item) => item.text().includes('只修图'))
    if (!retouch) throw new Error('missing retouch intent')
    await retouch.trigger('click')
    await flushPromises()
    const confirm = Array.from(document.querySelectorAll<HTMLButtonElement>('.compose-confirm button'))
      .find((button) => button.textContent?.includes('清空并切换'))
    confirm?.click()
    await flushPromises()

    expect(wrapper.get('.compose-page').attributes('data-compose-intent')).toBe('retouch')
    expect((wrapper.get('.common-ribbon select').element as HTMLSelectElement).value).toBe('normal')
    expect((wrapper.get('.note-field input').element as HTMLInputElement).value).toBe('')
    wrapper.unmount()
  })

  it('clears the uploading violation after a reference upload completes', async () => {
    mocks.route.query = { intent: 'modify_existing' }
    let resolveUpload: ((value: { asset_id: string }) => void) | undefined
    mocks.uploadReferenceFileRef.mockImplementation(() => new Promise((resolve) => {
      resolveUpload = resolve
    }))
    const wrapper = mount(UnifiedTaskCreateView, {
      global: { stubs: { UnifiedTaskGrid: true, IIdSelector: true, RouterLink: true } },
    })

    await wrapper.get('.asset-button').trigger('click')
    const input = wrapper.get('input[aria-label="上传参考图或产品图片"]')
    Object.defineProperty(input.element, 'files', {
      configurable: true,
      value: [new File(['image'], 'reference.png', { type: 'image/png' })],
    })
    await input.trigger('change')
    await flushPromises()

    expect(wrapper.text()).toContain('正在上传')
    expect(wrapper.text()).toContain('仍有文件正在上传')

    resolveUpload?.({ asset_id: 'reference-uploaded' })
    await flushPromises()

    expect(wrapper.text()).toContain('已上传')
    expect(wrapper.text()).not.toContain('仍有文件正在上传')
    wrapper.unmount()
  })

  it('blocks restored retouch drafts until local source files are selected again', async () => {
    mocks.route.query = { intent: 'retouch', draft_id: 'draft-1' }
    mocks.getDraft.mockResolvedValue({
      id: 'draft-1',
      payload: {
        intent: 'retouch',
        rows: [{
          id: 'retouch-row',
          design_requirement: '清理背景',
          reference_assets: [],
          source_assets: [{ id: 'source-1', name: 'original.psd', status: 'local' }],
          set_mode_hint: false,
        }],
      },
    })
    const wrapper = mount(UnifiedTaskCreateView, {
      global: { stubs: { UnifiedTaskGrid: true, IIdSelector: true, RouterLink: true } },
    })
    await flushPromises()

    expect(wrapper.text()).toContain('本地素材不会保存在草稿中，请重新选择该文件')
    expect(wrapper.text()).toContain('还有 1 处需要完善')
    expect(wrapper.get('.validation-dock .primary-button').attributes('disabled')).toBeDefined()
    wrapper.unmount()
  })

  it('keeps intent-change confirmation keyboard focus inside the dialog and restores it', async () => {
    const wrapper = mount(UnifiedTaskCreateView, {
      attachTo: document.body,
      global: { stubs: { UnifiedTaskGrid: true, IIdSelector: true, RouterLink: true } },
    })
    await flushPromises()
    await wrapper.get('[data-row-index="0"] textarea').setValue('尚未保存的策划说明')
    const trigger = wrapper.findAll('.intent-card').find((item) => item.text().includes('只修图'))
    if (!trigger) throw new Error('missing retouch intent')
    ;(trigger.element as HTMLButtonElement).focus()
    await trigger.trigger('click')
    await flushPromises()
    const modal = document.querySelector<HTMLElement>('[role="alertdialog"]')
    if (!modal) throw new Error('missing confirmation dialog')
    expect((document.activeElement as HTMLElement)?.textContent).toBe('先不了')
    modal.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true }))
    await flushPromises()
    expect(document.querySelector('[role="alertdialog"]')).toBeNull()
    expect(document.activeElement).toBe(trigger.element)
    wrapper.unmount()
  })
})
