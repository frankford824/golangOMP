// @vitest-environment jsdom
import { mount } from '@vue/test-utils'
import { ref } from 'vue'
import { beforeEach, describe, expect, it, vi } from 'vitest'

const mocks = vi.hoisted(() => ({
  bootstrapHook: vi.fn(),
  logout: vi.fn(),
  upsertMyProfile: vi.fn(),
}))

vi.mock('@aw/app/useAssetWorkbenchBootstrap', () => ({
  useAssetWorkbenchBootstrap: mocks.bootstrapHook,
}))

vi.mock('@aw/app/useWorkbenchSession', () => ({
  useWorkbenchSession: () => ({ logout: mocks.logout }),
}))

vi.mock('@aw/shared/api/assetWorkbenchApi', () => ({
  assetWorkbenchApi: { upsertMyProfile: mocks.upsertMyProfile },
}))

import AccountPage from './AccountPage.vue'

function fieldValue(wrapper: ReturnType<typeof mount>, label: string) {
  const field = wrapper.findAll('label').find((item) => item.text().startsWith(label))
  if (!field) throw new Error(`missing field ${label}`)
  return (field.find('input, select').element as HTMLInputElement | HTMLSelectElement).value
}

describe('AccountPage', () => {
  beforeEach(() => {
    mocks.bootstrapHook.mockReset()
    mocks.bootstrapHook.mockReturnValue({
      bootstrap: ref({
        user: { username: 'piece-worker' },
        profile: {
          id: 10,
          user_id: 302,
          worker_type: 'parttime',
          job_grade: 'J1',
          real_name: '张三',
          phone: '13800000302',
          province: '江苏',
          city: '苏州',
          id_card: '320500199001010302',
          gender: 'male',
          alipay_account: 'piece-worker@example.com',
          status: 'active',
          pii_completed: true,
        },
      }),
      loading: ref(false),
      error: ref(''),
      refresh: vi.fn().mockResolvedValue(undefined),
    })
  })

  it('hydrates a cached profile immediately when the page is opened again', () => {
    const wrapper = mount(AccountPage)

    expect(fieldValue(wrapper, '姓名')).toBe('张三')
    expect(fieldValue(wrapper, '手机号')).toBe('13800000302')
    expect(fieldValue(wrapper, '省份')).toBe('江苏')
    expect(fieldValue(wrapper, '城市')).toBe('苏州')
    expect(fieldValue(wrapper, '身份证')).toBe('320500199001010302')
    expect(fieldValue(wrapper, '支付宝')).toBe('piece-worker@example.com')
    expect(wrapper.text()).toContain('资料已完成')
  })

  it('marks every client-managed field as required and blocks an incomplete save', async () => {
    const wrapper = mount(AccountPage)
    const idCard = wrapper.findAll('label').find((item) => item.text().startsWith('身份证号'))?.find('input')
    const gender = wrapper.findAll('label').find((item) => item.text().startsWith('性别'))?.find('select')

    expect(idCard?.attributes('required')).toBeDefined()
    expect(idCard?.attributes('pattern')).toBe('[0-9]{18}')
    expect(idCard?.attributes('minlength')).toBe('18')
    expect(idCard?.attributes('maxlength')).toBe('18')
    expect(gender?.attributes('required')).toBeDefined()
    expect(gender?.findAll('option').map((option) => option.attributes('value')).filter(Boolean)).toEqual(['female', 'male'])
    expect(gender?.find('option[value=""]').attributes()).toHaveProperty('disabled')

    const city = wrapper.findAll('label').find((item) => item.text().startsWith('城市'))?.find('select')
    await city?.setValue('')
    await wrapper.find('.aw-form-grid .aw-primary-button').trigger('click')

    expect(mocks.upsertMyProfile).not.toHaveBeenCalled()
    expect(wrapper.text()).toContain('请填写城市')
  })
})
