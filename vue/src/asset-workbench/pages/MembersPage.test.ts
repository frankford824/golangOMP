// @vitest-environment jsdom
import { flushPromises, mount } from '@vue/test-utils'
import { ref } from 'vue'
import { beforeEach, describe, expect, it, vi } from 'vitest'

const api = vi.hoisted(() => ({ listMembers: vi.fn(), updateMemberRoles: vi.fn() }))
vi.mock('@aw/shared/api/assetWorkbenchApi', () => ({ assetWorkbenchApi: api }))
vi.mock('@aw/app/useRoutePageCopy', () => ({ useRoutePageCopy: () => ({ label: '成员维护', subtitle: '角色授权' }) }))
vi.mock('@aw/app/useAssetWorkbenchBootstrap', () => ({ useAssetWorkbenchBootstrap: () => ({ bootstrap: ref({capabilities:['asset.workbench.member.identity']}), refresh: vi.fn().mockResolvedValue(undefined) }) }))
import MembersPage from './MembersPage.vue'

const member = { user_id:331,username:'never-logged-in',real_name:'新授权人员',status:'active',worker_type:'',job_grade:'',roles:['AssetSubmitter'],policy_revision:8,can_edit_roles:true }
describe('shared member authorization', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    api.listMembers.mockResolvedValue({items:[{...member}], total:1})
    api.updateMemberRoles.mockResolvedValue({...member,roles:['AssetSubmitter','AssetManager'],policy_revision:9})
  })
  it('sends the shared revision and refreshes all members after a change', async () => {
    const wrapper = mount(MembersPage)
    await flushPromises()
    expect(wrapper.text()).toContain('新授权人员')
    await wrapper.get('.aw-member-row__controls input').setValue(true)
    await flushPromises()
    expect(api.updateMemberRoles).toHaveBeenCalledWith(331,['AssetSubmitter','AssetManager'],'workbench role update',8)
    expect(api.listMembers).toHaveBeenCalledTimes(2)
    wrapper.unmount()
  })
  it('refreshes on return from the operations app and removes the listener on unmount', async () => {
    const wrapper = mount(MembersPage)
    await flushPromises()
    window.dispatchEvent(new Event('focus'))
    await flushPromises()
    expect(api.listMembers).toHaveBeenCalledTimes(2)
    wrapper.unmount()
    window.dispatchEvent(new Event('focus'))
    expect(api.listMembers).toHaveBeenCalledTimes(2)
  })
  it('does not offer inherited policies as editable direct roles', async () => {
    api.listMembers.mockResolvedValue({items:[{...member,roles_managed_externally:true,can_edit_roles:false}],total:1})
    const wrapper = mount(MembersPage)
    await flushPromises()
    expect(wrapper.get('.aw-member-row__controls input').attributes('disabled')).toBeDefined()
    expect(wrapper.text()).toContain('包含组织继承角色')
    wrapper.unmount()
  })
  it('uses the supported page size and server total instead of truncating the roster at 20', async () => {
    api.listMembers.mockResolvedValue({items:[{...member}],total:51})
    const wrapper = mount(MembersPage)
    await flushPromises()
    expect(api.listMembers).toHaveBeenLastCalledWith(expect.objectContaining({page:1,page_size:50}),expect.any(AbortSignal))
    expect(wrapper.text()).toContain('共 51 人')
    await wrapper.findAll('button').find(button=>button.text()==='下一页')!.trigger('click')
    await flushPromises()
    expect(api.listMembers).toHaveBeenLastCalledWith(expect.objectContaining({page:2,page_size:50}),expect.any(AbortSignal))
    await wrapper.get('input[placeholder="输入姓名、账号或手机号"]').setValue('新授权人员')
    await wrapper.findAll('button').find(button=>button.text()==='搜索')!.trigger('click')
    await flushPromises()
    expect(api.listMembers).toHaveBeenLastCalledWith(expect.objectContaining({page:1,q:'新授权人员'}),expect.any(AbortSignal))
    wrapper.unmount()
  })
  it('returns to the last available page after the filtered roster shrinks', async () => {
    api.listMembers.mockResolvedValueOnce({items:[{...member}],total:51})
      .mockResolvedValueOnce({items:[],total:50})
      .mockResolvedValueOnce({items:[{...member}],total:50})
    const wrapper = mount(MembersPage)
    await flushPromises()
    await wrapper.findAll('button').find(button=>button.text()==='下一页')!.trigger('click')
    await flushPromises()
    expect(api.listMembers).toHaveBeenLastCalledWith(expect.objectContaining({page:1,page_size:50}),expect.any(AbortSignal))
    expect(wrapper.text()).toContain('第 1 / 1 页')
    wrapper.unmount()
  })
})
