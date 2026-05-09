import type { CodeRule } from '@/types'

export const mockRules: CodeRule[] = [
  {
    id: 'rule-sku-1',
    name: '新品 SKU 规则',
    ruleType: 'sku',
    prefix: 'SKU-NEW',
    dateFormat: 'yyyyMMdd',
    locationCode: '01',
    bizTypeCode: '',
    sequenceDigits: 2,
    dailyReset: true,
    enabled: true,
  },
  {
    id: 'rule-task-1',
    name: '任务号规则',
    ruleType: 'taskNo',
    prefix: 'T',
    dateFormat: 'yyyyMMdd',
    locationCode: '',
    bizTypeCode: '',
    sequenceDigits: 3,
    dailyReset: true,
    enabled: true,
  },
]

export const mockRuleById = (id: string): CodeRule | undefined =>
  mockRules.find((r) => r.id === id)
