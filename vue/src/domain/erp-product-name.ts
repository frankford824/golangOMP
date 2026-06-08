export const ERP_PRODUCT_NAME_MAX_LENGTH = 40

export function erpProductNameLength(value: unknown): number {
  return new TextEncoder().encode(String(value ?? '').trim()).length
}

export function isErpProductNameTooLong(value: unknown): boolean {
  return erpProductNameLength(value) > ERP_PRODUCT_NAME_MAX_LENGTH
}

export function erpProductNameHint(value: unknown): string {
  const length = erpProductNameLength(value)
  if (length > ERP_PRODUCT_NAME_MAX_LENGTH) {
    return `已超出 ${length - ERP_PRODUCT_NAME_MAX_LENGTH} 字节。产品名称会同步为聚水潭简称，最多 ${ERP_PRODUCT_NAME_MAX_LENGTH} 字节`
  }
  return `名称会同步为聚水潭简称，最多 ${ERP_PRODUCT_NAME_MAX_LENGTH} 字节，剩余 ${ERP_PRODUCT_NAME_MAX_LENGTH - length} 字节`
}

export function erpProductNameLimitMessage(label = '产品名称'): string {
  return `${label}会同步为聚水潭简称，最多 ${ERP_PRODUCT_NAME_MAX_LENGTH} 字节（中文约 13 个字，英文 40 个字符），请精简后再提交`
}
