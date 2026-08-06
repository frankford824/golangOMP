export const ERP_PRODUCT_NAME_MAX_LENGTH = 40

export function erpProductNameLength(value: unknown): number {
  return Array.from(String(value ?? '').trim()).length
}

export function isErpProductNameTooLong(value: unknown): boolean {
  return erpProductNameLength(value) > ERP_PRODUCT_NAME_MAX_LENGTH
}

export function erpProductNameHint(value: unknown): string {
  const length = erpProductNameLength(value)
  if (length > ERP_PRODUCT_NAME_MAX_LENGTH) {
    return `已超出 ${length - ERP_PRODUCT_NAME_MAX_LENGTH} 个字，名称与 ERP 简称需保持一致`
  }
  if (length === 0) return `名称将同步为 ERP 简称，最多可填写 ${ERP_PRODUCT_NAME_MAX_LENGTH} 个字`
  return `名称将同步为 ERP 简称，还可输入 ${ERP_PRODUCT_NAME_MAX_LENGTH - length} 个字`
}

export function erpProductNameLimitMessage(label = '产品名称'): string {
  return `${label}将同步为 ERP 简称，最多可填写 ${ERP_PRODUCT_NAME_MAX_LENGTH} 个字，请精简后再提交`
}

export function erpProductNameError(value: unknown, label = '产品名称'): string {
  if (!isErpProductNameTooLong(value)) return ''
  // 只说「超出上限」时运营得自己数字数，尤其是从旧系统整段粘过来的长名称。
  const overflow = erpProductNameLength(value) - ERP_PRODUCT_NAME_MAX_LENGTH
  return `${erpProductNameLimitMessage(label)}（当前超出 ${overflow} 个字）`
}
