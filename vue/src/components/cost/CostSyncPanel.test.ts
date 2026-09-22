// @vitest-environment jsdom
import {mount,flushPromises,enableAutoUnmount} from '@vue/test-utils'
import {afterEach,beforeEach,describe,it,expect,vi} from 'vitest'
enableAutoUnmount(afterEach)
const mocks=vi.hoisted(()=>({list:vi.fn(),resolve:vi.fn(),can:vi.fn()}))
vi.mock('@/services/api/costManagementApi',()=>({costManagementApi:{listCostSyncStates:mocks.list,resolveCostSync:mocks.resolve}}))
vi.mock('@/composables/usePermission',()=>({usePermission:()=>({can:mocks.can})}))
import CostSyncPanel from './CostSyncPanel.vue'
describe('CostSyncPanel',()=>{
 beforeEach(()=>{vi.clearAllMocks();mocks.can.mockReturnValue(true);mocks.resolve.mockResolvedValue({});mocks.list.mockResolvedValue({data:[{sku_code:'QA-1',local_cost:1.2,erp_cost:1.5,revision:3,erp_revision:7,status:'conflict',manual_lock:true,manual_origin:'local',checked_at:null}],pagination:{total:1},coverage:{total:10,tracked:10,verified:8,conflicts:1,unpriced:1}})})
 it('requires an explicit reason and sends both versions, not a blind overwrite',async()=>{
  const w=mount(CostSyncPanel);await flushPromises();expect(w.text()).toContain('10 / 10');await w.findAll('button').find(b=>b.text()==='采用 ERP 价')!.trigger('click');expect(mocks.resolve).not.toHaveBeenCalled()
  expect(w.get('.resolution button').attributes('disabled')).toBeDefined();await w.get('.resolution input').setValue('已核对采购报价');await w.get('.resolution button').trigger('click');await flushPromises()
  expect(mocks.resolve).toHaveBeenCalledWith('QA-1',{choice:'erp',revision:3,erp_revision:7,reason:'已核对采购报价'});expect(w.text()).toContain('同步完成以 ERP 回读状态为准')
 })
 it('keeps conflict failures visible and does not retry the decision automatically',async()=>{
  mocks.resolve.mockRejectedValue(new Error('价格已变化，请刷新后确认'));const w=mount(CostSyncPanel);await flushPromises();await w.findAll('button').find(b=>b.text()==='保留系统价')!.trigger('click');await w.get('.resolution input').setValue('核对');await w.get('.resolution button').trigger('click');await flushPromises();expect(w.text()).toContain('价格已变化');expect(mocks.resolve).toHaveBeenCalledTimes(1);expect(w.find('.resolution').exists()).toBe(false)
 })
 it('does not offer active price resolution without ERP permission',async()=>{mocks.can.mockReturnValue(false);const w=mount(CostSyncPanel);await flushPromises();expect(w.findAll('button').find(b=>b.text()==='采用 ERP 价')!.attributes('disabled')).toBeDefined()})
})
