/**
 * 批量创建场景下为预览 SKU 分配：在已有 SKU 集合内避免重复。
 * 说明：无法获知他任务占用，真实占用仍以提交时后端校验为准；此处解决同一次创建内的重复与多次 generate 碰撞。
 */
export function collectSkuCodesFromBatchForm(
  items: { newSku?: string; purchaseSku?: string }[],
  taskKind: 'NEW_PRODUCT_DEV' | 'PURCHASE_TASK',
): Set<string> {
  const s = new Set<string>()
  for (const it of items) {
    const code = (taskKind === 'PURCHASE_TASK' ? it.purchaseSku : it.newSku)?.trim()
    if (code) s.add(code)
  }
  return s
}

export function allocateUniquePreviewSku(
  generate: () => string,
  used: Set<string>,
  maxAttempts = 200,
): { sku: string | null; skipped: number } {
  let skipped = 0
  for (let i = 0; i < maxAttempts; i++) {
    const raw = generate().trim()
    if (!raw) continue
    if (used.has(raw)) {
      skipped++
      continue
    }
    used.add(raw)
    return { sku: raw, skipped }
  }
  return { sku: null, skipped }
}
