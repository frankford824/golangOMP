// @vitest-environment jsdom
import { flushPromises, mount } from '@vue/test-utils'
import DriveDeleteConfirmDialog from '@aw/shared/drive/DriveDeleteConfirmDialog.vue'
import UploadLedgerDateField from '@aw/features/upload/UploadLedgerDateField.vue'
import UploadLedgerCategoryFilter from '@aw/features/upload/UploadLedgerCategoryFilter.vue'
import { beforeEach, describe, expect, it, vi } from 'vitest'

const mocks = vi.hoisted(() => ({
  settlementVisible: true,
  capabilities: ['asset.workbench.manage', 'asset.workbench.settlement'],
  driveDirectories: vi.fn(),
  listUploadDirectories: vi.fn().mockResolvedValue([]),
  driveFiles: vi.fn(),
  driveUploadDays: vi.fn().mockResolvedValue([]),
  exportDriveFiles: vi.fn(),
  suggestPersonnel: vi.fn().mockResolvedValue({ items: [], total: 0 }),
  batchMoveFiles: vi.fn(),
  listSettlementSupplements: vi.fn(),
  getSubmissionDetail: vi.fn(),
  batchDeleteFiles: vi.fn(),
}))

vi.mock('vue-router', () => ({
  useRoute: () => ({ query: {} }),
  RouterLink: { props: ['to'], template: '<a><slot /></a>' },
}))

vi.mock('@aw/app/session.store', () => ({
  useAssetWorkbenchSessionStore: () => ({ bootstrap: { capabilities: mocks.capabilities, profile: { settlement_visible: mocks.settlementVisible } } }),
}))

vi.mock('@aw/shared/api/assetWorkbenchApi', () => ({
  assetWorkbenchApi: mocks,
}))

vi.mock('@aw/shared/download/useGlobalDownload', () => ({
  useGlobalDownload: () => ({ queueDriveFile: vi.fn() }),
}))

vi.mock('@aw/shared/drive/DriveThumb.vue', () => ({
  default: { template: '<div class="drive-thumb" />' },
}))

vi.mock('@aw/shared/drive/ArchiveVirtualThumb.vue', () => ({
  default: { template: '<div />' },
}))

vi.mock('@aw/shared/preview/WorkbenchPreviewDialog.vue', () => ({
  default: { props:['metaRows'], template: '<div data-testid="preview-metadata">{{ metaRows }}</div>' },
}))

import UploadOverviewPage from './UploadOverviewPage.vue'

