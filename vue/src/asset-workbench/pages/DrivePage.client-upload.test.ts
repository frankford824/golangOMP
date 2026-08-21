// @vitest-environment jsdom
import { flushPromises, shallowMount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it, vi } from 'vitest'

const mocks = vi.hoisted(() => ({
  listUploadDirectories: vi.fn(),
  listUploadDirectoriesAdmin: vi.fn(),
}))

vi.mock('vue-router', async (loadOriginal) => {
  const original = await loadOriginal<typeof import('vue-router')>()
  return { ...original, useRoute: () => ({ query: {} }) }
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
      listUploadDirectories: mocks.listUploadDirectories,
      listUploadDirectoriesAdmin: mocks.listUploadDirectoriesAdmin,
    },
  }
})

import { useAssetWorkbenchSessionStore } from '@aw/app/session.store'
import DrivePage from './DrivePage.vue'

const uploadDirectory = {
  id: 8,
  name: 'A类',
  oss_prefix: 'a',
  description: '',
  difficulty_class: 'A',
  allowed_file_types: [],
  enabled: true,
  sort_order: 1,
  created_by: 1,
}

function mountDrive(capabilities: string[], isAdmin: boolean) {
  const pinia = createPinia()
  setActivePinia(pinia)
  useAssetWorkbenchSessionStore().setBootstrap({
    actor: { id: isAdmin ? 1 : 22, display_name: isAdmin ? 'admin' : 'client' },
    is_admin: isAdmin,
    capabilities,
  } as never)
  return shallowMount(DrivePage, {
    global: {
      plugins: [pinia],
      stubs: { RouterLink: { template: '<a><slot /></a>' } },
    },
  })
}

describe('DrivePage client upload entry', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    mocks.listUploadDirectories.mockResolvedValue([uploadDirectory])
    mocks.listUploadDirectoriesAdmin.mockResolvedValue([uploadDirectory])
  })

  it('hides upload-to-current-directory for client users', async () => {
    const wrapper = mountDrive(['asset.workbench.submit'], false)
    await flushPromises()

    expect(mocks.listUploadDirectories).toHaveBeenCalled()
    expect(wrapper.findAll('button').some((button) => button.text().trim() === '上传到此处')).toBe(false)
  })

  it('keeps upload-to-current-directory for managers', async () => {
    const wrapper = mountDrive(['asset.workbench.manage'], true)
    await flushPromises()

    expect(mocks.listUploadDirectoriesAdmin).toHaveBeenCalled()
    expect(wrapper.findAll('button').some((button) => button.text().trim() === '上传到此处')).toBe(true)
  })
})
