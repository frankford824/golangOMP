// @vitest-environment jsdom
import { flushPromises, shallowMount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it, vi } from 'vitest'

const browseMaterials = vi.hoisted(() => vi.fn())
const listClientMaterials = vi.hoisted(() => vi.fn())
const batchUpdateClientMaterials = vi.hoisted(() => vi.fn())
const overviewSearch = vi.hoisted(() => vi.fn())
const previewClientMaterial = vi.hoisted(() => vi.fn())

vi.mock('vue-router', async (loadOriginal) => {
  const original = await loadOriginal<typeof import('vue-router')>()
  return {
    ...original,
    useRoute: () => ({ query: { scope: 'operational' } }),
  }
})

vi.mock('@aw/shared/download/useGlobalDownload', () => ({
  useGlobalDownload: () => ({ queueDriveFile: vi.fn(), queueMaterial: vi.fn() }),
}))

vi.mock('@aw/shared/api/assetWorkbenchApi', async (loadOriginal) => {
  const original = await loadOriginal<typeof import('@aw/shared/api/assetWorkbenchApi')>()
  return {
    ...original,
    assetWorkbenchApi: {
      ...original.assetWorkbenchApi,
      listDifficultyClasses: vi.fn().mockResolvedValue([]),
      driveDirectories: vi.fn().mockResolvedValue([]),
      listUploadDirectoriesAdmin: vi.fn().mockResolvedValue([]),
      listClientMaterials,
      browseMaterials,
      batchUpdateClientMaterials,
      overviewSearch,
      previewClientMaterial,
    },
  }
})

import { useAssetWorkbenchSessionStore } from '@aw/app/session.store'
import DrivePage from './DrivePage.vue'

function textButton(wrapper: ReturnType<typeof shallowMount>, text: string) {
  const found = wrapper.findAll('button').find((button) => button.text() === text)
  if (!found) throw new Error(`missing button ${text}`)
  return found
}