describe('UploadOverviewPage operation source', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    mocks.settlementVisible = true
    mocks.capabilities = ['asset.workbench.manage', 'asset.workbench.settlement']
    mocks.driveDirectories.mockResolvedValue([])
    mocks.listUploadDirectories.mockResolvedValue([])
    mocks.exportDriveFiles.mockImplementation(async () => (await mocks.driveFiles()).items)
    mocks.listSettlementSupplements.mockResolvedValue({ items: [], total: 0, page: 1, size: 100 })
  })

  it.each([
    { visible:false, admin:false, expected:false },
    { visible:true, admin:false, expected:true },
    { visible:false, admin:true, expected:true },
  ])('applies income visibility to ledger summaries and actual amount cells: %o', async ({visible,admin,expected}) => {
    mocks.settlementVisible = visible
    mocks.capabilities = admin ? ['asset.workbench.manage'] : ['asset.workbench.submit']
    mocks.driveFiles.mockResolvedValue({items:[{id:99, submission_item_id:99, original_filename:'sample.png', pricing_status:'pending_grade', gross_amount:123.45, page_count:1}],total:1})
    const wrapper = mount(UploadOverviewPage, {global:{stubs:{RouterLink:true}}})
    await flushPromises()
    expect(wrapper.find('.aw-upload-ledger__summary').text().includes('本页计价金额')).toBe(expected)
    expect(wrapper.findAll('th').some(th=>th.text()==='金额')).toBe(expected)
    expect(wrapper.text().includes('123.45')).toBe(expected)
    expect(wrapper.text()).toContain('匹配文件')
    expect(wrapper.get('tbody').text()).toContain('待定级')
    expect(wrapper.get('tbody').text()).not.toContain('pending_grade')
    await wrapper.get('button[aria-label="预览"]').trigger('click')
    await flushPromises()
    expect(wrapper.get('[data-testid="preview-metadata"]').text().includes('计件金额')).toBe(expected)
    let exported:Blob|undefined
    const create = vi.fn((blob:Blob)=>{exported=blob;return 'blob:ledger-test'})
    vi.stubGlobal('URL',Object.assign(URL,{createObjectURL:create,revokeObjectURL:vi.fn()}))
    const click = vi.spyOn(HTMLAnchorElement.prototype,'click').mockImplementation(()=>{})
    await wrapper.findAll('button').find(button=>button.text().includes('导出'))!.trigger('click')
    await flushPromises()
    const csv = await new Promise<string>((resolve,reject)=>{const reader=new FileReader();reader.onload=()=>resolve(String(reader.result));reader.onerror=reject;reader.readAsText(exported!)})
    expect(csv.includes('计件金额')).toBe(expected)
    expect(csv.includes('123.45')).toBe(expected)
    click.mockRestore()
    vi.unstubAllGlobals()
    wrapper.unmount()
  })

  it.each(['asset.workbench.manage', 'asset.workbench.submit', 'asset.workbench.settlement'])(
    'includes enabled empty directories and retired directories with valid records for %s',
    async (capability) => {
      mocks.capabilities = [capability]
      mocks.driveDirectories.mockResolvedValue([
        { directory_id: 8, name: 'C类旧名' },
        { directory_id: 9, name: '停用目录的历史记录' },
      ])
      mocks.listUploadDirectories.mockResolvedValue([
        { id: 8, name: 'C类', enabled: true },
        { id: 10, name: 'A类+小夜灯', enabled: true },
        { id: 11, name: 'A类+小夜灯未定稿', enabled: true },
        { id: 12, name: '停用且无记录的目录', enabled: false },
      ])
      mocks.driveFiles.mockResolvedValue({ items: [], total: 0 })
      const wrapper = mount(UploadOverviewPage, { global: { stubs: { RouterLink: true } } })
      await flushPromises()

      const category = wrapper.getComponent(UploadLedgerCategoryFilter)
      expect(category.props('options')).toEqual([
        { value: 'unassigned', label: '未分类' },
        { value: 8, label: 'C类' },
        { value: 9, label: '停用目录的历史记录' },
        { value: 10, label: 'A类+小夜灯' },
        { value: 11, label: 'A类+小夜灯未定稿' },
      ])
      category.vm.$emit('update:modelValue', [10, 11])
      await wrapper.get('form.aw-upload-ledger__filters').trigger('submit')
      await flushPromises()
      expect(mocks.driveFiles).toHaveBeenLastCalledWith(expect.objectContaining({ dir_ids: [10, 11] }), expect.any(AbortSignal))
      expect(wrapper.get('[aria-label="已应用筛选"]').text()).toContain('分类：A类+小夜灯、A类+小夜灯未定稿')
      expect(wrapper.get('tbody').text()).toContain('没有匹配的上传记录')
      wrapper.unmount()
    },
  )

  it('queries by the selected uploader ID and clears it when the creator text is edited', async () => {
    vi.useFakeTimers()
    mocks.suggestPersonnel.mockResolvedValueOnce({
      items: [
        { user_id: 41, name: '李明', username: 'qa-one', personnel_no: 'Q-41' },
        { user_id: 42, name: '李明', username: 'qa-two', personnel_no: 'Q-42' },
      ],
      total: 2,
    })
    mocks.driveFiles.mockResolvedValue({ items: [], total: 0 })
    const wrapper = mount(UploadOverviewPage, { global: { stubs: { RouterLink: true } } })
    try {
      await flushPromises()
      const input = wrapper.get('input[placeholder="姓名或账号"]')
      await input.setValue('liming')
      await vi.advanceTimersByTimeAsync(220)
      await flushPromises()
      expect(mocks.suggestPersonnel).toHaveBeenLastCalledWith('upload_owner', 'liming', expect.any(AbortSignal))
      const options = wrapper.get('[aria-label="创建人候选"]').findAll('[role="option"]')
      expect(options).toHaveLength(2)
      expect(options[1]!.text()).toContain('qa-two · 人员编号 Q-42')
      await options[1]!.trigger('click')
      const filters = wrapper.get('form.aw-upload-ledger__filters')
      await filters.trigger('submit')
      await flushPromises()
      expect(mocks.driveFiles).toHaveBeenLastCalledWith(expect.objectContaining({
        owner_user_id: 42, owner: undefined,
      }), expect.any(AbortSignal))
      expect(wrapper.get('[aria-label="已应用筛选"]').text()).toContain('创建人：李明（qa-two）')

      await input.setValue('李')
      await filters.trigger('submit')
      await flushPromises()
      expect(mocks.driveFiles).toHaveBeenLastCalledWith(expect.objectContaining({
        owner_user_id: undefined, owner: '李',
      }), expect.any(AbortSignal))
    } finally {
      wrapper.unmount()
      vi.useRealTimers()
    }
  })

  it('uses the same date and personnel filters for the calendar, list and complete export', async () => {
    const count = 22404
    const files = Array.from({ length: count }, (_, index) => ({
      id: index + 1, submission_item_id: index === 1 ? 1 : index + 1, owner_name: '测试兼职', owner_worker_type: 'parttime',
      original_filename: `work-${index + 1}.png`, page_count: 1, gross_amount: 0.4, created_at: '2026-11-24T15:59:59Z', supplement_date: '2026-10-27', operation_source: 'client_supplement',
    }))
    mocks.driveFiles.mockResolvedValue({ items: files.slice(0, 50), total: count })
    mocks.exportDriveFiles.mockResolvedValue(files)
    const wrapper = mount(UploadOverviewPage, { global: { stubs: { RouterLink: true } } })
    const click = vi.spyOn(HTMLAnchorElement.prototype, 'click').mockImplementation(() => {})
    const createURL = vi.fn((_blob: Blob) => 'blob:complete-export')
    vi.stubGlobal('URL', Object.assign(URL, { createObjectURL: createURL, revokeObjectURL: vi.fn() }))
    try {
      await flushPromises()
      const typeFilter = wrapper.get('select[aria-label="人员类型筛选"]')
      await typeFilter.setValue('parttime')
      await wrapper.get('input[aria-label="补录日期"]').setValue('2026-10-27')
      const exportButton = wrapper.findAll('button').find((button) => button.text().includes('导出全部结果'))!
      expect(exportButton.attributes('disabled')).toBeDefined()
      const dates = wrapper.findAllComponents(UploadLedgerDateField)
      dates[0].vm.$emit('update:modelValue', '2026-11-01')
      dates[1].vm.$emit('update:modelValue', '2026-11-24')
      await wrapper.get('form').trigger('submit')
      await flushPromises()
      expect(mocks.driveFiles).toHaveBeenLastCalledWith(expect.objectContaining({
        worker_type: 'parttime', supplement_date: '2026-10-27', created_from: '2026-11-01', created_to: '2026-11-24', page: 1, page_size: 50,
      }), expect.any(AbortSignal))
      await dates[0].props('loadDays')('2026-11', new AbortController().signal)
      expect(mocks.driveUploadDays).toHaveBeenLastCalledWith(expect.objectContaining({
        worker_type: 'parttime', supplement_date: '2026-10-27', month: '2026-11',
      }), expect.any(AbortSignal))
      expect(wrapper.get('[aria-label="已应用筛选"]').text()).toContain('人员类型：兼职')
      expect(wrapper.get('tbody').text()).toContain('兼职')
      const listCalls = mocks.driveFiles.mock.calls.length
      await exportButton.trigger('click')
      await flushPromises()
      expect(mocks.exportDriveFiles).toHaveBeenCalledExactlyOnceWith(expect.objectContaining({
        worker_type: 'parttime', supplement_date: '2026-10-27', created_from: '2026-11-01', created_to: '2026-11-24',
      }), expect.any(AbortSignal))
      expect(mocks.driveFiles.mock.calls.length).toBe(listCalls)
      const csv = await new Promise<string>((resolve, reject) => {
        const reader = new FileReader()
        reader.onload = () => resolve(String(reader.result))
        reader.onerror = reject
        reader.readAsText(createURL.mock.calls[0]![0] as Blob)
      })
      const lines = csv.split('\n')
      expect(lines).toHaveLength(count + 1)
      expect(lines[0]).toMatch(/创建人,人员类型,操作来源/)
      expect(lines[1]).toContain('客户端补录,2026-11-24 23:59,2026-10-27,')
      expect(lines[0]).toContain('创建日期,补录日期,')
      expect(lines[2]).not.toContain('¥0.40')
      expect(lines[count]).toContain('work-22404.png')
      expect(wrapper.text()).toContain('已导出全部 22,404 条匹配文件记录')
    } finally {
      click.mockRestore()
      vi.unstubAllGlobals()
      wrapper.unmount()
    }
  })

  it('marks client supplement files in the admin upload ledger and detail panel', async () => {
    mocks.driveFiles.mockResolvedValue({
      items: [{
        id: 301,
        submission_id: 201,
        submission_item_id: 101,
        submission_no: 'AWS20260820154500',
        owner_user_id: 9,
        owner_name: '张三',
        owner_username: 'zhangsan',
        upload_directory_name: 'C类',
        difficulty_class: 'C',
        order_no: '补录文件夹',
        original_filename: '补录文件.rar',
        display_name: '补录文件.rar',
        file_type: 'rar',
        mime_type: 'application/vnd.rar',
        file_size: 4096,
        preview_status: 'ready',
        pricing_status: 'priced',
        page_count: 1,
        gross_amount: 0.4,
        business_month: '2026-08',
        operation_source: 'client_supplement',
        created_at: '2026-08-20T07:45:00Z',
      }],
      total: 1,
      page: 1,
      size: 50,
    })

    const wrapper = mount(UploadOverviewPage, {
      global: { stubs: { RouterLink: { template: '<a><slot /></a>' } } },
    })
    await flushPromises()

    const headers = wrapper.findAll('thead th').map((header) => header.text().trim())
    expect(headers.slice(2, 6)).toEqual(['创建人', '人员类型', '操作来源', '创建时间'])
    expect(wrapper.get('.aw-upload-ledger__owner').text()).not.toContain('客户端补录')
    expect(wrapper.get('.aw-upload-ledger__source-cell').text()).toBe('客户端补录')

    await wrapper.get('tbody tr').trigger('click')
    expect(wrapper.get('.aw-upload-ledger__detail dl').text()).toContain('操作来源客户端补录')
  })

  it('falls back to supplement records when the deployed drive API has no operation source', async () => {
    mocks.driveFiles.mockResolvedValue({
      items: [{
        id: 302,
        submission_id: 202,
        submission_item_id: 102,
        submission_no: 'AWS20260821090900',
        owner_user_id: 9,
        owner_name: '张三',
        upload_directory_name: 'B类',
        order_no: '补录压缩包',
        original_filename: '补录压缩包.zip',
        file_type: 'zip',
        mime_type: 'application/zip',
        file_size: 901120,
        preview_status: 'ready',
        pricing_status: 'priced',
        page_count: 1,
        gross_amount: 0.63,
        business_month: '2026-08',
        created_at: '2026-08-21T01:09:00Z',
      }],
      total: 1,
      page: 1,
      size: 50,
    })
    mocks.listSettlementSupplements.mockResolvedValue({
      items: [{ submission_item_id: 102 }],
      total: 1,
      page: 1,
      size: 100,
    })

    const wrapper = mount(UploadOverviewPage, {
      global: { stubs: { RouterLink: { template: '<a><slot /></a>' } } },
    })
    await flushPromises()

    expect(mocks.listSettlementSupplements).toHaveBeenCalledWith({ business_month: '2026-08', page: 1, page_size: 100 }, expect.any(AbortSignal))
    expect(wrapper.get('.aw-upload-ledger__source-cell').text()).toBe('补录')
  })

  it('filters the ledger by normal uploads or supplement source', async () => {
    mocks.driveFiles.mockResolvedValue({ items: [], total: 0, page: 1, size: 50 })
    const wrapper = mount(UploadOverviewPage, {
      global: { stubs: { RouterLink: { template: '<a><slot /></a>' } } },
    })
    await flushPromises()

    const sourceFilter = wrapper.get('select[aria-label="操作来源筛选"]')
    expect(sourceFilter.findAll('option').map((option) => option.text())).toEqual([
      '全部来源', '正常上传', '补录（全部）', '客户端补录', '管理员补录',
    ])
    await sourceFilter.setValue('supplement')
    await wrapper.get('form.aw-upload-ledger__filters').trigger('submit')
    await flushPromises()

    expect(mocks.driveFiles).toHaveBeenLastCalledWith(expect.objectContaining({
      operation_source: 'supplement',
      page: 1,
    }), expect.any(AbortSignal))
    expect(wrapper.text()).toContain('重置筛选')
  })
})

