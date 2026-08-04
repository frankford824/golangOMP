import { describe, expect, it } from 'vitest'
import { handoverStatusLabel, taskDetailDisplayValue } from '@/domain/task-detail-display'

describe('task detail business labels', () => {
  it('converts technical enums to plain Simplified Chinese', () => {
    expect(taskDetailDisplayValue('priority', 'normal')).toBe('普通优先级')
    expect(taskDetailDisplayValue('filing_status', 'filed')).toBe('已完成 ERP 建档')
    expect(taskDetailDisplayValue('erp_sync_status', 'failed')).toBe('同步失败')
    expect(taskDetailDisplayValue('cost_price_mode', 'template')).toBe('按成本规则计算')
  })

  it('fails closed instead of exposing unknown English enum values', () => {
    expect(taskDetailDisplayValue('priority', 'brand_new_internal_state')).toBe('状态待确认')
    expect(taskDetailDisplayValue('priority', '')).toBe('未填写')
    expect(handoverStatusLabel('pending_takeover')).toBe('等待接手')
  })
})
