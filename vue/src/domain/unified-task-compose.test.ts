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
  it('flags the first/third duplicate from a pasted batch even when reference images differ', () => {
    const seed = { product_i_id: '雪弗板', product_name: '中秋挖挖乐80*21cm/厚度1cm', design_requirement: '生成编码', area: 0.168 }
    const rows = [
      createComposeRow({ ...seed, id: 'first' }),
      createComposeRow({ ...seed, id: 'second', product_name: '中秋挖福运80*21cm/厚度1cm' }),
      createComposeRow({ ...seed, id: 'third', product_name: ` ${seed.product_name} `, reference_assets: [{ id: 'ref', name: 'another.png', status: 'uploaded', upload_ref: 'ref' }] }),
    ]
    expect(validateCompose('new_design', common, rows, new Date('2026-07-16'))).toEqual([
      expect.objectContaining({ row_id: 'third', row_index: 2, field: 'product_name', message: expect.stringContaining('第 1 条明细（表格第 2 行）') }),
    ])
    rows[2].design_requirement = '另一版图案'
    expect(validateCompose('new_design', common, rows, new Date('2026-07-16'))).toEqual([])
  })

  it('allows genuine per-SKU dimension differences and does not apply batch dedupe to planning', () => {
    const row = createComposeRow({ product_i_id: 'KT', product_name: '同系列', design_requirement: '画图', area: 0.168, quantity: 1, description_spec: '同系列' })
    expect(validateCompose('new_design', common, [row, createComposeRow({ ...row, id: 'b', area: 0.48 })], new Date('2026-07-16'))).toEqual([])
    expect(validateCompose('planning_sku', common, [row, createComposeRow({ ...row, id: 'c' })], new Date('2026-07-16'))).toEqual([])
  })

  it.each(['app', 'axios', 'body'])('maps row-level duplicate errors from the %s envelope', (envelope) => {
    const rows = [createComposeRow({ id: 'a' }), createComposeRow({ id: 'b' }), createComposeRow({ id: 'c' })]
    const body = { error: { details: { violations: [{ field: 'batch_items[2]', code: 'duplicate_batch_item', message: '第 4 行与第 2 行内容重复' }] } } }
    const raw = envelope === 'app' ? Object.assign(new Error('请求参数有误'), { status: 400, responseData: body })
      : envelope === 'axios' ? { response: { status: 400, data: body } } : body
    expect(applyBackendViolations(rows, raw)).toEqual([
      { row_id: 'c', row_index: 2, field: 'product_name', message: '第 4 行与第 2 行内容重复' },
    ])
  })

  it('maps dimension paths and common fields to visible controls', () => {
    const issues = applyBackendViolations([createComposeRow({ id: 'a' })], { error: { details: { violations: [
      { field: 'batch_items[0].variant_json.area', message: '面积无效' },
      { field: 'deadline_at', message: '截止时间已过' },
    ] } } })
    expect(issues[0]).toMatchObject({ row_id: 'a', row_index: 0, field: 'area' })
    expect(issues[1]).toMatchObject({ row_id: undefined, field: 'due_at' })
  })
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

  it('uses one visible style code for planning numbering and ERP filing', () => {
    const row = createComposeRow({
      id: 'row-1',
      category_code: 'HZS',
      description_spec: '亚克力立牌 20cm',
      quantity: 2,
      product_i_id: 'HQT',
    })

    expect(composeColumns('planning_sku').map((column) => column.key)).not.toContain('category_code')
    expect(composeColumns('planning_sku').map((column) => column.key)).toContain('product_i_id')
    expect(composeColumns('planning_sku').map((column) => column.key)).not.toContain('product_name')
    expect(buildPlanningInputs([row])[0]).toMatchObject({
      category_code: 'HQT',
      erp_product_i_id: 'HQT',
      erp_product_name: '亚克力立牌 20cm',
    })
  })

  it('shows explicit business units for task dimensions', () => {
    const labels = composeColumns('new_design').map((column) => column.label)
    expect(labels).toContain('宽（cm）')
    expect(labels).toContain('高（cm）')
    expect(labels).toContain('面积（㎡）')
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

  it('accepts an existing ERP product whose historical name exceeds the new-product short-name limit', () => {
    const historicalName = '历史 ERP 商品名称'.repeat(8)
    const row = createComposeRow({
      id: 'existing-long-name',
      erp_product_id: 'ERP-LONG-001',
      erp_sku: 'ERP-LONG-001',
      erp_product_snapshot: {
        product_id: 'ERP-LONG-001',
        sku_code: 'ERP-LONG-001',
        product_name: historicalName,
      },
      product_name: historicalName,
      design_requirement: '只修改现有商品图片，不新建 ERP 商品',
    })

    expect(validateCompose('modify_existing', common, [row], new Date('2026-07-16T00:00:00Z')))
      .not.toEqual(expect.arrayContaining([
        expect.objectContaining({ field: 'product_name' }),
      ]))
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
      product_i_id: 'HZS',
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

  it('allows a 40-file retouch batch but rejects more than 50 or a source over 300 MB', () => {
    const source = (index: number, size = 1024) => ({
      id: `source-${index}`,
      name: `source-${index}.psd`,
      status: 'local' as const,
      file: { size } as File,
    })
    const forty = createComposeRow({
      id: 'retouch-40',
		erp_sku: 'SKU-40',
      design_requirement: '批量修图',
      source_assets: Array.from({ length: 40 }, (_, index) => source(index)),
    })
    expect(validateCompose('retouch', common, [forty], new Date('2026-07-16T00:00:00Z'))).toEqual([])

    const fiftyOne = createComposeRow({
      id: 'retouch-51',
		erp_sku: 'SKU-51',
      design_requirement: '批量修图',
      source_assets: Array.from({ length: 51 }, (_, index) => source(index)),
    })
    expect(validateCompose('retouch', common, [fiftyOne], new Date('2026-07-16T00:00:00Z')))
      .toContainEqual(expect.objectContaining({ field: 'source_assets', message: '每项待修素材最多 50 个文件' }))

    const oversized = createComposeRow({
      id: 'retouch-large',
		erp_sku: 'SKU-LARGE',
      design_requirement: '批量修图',
      source_assets: [source(1, 300 * 1024 * 1024 + 1)],
    })
    expect(validateCompose('retouch', common, [oversized], new Date('2026-07-16T00:00:00Z')))
      .toContainEqual(expect.objectContaining({ field: 'source_assets', message: expect.stringContaining('超过 300 MB') }))
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
      structure_type: 'three_dimensional',
      slotting: 'slotted',
      special_note: '注意出血位',
    })
    expect(validateCompose('new_design', { ...common, due_at: '2026-07-15T18:00' }, [row], new Date('2026-07-16T00:00:00Z')))
      .toContainEqual(expect.objectContaining({ field: 'due_at', message: '截止时间不能早于当前时间' }))

    const [unit] = buildTaskSubmissionUnits('new_design', common, [row])
    expect(unit.task).toMatchObject({
      width: 1.2, height: 0.8, area: 0.96,
      craftText: '立体 开槽', process: '开槽', note: '运营备注\n注意出血位',
    })
  })

  it('stores explicit structure and slotting semantics per batch SKU', () => {
    const rows = [
      createComposeRow({ id: 'a', product_i_id: 'A', product_name: 'A款', design_requirement: 'A需求', structure_type: 'flat', slotting: 'not_slotted' }),
      createComposeRow({ id: 'b', product_i_id: 'B', product_name: 'B款', design_requirement: 'B需求', structure_type: 'three_dimensional', slotting: 'slotted' }),
    ]
    const [unit] = buildTaskSubmissionUnits('new_design', common, rows)
    expect(unit.task.batchItems?.map((item) => item.variantJson)).toEqual([
      expect.objectContaining({ structure_type: 'flat', structure_text: '平面', slotting: 'not_slotted', process: '不开槽', craft_text: '平面 不开槽' }),
      expect.objectContaining({ structure_type: 'three_dimensional', structure_text: '立体', slotting: 'slotted', process: '开槽', craft_text: '立体 开槽' }),
    ])
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