describe('UploadOverviewPage cross-page deletion', () => {
  const files = Array.from({ length: 53 }, (_, index) => ({ id: index + 1, submission_id: 20, submission_item_id: 30,
    original_filename: `${index + 1}.jpg`, relative_path: `image/nested/${index + 1}.jpg`, is_folder_upload: true,
    business_month: '2026-10', operation_source: 'normal_upload', created_at: '2026-10-13T01:00:00Z', gross_amount: 1 }))
  const dialogStub = { props: ['selection', 'busy'], template: '<div />' }
  const pageButton = (wrapper: ReturnType<typeof mount>, label: string) => wrapper.findAll('button').find((button) => button.text().trim() === label)!
  const mountLedger = () => mount(UploadOverviewPage, { global: { stubs: { RouterLink: { template: '<a><slot /></a>' }, DriveDeleteConfirmDialog: dialogStub } } })

  beforeEach(() => {
    vi.clearAllMocks()
    mocks.capabilities = ['asset.workbench.manage', 'asset.workbench.settlement']
    mocks.driveDirectories.mockResolvedValue([])
    mocks.listSettlementSupplements.mockResolvedValue({ items: [], total: 0 })
    mocks.driveFiles.mockImplementation(async (params) => ({ items: params.page_size === 100 ? files : params.page === 2 ? files.slice(50) : files.slice(0, 50), total: 53 }))
    mocks.getSubmissionDetail.mockResolvedValue({ submission: { id: 20 }, items: [{ item: { id: 30 }, files }] })
    mocks.batchDeleteFiles.mockResolvedValue({ deleted: files.map((file) => file.id), failures: [] })
  })

  it('retains page-one choices and removes only page-two choices when unchecked', async () => {
    const wrapper = mountLedger()
    await flushPromises()
    expect(wrapper.text()).toContain('文件夹作品 · 本页 50 个文件 · 只计 1 件')
    expect(wrapper.text()).not.toContain('文件夹作品 · 共 50 个文件')
    await wrapper.get('input[aria-label="选择当前页"]').setValue(true)
    expect(wrapper.get('[aria-label="批量操作"] strong').text()).toContain('50')
    await pageButton(wrapper, '下一页').trigger('click')
    await flushPromises()
    await wrapper.get('input[aria-label="选择当前页"]').setValue(true)
    expect(wrapper.get('[aria-label="批量操作"] strong').text()).toContain('53')
    await wrapper.get('input[aria-label="选择当前页"]').setValue(false)
    expect(wrapper.get('[aria-label="批量操作"] strong').text()).toContain('50')
    wrapper.unmount()
  })

  it('selects all filtered files, but changing filters clears the selection', async () => {
    const wrapper = mountLedger()
    await flushPromises()
    await wrapper.get('input[aria-label="选择当前页"]').setValue(true)
    await pageButton(wrapper, '全选筛选结果（53个）').trigger('click')
    await flushPromises()
    expect(wrapper.get('[aria-label="批量操作"] strong').text()).toContain('53')
    await wrapper.get('form.aw-upload-ledger__filters').trigger('submit')
    await flushPromises()
    expect(wrapper.find('[aria-label="批量操作"]').exists()).toBe(false)
    wrapper.unmount()
  })

  it('reads the full 53-file work before confirmation and never deletes on cancel', async () => {
    const wrapper = mountLedger()
    await flushPromises()
    await wrapper.get('input[aria-label="选择当前页"]').setValue(true)
    await wrapper.get('input[aria-label="操作原因"]').setValue('错传')
    await pageButton(wrapper, '删除').trigger('click')
    await flushPromises()
    const dialog = wrapper.findComponent(DriveDeleteConfirmDialog)
    expect(dialog.props('selection')).toEqual(expect.objectContaining({ fileIds: files.map((file) => file.id), workCount: 1, extraCount: 3 }))
    expect(mocks.batchDeleteFiles).not.toHaveBeenCalled()
    dialog.vm.$emit('close')
    await flushPromises()
    expect(mocks.batchDeleteFiles).not.toHaveBeenCalled()
    await pageButton(wrapper, '删除').trigger('click')
    await flushPromises()
    dialog.vm.$emit('confirm')
    await flushPromises()
    expect(mocks.batchDeleteFiles).toHaveBeenCalledWith(files.map((file) => file.id), '错传')
    wrapper.unmount()
  })

  it('does not apply edited-but-unsubmitted filters to a cross-page selection', async () => {
    const wrapper = mountLedger()
    await flushPromises()
    await wrapper.get('input[aria-label="选择当前页"]').setValue(true)
    await wrapper.get('input[placeholder="姓名或账号"]').setValue('其他人员')
    expect(pageButton(wrapper, '全选筛选结果（53个）').attributes('disabled')).toBeDefined()
    await pageButton(wrapper, '下一页').trigger('click')
    await flushPromises()
    expect(mocks.driveFiles).toHaveBeenLastCalledWith(expect.objectContaining({ page: 2, owner: undefined }), expect.any(AbortSignal))
    expect(wrapper.get('[aria-label="批量操作"] strong').text()).toContain('50')
    wrapper.unmount()
  })

  it('keeps selection and never calls delete if full work details cannot be read', async () => {
    mocks.getSubmissionDetail.mockRejectedValue(new Error('无权读取作品'))
    const wrapper = mountLedger()
    await flushPromises()
    await wrapper.get('input[aria-label="选择当前页"]').setValue(true)
    await wrapper.get('input[aria-label="操作原因"]').setValue('错传')
    await pageButton(wrapper, '删除').trigger('click')
    await flushPromises()
    expect(wrapper.text()).toContain('无权读取作品')
    expect(wrapper.findComponent(DriveDeleteConfirmDialog).props('selection')).toBeNull()
    expect(mocks.batchDeleteFiles).not.toHaveBeenCalled()
    wrapper.unmount()
  })
})

