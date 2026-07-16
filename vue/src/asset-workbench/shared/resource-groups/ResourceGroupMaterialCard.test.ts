// @vitest-environment jsdom
import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'

const getResourceGroup = vi.hoisted(() => vi.fn())

vi.mock('@aw/shared/api/assetWorkbenchApi', async (loadOriginal) => {
  const original = await loadOriginal<typeof import('@aw/shared/api/assetWorkbenchApi')>()
  return { ...original, assetWorkbenchApi: { ...original.assetWorkbenchApi, getResourceGroup } }
})

import ResourceGroupMaterialCard from './ResourceGroupMaterialCard.vue'
import type { ClientMaterialRow, SystemAssetRow } from '@aw/shared/api/assetWorkbenchApi'

const asset = (revisionID = 70): SystemAssetRow => ({
  id: 8,
  resource_group_id: 8,
  finalized_revision_id: revisionID,
  cover_revision_item_id: 701,
  resource_mode: 'set',
  resource_item_count: 2,
  source_type: 'task_resource_group',
  resource_id: 'group:8',
  task_no: 'RW-008',
  scope_sku_code: 'SKU-008',
  product_name: '夏季主图套装',
})

const published = (revisionID = 60): ClientMaterialRow => ({
  id: 90,
  asset_id: 8,
  source_type: 'task_resource_group',
  source_ref: 'group:8',
  resource_id: 'group:8',
  title: '夏季主图套装',
  description: '',
  filename_snapshot: 'cover-old.png',
  mime_type_snapshot: 'image/png',
  file_size_snapshot: 100,
  enabled: true,
  sort_order: 1,
  published_by: 1,
  resource_group_id: 8,
  finalized_revision_id: revisionID,
  cover_revision_item_id: 601,
})

function currentGroup(revisionID = 70) {
  return {
    id: 8,
    task_id: 3,
    task_no: 'RW-008',
    sku_code: 'SKU-008',
    scope_kind: 'sku' as const,
    finalized_revision_id: revisionID,
    finalized_revision: {
      id: revisionID,
      mode: 'set' as const,
      items: [
        { id: 701, sort_order: 1, task_asset_id: 1001, file: { task_asset_id: 1001, file_name: 'front.png' } },
        { id: 702, sort_order: 2, task_asset_id: 1002, file: { task_asset_id: 1002, file_name: 'side.png' } },
      ],
    },
  }
}

function textButton(wrapper: ReturnType<typeof mount>, text: string) {
  const found = wrapper.findAll('button').find((button) => button.text() === text)
  if (!found) throw new Error(`missing button ${text}`)
  return found
}

describe('ResourceGroupMaterialCard', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    getResourceGroup.mockResolvedValue(currentGroup())
  })

  it('shows resource-group semantics and requires an explicit set cover', async () => {
    const wrapper = mount(ResourceGroupMaterialCard, {
      props: { asset: asset(), canPublish: true },
      attachTo: document.body,
    })
    expect(wrapper.text()).toContain('套装')
    expect(wrapper.text()).toContain('2 张最终成品')

    await textButton(wrapper, '发布到客户端').trigger('click')
    await flushPromises()
    expect(getResourceGroup).toHaveBeenCalledWith(8)
    expect(wrapper.get('[role="dialog"]').text()).toContain('请选择一张客户封面')
    expect(textButton(wrapper, '确认发布').attributes('disabled')).toBeDefined()

    const covers = wrapper.findAll('input[type="radio"]')
    await covers[1].setValue(true)
    await textButton(wrapper, '确认发布').trigger('click')
    expect(wrapper.emitted('publish')).toEqual([[{ finalizedRevisionId: 70, coverRevisionItemId: 702 }]])
    wrapper.unmount()
  })

  it('keeps an old publication pinned until an explicit republish selects the new revision', async () => {
    const wrapper = mount(ResourceGroupMaterialCard, {
      props: { asset: asset(70), published: published(60), canPublish: true },
      attachTo: document.body,
    })
    expect(wrapper.text()).toContain('客户端固定版本 60')
    expect(wrapper.text()).toContain('不会自动切换')
    expect(wrapper.emitted('publish')).toBeUndefined()

    await textButton(wrapper, '重新发布当前版本').trigger('click')
    await flushPromises()
    await wrapper.findAll('input[type="radio"]')[0].setValue(true)
    await textButton(wrapper, '确认重新发布').trigger('click')
    expect(wrapper.emitted('publish')).toEqual([[{ finalizedRevisionId: 70, coverRevisionItemId: 701 }]])
    wrapper.unmount()
  })

  it('emits preview and whole-group download separately', async () => {
    const wrapper = mount(ResourceGroupMaterialCard, { props: { asset: asset() } })
    await textButton(wrapper, '预览').trigger('click')
    await textButton(wrapper, '整组下载').trigger('click')
    expect(wrapper.emitted('preview')).toHaveLength(1)
    expect(wrapper.emitted('download')).toHaveLength(1)
  })

  it('contains keyboard focus, closes with Escape, and restores the publish trigger', async () => {
    const wrapper = mount(ResourceGroupMaterialCard, {
      props: { asset: asset(), canPublish: true, publishing: false },
      attachTo: document.body,
    })
    const trigger = textButton(wrapper, '发布到客户端')
    ;(trigger.element as HTMLButtonElement).focus()
    await trigger.trigger('click')
    await flushPromises()
    expect((document.activeElement as HTMLElement)?.getAttribute('aria-label')).toBe('关闭发布窗口')

    await wrapper.findAll('input[type="radio"]')[0].setValue(true)
    const last = textButton(wrapper, '确认发布')
    ;(last.element as HTMLButtonElement).focus()
    await wrapper.get('[role="dialog"]').trigger('keydown', { key: 'Tab' })
    expect((document.activeElement as HTMLElement)?.getAttribute('aria-label')).toBe('关闭发布窗口')

    await wrapper.setProps({ publishing: true })
    await wrapper.get('[role="dialog"]').trigger('keydown', { key: 'Escape' })
    expect(wrapper.find('[role="dialog"]').exists()).toBe(true)

    await wrapper.setProps({ publishing: false })
    await wrapper.get('[role="dialog"]').trigger('keydown', { key: 'Escape' })
    await flushPromises()
    expect(wrapper.find('[role="dialog"]').exists()).toBe(false)
    expect(document.activeElement).toBe(trigger.element)
    wrapper.unmount()
  })
})
