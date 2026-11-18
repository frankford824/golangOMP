// @vitest-environment jsdom
import { flushPromises, mount } from '@vue/test-utils'
import { expect, it, vi } from 'vitest'
import Fees from './Fees.vue'

const { feeAPI } = vi.hoisted(() => ({ feeAPI: vi.fn() }))
vi.mock('vue-router', () => ({
  RouterLink: { template: '<a><slot /></a>' },
  onBeforeRouteLeave: vi.fn(),
  useRoute: () => ({ query: {} }),
  useRouter: () => ({}),
}))
vi.mock('../access', () => ({
  oaEnabled: true,
  useOAAccess: () => ({ can: (action: string) => action === 'finance.apply', currentProfile: { value: { id: '7' } } }),
}))
vi.mock('../api', async (importOriginal) => ({
  ...await importOriginal<typeof import('../api')>(),
  api: feeAPI,
}))

it('shows permissions inside the fees workspace and skips the restricted report request', async () => {
  const wrapper = mount(Fees, { props: { mode: 'report' } })
  await flushPromises()
  expect(wrapper.get('nav').text()).toContain('我的申请')
  expect(wrapper.text()).toContain('当前账号没有费用报表权限')
  expect(feeAPI).not.toHaveBeenCalled()
  wrapper.unmount()
})

it('shows non-admin applicants how to get a published template', async () => {
  feeAPI.mockResolvedValue({ items: [], level: 'none' })
  const wrapper = mount(Fees, { props: { mode: 'apply' } })
  await flushPromises()
  expect(wrapper.text()).toContain('当前暂无可用费用模板，请联系财务管理员配置')
  expect(wrapper.find('form').exists()).toBe(false)
  wrapper.unmount()
})
