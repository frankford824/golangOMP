export interface CodeRule {
  id: string
  name: string
  ruleType: 'taskNo' | 'sku' | 'outsourceNo' | 'handoverNo'
  prefix: string
  dateFormat: string
  locationCode: string
  bizTypeCode: string
  sequenceDigits: number
  dailyReset: boolean
  enabled: boolean
}
