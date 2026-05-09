import { describe, expect, it } from 'vitest'
import { mergeDetailEnvelopeIntoTaskRaw } from '@/domain/mappers/task-detail-envelope'

describe('mergeDetailEnvelopeIntoTaskRaw', () => {
  it('merges task_detail into task root for category, design, note from remark, and filing', () => {
    const out = mergeDetailEnvelopeIntoTaskRaw({
      task: { id: '603', task_no: 'RW-1' },
      task_detail: {
        category_code: '常规海报',
        design_requirement: 'line1',
        remark: 'from remark',
        note: '',
        filing_status: 'pending_filing',
        spec_text: 's1',
      },
    })
    expect(out.category_code).toBe('常规海报')
    expect(out.design_requirement).toBe('line1')
    expect(out.note).toBe('from remark')
    expect(out.spec_text).toBe('s1')
    expect((out as { filing_status?: string }).filing_status).toBe('pending_filing')
  })

  it('uses demand_text when design_requirement empty', () => {
    const out = mergeDetailEnvelopeIntoTaskRaw({
      task: { id: '1' },
      task_detail: { demand_text: 'demand only' },
    })
    expect(out.design_requirement).toBe('demand only')
  })

  it('prefers note over remark when both set', () => {
    const out = mergeDetailEnvelopeIntoTaskRaw({
      task: { id: '1' },
      task_detail: { note: 'n', remark: 'r' },
    })
    expect(out.note).toBe('n')
  })

  it('prefers top-level reference_file_refs when both canonical and legacy fields exist', () => {
    const out = mergeDetailEnvelopeIntoTaskRaw({
      task: { id: '1' },
      task_detail: {
        reference_file_refs_json: '[{"download_url":"https://x"}]',
      },
      reference_file_refs: [{ download_url: 'https://top' }],
    })
    const refs = out.reference_file_refs as Array<{ download_url: string }>
    expect(refs).toHaveLength(1)
    expect(refs[0].download_url).toBe('https://top')
  })

  it('merges top-level workflow status and actor read-model fields into task root', () => {
    const out = mergeDetailEnvelopeIntoTaskRaw({
      task: {
        id: '612',
        designer_id: 198,
        task_status: 'InProgress',
      },
      workflow: {
        sub_status: {
          design: { code: 'in_progress' },
        },
      },
      design_sub_status: 'in_progress',
      designer_name: '设计超级管理员',
      assignee_id: 198,
      assignee_name: '设计超级管理员',
      current_handler_id: 198,
      current_handler_name: '设计超级管理员',
    })

    expect(out.workflow).toEqual({
      sub_status: {
        design: { code: 'in_progress' },
      },
    })
    expect(out.design_sub_status).toBe('in_progress')
    expect(out.designer_name).toBe('设计超级管理员')
    expect(out.assignee_name).toBe('设计超级管理员')
    expect(out.current_handler_name).toBe('设计超级管理员')
  })

  it('forwards top-level sku_items and asset_versions from detail envelope', () => {
    const out = mergeDetailEnvelopeIntoTaskRaw({
      task: { id: '617', task_no: 'RW-20260429-A-000614' },
      sku_items: [
        { sku_code: 'NSGE000004', sequence_no: 1 },
        { sku_code: 'NSGT000005', sequence_no: 2 },
      ],
      asset_versions: [
        { id: 101, scope_sku_code: 'NSGE000004', asset_type: 'delivery' },
        { id: 102, scope_sku_code: 'NSGT000005', asset_type: 'delivery' },
      ],
    })

    expect(out.sku_items).toEqual([
      { sku_code: 'NSGE000004', sequence_no: 1 },
      { sku_code: 'NSGT000005', sequence_no: 2 },
    ])
    expect(out.asset_versions).toEqual([
      { id: 101, scope_sku_code: 'NSGE000004', asset_type: 'delivery' },
      { id: 102, scope_sku_code: 'NSGT000005', asset_type: 'delivery' },
    ])
  })

  it('falls back to legacy reference_file_refs_json and maps url to download_url', () => {
    const out = mergeDetailEnvelopeIntoTaskRaw({
      task: { id: '1' },
      task_detail: {
        reference_file_refs_json: JSON.stringify([{ url: 'https://img/a.jpg', filename: 'a.jpg' }]),
      },
    })
    const refs = out.reference_file_refs as Array<{ download_url?: string; url?: string }>
    expect(refs).toHaveLength(1)
    expect(refs[0].download_url).toBe('https://img/a.jpg')
  })

  it('ignores empty reference_file_refs_json "[]"', () => {
    const out = mergeDetailEnvelopeIntoTaskRaw({
      task: { id: '1' },
      task_detail: { reference_file_refs_json: '[]' },
    })
    expect(out.reference_file_refs).toBeUndefined()
  })

  it('returns task copy when no task_detail', () => {
    const out = mergeDetailEnvelopeIntoTaskRaw({ task: { id: '9', x: 1 } })
    expect(out.id).toBe('9')
    expect(out.x).toBe(1)
  })
})
