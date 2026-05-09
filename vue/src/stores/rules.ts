/**
 * 规则及模板 Store
 * v0.6 对齐：FRONTEND_ALIGNMENT_v0.5(1).md G 节
 * 接口：GET /v1/rule-templates、GET /v1/rule-templates/:type、PUT /v1/rule-templates/:type
 */
import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import type { CodeRule } from '@/types'
import { rulesApi, type RuleTemplateType, type RuleTemplate } from '@/services/api/rulesApi'
import { getBeijingDateCompactString } from '@/utils/date'

const TEMPLATE_TYPES: RuleTemplateType[] = ['cost-pricing', 'product-code', 'short-name']

const TEMPLATE_TYPE_LABELS: Record<RuleTemplateType, string> = {
  'cost-pricing': '成本核算规则',
  'product-code': '产品编码规则',
  'short-name': '商品简称规则',
}

/** 从后端模板提取 config（含 config_json 解析） */
function extractTemplateConfig(t: Record<string, unknown>): Record<string, unknown> {
  let data = { ...t }
  if (typeof t.config_json === 'string') {
    try {
      const parsed = JSON.parse(t.config_json) as Record<string, unknown>
      data = { ...t, ...parsed }
    } catch {
      // 解析失败时保持原样
    }
  }
  return data
}

/** v0.6 对齐：将后端模板转为 CodeRule 兼容格式，供现有 UI 使用 */
function templateToCodeRule(type: RuleTemplateType, data: Record<string, unknown>): CodeRule {
  const raw = extractTemplateConfig(data) as Record<string, unknown>
  return {
    id: type,
    name: String(raw.name ?? raw.template_name ?? TEMPLATE_TYPE_LABELS[type]),
    ruleType: type === 'product-code' ? 'sku' : 'taskNo',
    prefix: String(raw.prefix ?? ''),
    dateFormat: String(raw.date_format ?? raw.dateFormat ?? 'yyyyMMdd'),
    locationCode: String(raw.location_code ?? raw.locationCode ?? ''),
    bizTypeCode: String(raw.biz_type_code ?? raw.bizTypeCode ?? ''),
    sequenceDigits: Number(raw.sequence_digits ?? raw.sequenceDigits ?? 3),
    dailyReset: Boolean(raw.daily_reset ?? raw.dailyReset ?? true),
    enabled: Boolean(raw.enabled ?? true),
  }
}

/** v0.6 对齐：将 CodeRule 转为后端 PUT 请求体 */
function codeRuleToTemplatePayload(rule: CodeRule): Record<string, unknown> {
  return {
    name: rule.name,
    prefix: rule.prefix,
    date_format: rule.dateFormat,
    location_code: rule.locationCode,
    biz_type_code: rule.bizTypeCode,
    sequence_digits: rule.sequenceDigits,
    daily_reset: rule.dailyReset,
    enabled: rule.enabled,
  }
}

