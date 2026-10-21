// @vitest-environment jsdom
import { flushPromises, mount } from '@vue/test-utils'
import { createMemoryHistory, createRouter } from 'vue-router'
import { beforeEach, describe, expect, it, vi } from 'vitest'

const mocks = vi.hoisted(() => ({
  bootstrap: vi.fn(),
  listProfiles: vi.fn(),
  getProfile: vi.fn(),
  upsertProfile: vi.fn(),
  upsertMyProfile: vi.fn(),
  setSettlementVisibility: vi.fn(),
}))

vi.mock('@aw/shared/api/assetWorkbenchApi', () => ({
  assetWorkbenchApi: mocks,
}))

vi.mock('@aw/app/useRoutePageCopy', () => ({
  useRoutePageCopy: () => ({ label: '人员定级', subtitle: '维护人员资料' }),
}))

import PeoplePage from './PeoplePage.vue'

const maskedProfile = {
  id: 10,
  user_id: 330,
  worker_type: 'parttime',
  job_grade: 'J1',
  real_name: '测试李梅',
  phone: '138****0330',
  province: '江苏',
  city: '南京',
  id_card: '**************0330',
  gender: 'female',
  alipay_account: 'te*****@mail.com',
  status: 'active',
  pii_completed: true,
}

const completeProfile = {
  ...maskedProfile,
  phone: '13800000330',
  id_card: '320100199001010330',
  alipay_account: 'test330@mail.com',
}

function editorField(wrapper: ReturnType<typeof mount>, label: string) {
  const editor = wrapper.find('.aw-grade-editor')
  const field = editor.findAll('label').find((item) => item.text().startsWith(label))
  if (!field) throw new Error(`missing editor field ${label}`)
  return field.find('input, select')
}

