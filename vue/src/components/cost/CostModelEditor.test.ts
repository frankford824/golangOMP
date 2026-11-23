// @vitest-environment jsdom
import {mount} from '@vue/test-utils'
import {describe,expect,it} from 'vitest'
import CostModelEditor from './CostModelEditor.vue'
import CostInputFields from './CostInputFields.vue'
import {emptyCostInput,emptyCostModel} from '@/domain/cost-model'

describe('material price table',()=>{
  it('keeps blank prices unquoted and explicit zero free',async()=>{
    const w=mount(CostModelEditor,{props:{modelValue:JSON.stringify({...emptyCostModel(),material:'超透PVC',variant_prices:[{thickness_mm:0,normal_price:null,irregular_price:null}]})}})
    expect((w.get('input[aria-label="第1行普通单价"]').element as HTMLInputElement).value).toBe('')
    await w.get('input[aria-label="第1行异形单价"]').setValue('0')
    let model=JSON.parse(w.emitted('update:modelValue')!.at(-1)![0] as string)
    expect(model.variant_prices[0]).toEqual({thickness_mm:0,normal_price:null,irregular_price:0})
    await w.get('input[aria-label="第1行异形单价"]').setValue('')
    model=JSON.parse(w.emitted('update:modelValue')!.at(-1)![0] as string)
    expect(model.variant_prices[0].irregular_price).toBeNull()
    w.unmount()
  })
  it('only converts legacy thickness prices after an explicit action',async()=>{
    const w=mount(CostModelEditor,{props:{modelValue:JSON.stringify({...emptyCostModel(),material:'亚克力',unit_price:10,thickness_prices:[{thickness_mm:3,unit_price:20}]})}})
    expect(w.emitted('update:modelValue')).toBeUndefined()
    await w.get('.enable-variants').trigger('click')
    const model=JSON.parse(w.emitted('update:modelValue')!.at(-1)![0] as string)
    expect(model.unit_price).toBe(10)
    expect(model.thickness_prices).toEqual([])
    expect(model.variant_prices).toEqual([{thickness_mm:3,normal_price:20,irregular_price:null}])
    w.unmount()
  })
  it('switches to one area price and clears conflicting pricing choices', async () => {
    const wrapper = mount(CostModelEditor, {
      props: {
        modelValue: JSON.stringify({
          ...emptyCostModel(),
          material: '超透PVC',
          unit_price: 12,
          variant_prices: [{ thickness_mm: 0, normal_price: 10, irregular_price: 14 }],
          processes: [{ code: 'slot', unit: 'sku', unit_price: 1, multiplier: 1 }],
        }),
      },
    })
    await wrapper.get('input[type="checkbox"]').setValue(true)
    const model = JSON.parse(wrapper.emitted('update:modelValue')!.at(-1)![0] as string)
    expect(model).toMatchObject({ area_only: true, unit_price: 12, thickness_prices: [], processes: [] })
    expect(model).not.toHaveProperty('variant_prices')
    expect(wrapper.find('.variant-prices').exists()).toBe(false)
    expect(wrapper.text()).toContain('普通、异形统一按面积单价计费')
    wrapper.unmount()
  })

  it('preserves explicit shape through specification edits and allows clearing it',async()=>{
    const w=mount(CostInputFields,{props:{modelValue:{...emptyCostInput(),shape:'irregular',area_m2:.5}}})
    const shape=w.findAll('select').find(s=>s.find('option[value="irregular"]').exists())!
    expect(shape.element.value).toBe('irregular')
    await w.get('input[type="number"]').setValue('2')
    expect(w.emitted('update:modelValue')!.at(-1)![0]).toEqual(expect.objectContaining({shape:'irregular',area_m2:.5}))
    await shape.setValue('')
    expect(w.emitted('update:modelValue')!.at(-1)![0]).not.toHaveProperty('shape')
    w.unmount()
  })
})
