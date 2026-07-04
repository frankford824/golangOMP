import type { ICellData, IWorkbookData, IWorksheetData } from '@univerjs/core'

import type {
  WorkbenchSpreadsheetColumn,
  WorkbenchSpreadsheetPalette,
  WorkbenchSpreadsheetRow,
  WorkbenchSpreadsheetRowsBySheet,
  WorkbenchSpreadsheetSheet,
  WorkbenchSpreadsheetSource,
  WorkbenchSpreadsheetValidation,
} from './types'

const BOOLEAN_FALSE = 0
const BOOLEAN_TRUE = 1
const CELL_STRING = 1
const CELL_NUMBER = 2
const CELL_BOOLEAN = 3
const ALIGN_CENTER = 2
const ALIGN_RIGHT = 3
const ALIGN_MIDDLE = 2
const WRAP_CLIP = 2
const STYLE_DEFAULT = 'aw-default'
const STYLE_HEADER = 'aw-header'
const STYLE_TEXT = 'aw-text'
const STYLE_CENTER = 'aw-center'
const STYLE_NUMBER = 'aw-number'
const STYLE_MONEY = 'aw-money'
const STYLE_READONLY = 'aw-readonly'
const STYLE_WARN = 'aw-warn'
const STYLE_DANGER = 'aw-danger'
const STYLE_SUCCESS = 'aw-success'
const STYLE_INFO = 'aw-info'

const FALLBACK_PALETTE: WorkbenchSpreadsheetPalette = {
  surface: '#ffffff',
  surfaceMuted: '#f2f0eb',
  surfaceRaised: '#faf9f6',
  hairline: '#e4e0d8',
  ink: '#1c1a16',
  inkMuted: '#8f897c',
  accent: '#0f7a5a',
  money: '#936023',
  danger: '#c0453b',
  warn: '#c5882a',
  info: '#4a6fa5',
  success: '#0f7a5a',
}

export function readAwSpreadsheetPalette(scope: HTMLElement | null): WorkbenchSpreadsheetPalette {
  if (!scope) return FALLBACK_PALETTE
  const style = window.getComputedStyle(scope)
  const read = (name: string, fallback: string) => style.getPropertyValue(name).trim() || fallback
  return {
    surface: read('--aw-surface', FALLBACK_PALETTE.surface),
    surfaceMuted: read('--aw-surface-muted', FALLBACK_PALETTE.surfaceMuted),
    surfaceRaised: read('--aw-surface-raised', FALLBACK_PALETTE.surfaceRaised),
    hairline: read('--aw-hairline', FALLBACK_PALETTE.hairline),
    ink: read('--aw-ink', FALLBACK_PALETTE.ink),
    inkMuted: read('--aw-ink-muted', FALLBACK_PALETTE.inkMuted),
    accent: read('--aw-accent', FALLBACK_PALETTE.accent),
    money: read('--aw-money', FALLBACK_PALETTE.money),
    danger: read('--aw-danger', FALLBACK_PALETTE.danger),
    warn: read('--aw-warn', FALLBACK_PALETTE.warn),
    info: read('--aw-status-submitted', FALLBACK_PALETTE.info),
    success: read('--aw-success', FALLBACK_PALETTE.success),
  }
}

export function buildWorkbookSnapshot(
  source: WorkbenchSpreadsheetSource,
  palette: WorkbenchSpreadsheetPalette = FALLBACK_PALETTE,
): Partial<IWorkbookData> {
  const sheets = source.sheets.length ? source.sheets : [emptySheetSource()]
  const sheetOrder = sheets.map((sheet, index) => sheetIdFor(sheet, index))
  return {
    id: source.id,
    name: source.title,
    appVersion: '3.0.0-alpha',
    locale: 'zhCN' as IWorkbookData['locale'],
    styles: buildStyles(palette),
    sheetOrder,
    sheets: Object.fromEntries(sheets.map((sheet, index) => [sheetOrder[index], buildWorksheet(sheet, sheetOrder[index], palette, source.readonly)])),
  }
}

export function extractRowsFromSnapshot(
  source: WorkbenchSpreadsheetSource,
  snapshot: Partial<IWorkbookData> | null | undefined,
): WorkbenchSpreadsheetRowsBySheet[] {
  if (!snapshot?.sheets) return []
  return source.sheets.map((sheet, index) => {
    const sheetId = sheetIdFor(sheet, index)
    const worksheet = snapshot.sheets?.[sheetId]
    return {
      sheetId: sheet.id,
      rows: extractRowsFromWorksheet(sheet, worksheet),
    }
  })
}