describe('DrivePage operational browsing', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    browseMaterials.mockResolvedValue({ path: '', folders: [], files: [], total: 0, page: 1, page_size: 100 })
    listClientMaterials.mockResolvedValue([])
    batchUpdateClientMaterials.mockResolvedValue({
      requested: 1, created: 1, updated: 0, removed: 0, skipped: 0, failed: 0, items: [], failures: [], async_required: false,
    })
    overviewSearch.mockResolvedValue({ items: [], total: 0, page: 1, page_size: 60 })
    previewClientMaterial.mockResolvedValue({ download_mode: 'direct', download_url: 'https://preview.test/client.png', filename: 'client.png', mime_type: 'image/png' })
    const pinia = createPinia()
    setActivePinia(pinia)
    useAssetWorkbenchSessionStore().setBootstrap({
      actor: { id: 1, display_name: 'admin' },
      is_admin: true,
      capabilities: ['asset.workbench.material.download', 'asset.publish'],
    } as never)
  })

  it('switches from canonical resource groups to path browsing with normal/customization filters', async () => {
    const pinia = createPinia()
    setActivePinia(pinia)
    useAssetWorkbenchSessionStore().setBootstrap({
      actor: { id: 1, display_name: 'admin' },
      is_admin: true,
      capabilities: ['asset.workbench.manage', 'asset.workbench.material.download'],
    } as never)
    const wrapper = shallowMount(DrivePage, {
      global: {
        plugins: [pinia],
        stubs: { RouterLink: { template: '<a><slot /></a>' } },
      },
    })
    await flushPromises()

    expect(wrapper.find('[data-testid="resource-panel"]').exists()).toBe(false)
    expect(wrapper.text()).toContain('主工程资源库')
    await textButton(wrapper, '路径浏览').trigger('click')
    await flushPromises()

    expect(browseMaterials).toHaveBeenCalledWith(expect.objectContaining({
      path: '',
      source: 'all',
      business_lane: 'all',
    }), expect.any(AbortSignal))
    const laneOptions = wrapper.get('select[aria-label="素材分类"]').findAll('option').map((option) => option.text())
    expect(laneOptions).toEqual(['全部分类', '常规', '定制'])

    browseMaterials.mockResolvedValue({
      path: '/系统资源', folders: [], total: 1, page: 1, size: 100,
      files: [{
        id: 8,
        source_type: 'task_resource_group',
        resource_id: 'group:8',
        resource_group_id: 8,
        finalized_revision_id: 70,
        resource_mode: 'set',
        resource_item_count: 2,
        task_no: 'RW-008',
        sku_code: 'SKU-008',
        product_name: '定制套装',
        business_lane: 'customization',
        file_name: 'cover.png',
        mime_type: 'image/png',
      }],
    })
    await wrapper.get('select[aria-label="素材来源"]').setValue('system')
    await wrapper.get('select[aria-label="素材分类"]').setValue('customization')
    await wrapper.get('select[aria-label="素材格式"]').setValue('design')
    await flushPromises()

    expect(browseMaterials).toHaveBeenLastCalledWith(expect.objectContaining({
      source: 'system',
      business_lane: 'customization',
      format_category: 'design',
    }), expect.any(AbortSignal))
    expect(wrapper.find('resource-group-material-card-stub').exists()).toBe(true)
  })

  it('publishes selected canonical resource groups through the existing batch endpoint', async () => {
    const pinia = createPinia()
    setActivePinia(pinia)
    useAssetWorkbenchSessionStore().setBootstrap({
      actor: { id: 1, display_name: 'admin' },
      is_admin: true,
      capabilities: ['asset.workbench.material.download', 'asset.publish'],
    } as never)
    const wrapper = shallowMount(DrivePage, {
      global: {
        plugins: [pinia],
        stubs: { RouterLink: { template: '<a><slot /></a>' } },
      },
    })
    await flushPromises()

    const panel = wrapper.findComponent({ name: 'ResourceLibraryPanel' })
    expect(panel.exists()).toBe(true)
    expect(panel.props('canPublish')).toBe(true)
    panel.vm.$emit('batch-publish', {
      assets: [{
        id: 9,
        source_type: 'task_resource_group',
        resource_id: 'group:9',
        resource_group_id: 9,
        finalized_revision_id: 80,
        cover_revision_item_id: 801,
        resource_mode: 'single',
        resource_item_count: 1,
        product_name: '可批量上架单图',
      }],
    })
    await flushPromises()

    expect(batchUpdateClientMaterials).toHaveBeenCalledWith({
      action: 'publish',
      items: [expect.objectContaining({
        source_type: 'task_resource_group',
        source_ref: 'group:9',
        resource_group_id: 9,
        finalized_revision_id: 80,
        cover_revision_item_id: 801,
      })],
      selection_scope: 'selected',
    })
  })

  it('shows enabled client materials instead of task-scoped resource groups for non-publishers', async () => {
    const pinia = createPinia()
    setActivePinia(pinia)
    useAssetWorkbenchSessionStore().setBootstrap({
      actor: { id: 22, display_name: 'client-user' },
      is_admin: false,
      capabilities: ['asset.workbench.material.download'],
    } as never)
    listClientMaterials.mockResolvedValue([{
      id: 501,
      asset_id: 0,
      source_type: 'task_resource_group',
      source_ref: 'group:8',
      resource_id: 'group:8',
      resource_group_id: 8,
      finalized_revision_id: 70,
      cover_revision_item_id: 701,
      title: '管理员已上架定制素材',
      filename_snapshot: 'cover.png',
      mime_type_snapshot: 'image/png',
      file_size_snapshot: 1024,
      business_lane: 'customization',
      enabled: true,
      sort_order: 1,
      published_by: 1,
      published_at: '2026-08-20T00:00:00Z',
      created_at: '2026-08-20T00:00:00Z',
      updated_at: '2026-08-20T00:00:00Z',
    }])

    const wrapper = shallowMount(DrivePage, {
      global: {
        plugins: [pinia],
        stubs: { RouterLink: { template: '<a><slot /></a>' } },
      },
    })
    await flushPromises()

    expect(listClientMaterials).toHaveBeenCalledWith(false, expect.any(AbortSignal))
    expect(wrapper.findComponent({ name: 'ResourceLibraryPanel' }).exists()).toBe(false)
    expect(wrapper.text()).toContain('已上架素材')
    const materialCard = wrapper.findComponent({ name: 'ResourceGroupMaterialCard' })
    expect(materialCard.exists()).toBe(true)
    expect(materialCard.props('asset')).toMatchObject({ material_id: 501, resource_group_id: 8, product_name: '管理员已上架定制素材' })
    expect(wrapper.findAll('button').some((button) => button.text() === '资源组')).toBe(false)
  })

  it('keeps published client-material hits in client unified search and hides raw system resources', async () => {
    const pinia = createPinia()
    setActivePinia(pinia)
    useAssetWorkbenchSessionStore().setBootstrap({
      actor: { id: 22, display_name: 'client-user' },
      is_admin: false,
      capabilities: ['asset.workbench.material.download'],
    } as never)
    overviewSearch.mockResolvedValue({
      items: [
        {
          source: 'client_material', scope: 'operational', source_label: '可下载素材', id: 501,
          title: 'DZK000394 已上架素材', primary_code: 'group:8', secondary_code: 'DZK000394', status: 'enabled',
          created_at: '2026-08-20T00:00:00Z', updated_at: '2026-08-20T00:00:00Z', route_path: '/drive?scope=operational&material_id=501',
          meta_json: { material_id: 501, resource_group_id: 8, source_type: 'task_resource_group', source_ref: 'group:8', resource_id: 'group:8', filename: 'cover.png', mime_type: 'image/png', preview_available: true },
        },
        {
          source: 'system_asset', scope: 'operational', source_label: '系统资源', id: 9001,
          title: '未发布主工程素材', primary_code: 'raw:9001', secondary_code: 'DZK000394', status: 'active',
          created_at: '2026-08-20T00:00:00Z', updated_at: '2026-08-20T00:00:00Z', route_path: '/drive?scope=operational', meta_json: {},
        },
      ],
      total: 2,
      page: 1,
      page_size: 60,
    })
    const wrapper = shallowMount(DrivePage, {
      global: {
        plugins: [pinia],
        stubs: { RouterLink: { template: '<a><slot /></a>' } },
      },
    })
    await flushPromises()

    await wrapper.get('input[placeholder="搜索运营素材、文件名、上传目录"]').setValue('DZK000394')
    await wrapper.get('form.aw-drive__search--global').trigger('submit')
    await flushPromises()

    expect(overviewSearch).toHaveBeenCalledWith(expect.objectContaining({ q: 'DZK000394', scope: 'all' }), expect.any(AbortSignal))
    expect(wrapper.text()).toContain('DZK000394 已上架素材')
    expect(wrapper.text()).not.toContain('未发布主工程素材')
    expect(wrapper.text()).toContain('共 1 条')
  })
})
