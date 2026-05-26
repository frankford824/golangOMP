export const ERP_PRODUCT_NAME_MAX_LENGTH = 50

export function erpProductNameLength(value: unknown): number {
  return Array.from(String(value ?? '').trim()).length
}

export function isErpProductNameTooLong(value: unknown): boolean {
  return erpProductNameLength(value) > ERP_PRODUCT_NAME_MAX_LENGTH
}

export function erpProductNameHint(value: unknown): string {
  const length = erpProductNameLength(value)
  if (length > ERP_PRODUCT_NAME_MAX_LENGTH) {
    return `已超出 ${length - ERP_PRODUCT_NAME_MAX_LENGTH} 个字符，产品名称最多 ${ERP_PRODUCT_NAME_MAX_LENGTH} 个字符`
  }
  return `最多 ${ERP_PRODUCT_NAME_MAX_LENGTH} 个字符，剩余 ${ERP_PRODUCT_NAME_MAX_LENGTH - length} 个`
}