function buildWorksheet(
  sheet: WorkbenchSpreadsheetSheet,
  sheetId: string,
  palette: WorkbenchSpreadsheetPalette,
  sourceReadonly?: boolean,
): Partial<IWorksheetData> {
  const columns = sheet.columns
  const rows = sheet.rows
  const readonly = sourceReadonly || sheet.readonly
  const validations = validationMap(sheet.validations ?? [])
  const cellData: NonNullable<Partial<IWorksheetData>['cellData']> = {}

  columns.forEach((column, columnIndex) => {
    setCell(cellData, 0, columnIndex, {
      v: column.label,
      t: CELL_STRING,
      s: STYLE_HEADER,
    })
  })

  rows.forEach((row, rowIndex) => {
    const rowKey = row[sheet.rowKey]
    columns.forEach((column, columnIndex) => {
      const validation = validationFor(validations, rowKey, column.key)
      const cell = cellFor(row[column.key], column)
      cell.s = validationStyle(validation) ?? columnStyle(column, readonly || column.readonly)
      setCell(cellData, rowIndex + 1, columnIndex, cell)
    })
  })

  return {
    id: sheetId,
    name: sheet.name,
    tabColor: '',
    hidden: BOOLEAN_FALSE,
    rowCount: Math.max(rows.length + 8, 40),
    columnCount: Math.max(columns.length + 2, 12),
    freeze: sheet.freezeHeader === false ? { xSplit: 0, ySplit: 0, startRow: 0, startColumn: 0 } : { xSplit: 0, ySplit: 1, startRow: 1, startColumn: 0 },
    zoomRatio: 1,
    scrollTop: 0,
    scrollLeft: 0,
    defaultColumnWidth: 128,
    defaultRowHeight: 34,
    mergeData: [],
    cellData,
    rowData: {
      0: { h: 36 },
      ...Object.fromEntries(rows.map((_, index) => [index + 1, { h: 34 }])),
    },
    columnData: Object.fromEntries(columns.map((column, index) => [index, { w: Math.max(72, column.width ?? 128) }])),
    rowHeader: { width: 44 },
    columnHeader: { height: 28 },
    showGridlines: BOOLEAN_TRUE,
    gridlinesColor: palette.hairline,
    rightToLeft: BOOLEAN_FALSE,
    defaultStyle: STYLE_DEFAULT,
    custom: { awReadonly: readonly },
  }
}

function buildStyles(palette: WorkbenchSpreadsheetPalette): NonNullable<Partial<IWorkbookData>['styles']> {
  const base = {
    ff: 'Alibaba PuHuiTi 2.0',
    fs: 10,
    vt: ALIGN_MIDDLE,
    tb: WRAP_CLIP,
    pd: { t: 4, r: 8, b: 4, l: 8 },
  }
  return {
    [STYLE_DEFAULT]: {
      ...base,
      cl: { rgb: palette.ink },
      bg: { rgb: palette.surface },
      bd: { b: { s: 1, cl: { rgb: palette.hairline } } },
    },
    [STYLE_HEADER]: {
      ...base,
      bl: BOOLEAN_TRUE,
      cl: { rgb: palette.ink },
      bg: { rgb: palette.surfaceMuted },
      bd: { b: { s: 1, cl: { rgb: palette.hairline } } },
    },
    [STYLE_TEXT]: {
      ...base,
      cl: { rgb: palette.ink },
      bg: { rgb: palette.surface },
    },
    [STYLE_CENTER]: {
      ...base,
      ht: ALIGN_CENTER,
      cl: { rgb: palette.ink },
      bg: { rgb: palette.surface },
    },
    [STYLE_NUMBER]: {
      ...base,
      ht: ALIGN_RIGHT,
      cl: { rgb: palette.ink },
      bg: { rgb: palette.surface },
      n: { pattern: '#,##0' },
    },
    [STYLE_MONEY]: {
      ...base,
      ht: ALIGN_RIGHT,
      bl: BOOLEAN_TRUE,
      cl: { rgb: palette.money },
      bg: { rgb: palette.surface },
      n: { pattern: '¥#,##0.00;¥-#,##0.00' },
    },
    [STYLE_READONLY]: {
      ...base,
      cl: { rgb: palette.inkMuted },
      bg: { rgb: palette.surfaceRaised },
    },
    [STYLE_WARN]: {
      ...base,
      cl: { rgb: palette.ink },
      bg: { rgb: colorMixLike(palette.warn, palette.surface, 0.14) },
    },
    [STYLE_DANGER]: {
      ...base,
      cl: { rgb: palette.danger },
      bg: { rgb: colorMixLike(palette.danger, palette.surface, 0.12) },
    },
    [STYLE_SUCCESS]: {
      ...base,
      cl: { rgb: palette.success },
      bg: { rgb: colorMixLike(palette.success, palette.surface, 0.12) },
    },
    [STYLE_INFO]: {
      ...base,
      cl: { rgb: palette.info },
      bg: { rgb: colorMixLike(palette.info, palette.surface, 0.12) },
    },
  }
}

function setCell(cellData: NonNullable<Partial<IWorksheetData>['cellData']>, row: number, column: number, cell: ICellData) {
  cellData[row] = cellData[row] ?? {}
  cellData[row][column] = cell
}

