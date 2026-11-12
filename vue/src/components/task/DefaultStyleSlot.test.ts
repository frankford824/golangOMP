// @vitest-environment jsdom
import { flushPromises, mount } from '@vue/test-utils'
import { expect, it, vi } from 'vitest'
import DefaultStyleSlot from './DefaultStyleSlot.vue'
import type { ResourceGroup } from '@/services/api/resourceGroupsApi'

const mocks = vi.hoisted(() => ({
  upload: vi.fn(),
  update: vi.fn(),
  bundle: vi.fn(),
}))
vi.mock('@/services/upload/assetUploadFlow', () => ({ uploadTaskFileViaAssetSession: mocks.upload }))
vi.mock('@/services/api/resourceGroupsApi', () => ({
  resourceGroupsApi: { updateDefaultStyle: mocks.update, taskBundle: mocks.bundle },
}))

it('uploads one SKU file, saves against the group version, and displays its pending state', async () => {
  const group: ResourceGroup = {
    id: 12, task_id: 41, scope_kind: 'sku', sku_code: 'DZ-001', lock_version: 3, migration_incomplete: false,
  }
  const updated: ResourceGroup = {
    ...group,
    lock_version: 4,
    pending_default_style: {
      id: 90, group_id: 12, created_by: 7, removed: false, reason: '用于日常制作', created_at: '2026-11-12T08:00:00Z',
      file: { task_asset_id: 500, file_name: '默认款.psb' },
    },
  }
  mocks.upload.mockResolvedValue({ version: { id: 500 } })
  mocks.update.mockResolvedValue({ group_lock_version: 4, revision_id: 90 })
  mocks.bundle.mockResolvedValue({ task_id: 41, workflow_revision: 9, groups: [updated] })
  const wrapper = mount(DefaultStyleSlot, { props: { group, editable: true } })
  const fileInput = wrapper.get('input[type="file"]')
  expect(fileInput.attributes('disabled')).toBeUndefined()
  expect(wrapper.find('input[aria-label="默认款变更原因"]').exists()).toBe(false)
  const file = new File(['psb'], '默认款.psb', { type: 'application/octet-stream' })
  Object.defineProperty(fileInput.element, 'files', { configurable: true, value: [file] })
  await fileInput.trigger('change')
  expect(wrapper.text()).toContain('默认款.psb')
  expect(wrapper.get('button[type="submit"]').attributes('disabled')).toBeDefined()
  expect(mocks.upload).not.toHaveBeenCalled()
  await wrapper.get('input[aria-label="默认款变更原因"]').setValue('用于日常制作')
  await wrapper.get('form').trigger('submit')
  await flushPromises()
  expect(mocks.upload).toHaveBeenCalledWith('41', file, {
    asset_kind: 'default_style', target_sku_code: 'DZ-001', remark: '默认款.psb',
  }, { retouchRequirementId: undefined })
  expect(mocks.update).toHaveBeenCalledWith(group, { task_asset_id: 500 }, '用于日常制作', expect.any(String))
  expect(wrapper.emitted('updated')?.at(-1)).toEqual([updated])
  await wrapper.setProps({ group: updated })
  expect(wrapper.text()).toContain('结单后生效')
  expect(wrapper.text()).toContain('默认款.psb')
  expect(wrapper.findAll('button').map((button) => button.text())).not.toContain('预览')
})
