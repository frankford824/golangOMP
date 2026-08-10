// @vitest-environment jsdom
import { mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'

vi.mock('@/services/api/tasksApi', () => ({
  tasksApi: {
    patchSkuItem: vi.fn(),
    patchSkuItemCostInfo: vi.fn(),
  },
}))

import TaskSkuItemEditor from './TaskSkuItemEditor.vue'

describe('TaskSkuItemEditor', () => {
  it('keeps batch reference images visibly paired with their SKU rows', () => {
    const wrapper = mount(TaskSkuItemEditor, {
      props: {
        taskId: 2915,
        canEdit: false,
        canEditCost: false,
        items: [
          {
            id: 3136,
            sku_code: 'DZK000217',
            product_name_snapshot: '60×90cm 款',
            reference_file_refs: [{
              ref_id: 'ref-a',
              filename: '60x90-reference.png',
              mime_type: 'image/png',
              preview_url: '/v1/assets/preview/ref-a',
              download_url: '/v1/assets/download/ref-a',
            }],
          },
          {
            id: 3137,
            sku_code: 'DZK000218',
            product_name_snapshot: '50×70cm 款',
            referenceFileRefs: [{
              ref_id: 'ref-b',
              filename: '50x70-reference.jpg',
              mime_type: 'image/jpeg',
              url: '/v1/assets/files/ref-b',
            }],
          },
        ],
      },
    })

    const rows = wrapper.findAll('.sku-editor-row')
    expect(rows).toHaveLength(2)
    expect(rows[0].text()).toContain('DZK000217')
    expect(rows[0].text()).toContain('60x90-reference.png')
    expect(rows[0].find('img').attributes('src')).toBe('/v1/assets/preview/ref-a')
    expect(rows[1].text()).toContain('DZK000218')
    expect(rows[1].text()).toContain('50x70-reference.jpg')
    expect(rows[1].find('img').attributes('src')).toBe('/v1/assets/files/ref-b')
  })

  it('lets a task creator save business fields without treating an unchanged visible cost as an override', async () => {
    const { tasksApi } = await import('@/services/api/tasksApi')
    const wrapper = mount(TaskSkuItemEditor, {
      props: {
        taskId: 2915,
        canEdit: true,
        canEditCost: true,
        items: [{
          id: 3136,
          sku_code: 'DZK000217',
          product_name_snapshot: '旧名称',
          cost_price: 18.8,
        }],
      },
    })

    await wrapper.get('input').setValue('创建者修改后的名称')
    await wrapper.get('form').trigger('submit')

    expect(tasksApi.patchSkuItem).toHaveBeenCalledWith('2915', 3136, expect.objectContaining({
      product_name: '创建者修改后的名称',
    }))
    expect(tasksApi.patchSkuItemCostInfo).not.toHaveBeenCalled()
    expect(wrapper.findAll('input').find((input) => input.element.value === '18.8')?.attributes('disabled')).toBeUndefined()
  })
})
