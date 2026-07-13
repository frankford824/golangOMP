import { describe, expect, it } from 'vitest'

import { validateClientProfile } from './clientProfileValidation'

const completeProfile = {
  real_name: '张三',
  phone: '13800000000',
  province: '江苏',
  city: '南京',
  id_card: '320100199001010000',
  gender: 'male',
  alipay_account: '13800000000',
}

describe('validateClientProfile', () => {
  it('requires every client-managed profile field', () => {
    expect(validateClientProfile({ ...completeProfile, province: '', gender: '' })).toBe('请填写省份、性别')
  })

  it('requires an 18 digit ID card number', () => {
    expect(validateClientProfile({ ...completeProfile, id_card: '32010019900101000X' })).toBe('身份证号必须为 18 位数字')
    expect(validateClientProfile({ ...completeProfile, id_card: '32010019900101000' })).toBe('身份证号必须为 18 位数字')
  })

  it('accepts a complete profile with male or female gender', () => {
    expect(validateClientProfile(completeProfile)).toBe('')
    expect(validateClientProfile({ ...completeProfile, gender: 'female' })).toBe('')
  })
})
