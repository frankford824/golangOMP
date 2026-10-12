export interface ClientProfileFields {
  real_name: string
  phone: string
  province: string
  city: string
  id_card: string
  gender: string
  alipay_account: string
}

const requiredFields: Array<[keyof ClientProfileFields, string]> = [
  ['real_name', '姓名'],
  ['phone', '手机号'],
  ['province', '省份'],
  ['city', '城市'],
  ['id_card', '身份证号'],
  ['gender', '性别'],
  ['alipay_account', '支付宝账号'],
]

export function validateProfileIdentity(fields: { phone: string; id_card: string }): string {
  if (fields.phone.trim() && !/^1[3-9][0-9]{9}$/.test(fields.phone.trim())) return '手机号格式不正确'
  if (fields.id_card.trim() && !/^[0-9]{17}[0-9Xx]$/.test(fields.id_card.trim())) return '身份证号须为18位，前17位数字，末位数字或X'
  return ''
}

export function validateClientProfile(fields: ClientProfileFields): string {
  const missing = requiredFields
    .filter(([key]) => !String(fields[key] || '').trim())
    .map(([, label]) => label)
  if (missing.length) return `请填写${missing.join('、')}`
  const identityError = validateProfileIdentity(fields)
  if (identityError) return identityError
  if (fields.gender !== 'female' && fields.gender !== 'male') return '性别只能选择女或男'
  return ''
}
