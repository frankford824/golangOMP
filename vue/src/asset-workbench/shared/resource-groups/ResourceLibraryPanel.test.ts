// @vitest-environment jsdom
import { flushPromises, mount } from '@vue/test-utils'
import { createPinia } from 'pinia'
import { beforeEach, describe, expect, it, vi } from 'vitest'

const listResourceGroups = vi.hoisted(() => vi.fn())
const previewTaskAsset = vi.hoisted(() => vi.fn())
const getWorkbenchResourceGroup = vi.hoisted(() => vi.fn())

vi.mock('@/services/api/resourceGroupsApi', async (loadOriginal) => {
  const original = await loadOriginal<typeof import('@/services/api/resourceGroupsApi')>()
  return {
    ...original,
    resourceGroupsApi: {
      ...original.resourceGroupsApi,
      list: listResourceGroups,
      previewTaskAsset,
    },
  }
})

vi.mock('@aw/shared/api/assetWorkbenchApi', async (loadOriginal) => {
  const original = await loadOriginal<typeof import('@aw/shared/api/assetWorkbenchApi')>()
  return {
    ...original,
    assetWorkbenchApi: {
      ...original.assetWorkbenchApi,
      getResourceGroup: getWorkbenchResourceGroup,
    },
  }
})

import ResourceLibraryPanel from './ResourceLibraryPanel.vue'

const finalizedRevision = {
  id: 70,
  group_id: 8,
  revision_no: 2,
  status: 'finalized' as const,
  mode: 'set' as const,
  source_stage: 'audit' as const,
  created_by: 1,
  legacy_migration: false,
  created_at: '2026-08-18T00:00:00Z',
  references: [],
  items: [
    { id: 701, revision_id: 70, task_asset_id: 1001, sort_order: 1, file: { task_asset_id: 1001, file_name: 'front.png', mime_type: 'image/png' } },
    { id: 702, revision_id: 70, task_asset_id: 1002, sort_order: 2, file: { task_asset_id: 1002, file_name: 'side.png', mime_type: 'image/png' } },
  ],
}

const group = {
  id: 8,
  task_id: 3,
  scope_kind: 'sku' as const,
  finalized_revision_id: 70,
  lock_version: 2,
  migration_incomplete: false,
  task_no: 'RW-008',
  sku_code: 'SKU-008',
  product_name: '夏季主图套装',
  business_lane: 'customization',
  finalized_revision: finalizedRevision,
}

function listResult() {
  return { items: [group], flat_items: [], view_mode: 'group' as const, page: 1, page_size: 36, total: 1 }
}

function textButton(wrapper: ReturnType<typeof mount>, text: string) {
  const found = wrapper.findAll('button').find((button) => button.text() === text)
  if (!found) throw new Error(`missing button ${text}`)
  return found
}

describe('ResourceLibraryPanel', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    listResourceGroups.mockResolvedValue(listResult())
    previewTaskAsset.mockResolvedValue({ download_mode: 'direct', download_url: 'https://preview.test/front.png', filename: 'front.png', file_size: 10 })
    getWorkbenchResourceGroup.mockResolvedValue(group)
  })

  it('sends the explicit normal/customization filter to the canonical resource-group endpoint', async () => {
    const wrapper = mount(ResourceLibraryPanel, { global: { plugins: [createPinia()] } })
    await flushPromises()

    await wrapper.get('select[aria-label="业务分类"]').setValue('customization')
    await flushPromises()

    expect(listResourceGroups).toHaveBeenLastCalledWith(expect.objectContaining({
      business_lane: 'customization',
      page: 1,
      page_size: 36,
    }))
  })

  it('publishes a finalized resource group after an explicit cover selection', async () => {
    const wrapper = mount(ResourceLibraryPanel, {
      props: { canPublish: true, clientMaterials: [] },
      global: { plugins: [createPinia()] },
      attachTo: document.body,
    })
    await flushPromises()

    expect(wrapper.text()).toContain('定制')
    await textButton(wrapper, '发布到客户端').trigger('click')
    await flushPromises()
    await wrapper.findAll('input[type="radio"]')[1].setValue(true)
    await textButton(wrapper, '确认发布').trigger('click')

    expect(wrapper.emitted('publish')?.[0]?.[0]).toMatchObject({
      asset: { resource_group_id: 8, business_lane: 'customization' },
      selection: { finalizedRevisionId: 70, coverRevisionItemId: 702 },
    })
    wrapper.unmount()
  })
})
