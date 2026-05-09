const INTERNAL_CATEGORY_CODE_RE = /^(KT|OUT)_[A-Z0-9_]+$/

/**
 * business-info category field routing:
 * - Internal code (prefix KT_ or OUT_) => category_code
 * - Display text / i_id => category
 */
export function buildCategoryPatchFields(
  value: string | null | undefined,
): { category_code?: string; category?: string } {
  const category = String(value ?? '').trim()
  if (!category) return {}
  if (INTERNAL_CATEGORY_CODE_RE.test(category)) {
    return { category_code: category }
  }
  return { category }
}