export const useRulesStore = defineStore('rules', () => {
  const items = ref<CodeRule[]>([])
  const loading = ref(false)
  const loadError = ref('')

  const list = computed(() => items.value)
  const getById = (id: string) => items.value.find((r) => r.id === id)
  const skuRules = computed(() =>
    items.value.filter((r) => (r.id === 'product-code' || r.ruleType === 'sku') && r.enabled),
  )
  /** v0.6 对齐：taskNo 与 product-code 共用，无独立 taskNo 模板时用 product-code */
  const taskNoRules = computed(() => {
    const byTaskNo = items.value.filter((r) => r.ruleType === 'taskNo' && r.enabled)
    if (byTaskNo.length > 0) return byTaskNo
    const productCode = items.value.find((r) => r.id === 'product-code' && r.enabled)
    return productCode ? [productCode] : []
  })

  function setRules(rules: CodeRule[]) {
    items.value = rules
  }

  async function loadRules() {
    loading.value = true
    loadError.value = ''
    try {
      const res = await rulesApi.list(undefined, true)
      const body = res?.data as { data?: Record<string, RuleTemplate> | RuleTemplate[] } | undefined
      // 兼容 { data: [...] }、{ data: { data: [...] } }、直接数组
      let raw = body?.data ?? body
      if (raw && typeof raw === 'object' && !Array.isArray(raw)) {
        const inner = (raw as Record<string, unknown>).data ?? (raw as Record<string, unknown>).items
        if (Array.isArray(inner)) raw = inner
      }
      const templates: { type: RuleTemplateType; data: Record<string, unknown> }[] = []

      if (Array.isArray(raw)) {
        raw.forEach((t: RuleTemplate) => {
          const type = (t.type ?? t.template_type) as RuleTemplateType | undefined
          if (type && TEMPLATE_TYPES.includes(type)) {
            templates.push({ type, data: t as Record<string, unknown> })
          }
        })
      } else if (raw && typeof raw === 'object') {
        const obj = raw as Record<string, Record<string, unknown>>
        const keyMap: [string, RuleTemplateType][] = [
          ['cost_pricing', 'cost-pricing'],
          ['product_code', 'product-code'],
          ['short_name', 'short-name'],
        ]
        keyMap.forEach(([key, type]) => {
          const alt = key.replace('_', '-')
          const data = (obj[key] ?? obj[alt]) as Record<string, unknown> | undefined
          if (data && typeof data === 'object') {
            templates.push({ type, data })
          }
        })
      }

      if (templates.length > 0) {
        items.value = templates.map(({ type, data }) => templateToCodeRule(type, data))
      } else {
        items.value = TEMPLATE_TYPES.map((type) =>
          templateToCodeRule(type, { name: TEMPLATE_TYPE_LABELS[type], enabled: true }),
        )
      }
    } catch (e) {
      loadError.value = e instanceof Error ? e.message : '加载规则模板失败'
      items.value = TEMPLATE_TYPES.map((type) =>
        templateToCodeRule(type, { name: TEMPLATE_TYPE_LABELS[type], enabled: true }),
      )
    } finally {
      loading.value = false
    }
  }

  function optimisticUpdateRule(rule: CodeRule) {
    const idx = items.value.findIndex((r) => r.id === rule.id)
    if (idx >= 0) {
      const next = [...items.value]
      next[idx] = { ...next[idx], ...rule } as CodeRule
      items.value = next
    }
  }

  /** v0.6 对齐：按 type 更新，调用 PUT /v1/rule-templates/:type */
  async function saveRule(rule: CodeRule): Promise<void> {
    const type = rule.id as RuleTemplateType
    if (!TEMPLATE_TYPES.includes(type)) {
      throw new Error(`不支持的规则类型: ${type}`)
    }
    const payload = codeRuleToTemplatePayload(rule)
    await rulesApi.updateByType(type, payload)
    optimisticUpdateRule(rule)
    // 不再调用 loadRules()，避免 GET 缓存返回旧数据覆盖 prefix 等字段
  }

  const sequenceCounters = ref<Record<string, number>>({})

  /** 仅用于 UI 预览，不作为真实 SKU/任务号依据 */
  function previewCode(rule: CodeRule, _context: Record<string, unknown>): string {
    const dateStr = getBeijingDateCompactString()
    const seq = (sequenceCounters.value[rule.id] ?? 0) + 1
    const seqStr = String(seq).padStart(rule.sequenceDigits, '0')
    return `${rule.prefix}${dateStr}${rule.locationCode}${rule.bizTypeCode}${seqStr}`
  }

  /** 仅用于 UI 预览，不作为真实 SKU 依据 */
  function generatePreviewSku(ruleId: string): string {
    const rule = items.value.find((r) => r.id === ruleId)
    if (!rule || rule.ruleType !== 'sku') return ''
    const nextSeq = (sequenceCounters.value[ruleId] ?? 0) + 1
    const dateStr = getBeijingDateCompactString()
    const seqStr = String(nextSeq).padStart(rule.sequenceDigits, '0')
    const code = `${rule.prefix}-${dateStr}${rule.locationCode}${seqStr}`
    sequenceCounters.value[ruleId] = nextSeq
    return code
  }

  /** 仅用于 UI 预览，使用传入的 rule 对象（支持编辑中未保存的值） */
  function previewSkuWithRule(rule: CodeRule): string {
    if (rule.ruleType !== 'sku') return ''
    const seq = (sequenceCounters.value[rule.id] ?? 0) + 1
    const dateStr = getBeijingDateCompactString()
    const seqStr = String(seq).padStart(rule.sequenceDigits, '0')
    return `${rule.prefix}-${dateStr}${rule.locationCode}${seqStr}`
  }

  return {
    list,
    getById,
    skuRules,
    taskNoRules,
    loading,
    loadError,
    setRules,
    loadRules,
    saveRule,
    previewCode,
    generatePreviewSku,
    previewSkuWithRule,
  }
})
