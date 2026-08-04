import { describe, expect, it } from 'vitest'
import {
  formatBusinessTechnicalText,
  formatErpSyncFailureMessage,
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
})
