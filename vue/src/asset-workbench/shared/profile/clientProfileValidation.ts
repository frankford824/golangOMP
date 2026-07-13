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

export function validateClientProfile(fields: ClientProfileFields): string {
  const missing = requiredFields
    .filter(([key]) => !String(fields[key] || '').trim())
    .map(([, label]) => label)
  if (missing.length) return `请填写${missing.join('、')}`
  if (!/^[0-9]{18}$/.test(fields.id_card.trim())) return '身份证号必须为 18 位数字'
  if (fields.gender !== 'female' && fields.gender !== 'male') return '性别只能选择女或男'
  return ''
}
