/**
 * GET /v1/tasks/{id}/detail 常将主档与 `task_detail` 子表分开展示，仅传 `envelope.task` 会丢设计需求、
 * 分类、备注、参考图等。合并为扁平 raw 再交给 `normalizeBackendTask`。
 */
export function mergeDetailEnvelopeIntoTaskRaw(
  envelope: Record<string, unknown>,
): Record<string, unknown> {
  const taskObj = envelope.task
  if (!taskObj || typeof taskObj !== 'object') {
    return taskObj as Record<string, unknown>
  }
  const merged: Record<string, unknown> = { ...(taskObj as Record<string, unknown>) }
  for (const k of [
    'workflow',
    'design_sub_status',
    'designSubStatus',
    'creator_id',
    'creatorId',
    'requester_id',
    'requesterId',
    'designer_id',
    'designerId',
    'assignee_id',
    'assigneeId',
    'current_handler_id',
    'currentHandlerId',
    'creator_name',
    'creatorName',
    'requester_name',
    'requesterName',
    'designer_name',
    'designerName',
    'assignee_name',
    'assigneeName',
    'current_handler_name',
    'currentHandlerName',
    'sku_items',
    'skuItems',
    'asset_versions',
    'assetVersions',
    'retouch_requirements',
    'retouchRequirements',
    'matched_rule_governance',
    'matchedRuleGovernance',
    'override_summary',
    'overrideSummary',
    'governance_audit_summary',
    'governanceAuditSummary',
    'override_governance_boundary',
    'overrideGovernanceBoundary',
    'platform_entry_boundary',
    'platformEntryBoundary',
  ] as const) {
    if (envelope[k] != null) merged[k] = envelope[k]
  }
  const detail = (envelope.task_detail ?? envelope.taskDetail) as Record<string, unknown> | undefined
  if (detail && typeof detail === 'object') {
    const d = detail
    const nonEmpty = (v: unknown) => v != null && String(v).trim() !== ''

    // 成本与数量字段从 task_detail 提升到 task root，供统一详情映射读取。
    if (typeof d.cost_price === 'number' && Number.isFinite(d.cost_price)) {
      merged.cost_price = d.cost_price
    }
    if (typeof d.quantity === 'number' && Number.isFinite(d.quantity)) {
      merged.quantity = d.quantity
    }
    if (typeof d.cost_price_mode === 'string' && d.cost_price_mode.trim() !== '') {
      merged.cost_price_mode = d.cost_price_mode.trim()
    }
    if (typeof d.set_mode_hint === 'boolean') {
      merged.set_mode_hint = d.set_mode_hint
    }

    if (nonEmpty(d.category_code ?? d.categoryCode)) {
      merged.category_code = d.category_code ?? d.categoryCode
    }
    if (nonEmpty(d.category_name ?? d.categoryName)) {
      merged.category_name = d.category_name ?? d.categoryName
    }
    if (nonEmpty(d.category)) {
      merged.category = d.category
    }
    const designReq = d.design_requirement ?? d.designRequirement
    const demandText = d.demand_text
    if (typeof designReq === 'string' && designReq.trim() !== '') {
      merged.design_requirement = designReq.trim()
    } else if (typeof demandText === 'string' && demandText.trim() !== '') {
      merged.design_requirement = demandText.trim()
    }
    if (typeof d.change_request === 'string' && d.change_request.trim() !== '') {
      merged.change_request = d.change_request
    }
    if (typeof d.copy_text === 'string' && d.copy_text.trim() !== '') {
      merged.copy_content = d.copy_text
    }
    if (typeof d.style_keywords === 'string' && d.style_keywords.trim() !== '') {
      merged.style_keywords = d.style_keywords
    }
    const noteN = d.note
    const remarkR = d.remark
    const noteStr = typeof noteN === 'string' && noteN.trim() !== '' ? noteN.trim() : ''
    const remarkStr = typeof remarkR === 'string' && remarkR.trim() !== '' ? remarkR.trim() : ''
    if (noteStr || remarkStr) {
      merged.note = noteStr || remarkStr
    }
    for (const k of [
      'spec_text',
      'size_text',
      'material',
      'craft_text',
      'process',
      'product_short_name',
      'reference_link',
    ] as const) {
      if (nonEmpty(d[k])) merged[k] = d[k]
    }
    for (const k of [
      'filing_status',
      'filing_error_message',
      'filing_trigger_source',
      'erp_sync_required',
      'erp_sync_version',
    ] as const) {
      if (d[k] != null) merged[k] = d[k]
    }
  }

  // Canonical source for detail envelope references (v1 contract).
  const canonicalRefs = envelope.reference_file_refs ?? envelope.referenceFileRefs
  // Legacy fallback kept for backward compatibility with older payloads.
  const legacyRefsJson =
    detail && typeof detail === 'object'
      ? (detail.reference_file_refs_json ?? detail.referenceFileRefsJson)
      : undefined
  let refArray: unknown[] | null = null
  if (Array.isArray(canonicalRefs) && canonicalRefs.length > 0) {
    refArray = canonicalRefs
  } else if (
    typeof legacyRefsJson === 'string' &&
    legacyRefsJson.trim() !== '' &&
    legacyRefsJson.trim() !== '[]'
  ) {
    try {
      const parsed = JSON.parse(legacyRefsJson) as unknown
      if (Array.isArray(parsed) && parsed.length > 0) refArray = parsed
    } catch {
      // ignore
    }
  }
  if (refArray) {
    merged.reference_file_refs = refArray.map((item) => {
      if (!item || typeof item !== 'object') return item
      const o = { ...(item as Record<string, unknown>) }
      if (!o.download_url && typeof o.url === 'string') o.download_url = o.url
      return o
    })
  }
  return merged
}
