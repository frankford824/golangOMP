// @vitest-environment jsdom
import {mount,flushPromises,enableAutoUnmount} from '@vue/test-utils'
import {describe,it,expect,vi,afterEach} from 'vitest'
enableAutoUnmount(afterEach)
const read=vi.hoisted(()=>vi.fn())
vi.mock('@/services/api/costManagementApi',()=>({costManagementApi:{listBoundSKUs:read}}))
import CostBoundSKUList from './CostBoundSKUList.vue'
describe('current bound SKU browser',()=>{
 it('searches and paginates on the server',async()=>{
  read.mockResolvedValue({data:[{id:1,sku_code:'CGK1',product_name:'板材',style_code:'KT',cost_price:null}],pagination:{total:21}})
  const w=mount(CostBoundSKUList,{props:{group:'KT',revision:0}});await flushPromises()
  expect(w.text()).toContain('待确认')
  await w.findAll('button').find(b=>b.text()==='下一页')!.trigger('click');await flushPromises()
  expect(read).toHaveBeenLastCalledWith(expect.objectContaining({rule_group:'KT',page:2}))
  await w.get('input').setValue('CGK1');await w.get('form').trigger('submit');await flushPromises()
  expect(read).toHaveBeenLastCalledWith(expect.objectContaining({keyword:'CGK1',page:1}))
 })
 it('does not show a stale response after switching schemes',async()=>{
  let finish:(v:unknown)=>void=()=>{}
  read.mockImplementationOnce(()=>new Promise(resolve=>{finish=resolve})).mockResolvedValueOnce({data:[{id:2,sku_code:'NEW',style_code:'B'}],pagination:{total:1}})
  const w=mount(CostBoundSKUList,{props:{group:'A',revision:0}})
  await w.setProps({group:'B'});await flushPromises();finish({data:[{id:1,sku_code:'OLD'}],pagination:{total:1}});await flushPromises()
  expect(w.text()).toContain('NEW');expect(w.text()).not.toContain('OLD')
 })
})
