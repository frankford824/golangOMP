// @vitest-environment jsdom
import { mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import SkuResourceMatrix from './SkuResourceMatrix.vue'
import type { ResourceBundle } from '@/services/api/resourceGroupsApi'

const mocks = vi.hoisted(() => ({
  download: vi.fn(async (): Promise<{ ok: boolean; message?: string }> => ({ ok: true })),
}))

vi.mock('@/utils/assetFileDownload', () => ({
  downloadAssetFileWithOriginalFilename: mocks.download,
}))

vi.mock('@/components/media/AssetPreviewMedia.vue', () => ({
  default: {
    props: ['taskAssetId', 'fallbackSrc', 'alt'],
    emits: ['open-full'],
    template: '<img class="asset-preview-media-stub" :data-task-asset-id="taskAssetId || undefined" :src="fallbackSrc || undefined" :alt="alt" @click.stop="$emit(\'open-full\', fallbackSrc)" />',
  },
}))

vi.mock('@/components/media/ImagePreviewLightbox.vue', () => ({
  default: {
    props: ['modelValue', 'items', 'ariaLabel', 'fallbackTitle'],
    emits: ['update:modelValue'],
    template: `
      <div v-if="modelValue" class="image-preview-lightbox-stub" role="dialog" :aria-label="ariaLabel">
        <img :src="items[0]?.src" :alt="items[0]?.alt" />
        <button type="button" aria-label="缩小预览">−</button>
        <button type="button" aria-label="重置缩放">100%</button>
        <button type="button" aria-label="放大预览">+</button>
        <button type="button" aria-label="适应窗口">适应</button>
        <button type="button" aria-label="关闭预览" @click="$emit('update:modelValue', false)">×</button>
      </div>
    `,
  },
}))

const bundle: ResourceBundle = {
  task_id: 9,
  workflow_revision: 4,
  groups: [{
    id: 8,
    task_id: 9,
    task_no: 'RW-009',
    scope_kind: 'sku',
    task_sku_item_id: 11,
    sku_code: 'SKU-009',
    product_name: '北欧客厅组合',
    creator_name: '李运营',
    lock_version: 1,
    migration_incomplete: false,
    finalized_revision: {
      id: 70,
      group_id: 8,
      revision_no: 2,
      status: 'finalized',
      mode: 'set',
      source_stage: 'audit',
      created_by: 7,
      legacy_migration: false,
      created_at: '2026-07-22T08:00:00Z',
      source_file: { task_asset_id: 100, file_name: 'living-room.psd', file_size: 10485760, download_url: '/source' },
      references: [{ id: 1, reference_file_ref_id: 2, formal_task_asset_id: 103, sort_order: 0, ref_id: 'ref-1', file_name: 'direction.jpg', preview_url: '/reference' }],
      items: [
        { id: 2, revision_id: 70, task_asset_id: 102, sort_order: 1, file: { task_asset_id: 102, file_name: 'detail.png', preview_url: '/detail' } },
        { id: 1, revision_id: 70, task_asset_id: 101, sort_order: 0, file: { task_asset_id: 101, file_name: 'cover.png', preview_url: '/cover' } },
      ],
    },
  }],
}

describe('SkuResourceMatrix', () => {
  beforeEach(() => {
    mocks.download.mockReset()
    mocks.download.mockResolvedValue({ ok: true })
  })

  it('uses task-level references only as a fallback for an empty legacy resource shell', () => {
    const emptyBundle = structuredClone(bundle)
    emptyBundle.groups[0].finalized_revision = null
    emptyBundle.groups[0].working_revision = null
    const wrapper = mount(SkuResourceMatrix, {
      props: {
        bundle: emptyBundle,
        taskReferences: [{ file_name: 'legacy-task-reference.png', preview_url: '/legacy-task-reference' }],
      },
      global: { stubs: { Teleport: true } },
    })

    expect(wrapper.get('.reference-stage .tile-caption').text()).toContain('legacy-task-reference.png')
    expect(wrapper.get('.reference-stage .asset-preview-media-stub').attributes('src')).toBe('/legacy-task-reference')
  })

  it('does not mix task-level references into a resource group that already has a revision', () => {
    const wrapper = mount(SkuResourceMatrix, {
      props: {
        bundle,
        taskReferences: [{ file_name: 'task-only.png', preview_url: '/task-only' }],
      },
      global: { stubs: { Teleport: true } },
    })

    expect(wrapper.text()).not.toContain('task-only.png')
  })

  it('shows per-SKU reference images uploaded at creation time alongside revision references', () => {
    const wrapper = mount(SkuResourceMatrix, {
      props: {
        bundle,
        skuItems: [
          {
            sku_code: 'SKU-009',
            reference_file_refs: [
              { file_name: 'direction.jpg', preview_url: '/reference' },
              { file_name: '运营手绘稿.png', mime_type: 'image/png', download_url: '/sku-item-reference' },
            ],
          },
          { sku_code: 'SKU-010', reference_file_refs: [{ file_name: '别的SKU.png', download_url: '/other' }] },
        ],
      },
      global: { stubs: { Teleport: true } },
    })

    const captions = wrapper.findAll('.reference-stage .tile-caption').map((item) => item.text())
    expect(captions.some((caption) => caption.includes('运营手绘稿.png'))).toBe(true)
    expect(captions.some((caption) => caption.includes('别的SKU.png'))).toBe(false)
    expect(captions.filter((caption) => caption.includes('direction.jpg'))).toHaveLength(1)
  })

  it('keeps distinct reference files that happen to share a filename', () => {
    const wrapper = mount(SkuResourceMatrix, {
      props: {
        bundle,
        skuItems: [{
          sku_code: 'SKU-009',
          reference_file_refs: [
            { file_name: 'direction.jpg', preview_url: '/different-reference' },
          ],
        }],
      },
      global: { stubs: { Teleport: true } },
    })

    const captions = wrapper.findAll('.reference-stage .tile-caption').map((item) => item.text())
    expect(captions.filter((caption) => caption.includes('direction.jpg'))).toHaveLength(2)
    expect(wrapper.findAll('.reference-stage .asset-preview-media-stub').map((item) => item.attributes('src'))).toEqual([
      '/reference',
      '/different-reference',
    ])
  })

  it('renders the three business stages and makes set order immediately visible', async () => {
    const wrapper = mount(SkuResourceMatrix, { props: { bundle }, global: { stubs: { Teleport: true } } })
    expect(wrapper.findAll('.stage-card h3').map((item) => item.text())).toEqual(['运营参考图', '当前有效源文件', '最终成品图'])
    expect(wrapper.text()).toContain('审核人员上传的替换源文件')
    expect(wrapper.text()).toContain('套装 · 2 张')
    expect(wrapper.findAll('.final-tile')[0].text()).toContain('封面')
    expect(wrapper.findAll('.final-tile')[0].text()).toContain('cover.png')
    expect(wrapper.text()).toContain('来源任务 · RW-009')
  })

  it('opens a visual preview without exposing workflow revision or group ids', async () => {
    const wrapper = mount(SkuResourceMatrix, { props: { bundle }, global: { stubs: { Teleport: true } } })
    await wrapper.get('.reference-grid .asset-preview-media-stub').trigger('click')
    expect(wrapper.get('.image-preview-lightbox-stub img').attributes('src')).toBe('/reference')
    expect(wrapper.get('[aria-label="缩小预览"]').attributes('aria-label')).toBe('缩小预览')
    expect(wrapper.get('[aria-label="放大预览"]').attributes('aria-label')).toBe('放大预览')
    expect(wrapper.get('[aria-label="适应窗口"]').attributes('aria-label')).toBe('适应窗口')
    expect(wrapper.text()).not.toContain('工作流修订')
    await wrapper.get('[aria-label="关闭预览"]').trigger('click')
    expect(wrapper.find('.image-preview-lightbox-stub').exists()).toBe(false)
  })

  it('loads current snapshot images through immutable task-asset ids', () => {
    const wrapper = mount(SkuResourceMatrix, { props: { bundle }, global: { stubs: { Teleport: true } } })

    expect(wrapper.get('.reference-grid .asset-preview-media-stub').attributes('data-task-asset-id')).toBe('103')
    expect(wrapper.findAll('.final-gallery .asset-preview-media-stub').map((item) => item.attributes('data-task-asset-id'))).toEqual(['101', '102'])
  })

  it('uses authenticated download resolution for source, reference, and final files', async () => {
    const fileBundle = structuredClone(bundle)
    const revision = fileBundle.groups[0].finalized_revision!
    revision.references = [{
      id: 9,
      reference_file_ref_id: 19,
      formal_task_asset_id: 109,
      sort_order: 0,
      ref_id: 'ref-xls',
      file_name: 'requirements.xls',
      mime_type: 'application/vnd.ms-excel',
      preview_url: '/controlled/requirements.xls',
      download_url: '/controlled/requirements.xls?download=1',
    }]
    revision.items = [{
      id: 10,
      revision_id: 70,
      task_asset_id: 110,
      sort_order: 0,
      file: {
        task_asset_id: 110,
        file_name: 'delivery.zip',
        mime_type: 'application/zip',
        preview_url: '/controlled/delivery.zip',
        download_url: '/controlled/delivery.zip?download=1',
      },
    }]
    const wrapper = mount(SkuResourceMatrix, {
      props: { bundle: fileBundle },
      global: { stubs: { Teleport: true } },
    })

    expect(wrapper.findAll('a[download]')).toHaveLength(0)

    await wrapper.get('.source-file-card .file-download-button').trigger('click')
    await vi.waitFor(() => expect(mocks.download).toHaveBeenNthCalledWith(1, {
      taskAssetId: '100',
      downloadUrl: undefined,
      preferredFilename: 'living-room.psd',
    }))

    await wrapper.get('.reference-grid .file-download-tile').trigger('click')
    await vi.waitFor(() => expect(mocks.download).toHaveBeenNthCalledWith(2, {
      taskAssetId: '109',
      downloadUrl: undefined,
      preferredFilename: 'requirements.xls',
    }))

    await wrapper.get('.final-gallery .file-download-tile').trigger('click')
    await vi.waitFor(() => expect(mocks.download).toHaveBeenNthCalledWith(3, {
      taskAssetId: '110',
      downloadUrl: undefined,
      preferredFilename: 'delivery.zip',
    }))
  })

  it('keeps URL fallback only for a legacy reference without a task-asset id', async () => {
    const fileBundle = structuredClone(bundle)
    fileBundle.groups[0].finalized_revision!.references = [{
      id: 9,
      reference_file_ref_id: 19,
      sort_order: 0,
      ref_id: 'legacy-ref',
      file_name: 'legacy-requirements.xls',
      mime_type: 'application/vnd.ms-excel',
      download_url: '/legacy/requirements.xls',
    }]
    const wrapper = mount(SkuResourceMatrix, { props: { bundle: fileBundle }, global: { stubs: { Teleport: true } } })

    await wrapper.get('.reference-grid .file-download-tile').trigger('click')
    await vi.waitFor(() => expect(mocks.download).toHaveBeenCalledWith({
      taskAssetId: undefined,
      downloadUrl: '/legacy/requirements.xls',
      preferredFilename: 'legacy-requirements.xls',
    }))
  })

  it('shows a user-facing error when controlled download resolution fails', async () => {
    mocks.download.mockResolvedValueOnce({ ok: false, message: '获取下载地址失败' })
    const wrapper = mount(SkuResourceMatrix, { props: { bundle }, global: { stubs: { Teleport: true } } })

    await wrapper.get('.source-file-card .file-download-button').trigger('click')
    await vi.waitFor(() => expect(wrapper.get('[role="alert"]').text()).toBe('获取下载地址失败'))
  })

  it('does not promote preview-only non-image files to downloads', () => {
    const fileBundle = structuredClone(bundle)
    const revision = fileBundle.groups[0].finalized_revision!
    revision.references = [{
      id: 9,
      reference_file_ref_id: 19,
      sort_order: 0,
      ref_id: 'ref-xls',
      file_name: 'requirements.xls',
      mime_type: 'application/vnd.ms-excel',
      preview_url: '/controlled/requirements.xls',
    }]
    revision.items = [{
      id: 10,
      revision_id: 70,
      task_asset_id: 110,
      sort_order: 0,
      file: {
        task_asset_id: 110,
        file_name: 'delivery.zip',
        mime_type: 'application/zip',
        preview_url: '/controlled/delivery.zip',
      },
    }]
    const wrapper = mount(SkuResourceMatrix, {
      props: { bundle: fileBundle },
      global: { stubs: { Teleport: true } },
    })

    expect(wrapper.findAll('.reference-grid a[download]')).toHaveLength(0)
    expect(wrapper.findAll('.final-gallery a[download]')).toHaveLength(0)
    expect(wrapper.get('.reference-grid a').attributes('href')).toBe('/controlled/requirements.xls')
    expect(wrapper.get('.reference-grid a').text()).toContain('打开')
    expect(wrapper.get('.final-gallery a').attributes('href')).toBe('/controlled/delivery.zip')
    expect(wrapper.get('.final-gallery a').text()).toContain('打开')
  })

  it('opens revision history only when the task detail enables it', async () => {
    const wrapper = mount(SkuResourceMatrix, {
      props: { bundle, enableRevisionHistory: true },
      global: { stubs: { Teleport: true, ResourceRevisionDrawer: { props: ['group'], template: '<aside class="revision-drawer-stub">历史修订 {{ group.sku_code }}</aside>' } } },
    })
    expect(wrapper.find('.revision-drawer-stub').exists()).toBe(false)
    await wrapper.get('.revision-history-button').trigger('click')
    expect(wrapper.get('.revision-drawer-stub').text()).toContain('SKU-009')
  })
})
