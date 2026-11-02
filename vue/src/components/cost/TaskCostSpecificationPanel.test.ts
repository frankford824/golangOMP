// @vitest-environment jsdom
import {mount,flushPromises,enableAutoUnmount} from '@vue/test-utils'
import {afterEach,beforeEach,describe,expect,it,vi} from 'vitest'
import {emptyCostInput} from '@/domain/cost-model'
const mocks=vi.hoisted(()=>({list:vi.fn(),save:vi.fn()}))
vi.mock('@/services/api/taskCostSpecificationsApi',()=>({taskCostSpecificationsApi:mocks}))
import TaskCostSpecificationPanel from './TaskCostSpecificationPanel.vue'
enableAutoUnmount(afterEach)
const row={task_id:1,task_no:'RW-1',task_status:'Completed',sku_item_id:2,sku_code:'SKU-2',product_name:'测试产品',requires_area:true,missing_area:true,cost_input:emptyCostInput(),cost_price:10,cost_review_required:true,can_edit:true,token:'old'}
beforeEach(()=>{vi.clearAllMocks();mocks.list.mockResolvedValue({data:[row],total:1,page:1,page_size:20});mocks.save.mockResolvedValue({data:[],total:0,page:1,page_size:20})})
describe('persistent specification work',()=>{
  it('returns to a valid page when completing the last item on the last page',async()=>{
    mocks.list.mockResolvedValue({data:[row],total:41,page:1,page_size:20})
    const w=mount(TaskCostSpecificationPanel,{global:{stubs:{Teleport:true}}});await flushPromises();await w.get('.spec-heading').trigger('click')
    await w.findAll('.spec-content footer button').at(-1)!.trigger('click');await flushPromises()
    await w.findAll('.spec-content footer button').at(-1)!.trigger('click');await flushPromises()
    await w.get('.spec-row button').trigger('click')
    mocks.list.mockResolvedValue({data:[row],total:40,page:2,page_size:20})
    await w.get('form').trigger('submit');await flushPromises()
    expect(mocks.list).toHaveBeenLastCalledWith(undefined,2)
    expect(w.get('.spec-content footer').text()).toContain('2 / 2')
  })
  it('keeps completed work visible and saves geometry only',async()=>{
    const w=mount(TaskCostSpecificationPanel,{global:{stubs:{Teleport:true}}});await flushPromises()
    expect(w.get('.spec-heading').text()).toContain('我的待补计价资料')
    await w.get('.spec-heading').trigger('click')
    expect(w.text()).toContain('已结单');expect(w.text()).toContain('待补面积')
    await w.get('.spec-row button').trigger('click');await flushPromises()
    const area=w.findAll('input').find(x=>x.element.type==='number'&&x.element.parentElement?.textContent?.includes('整套总面积'))!
    await area.setValue('0.5')
    mocks.list.mockResolvedValue({data:[],total:0,page:1,page_size:20})
    await w.get('form').trigger('submit');await flushPromises()
    expect(mocks.save).toHaveBeenCalledWith(expect.objectContaining({task_status:'Completed',token:'old'}),expect.objectContaining({area_m2:.5}),'补录缺失计价资料')
    expect(w.get('[role="status"]').text()).toContain('原价格保留')
    expect(w.find('.spec-row').exists()).toBe(false)
  })
  it('retains the form on a version conflict and never repeats a save',async()=>{
    mocks.save.mockRejectedValue(new Error('计价资料已更新，请刷新后重试'))
    const w=mount(TaskCostSpecificationPanel,{props:{taskId:1},global:{stubs:{Teleport:true}}});await flushPromises()
    await w.get('.spec-heading').trigger('click');await w.get('.spec-row button').trigger('click')
    await w.get('form').trigger('submit');await flushPromises()
    expect(w.get('[role="dialog"]').text()).toContain('请刷新后重试');expect(mocks.save).toHaveBeenCalledTimes(1)
  })
  it('shows a read-only row when correction access is absent',async()=>{
    mocks.list.mockResolvedValue({data:[{...row,can_edit:false}],total:1,page:1,page_size:20})
    const w=mount(TaskCostSpecificationPanel);await flushPromises();await w.get('.spec-heading').trigger('click')
    expect(w.find('.spec-row button').exists()).toBe(false)
  })
})
