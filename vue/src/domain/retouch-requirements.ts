import type { RetouchRequirementDraft } from '@/domain/types/retouch-requirement'
import type { TaskCreateFormModel } from '@/domain/types'

export function normalizeRetouchRequirementDrafts(
  items: RetouchRequirementDraft[] | undefined,
): RetouchRequirementDraft[] {
  if (!Array.isArray(items)) return []
  const out: RetouchRequirementDraft[] = []
  for (let i = 0; i < items.length; i++) {
    const item = items[i]
    if (!item) continue
    const description = String(item.description ?? '').trim()
    if (!description) continue
    out.push({
      description,
      skuCode: String(item.skuCode ?? '').trim() || undefined,
      spec: String(item.spec ?? '').trim() || undefined,
      remark: String(item.remark ?? '').trim() || undefined,
      sortOrder: item.sortOrder && item.sortOrder > 0 ? item.sortOrder : i + 1,
    })
  }
  return out
}

export function hasValidRetouchRequirementDrafts(form: Pick<TaskCreateFormModel, 'retouchRequirements'>): boolean {
  return normalizeRetouchRequirementDrafts(form.retouchRequirements).length > 0
}

/** Task-level summary for design_requirement / demand_text (never JSON). */
export function resolveRetouchTaskDesignRequirementText(
  form: Pick<TaskCreateFormModel, 'designRequirement' | 'retouchRequirements'>,
): string {
  const summary = String(form.designRequirement ?? '').trim()
  if (summary) return summary
  const items = normalizeRetouchRequirementDrafts(form.retouchRequirements)
  return items[0]?.description ?? ''
}

export function buildRetouchRequirementsPayload(
  items: RetouchRequirementDraft[] | undefined,
): Array<Record<string, unknown>> {
  return normalizeRetouchRequirementDrafts(items).map((item, index) => ({
    description: item.description,
    ...(item.skuCode ? { sku_code: item.skuCode } : {}),
    ...(item.spec ? { spec: item.spec } : {}),
    ...(item.remark ? { remark: item.remark } : {}),
    sort_order: item.sortOrder ?? index + 1,
  }))
}
