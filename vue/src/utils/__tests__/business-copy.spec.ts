import { describe, expect, it } from 'vitest'
import {
  formatBusinessTechnicalText,
  formatErpSyncFailureMessage,
  normalizePredictionSuggestionForBusiness,
} from '@/utils/business-copy'

describe('business-copy', () => {
  it('maps ERP product name length errors to user-facing business copy', () => {
    expect(formatErpSyncFailureMessage('ERP product name length validation failed')).toContain('产品名称过长')
  })

  it('maps cost readback mismatch to actionable copy', () => {
    expect(formatErpSyncFailureMessage('c_price readback mismatch: expected 5.69 actual 0.96')).toContain(
      '成本价同步后与聚水潭当前值不一致',
    )
  })

  it('replaces technical field names in mixed messages', () => {
    expect(formatBusinessTechnicalText('request_payload_json missing sku_id and i_id')).toBe(
      '请求内容 缺少 商品编码 和 款式编码',
    )
  })

  it('normalizes prediction fields without changing routing fields', () => {
    const item = normalizePredictionSuggestionForBusiness({
      id: 'p1',
      type: 'task_next_action',
      title: '设计环节状态：in_progress',
      detail: 'pending_filing, current c_price is empty',
      source: '',
      action_label: '',
      action_type: 'open_task_erp',
      target_type: 'task',
      target_id: '100',
    })

    expect(item.source).toBe('查看 ERP 同步')
    expect(item.title).toBe('设计环节状态：处理中')
    expect(item.detail).toContain('待补齐资料后同步')
    expect(item.detail).toContain('成本价')
    expect(item.action_label).toBe('查看 ERP 同步')
    expect(item.action_type).toBe('open_task_erp')
    expect(item.target_id).toBe('100')
  })
})