describe('UploadOverviewPage self-service date scope', () => {
  const files = Array.from({ length: 53 }, (_, index) => ({ id: index + 1, submission_id: 20, submission_item_id: 30,
    owner_user_id: 22, original_filename: `${index + 1}.jpg`, is_folder_upload: true,
    business_month: '2026-10', operation_source: 'normal_upload', created_at: '2026-10-13T01:00:00Z', gross_amount: 0.4 }))
  const mountSelfLedger = () => mount(UploadOverviewPage, { global: { stubs: { RouterLink: { template: '<a><slot /></a>' }, DriveDeleteConfirmDialog: { props: ['selection', 'busy'], template: '<div />' } } } })
  const button = (wrapper: ReturnType<typeof mount>, label: string) => wrapper.findAll('button').find((item) => item.text().trim() === label)!
  const reasonAndDelete = async (wrapper: ReturnType<typeof mount>) => {
    await wrapper.get('input[aria-label="操作原因"]').setValue('错传')
    await button(wrapper, '删除').trigger('click')
    await flushPromises()
  }

  beforeEach(() => {
    vi.clearAllMocks()
    mocks.capabilities = ['asset.workbench.submit']
    mocks.driveDirectories.mockResolvedValue([])
    mocks.listUploadDirectories.mockResolvedValue([
      { id: 8, name: 'A类', enabled: true },
      { id: 9, name: '已停用', enabled: false },
    ])
    mocks.driveFiles.mockImplementation(async (params) => ({ items: params.page_size === 100 ? files : files.slice(0, 50), total: 53 }))
    mocks.getSubmissionDetail.mockResolvedValue({ submission: { id: 20 }, items: [{ item: { id: 30 }, files }] })
    mocks.batchDeleteFiles.mockResolvedValue({ deleted: files.map((file) => file.id), failures: [] })
  })

  it('opens an own-uploads ledger with default dates and move controls without exposing creator or rename controls', async () => {
    const wrapper = mountSelfLedger()
    await flushPromises()
    const dateInputs = wrapper.findAllComponents(UploadLedgerDateField)
    const day = dateInputs[0]!.props('modelValue')
    expect(day).toMatch(/^\d{4}-\d{2}-\d{2}$/)
    expect(dateInputs[1]!.props('modelValue')).toBe(day)
    expect(mocks.driveFiles).toHaveBeenCalledWith(expect.objectContaining({ created_from: day, created_to: day, owner: undefined }), expect.any(AbortSignal))
    expect(wrapper.text()).toContain('我的上传台账')
    expect(wrapper.text()).not.toContain('全站上传台账')
    expect(wrapper.find('input[placeholder="姓名或账号"]').exists()).toBe(false)
    await wrapper.get('.aw-upload-ledger__name button').trigger('click')
    expect(wrapper.find('input[aria-label="作品名称"]').exists()).toBe(false)
    await wrapper.get('input[aria-label="选择当前页"]').setValue(true)
    const target = wrapper.get('select[aria-label="移动目标分类"]')
    expect(target.text()).toContain('A类')
    expect(target.text()).not.toContain('已停用')
    expect(button(wrapper, '移动').attributes('disabled')).toBeDefined()
    expect(wrapper.findAll('button').some((item) => item.text().trim() === '删除')).toBe(true)
    wrapper.unmount()
  })

  it('submits a move for a client and shows the returned restriction in Chinese', async () => {
    mocks.driveFiles.mockResolvedValue({ items: files.slice(0, 1), total: 1 })
    mocks.batchMoveFiles.mockResolvedValue({ files: [], failures: [{
      file_id: 1,
      reason: 'Only works awaiting quality review can be moved by their submitter.',
    }] })
    const wrapper = mountSelfLedger()
    await flushPromises()

    await wrapper.get('input[aria-label="选择当前页"]').setValue(true)
    await wrapper.get('select[aria-label="移动目标分类"]').setValue('8')
    await button(wrapper, '移动').trigger('click')
    await flushPromises()

    expect(mocks.batchMoveFiles).toHaveBeenCalledWith([1], 8, '上传总览移动文件')
    expect(wrapper.text()).toContain('普通员工只能移动尚未质检的作品。需要调整时，请联系素材管理员。')
    wrapper.unmount()
  })

  it('queries explicit dates and clears the previous selection before replacing the scope', async () => {
    const wrapper = mountSelfLedger()
    await flushPromises()
    await wrapper.get('input[aria-label="选择当前页"]').setValue(true)
    const dateInputs = wrapper.findAllComponents(UploadLedgerDateField)
    dateInputs[0]!.vm.$emit('update:modelValue', '2026-01-01')
    dateInputs[1]!.vm.$emit('update:modelValue', '2026-01-02')
    await wrapper.get('input[aria-label="操作原因"]').setValue('错传')
    expect(button(wrapper, '删除').attributes('disabled')).toBeDefined()
    await wrapper.get('form.aw-upload-ledger__filters').trigger('submit')
    await flushPromises()
    expect(mocks.driveFiles).toHaveBeenLastCalledWith(expect.objectContaining({ created_from: '2026-01-01', created_to: '2026-01-02' }), expect.any(AbortSignal))
    expect(wrapper.find('[aria-label="批量操作"]').exists()).toBe(false)
    wrapper.unmount()
  })

  it('blocks deletion when completing a folder would include uploads outside the queried date range', async () => {
    mocks.driveFiles.mockImplementation(async () => ({ items: files.slice(0, 1), total: 1 }))
    mocks.getSubmissionDetail.mockResolvedValue({ submission: { id: 20 }, items: [{ item: { id: 30 }, files: files.slice(0, 2) }] })
    const wrapper = mountSelfLedger()
    await flushPromises()
    await wrapper.get('input[aria-label="选择当前页"]').setValue(true)
    await reasonAndDelete(wrapper)
    expect(wrapper.text()).toContain('包含当前日期范围外的文件')
    expect(wrapper.findComponent(DriveDeleteConfirmDialog).props('selection')).toBeNull()
    expect(mocks.batchDeleteFiles).not.toHaveBeenCalled()
    wrapper.unmount()
  })

  it('completes 53 same-date files and includes the queried date range in confirmation', async () => {
    const wrapper = mountSelfLedger()
    await flushPromises()
    await wrapper.get('input[aria-label="选择当前页"]').setValue(true)
    await reasonAndDelete(wrapper)
    const plan = wrapper.findComponent(DriveDeleteConfirmDialog).props('selection')
    if (!plan) throw new Error('expected a date-scoped deletion confirmation')
    expect(plan.fileIds).toEqual(files.map((file) => file.id))
    expect(plan.extraCount).toBe(3)
    expect(plan.scopeLabel).toContain('仅删除我的上传；上传日期（北京时间）')
    expect(mocks.batchDeleteFiles).not.toHaveBeenCalled()
    wrapper.unmount()
  })

  it('requires a queried date range even if all historical rows have been selected', async () => {
    const wrapper = mountSelfLedger()
    await flushPromises()
    for (const input of wrapper.findAllComponents(UploadLedgerDateField)) input.vm.$emit('update:modelValue', '')
    await wrapper.get('form.aw-upload-ledger__filters').trigger('submit')
    await flushPromises()
    await wrapper.get('input[aria-label="选择当前页"]').setValue(true)
    await wrapper.get('input[aria-label="操作原因"]').setValue('错传')
    expect(button(wrapper, '删除').attributes('disabled')).toBeDefined()
    expect(mocks.batchDeleteFiles).not.toHaveBeenCalled()
    await button(wrapper, '恢复今天').trigger('click')
    await flushPromises()
    expect(wrapper.find('[aria-label="批量操作"]').exists()).toBe(false)
    expect(wrapper.findAllComponents(UploadLedgerDateField)[0]!.props('modelValue')).not.toBe('')
    wrapper.unmount()
  })
})
