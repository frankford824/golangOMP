// @vitest-environment jsdom
import { flushPromises, mount } from '@vue/test-utils'
import { expect, it, vi } from 'vitest'
import Fees from './Fees.vue'

const { feeAPI, send, push, profile, actions, query } = vi.hoisted(() => ({
  feeAPI: vi.fn(), send: vi.fn(), push: vi.fn(), profile: { id: '7' },
  actions: new Set<string>(['finance.apply']), query: {} as Record<string, unknown>,
}))
vi.mock('vue-router', async () => {
  const { reactive } = await import('vue')
  type Target = { path: string; query?: Record<string, string | undefined> }
  const route = reactive({ fullPath: '', path: '', query })
  const resolve = (to: Target) => {
    const params = new URLSearchParams(Object.entries(to.query ?? {}).flatMap(([key, value]) => value === undefined ? [] : [[key, value]]))
    return { fullPath: to.path + (params.size ? '?' + params.toString() : ''), path: to.path, query: to.query ?? {} }
  }
  const stack: Target[] = []
  const apply = (target: Target) => { Object.assign(route, resolve(target)) }
  const history = { state: {} as { back?: string } }
  return {
    RouterLink: { template: `<a><slot href="#/fixture" /></a>` },
    onBeforeRouteLeave: vi.fn(),
    onBeforeRouteUpdate: vi.fn(),
    useRoute: () => {
      Object.assign(route, { fullPath: '', path: '', query: { ...query } })
      stack.length = 0
      return route
    },
    useRouter: () => ({
      options: { history },
      resolve,
      push(target: Target) {
        push(target)
        stack.push({ path: target.path, query: { ...route.query } as Record<string, string> })
        history.state.back = resolve(stack.at(-1)!).fullPath
        apply(target)
      },
      replace: apply,
      back() {
        const previous = stack.pop()
        if (previous) apply(previous)
      },
    }),
  }
})
vi.mock('../components/OAReturnLink.vue', () => ({ default: { template: '<a><slot /></a>' } }))
vi.mock('../access', () => ({
  oaEnabled: true,
  useOAAccess: () => ({ can: (action: string) => actions.has(action), currentProfile: { value: profile } }),
}))
vi.mock('../api', async (importOriginal) => ({
  ...await importOriginal<typeof import('../api')>(),
  api: feeAPI,
  send,
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
  expect(wrapper.text()).toContain('当前暂无可用费用模板')
  expect(wrapper.text()).toContain('请联系财务管理员安排模板发布及付款经办')
  expect(wrapper.find('form').exists()).toBe(false)
  wrapper.unmount()
})

it('moves an isolated request through application, two approvers, payment and return to the refreshed queue', async () => {
  const template = {
    id: 'travel', name: '演示差旅', type: '报销', channels: ['person'], fields: ['项目'],
    required: ['项目'], evidence: '发票', nodes: ['fixed:8', 'fixed:9'], threshold_cents: 0, extra: '', version: 1,
  }
  let domain = {
    id: 'DEMO-FEE', applicant_id: 7, template, amount_cents: 12050, reason: '演示用途',
    channel: 'person', holder: '演示收款人', account: '•••• 1234', bank: '演示银行',
    extra: { 项目: '演示项目' }, files: ['demo-evidence'], site: 'main',
    state: 'waiting', current: 0, version: 1, route: [8, 9],
    payment: null as { amount_cents: number; date: string; receipt: string } | null,
  }
  feeAPI.mockImplementation(async (url: string, options?: { body?: FormData }) => {
    if (url.endsWith('/templates')) return { items: [template], level: 'none' }
    if (url.endsWith('/files')) {
      return { id: (options?.body?.get('file') as File).name.startsWith('receipt') ? 'demo-receipt' : 'demo-evidence', name: (options?.body?.get('file') as File).name }
    }
    if (url.includes('queue=')) {
      return { items: url.includes('queue=pay') ? (domain.state === 'approved' ? [domain] : []) : (domain.state === 'waiting' ? [domain] : []) }
    }
    if (url.endsWith('DEMO-FEE')) return domain
    throw Error(url)
  })
  send.mockImplementation(async (url: string, body: Record<string, unknown>) => {
    if (url.endsWith('/documents')) return domain
    if (url.endsWith('/decision')) {
      expect(body.current).toBe(domain.current)
      expect(body.version).toBe(domain.version)
      const current = domain.current + 1
      domain = { ...domain, current, state: current < 2 ? 'waiting' : 'approved', version: domain.version + 1 }
      return domain
    }
    if (url.endsWith('/payment')) {
      expect(body).toEqual({ date: expect.stringMatching(/^\d{4}-\d{2}-\d{2}$/), amount_cents: 12050, receipt: 'demo-receipt' })
      domain = { ...domain, state: 'paid', version: domain.version + 1, payment: body as typeof domain.payment }
      return domain
    }
    throw Error(url)
  })
  const inputUpload = async (wrapper: ReturnType<typeof mount>, name: string) => {
    const input = wrapper.get('input[type=file]')
    Object.defineProperty(input.element, 'files', { configurable: true, value: [new File(['demo'], name, { type: 'application/pdf' })] })
    await input.trigger('change')
    await flushPromises()
  }
  const click = async (wrapper: ReturnType<typeof mount>, name: string) => {
    await wrapper.findAll('button').find(item => item.text().includes(name))!.trigger('click')
    await flushPromises()
  }
  let wrapper = mount(Fees, { props: { mode: 'apply' } })
  await flushPromises()
  await wrapper.get('form').trigger('submit')
  await flushPromises()
  expect(wrapper.text()).toContain('请输入项目')
  expect(send).not.toHaveBeenCalled()
  const inputs = wrapper.findAll('input:not([type=file])')
  for (const [index, value] of ['120.50', '演示收款人', 'DEMO-1234', '演示银行', '演示项目'].entries()) {
    await inputs[index].setValue(value)
  }
  await wrapper.get('textarea').setValue('演示用途')
  await inputUpload(wrapper, 'evidence.pdf')
  await wrapper.get('form').trigger('submit')
  await flushPromises()
  expect(send).toHaveBeenCalledWith('/oa-api/finance/documents', expect.objectContaining({ amount_cents: 12050, files: ['demo-evidence'] }))
  expect(push).toHaveBeenCalledWith({ path: '/fees/mine', query: { id: 'DEMO-FEE', submitted: '1' } })
  wrapper.unmount()

  actions.clear()
  actions.add('finance.approve')
  for (const [actor, feedback] of [['8', '申请已转交下一位审批人'], ['9', '单据已进入待付款']]) {
    profile.id = actor
    wrapper = mount(Fees, { props: { mode: 'approval' } })
    await flushPromises()
    await click(wrapper, '打开单据')
    const readCount = feeAPI.mock.calls.length
    await wrapper.get('form').trigger('submit')
    await flushPromises()
    expect(wrapper.text()).toContain(feedback)
    expect(feeAPI.mock.calls).toHaveLength(readCount)
    wrapper.unmount()
  }

  profile.id = '10'
  actions.clear()
  actions.add('finance.pay')
  vi.spyOn(window, 'confirm').mockReturnValue(true)
  wrapper = mount(Fees, { props: { mode: 'approval' } })
  await flushPromises()
  await click(wrapper, '打开单据')
  expect(wrapper.text()).toContain('应付金额')
  await inputUpload(wrapper, 'receipt.pdf')
  expect(wrapper.text()).toContain('已上传：receipt.pdf')
  await wrapper.get('form').trigger('submit')
  await flushPromises()
  expect(wrapper.text()).toContain('付款已登记，回单已归档')
  expect(wrapper.text()).toContain('下载回单')
  await click(wrapper, '返回待付款')
  expect(wrapper.text()).toContain('没有待付款单据')
  expect(feeAPI).toHaveBeenLastCalledWith('/oa-api/finance/documents?queue=pay')
  wrapper.unmount()
  vi.restoreAllMocks()
})
