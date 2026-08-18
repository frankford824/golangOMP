// @vitest-environment jsdom
import { flushPromises, shallowMount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it, vi } from 'vitest'

const browseMaterials = vi.hoisted(() => vi.fn())
const listClientMaterials = vi.hoisted(() => vi.fn())

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
    const pinia = createPinia()
    setActivePinia(pinia)
    useAssetWorkbenchSessionStore().setBootstrap({
      actor: { id: 1, display_name: 'admin' },
      is_admin: true,
      capabilities: ['asset.workbench.manage', 'asset.workbench.material.download'],
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
  })
})
