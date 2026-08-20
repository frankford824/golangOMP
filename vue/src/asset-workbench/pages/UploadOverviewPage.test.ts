// @vitest-environment jsdom
import { flushPromises, mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'

const mocks = vi.hoisted(() => ({
  driveDirectories: vi.fn(),
  driveFiles: vi.fn(),
}))

vi.mock('vue-router', () => ({
  useRoute: () => ({ query: {} }),
  RouterLink: { props: ['to'], template: '<a><slot /></a>' },
}))

vi.mock('@aw/app/session.store', () => ({
  useAssetWorkbenchSessionStore: () => ({ bootstrap: { capabilities: ['asset.workbench.manage'] } }),
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
  it('marks client supplement files in the admin upload ledger and detail panel', async () => {
    mocks.driveDirectories.mockResolvedValue([])
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

    expect(wrapper.get('.aw-upload-ledger__source').text()).toBe('客户端补录')

    await wrapper.get('tbody tr').trigger('click')
    expect(wrapper.get('.aw-upload-ledger__detail dl').text()).toContain('操作来源客户端补录')
  })
})
