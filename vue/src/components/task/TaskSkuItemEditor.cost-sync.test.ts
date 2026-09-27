// @vitest-environment jsdom
import {mount,flushPromises} from '@vue/test-utils'
import {describe,it,expect,vi,beforeEach,afterEach} from 'vitest'
const api=vi.hoisted(()=>({patchSkuItem:vi.fn(),patchSkuItemCostInfo:vi.fn(),getSkuItemCostSync:vi.fn()}))
vi.mock('@/services/api/tasksApi',()=>({tasksApi:api}))
import Editor from './TaskSkuItemEditor.vue'
const item={id:10305,sku_code:'QA-MANUAL',product_name_snapshot:'测试SKU',cost_price:3.3,manual_cost_override_reason:'核价'}
const view=(status='conflict')=>({data:{data:{state:{status,erp_cost:9.9,reason:status==='conflict'?'需要确认':''},baseline:{revision:2,erp_cost:9.9},erp_available:true,message:''}}})
let wrapper:ReturnType<typeof mount>|undefined
beforeEach(()=>{vi.clearAllMocks();api.getSkuItemCostSync.mockResolvedValue(view());api.patchSkuItemCostInfo.mockResolvedValue({});api.patchSkuItem.mockResolvedValue({})})
afterEach(()=>wrapper?.unmount())
const setup=()=>wrapper=mount(Editor,{props:{taskId:7576,items:[item],canEdit:true,canEditCost:true},global:{stubs:{CostInputFields:true,ImagePreviewLightbox:true}}})
describe('manual cost delivery',()=>{
 it('requeues the same manual amount after reviewing ERP without resending product fields',async()=>{
  const w=setup()
  await w.find('input[placeholder="修改后按人工成本保存"]').trigger('focus');await flushPromises()
  expect(w.text()).toContain('ERP 当前成本：¥9.9')
  api.getSkuItemCostSync.mockResolvedValue(view('pending'))
  await w.find('form').trigger('submit');await flushPromises()
  expect(api.patchSkuItem).not.toHaveBeenCalled()
  expect(api.patchSkuItemCostInfo).toHaveBeenCalledWith('7576',10305,expect.objectContaining({cost_price:3.3,sync_baseline:{revision:2,erp_cost:9.9}}))
  expect(w.text()).toContain('等待 ERP 回读')
  expect(w.text()).not.toContain('已回读一致')
 })
 it('preserves typed price after a competing ERP edit rejects the save',async()=>{
  const w=setup(),input=w.find('input[placeholder="修改后按人工成本保存"]')
  await input.trigger('focus');await flushPromises();await input.setValue('4.2')
  api.patchSkuItemCostInfo.mockRejectedValue(new Error('ERP价格在编辑期间发生变化'))
  await w.find('form').trigger('submit');await flushPromises()
  expect((input.element as HTMLInputElement).value).toBe('4.2')
  expect(w.text()).toContain('ERP价格在编辑期间发生变化')
  expect(w.emitted('saved')).toBeUndefined()
 })
 it('does not silently overwrite ERP when no baseline has been displayed',async()=>{
  const w=setup();await w.find('input[placeholder="修改后按人工成本保存"]').setValue('4.2')
  await w.find('form').trigger('submit');await flushPromises()
  expect(api.patchSkuItemCostInfo).not.toHaveBeenCalled()
  expect(w.text()).toContain('核对 ERP 成本后再次保存')
 })
})
