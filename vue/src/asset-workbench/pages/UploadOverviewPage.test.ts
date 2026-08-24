// @vitest-environment jsdom
import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'

const mocks = vi.hoisted(() => ({
  driveDirectories: vi.fn(),
  driveFiles: vi.fn(),
  listSettlementSupplements: vi.fn(),
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
