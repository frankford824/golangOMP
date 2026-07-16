/**
 * Round I.g · D2 单测：`sanitizeCreateTaskPayload` 不依赖 customization_required。
 *
 * 运行方式：需本仓库安装 vitest（`npm i -D vitest`），随后 `npx vitest run`。
 * 当前仓库暂未引入 vitest runner；本文件作为回归契约固定下来，装上框架即可运行。
 */
import { describe, it, expect } from 'vitest'
import {
  sanitizeCreateTaskPayload,
  TASK_TYPE_FIELD_WHITELIST,
  humanizeTaskCreateFields,
  pickFieldWhitelistViolations,
  humanizeViolationCode,
} from '../src/domain/task-create-fields'

describe('sanitizeCreateTaskPayload — customization is orthogonal', () => {
  it('case 2 regression: original + customization=true 必须 strip material_*/purchase_sku/product_channel/design_requirement', () => {
    const raw = {
      task_type: 'original_product_development',
      change_request: '把 logo 改成金色',
      customization_required: true,
      customization_source_type: 'existing_product',
      product_selection: { erp_product: { product_id: 11039, sku_code: 'HSC11039' } },
      material_mode: 'other',
      material_other: '定制海报',
      material: '纸',
      purchase_sku: 'PS-ABC',
      product_channel: 'TB',
      design_requirement: '做一张海报',
      owner_department: '运营部',
      owner_org_team: '运营一组',
    }
    const out = sanitizeCreateTaskPayload(raw, 'original_product_development')
    expect(out.task_type).toBe('original_product_development')
    expect(out.change_request).toBe('把 logo 改成金色')
    expect(out.customization_required).toBe(true)
    expect(out.customization_source_type).toBe('existing_product')
    expect((out as Record<string, unknown>).material_mode).toBeUndefined()
    expect((out as Record<string, unknown>).material_other).toBeUndefined()
    expect((out as Record<string, unknown>).material).toBeUndefined()
    expect((out as Record<string, unknown>).purchase_sku).toBeUndefined()
    expect((out as Record<string, unknown>).product_channel).toBeUndefined()
    expect((out as Record<string, unknown>).design_requirement).toBeUndefined()
  })

  it('case 1: original + customization=false 时同样 strip 掉 forbidden 字段', () => {
    const raw = {
      task_type: 'original_product_development',
      change_request: 'x',
      customization_required: false,
      material_mode: 'other',
    }
    const out = sanitizeCreateTaskPayload(raw, 'original_product_development')
    expect((out as Record<string, unknown>).material_mode).toBeUndefined()
  })

  it('case 3: new + customization=true 移除旧材质/简称字段，保留设计需求', () => {
    const raw = {
      task_type: 'new_product_development',
      i_id: '常规kt板',
      product_name: '海报 A',
      product_short_name: '海报',
      material_mode: 'other',
      material_other: '亚光纸',
      design_requirement: '新品',
      customization_required: true,
      customization_source_type: 'new_product',
    }
    const out = sanitizeCreateTaskPayload(raw, 'new_product_development')
    expect((out as Record<string, unknown>).material_mode).toBeUndefined()
    expect((out as Record<string, unknown>).material_other).toBeUndefined()
    expect((out as Record<string, unknown>).product_short_name).toBeUndefined()
    expect((out as Record<string, unknown>).design_requirement).toBe('新品')
  })

  it('new product required fields include i_id', () => {
    expect(TASK_TYPE_FIELD_WHITELIST.new_product_development.required).toContain('i_id')
    expect('purchase_task' in TASK_TYPE_FIELD_WHITELIST).toBe(false)
  })

  it('retired task types do not receive an active sanitizer branch', () => {
    const raw = {
      task_type: 'purchase_task',
      unsupported: true,
    }
    const out = sanitizeCreateTaskPayload(raw, 'purchase_task')
    expect(out).toEqual(raw)
  })

  it('sanitizer 不修改入参（深克隆）', () => {
    const raw = { task_type: 'original_product_development', material_mode: 'other' }
    sanitizeCreateTaskPayload(raw, 'original_product_development')
    expect(raw.material_mode).toBe('other')
  })

  it('unknown task_type 原样返回', () => {
    const raw = { task_type: 'exotic', material_mode: 'other' } as Record<string, unknown>
    const out = sanitizeCreateTaskPayload(raw, 'exotic')
    expect(out.material_mode).toBe('other')
  })

  it('TASK_TYPE_FIELD_WHITELIST.original_product_development.forbidden 必须与 Round I.g 不变式一致', () => {
    expect(new Set(TASK_TYPE_FIELD_WHITELIST.original_product_development.forbidden)).toEqual(
      new Set(['material_mode', 'material', 'material_other', 'purchase_sku', 'product_channel', 'design_requirement']),
    )
  })

  it('humanizeTaskCreateFields 把 snake_case 翻译为中文', () => {
    expect(humanizeTaskCreateFields(['material_mode', 'material_other'])).toEqual(['材料方式', '自定义材料'])
  })

  it('pickFieldWhitelistViolations 只挑 field_not_allowed_for_task_type 的条目', () => {
    const violations = [
      { code: 'field_not_allowed_for_task_type', field: 'material_mode' },
      { code: 'something_else', field: 'x' },
      { code: 'field_not_allowed_for_task_type', field: '' },
    ]
    expect(pickFieldWhitelistViolations(violations)).toEqual(['material_mode'])
  })
})

describe('humanizeViolationCode', () => {
  it('insufficient_batch_items 返回中文提示', () => {
    expect(humanizeViolationCode('insufficient_batch_items', 'batch_items'))
      .toBe('批量 SKU 模式下至少需要 2 个商品')
  })

  it('missing_required_field 返回含字段中文名的提示', () => {
    expect(humanizeViolationCode('missing_required_field', 'batch_items[0].material_mode'))
      .toBe('必填字段缺失：材料方式')
  })

  it('missing_required_field 对 i_id 返回款式编码文案', () => {
    expect(humanizeViolationCode('missing_required_field', 'i_id'))
      .toBe('必填字段缺失：款式编码 (i_id)')
  })

  it('field_not_allowed_for_task_type 返回含字段中文名的提示', () => {
    expect(humanizeViolationCode('field_not_allowed_for_task_type', 'design_requirement'))
      .toBe('字段不符合当前任务类型：设计要求')
  })

  it('field_not_allowed_for_task_type 对带路径前缀的 field 提取 leaf 后中文化', () => {
    expect(humanizeViolationCode('field_not_allowed_for_task_type', 'batch_items[0].design_requirement'))
      .toBe('字段不符合当前任务类型：设计要求')
  })

  it('未识别 code 返回空字符串', () => {
    expect(humanizeViolationCode('unknown_code', 'some_field')).toBe('')
  })
})
