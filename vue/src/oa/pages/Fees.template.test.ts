// @vitest-environment jsdom
import { flushPromises, mount } from '@vue/test-utils'
import { NSelect } from 'naive-ui'
import { afterEach, beforeEach, expect, it, vi } from 'vitest'
import { RouterView, createMemoryHistory, createRouter } from 'vue-router'
import BaseSelect from '@/components/base/BaseSelect.vue'
import FeePersonSelect from '../components/FeePersonSelect.vue'
import Fees from './Fees.vue'

const { api, send, access } = vi.hoisted(() => ({
  api: vi.fn(),
  send: vi.fn(),
  access: { admin: true },
}))
vi.mock('../access', () => ({
  oaEnabled: true,
  useOAAccess: () => ({ can: (action: string) => action === 'finance.templates' || (action === 'finance.admin' && access.admin), currentProfile: { value: { id: '7' } } }),
}))
vi.mock('../api', async (importOriginal) => ({
  ...await importOriginal<typeof import('../api')>(),
  api,
  send,
}))

const reviewers = [
  { id: 42, name: '王一', employee_no: 23 },
  { id: 51, name: '李二', employee_no: 24 },
  { id: 68, name: '张三', employee_no: 25 },
]

beforeEach(() => {
  access.admin = true
  api.mockReset()
  send.mockReset()
  api.mockImplementation(async (url: string) => url.includes('/templates') ? { items: [], level: 'publish' } : { items: [] })
  send.mockImplementation(async (url: string, body: { q?: string }) =>
    url.endsWith('/approvers/search') ? { items: reviewers.filter(person => String(person.employee_no) === body.q) } : {})
})
afterEach(() => vi.restoreAllMocks())

async function openEditor() {
  const router = createRouter({ history: createMemoryHistory(), routes: [{ path: '/:pathMatch(.*)*', component: Fees, props: { mode: 'templates' } }] })
  await router.push('/fees/templates?new=1')
  const wrapper = mount(RouterView, { global: { plugins: [router] } })
  await flushPromises()
  return wrapper
}

type TemplateEditor = Awaited<ReturnType<typeof openEditor>>
const button = (wrapper: TemplateEditor, label: string) => wrapper.findAll('form button').find(element => element.text() === label)!
const select = (wrapper: TemplateEditor, label: string) => wrapper.findAllComponents(BaseSelect).find(element => element.props('label')?.replace(/ \*$/, '') === label)!
async function selectValue(wrapper: TemplateEditor, label: string, value: string) {
  select(wrapper, label).vm.$emit('update:modelValue', value)
  await flushPromises()
}

async function fillBasicFields(wrapper: TemplateEditor) {
  await wrapper.get('input[placeholder="例如差旅报销"]').setValue('差旅费用')
  await selectValue(wrapper, '费用类型', '报销')
  await wrapper.get('input[placeholder="例如发票"]').setValue('发票')
  await wrapper.get('input[value="person"]').setValue(true)
}

async function chooseReviewer(wrapper: TemplateEditor, label: string, employeeNumber: string, id: number) {
  const picker = wrapper.findAllComponents(FeePersonSelect).find(component => component.props('label') === label)!
  const select = picker.getComponent(NSelect)
  select.vm.$emit('search', employeeNumber)
  await vi.waitFor(() => expect(select.props('options')).toEqual([expect.objectContaining({ value: id })]))
  expect(send).toHaveBeenCalledWith('/oa-api/finance/approvers/search', { q: employeeNumber, ids: '', after_id: 0 })
  select.vm.$emit('update:value', id)
  await flushPromises()
}

it('requires an explicit reviewer and publishes fixed nodes in displayed order with an additional reviewer', async () => {
  const wrapper = await openEditor()
  try {
    await fillBasicFields(wrapper)
    await button(wrapper, '添加审批节点').trigger('click')
    expect(select(wrapper, '第 1 步审批人类型').props('modelValue')).toBe('')
    await button(wrapper, '发布模板').trigger('click')
    expect(wrapper.text()).toContain('请为第 1 步选择审批人')
    expect(send).not.toHaveBeenCalled()

    await selectValue(wrapper, '第 1 步审批人类型', 'fixed')
    await button(wrapper, '发布模板').trigger('click')
    expect(wrapper.text()).toContain('请为第 1 步选择审批人')
    expect(send).not.toHaveBeenCalled()
    await chooseReviewer(wrapper, '第 1 步指定审批人', '23', 42)

    await button(wrapper, '添加审批节点').trigger('click')
    await selectValue(wrapper, '第 2 步审批人类型', 'fixed')
    await chooseReviewer(wrapper, '第 2 步指定审批人', '24', 51)
    await wrapper.get('button[aria-label="上移第 2 步"]').trigger('click')
    expect(wrapper.findAllComponents(FeePersonSelect).find(component => component.props('label') === '第 1 步指定审批人')?.props('modelValue')?.id).toBe(51)

    await wrapper.get('input[placeholder="留空或填 0 则不加签"]').setValue('100.05')
    await button(wrapper, '发布模板').trigger('click')
    expect(wrapper.text()).toContain('请选择加签审批人')
    expect(send.mock.calls.every(([url]) => url.endsWith('/approvers/search'))).toBe(true)
    await selectValue(wrapper, '加签审批人类型', 'fixed')
    await chooseReviewer(wrapper, '加签指定审批人', '25', 68)
    await button(wrapper, '发布模板').trigger('click')
    await flushPromises()
    expect(send).toHaveBeenCalledWith(expect.stringMatching(/^\/oa-api\/finance\/templates\//), expect.objectContaining({
      type: '报销', nodes: ['fixed:51', 'fixed:42'], threshold_cents: 10005, extra: 'fixed:68', publish: true,
    }), 'PUT')
  } finally {
    wrapper.unmount()
  }
})

it('preserves draft-only delegation and clears an unused additional reviewer', async () => {
  access.admin = false
  api.mockResolvedValue({ items: [], level: 'draft' })
  const wrapper = await openEditor()
  try {
    expect(button(wrapper, '发布模板')).toBeUndefined()
    await fillBasicFields(wrapper)
    await button(wrapper, '添加审批节点').trigger('click')
    await selectValue(wrapper, '第 1 步审批人类型', 'manager')
    const threshold = wrapper.get('input[placeholder="留空或填 0 则不加签"]')
    await threshold.setValue('100')
    await selectValue(wrapper, '加签审批人类型', 'manager')
    await threshold.setValue('0')
    await wrapper.get('form').trigger('submit')
    await flushPromises()
    expect(send).toHaveBeenCalledWith(expect.stringMatching(/^\/oa-api\/finance\/templates\//), expect.objectContaining({
      nodes: ['manager'], threshold_cents: 0, extra: '', publish: false,
    }), 'PUT')
  } finally {
    wrapper.unmount()
  }
})