function cellFor(value: unknown, column: WorkbenchSpreadsheetColumn): ICellData {
  if (column.kind === 'number' || column.kind === 'money') {
    const num = typeof value === 'number' ? value : Number(value)
    return Number.isFinite(num) ? { v: num, t: CELL_NUMBER } : { v: '', t: CELL_STRING }
  }
  if (column.kind === 'boolean') {
    return { v: Boolean(value), t: CELL_BOOLEAN }
  }
  if (typeof value === 'number') return { v: value, t: CELL_NUMBER }
  if (typeof value === 'boolean') return { v: value, t: CELL_BOOLEAN }
  return { v: value == null ? '' : String(value), t: CELL_STRING }
}

function columnStyle(column: WorkbenchSpreadsheetColumn, readonly?: boolean) {
  if (column.kind === 'money') return STYLE_MONEY
  if (column.kind === 'number') return STYLE_NUMBER
  if (readonly) return STYLE_READONLY
  if (column.align === 'right') return STYLE_NUMBER
  if (column.align === 'center') return STYLE_CENTER
  return STYLE_TEXT
}

function validationMap(validations: WorkbenchSpreadsheetValidation[]) {
  const map = new Map<string, WorkbenchSpreadsheetValidation>()
  for (const validation of validations) {
    const key = `${String(validation.rowKey ?? '*')}::${validation.columnKey ?? '*'}`
    map.set(key, validation)
  }
  return map
}

function validationFor(map: Map<string, WorkbenchSpreadsheetValidation>, rowKey: unknown, columnKey: string) {
  return map.get(`${String(rowKey ?? '')}::${columnKey}`) ?? map.get(`${String(rowKey ?? '')}::*`)
}

function validationStyle(validation: WorkbenchSpreadsheetValidation | undefined) {
  if (!validation) return ''
  if (validation.tone === 'danger') return STYLE_DANGER
  if (validation.tone === 'warn') return STYLE_WARN
  if (validation.tone === 'success') return STYLE_SUCCESS
  if (validation.tone === 'info') return STYLE_INFO
  return ''
}

function extractRowsFromWorksheet(sheet: WorkbenchSpreadsheetSheet, worksheet: Partial<IWorksheetData> | undefined): WorkbenchSpreadsheetRow[] {
  if (!worksheet?.cellData) return sheet.rows
  return sheet.rows.map((original, rowIndex) => {
    const row: WorkbenchSpreadsheetRow = { ...original }
    const cells = worksheet.cellData?.[rowIndex + 1] ?? {}
    for (const [columnIndex, column] of sheet.columns.entries()) {
      if (column.readonly || column.key === sheet.rowKey) continue
      const cell = cells[columnIndex]
      if (!cell) continue
      row[column.key] = normalizeCellValue(cell.v, column, original[column.key])
    }
    return row
  })
}

function normalizeCellValue(value: unknown, column: WorkbenchSpreadsheetColumn, fallback: unknown) {
  if (column.kind === 'number' || column.kind === 'money') {
    const next = typeof value === 'number' ? value : Number(value)
    return Number.isFinite(next) ? next : fallback
  }
  if (column.kind === 'boolean') {
    if (typeof value === 'boolean') return value
    const normalized = String(value ?? '').trim().toLowerCase()
    if (['true', '1', 'yes', 'y', '是', '定稿', '已完成'].includes(normalized)) return true
    if (['false', '0', 'no', 'n', '否', '未完成'].includes(normalized)) return false
    return fallback
  }
  return value == null ? '' : String(value)
}

function sheetIdFor(sheet: WorkbenchSpreadsheetSheet, index: number) {
  const sanitized = sheet.id.replace(/[^a-zA-Z0-9_-]/g, '_')
  return sanitized || `sheet_${index + 1}`
}

function emptySheetSource(): WorkbenchSpreadsheetSheet {
  return {
    id: 'empty',
    name: '空表',
    rowKey: 'id',
    columns: [{ key: 'message', label: '说明', width: 240, readonly: true }],
    rows: [{ id: 'empty', message: '暂无可展示数据' }],
    readonly: true,
  }
}

function colorMixLike(foreground: string, background: string, weight: number) {
  const fg = parseHexColor(foreground)
  const bg = parseHexColor(background)
  if (!fg || !bg) return background
  const mix = (a: number, b: number) => Math.round(a * weight + b * (1 - weight))
  return `#${toHex(mix(fg.r, bg.r))}${toHex(mix(fg.g, bg.g))}${toHex(mix(fg.b, bg.b))}`
}

function parseHexColor(color: string) {
  const match = color.trim().match(/^#([0-9a-f]{6})$/i)
  if (!match) return null
  const raw = match[1]
  return {
    r: Number.parseInt(raw.slice(0, 2), 16),
    g: Number.parseInt(raw.slice(2, 4), 16),
    b: Number.parseInt(raw.slice(4, 6), 16),
  }
}

function toHex(value: number) {
  return value.toString(16).padStart(2, '0')
}
