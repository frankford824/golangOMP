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

  it('accepts X and x only in the last ID card position', () => {
    expect(validateClientProfile({ ...completeProfile, id_card: '32010019900101000X' })).toBe('')
    expect(validateClientProfile({ ...completeProfile, id_card: '32010019900101000x' })).toBe('')
    expect(validateClientProfile({ ...completeProfile, id_card: '32010019900101000' })).toContain('身份证号须为18位')
    expect(validateClientProfile({ ...completeProfile, id_card: 'X20100199001010000' })).toContain('身份证号须为18位')
  })

  it('accepts a complete profile with male or female gender', () => {
    expect(validateClientProfile(completeProfile)).toBe('')
    expect(validateClientProfile({ ...completeProfile, gender: 'female' })).toBe('')
  })
})