describe('PeoplePage', () => {
  it('saves client income visibility independently of grading and PII', async () => {
    mocks.setSettlementVisibility.mockResolvedValue({ user_id: 330, settlement_visible: false })
    const pending = { ...completeProfile, job_grade: '', status: 'pending' }
    mocks.listProfiles.mockResolvedValue({ items: [pending], total: 1 })
    mocks.getProfile.mockResolvedValue(pending)
    const router = createRouter({ history: createMemoryHistory(), routes: [{ path: '/settings/people', component: PeoplePage }] })
    await router.push('/settings/people')
    const wrapper = mount(PeoplePage, { global: { plugins: [router] } })
    await flushPromises()
    await wrapper.get('[role="switch"]').trigger('click')
    const save = wrapper.findAll('button').find((button) => button.text() === '保存展示权限')!
    await save.trigger('click')
    await flushPromises()
    expect(mocks.setSettlementVisibility).toHaveBeenCalledWith(330, false)
    expect(mocks.upsertProfile).not.toHaveBeenCalled()
    expect(wrapper.text()).toContain('工资和计件记录保持不变')
    wrapper.unmount()
  })
  beforeEach(() => {
    vi.clearAllMocks()
    mocks.bootstrap.mockResolvedValue({ capabilities: ['asset.workbench.profile.manage'] })
    mocks.listProfiles.mockResolvedValue({ items: [maskedProfile], total: 1 })
    mocks.getProfile.mockResolvedValue(completeProfile)
    mocks.upsertProfile.mockResolvedValue(completeProfile)
    mocks.upsertMyProfile.mockResolvedValue(completeProfile)
  })

  it('saves the selected pending person without assigning a grade or editing the admin profile', async () => {
    const pending = { ...completeProfile, job_grade: '', status: 'pending' }
    mocks.listProfiles.mockResolvedValue({ items: [pending], total: 1 })
    mocks.getProfile.mockResolvedValue(pending)
    mocks.upsertProfile.mockResolvedValue(pending)
    const router = createRouter({ history: createMemoryHistory(), routes: [{ path: '/settings/people', component: PeoplePage }] })
    await router.push('/settings/people')
    await router.isReady()
    const wrapper = mount(PeoplePage, { global: { plugins: [router] } })
    await flushPromises()

    expect((editorField(wrapper, '状态').element as HTMLSelectElement).value).toBe('pending')
    expect(wrapper.find('.aw-two-column').exists()).toBe(false)
    const save = wrapper.find('.aw-page-bar__actions .aw-primary-button')
    expect(save.attributes('disabled')).toBeUndefined()
    await save.trigger('click')
    await flushPromises()
    expect(mocks.upsertProfile).toHaveBeenCalledWith(330, expect.objectContaining({ job_grade: '', status: 'pending' }))
    expect(mocks.upsertMyProfile).not.toHaveBeenCalled()
    expect(wrapper.get('[role="status"]').text()).toContain('已更新 测试李梅 的人员资料')

    await editorField(wrapper, '状态').setValue('active')
    expect(wrapper.find('.aw-grade-editor .aw-primary-button').attributes('disabled')).toBeDefined()
  })

  it('disables both manager save buttons when full profile details cannot be read', async () => {
    mocks.getProfile.mockRejectedValue(new Error('完整资料加载失败'))
    const router = createRouter({ history: createMemoryHistory(), routes: [{ path: '/settings/people', component: PeoplePage }] })
    await router.push('/settings/people')
    await router.isReady()
    const wrapper = mount(PeoplePage, { global: { plugins: [router] } })
    await flushPromises()
    expect(wrapper.find('.aw-page-bar__actions .aw-primary-button').attributes('disabled')).toBeDefined()
    expect(wrapper.find('.aw-grade-editor .aw-primary-button').attributes('disabled')).toBeDefined()
    expect(mocks.upsertProfile).not.toHaveBeenCalled()
    expect(mocks.upsertMyProfile).not.toHaveBeenCalled()
  })

  it('loads the authorized full profile and submits every PII field', async () => {
    const router = createRouter({
      history: createMemoryHistory(),
      routes: [{ path: '/settings/people', component: PeoplePage }],
    })
    await router.push('/settings/people')
    await router.isReady()
    const wrapper = mount(PeoplePage, { global: { plugins: [router] } })
    await flushPromises()

    expect(mocks.getProfile).toHaveBeenCalledWith(330)
    expect((editorField(wrapper, '手机').element as HTMLInputElement).value).toBe('13800000330')
    expect((editorField(wrapper, '身份证号').element as HTMLInputElement).value).toBe('320100199001010330')
    expect((editorField(wrapper, '支付宝账号').element as HTMLInputElement).value).toBe('test330@mail.com')
    expect((editorField(wrapper, '省份').element as HTMLSelectElement).value).toBe('江苏')
    expect((editorField(wrapper, '城市').element as HTMLSelectElement).value).toBe('南京')

    await wrapper.find('.aw-grade-editor .aw-primary-button').trigger('click')
    await flushPromises()

    expect(mocks.upsertProfile).toHaveBeenCalledWith(330, expect.objectContaining({
      phone: '13800000330',
      id_card: '320100199001010330',
      alipay_account: 'test330@mail.com',
      province: '江苏',
      city: '南京',
    }))
  })

  it('does not call the manager profile list for a self-service user', async () => {
    mocks.bootstrap.mockResolvedValue({
      capabilities: ['asset.workbench.profile'],
      profile: completeProfile,
    })
    const router = createRouter({
      history: createMemoryHistory(),
      routes: [{ path: '/settings/people', component: PeoplePage }],
    })
    await router.push('/settings/people')
    await router.isReady()

    const wrapper = mount(PeoplePage, { global: { plugins: [router] } })
    await flushPromises()

    expect(mocks.listProfiles).not.toHaveBeenCalled()
    expect(wrapper.text()).toContain('我的资料')
    const selfPanel = wrapper.find('.aw-two-column .aw-panel')
    const nameField = selfPanel.findAll('label').find((item) => item.text().startsWith('姓名'))
    expect((nameField?.find('input').element as HTMLInputElement).value).toBe('测试李梅')
  })

  it('normalizes an X suffix on the manager profile editor before saving', async () => {
    const router = createRouter({ history: createMemoryHistory(), routes: [{ path: '/settings/people', component: PeoplePage }] })
    await router.push('/settings/people')
    await router.isReady()
    const wrapper = mount(PeoplePage, { global: { plugins: [router] } })
    await flushPromises()
    await editorField(wrapper, '身份证号').setValue('32010019900101033x')
    await wrapper.find('.aw-grade-editor .aw-primary-button').trigger('click')
    await flushPromises()
    expect(mocks.upsertProfile).toHaveBeenCalledWith(330, expect.objectContaining({ id_card: '32010019900101033X' }))
  })

  it('blocks an incomplete self-service profile before calling the API', async () => {
    mocks.bootstrap.mockResolvedValue({
      capabilities: ['asset.workbench.profile'],
      profile: completeProfile,
    })
    const router = createRouter({
      history: createMemoryHistory(),
      routes: [{ path: '/settings/people', component: PeoplePage }],
    })
    await router.push('/settings/people')
    await router.isReady()
    const wrapper = mount(PeoplePage, { global: { plugins: [router] } })
    await flushPromises()

    const selfPanel = wrapper.find('.aw-two-column .aw-panel')
    const idCard = selfPanel.findAll('label').find((item) => item.text().startsWith('身份证号'))?.find('input')
    await idCard?.setValue('32010019900101033')
    await wrapper.find('.aw-page-bar__actions .aw-primary-button').trigger('click')

    expect(mocks.upsertMyProfile).not.toHaveBeenCalled()
    expect(wrapper.text()).toContain('身份证号须为18位')
  })
})
