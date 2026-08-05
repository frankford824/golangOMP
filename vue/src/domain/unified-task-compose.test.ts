import { describe, expect, it } from 'vitest'

import {
  applyBackendViolations,
  buildPlanningInputs,
  buildTaskSubmissionUnits,
  composeColumns,
  composeIdempotencyKey,
  createComposeRow,
  defaultErpSyncMode,
  validateCompose,
  type ComposeCommonInfo,
} from './unified-task-compose'

const common: ComposeCommonInfo = {
  due_at: '2026-07-20T18:00',
  priority: 'high',
  note: '运营备注',
  customization_required: false,
  erp_sync_mode: 'none',
}

describe('unified task compose domain', () => {
  it('keeps the pre-workbench ERP sync defaults per intent', () => {
    expect(defaultErpSyncMode('new_design')).toBe('async')
    expect(buildTaskSubmissionUnits('new_design', { ...common, erp_sync_mode: defaultErpSyncMode('new_design') }, [
      createComposeRow({ id: 'row-1', product_i_id: 'HZS001', product_name: '亚克力立牌', design_requirement: '按参考图设计' }),
    ])[0].task).toMatchObject({ syncErpOnCreate: true })

    expect(defaultErpSyncMode('planning_sku')).toBe('async')
    expect(validateCompose('planning_sku', { ...common, erp_sync_mode: defaultErpSyncMode('planning_sku') }, [
      createComposeRow({ id: 'row-1', category_code: 'HZS', description_spec: '亚克力立牌 20cm', quantity: 2, product_i_id: 'HQT' }),
    ])).toEqual([])
  })

  it('keeps planning numbering identity separate from ERP identity and derives the ERP name', () => {
    const row = createComposeRow({
      id: 'row-1',
      category_code: 'HZS',
      description_spec: '亚克力立牌 20cm',
      quantity: 2,
      product_i_id: 'HQT',
    })

    expect(composeColumns('planning_sku').map((column) => column.key)).toContain('category_code')
    expect(composeColumns('planning_sku').map((column) => column.key)).toContain('product_i_id')
    expect(composeColumns('planning_sku').map((column) => column.key)).not.toContain('product_name')
    expect(buildPlanningInputs([row])[0]).toMatchObject({
      category_code: 'HZS',
      erp_product_i_id: 'HQT',
      erp_product_name: '亚克力立牌 20cm',
    })
  })

  it('maps one new-design row and keeps the operations set hint non-authoritative', () => {
    const row = createComposeRow({
      id: 'row-1',
      product_i_id: 'KT_STANDARD',
      product_name: '桌面立牌',
      design_requirement: '白底主图',
      set_mode_hint: true,
    })

    expect(validateCompose('new_design', common, [row], new Date('2026-07-16T00:00:00Z'))).toEqual([])
    const [unit] = buildTaskSubmissionUnits('new_design', common, [row])
    expect(unit.row_ids).toEqual(['row-1'])
    expect(unit.task).toMatchObject({
      taskType: 'NEW_PRODUCT_DEV',
      category: 'KT_STANDARD',
      productName: '桌面立牌',
      setModeHint: true,
    })
    expect(unit.task.dueAt).toBe('2026-07-20T10:00:00.000Z')
  })

  it('keeps numeric ERP product codes out of the local product_id field', () => {
    const erpSnapshot = {
      product_id: '12324546567',
      sku_code: '12324546567',
      product_name: '数字编码 ERP 商品',
    }
    const row = createComposeRow({
      id: 'existing-erp-product',
      erp_product_id: '12324546567',
      erp_sku: '12324546567',
      erp_product_snapshot: erpSnapshot,
      product_name: '数字编码 ERP 商品',
      design_requirement: '更换图案',
    })

    const [unit] = buildTaskSubmissionUnits('modify_existing', common, [row])

    expect(unit.task).toMatchObject({
      taskType: 'ORIGINAL_PRODUCT_DEV',
      productId: null,
      sku: '12324546567',
      erpProductSnapshot: erpSnapshot,
    })
  })

  it('flags non-numeric text typed into number columns', () => {
    const row = createComposeRow({ id: 'n1', product_i_id: 'KT_STANDARD', product_name: '桌牌', design_requirement: '需求', width: Number.NaN, area: Number.NaN })
    const issues = validateCompose('new_design', common, [row], new Date('2026-07-16T00:00:00Z'))
    expect(issues).toEqual([
      expect.objectContaining({ field: 'width', message: '宽只能填数字' }),
      expect.objectContaining({ field: 'area', message: '面积只能填数字' }),
    ])
  })

  it('rejects negative dimensions instead of forwarding invalid create data', () => {
    const row = createComposeRow({ id: 'negative', product_i_id: 'KT_STANDARD', product_name: '桌牌', design_requirement: '需求', width: -1, height: -2, area: -3 })
    const issues = validateCompose('new_design', common, [row], new Date('2026-07-16T00:00:00Z'))
    expect(issues).toEqual(expect.arrayContaining([
      expect.objectContaining({ field: 'width', message: '宽不能小于 0' }),
      expect.objectContaining({ field: 'height', message: '高不能小于 0' }),
      expect.objectContaining({ field: 'area', message: '面积不能小于 0' }),
    ]))
  })

  it('maps multiple new-design rows to one mother task with per-SKU hints', () => {
    const rows = [
      createComposeRow({ id: 'a', product_i_id: 'A', product_name: 'A款', design_requirement: 'A需求', set_mode_hint: true }),
      createComposeRow({ id: 'b', product_i_id: 'B', product_name: 'B款', design_requirement: 'B需求', set_mode_hint: false }),
    ]
    const [unit] = buildTaskSubmissionUnits('new_design', common, rows)
    expect(unit.task.skuMode).toBe('multiple')
    expect(unit.task.batchItems?.map((item) => item.setModeHint)).toEqual([true, false])
  })

  it('maps retouch rows to one task and planning rows to the planning contract', () => {
    const retouchRows = [
      createComposeRow({ id: 'r1', design_requirement: '去除背景' }),
      createComposeRow({ id: 'r2', design_requirement: '校正颜色' }),
    ]
    const [retouch] = buildTaskSubmissionUnits('retouch', common, retouchRows)
    expect(retouch.row_ids).toEqual(['r1', 'r2'])
    expect(retouch.task.taskType).toBe('RETOUCH_TASK')
    expect(retouch.task.retouchRequirements).toHaveLength(2)

    const planning = createComposeRow({
      id: 'p1',
      category_code: 'HZS',
      description_spec: '亚克力立牌 20cm',
      quantity: 3,
      target_price: '12.50',
      reference_url: 'https://example.com/item',
    })
    expect(validateCompose('planning_sku', { ...common, due_at: '' }, [planning], new Date('2026-07-16T00:00:00Z'))).toEqual([])
    expect(buildPlanningInputs([planning])).toEqual([expect.objectContaining({
      client_item_id: 'p1',
      category_code: 'HZS',
      sku_code_type: 'regular',
      description_spec: '亚克力立牌 20cm',
      quantity: 3,
      target_price: '12.50',
    })])
    expect(buildPlanningInputs([planning], true)[0]?.sku_code_type).toBe('customization')
  })

  it('rejects expired deadlines and preserves single-task dimensions and row notes', () => {
    const row = createComposeRow({
      id: 'dimension-row',
      product_i_id: 'KT_STANDARD',
      product_name: '尺寸款',
      design_requirement: '按尺寸排版',
      width: 1.2,
      height: 0.8,
      area: 0.96,
      special_note: '注意出血位',
    })
    expect(validateCompose('new_design', { ...common, due_at: '2026-07-15T18:00' }, [row], new Date('2026-07-16T00:00:00Z')))
      .toContainEqual(expect.objectContaining({ field: 'due_at', message: '截止时间不能早于当前时间' }))

    const [unit] = buildTaskSubmissionUnits('new_design', common, [row])
    expect(unit.task).toMatchObject({ width: 1.2, height: 0.8, area: 0.96, note: '运营备注\n注意出血位' })
  })

  it('keeps the create idempotency key inside the 128-character backend limit for large batches', () => {
    const sessionId = crypto.randomUUID()
    const rowIds = Array.from({ length: 100 }, () => crypto.randomUUID())

    expect(composeIdempotencyKey(sessionId, rowIds.slice(0, 1)).length).toBeLessThanOrEqual(128)
    expect(composeIdempotencyKey(sessionId, rowIds.slice(0, 3)).length).toBeLessThanOrEqual(128)
    expect(composeIdempotencyKey(sessionId, rowIds).length).toBeLessThanOrEqual(128)
  })

  it('derives a stable key per row set so retrying only the failed rows is not treated as a replay', () => {
    const sessionId = 'session-1'
    const allRows = ['row-a', 'row-b', 'row-c']

    expect(composeIdempotencyKey(sessionId, allRows)).toBe(composeIdempotencyKey(sessionId, [...allRows].reverse()))
    expect(composeIdempotencyKey(sessionId, allRows)).not.toBe(composeIdempotencyKey(sessionId, ['row-b', 'row-c']))
    expect(composeIdempotencyKey(sessionId, allRows)).not.toBe(composeIdempotencyKey('session-2', allRows))
  })

  it('maps backend indexed violations back to the originating row', () => {
    const rows = [createComposeRow({ id: 'a' }), createComposeRow({ id: 'b' })]
    expect(applyBackendViolations(rows, {
      error: { details: { violations: [{ field: 'batch_items[1].product_i_id', message: 'i_id 不存在' }] } },
    })).toEqual([expect.objectContaining({ row_id: 'b', row_index: 1, field: 'product_i_id', message: 'i_id 不存在' })])
  })
})
