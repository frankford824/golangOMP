export type WorkbenchSpreadsheetMode = 'settlement' | 'drive' | 'import-review'
export type SpreadsheetAlign = 'left' | 'right' | 'center'
export type SpreadsheetTone = 'neutral' | 'success' | 'warn' | 'danger' | 'info' | 'money'

export interface WorkbenchSpreadsheetColumn {
  key: string
  label: string
  width?: number
  align?: SpreadsheetAlign
  kind?: 'text' | 'number' | 'money' | 'boolean' | 'status' | 'date'
  readonly?: boolean
}

export type WorkbenchSpreadsheetRow = Record<string, unknown>

export interface WorkbenchSpreadsheetValidation {
  rowKey?: string | number
  columnKey?: string
  tone: Exclude<SpreadsheetTone, 'money'>
  message: string
}

export interface WorkbenchSpreadsheetSheet {
  id: string
  name: string
  rowKey: string
  columns: WorkbenchSpreadsheetColumn[]
  rows: WorkbenchSpreadsheetRow[]
  readonly?: boolean
  freezeHeader?: boolean
  validations?: WorkbenchSpreadsheetValidation[]
}

export interface WorkbenchSpreadsheetAction {
  key: string
  label: string
  tone?: SpreadsheetTone
  disabled?: boolean
}

export interface WorkbenchSpreadsheetSource {
  id: string
  revision: string | number
  mode: WorkbenchSpreadsheetMode
  title: string
  description?: string
  readonly?: boolean
  sheets: WorkbenchSpreadsheetSheet[]
  actions?: WorkbenchSpreadsheetAction[]
}

export interface WorkbenchSpreadsheetPalette {
  surface: string
  surfaceMuted: string
  surfaceRaised: string
  hairline: string
  ink: string
  inkMuted: string
  accent: string
  money: string
  danger: string
  warn: string
  info: string
  success: string
}

export interface WorkbenchSpreadsheetRowsBySheet {
  sheetId: string
  rows: WorkbenchSpreadsheetRow[]
}

export interface WorkbenchSpreadsheetActionPayload {
  action: WorkbenchSpreadsheetAction
  sheets: WorkbenchSpreadsheetRowsBySheet[]
}
