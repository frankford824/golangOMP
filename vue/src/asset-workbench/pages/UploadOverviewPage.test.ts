// @vitest-environment jsdom
import { flushPromises, mount } from '@vue/test-utils'
import DriveDeleteConfirmDialog from '@aw/shared/drive/DriveDeleteConfirmDialog.vue'
import { beforeEach, describe, expect, it, vi } from 'vitest'

const mocks = vi.hoisted(() => ({
  driveDirectories: vi.fn(),
  driveFiles: vi.fn(),
  listSettlementSupplements: vi.fn(),
  getSubmissionDetail: vi.fn(),
  batchDeleteFiles: vi.fn(),
}))

vi.mock('vue-router', () => ({
  useRoute: () => ({ query: {} }),
  RouterLink: { props: ['to'], template: '<a><slot /></a>' },
}))

vi.mock('@aw/app/session.store', () => ({
  useAssetWorkbenchSessionStore: () => ({ bootstrap: { capabilities: ['asset.workbench.manage', 'asset.workbench.settlement'] } }),
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
  default: { template: '<div />' },
}))

import UploadOverviewPage from './UploadOverviewPage.vue'

describe('UploadOverviewPage operation source', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    mocks.driveDirectories.mockResolvedValue([])
    mocks.listSettlementSupplements.mockResolvedValue({ items: [], total: 0, page: 1, size: 100 })
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
    expect(headers.slice(2, 5)).toEqual(['创建人', '操作来源', '创建时间'])
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
    expect(wrapper.text()).toContain('重置 1')
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
