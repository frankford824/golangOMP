/**
 * 规则及模板 API
 * v0.6 对齐：FRONTEND_ALIGNMENT_v0.5(1).md G 节
 * 接口：GET /v1/rule-templates、GET /v1/rule-templates/:type、PUT /v1/rule-templates/:type
 */
import http from '@/services/http'

export type RuleTemplateType = 'cost-pricing' | 'product-code' | 'short-name'

export interface RuleTemplate {
  type?: RuleTemplateType
  [key: string]: unknown
}

export interface RuleTemplatesListResponse {
  data?: Record<string, RuleTemplate> | RuleTemplate[]
  items?: RuleTemplate[]
}

export const rulesApi = {
  /**
   * GET /v1/rule-templates
   * 获取所有规则模板
   * @param cacheBust 传 true 时加 _t 参数避免缓存覆盖刚保存的数据
   */
  list: (signal?: AbortSignal, cacheBust?: boolean) =>
    http.get<RuleTemplatesListResponse>('/v1/rule-templates', {
      params: cacheBust ? { _t: Date.now() } : undefined,
      signal,
    }),

  /**
   * GET /v1/rule-templates/:type
   * 按类型获取规则模板（cost-pricing | product-code | short-name）
   */
  getByType: (type: RuleTemplateType, signal?: AbortSignal) =>
    http.get<{ data?: RuleTemplate }>(`/v1/rule-templates/${type}`, { signal }),

  /**
   * PUT /v1/rule-templates/:type
   * 更新指定类型的规则模板
   */
  updateByType: (
    type: RuleTemplateType,
    payload: Record<string, unknown>,
    signal?: AbortSignal,
  ) =>
    http.put<{ data?: RuleTemplate }>(`/v1/rule-templates/${type}`, payload, { signal }),
}
