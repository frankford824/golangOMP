<script setup lang="ts">
import type { IWorkbookData, Plugin, PluginCtor, Univer } from '@univerjs/core'
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { AlertTriangle, ImagePlus, LoaderCircle, Table2 } from 'lucide-vue-next'

import type { ComposeColumn, ComposeIntent, ComposeRow, ComposeViolation } from '@/domain/unified-task-compose'
import { composeColumns, createComposeRow } from '@/domain/unified-task-compose'
import { bindComposeGridImageCells, type ComposeGridCellPosition } from './compose-grid-image-cell'
import { composeRowIdsFromSelection } from './compose-grid-selection'

const props = defineProps<{
  intent: ComposeIntent
  rows: ComposeRow[]
  revision?: number
  violations?: ComposeViolation[]
}>()

const emit = defineEmits<{
  'update:rows': [ComposeRow[]]
  select: [rowId: string]
  selection: [rowIds: string[]]
  files: [payload: { rowId: string; field: 'reference_assets' | 'source_assets'; files: File[] }]
}>()

const rootRef = ref<HTMLElement | null>(null)
const canvasRef = ref<HTMLElement | null>(null)
const ready = ref(false)
const loading = ref(false)
const errorMessage = ref('')
const activeRowId = ref('')
const compactViewport = ref(false)
interface UniverRangeLike {
  getValues(): unknown[][]
  insertCellImageAsync(file: File | string): Promise<boolean>
  setBackgroundColor(color: string): unknown
  activate(): unknown
}
interface UniverWorksheetLike {
  getRange(row: number, column: number, numRows?: number, numColumns?: number): UniverRangeLike
  setColumnWidth(columnPosition: number, width: number): unknown
  setRowHeight(rowPosition: number, height: number): unknown
  getMaxRows?(): number
}
let univerInstance: Univer | null = null
let api: {
  Event: Record<string, string>
  addEvent(name: string, callback: (params: Record<string, unknown>) => void): { dispose(): void }
  createWorkbook(data: IWorkbookData): { dispose?(): void }
  getActiveSheet(): { worksheet: UniverWorksheetLike } | null
  getSheetHooks(): {
    onCellDragOver(callback: (position: ComposeGridCellPosition | null) => void): { dispose(): void }
    onCellDrop(callback: (position: ComposeGridCellPosition | null) => void): { dispose(): void }
  }
} | null = null
let workbook: { dispose?(): void } | null = null
let eventDisposables: Array<{ dispose(): void }> = []
let imageBinding: { dispose(): void; setActive?(position: ComposeGridCellPosition): void } | null = null
let bootSequence = 0
let syncTimer: ReturnType<typeof setTimeout> | undefined
let stylesReady = false
let canvasObserver: MutationObserver | null = null
let compactMedia: MediaQueryList | null = null
let resizeObserver: ResizeObserver | null = null
let resizeTimer: ReturnType<typeof setTimeout> | undefined
let applyingVisuals = false
let highlightedCells = new Set<string>()

/** Univer 行头 + 纵向滚动条大约占用的宽度，用于把剩余空间分配给业务列。 */
const GRID_CHROME_WIDTH = 88

type UniverPresetPlugin = PluginCtor<Plugin> | [PluginCtor<Plugin>, ConstructorParameters<PluginCtor<Plugin>>[0]]

const columns = computed(() => composeColumns(props.intent))

onMounted(() => {
  if (typeof window.matchMedia === 'function') {
    compactMedia = window.matchMedia('(max-width: 760px)')
    compactViewport.value = compactMedia.matches
    compactMedia.addEventListener('change', onCompactViewportChange)
  }
  if (!compactViewport.value) void boot()
})
onBeforeUnmount(() => {
  compactMedia?.removeEventListener('change', onCompactViewportChange)
  compactMedia = null
  dispose()
})
// 只有类型切换或父级主动改动（加行/删行/导入/恢复草稿）才重建表格；
// 表格内编辑产生的行模型变化不重建，避免打断输入焦点。
watch(() => [props.intent, props.revision], () => {
  if (!compactViewport.value) void boot()
})
watch(() => props.violations, () => applyViolationHighlights(), { deep: true })

function onCompactViewportChange(event: MediaQueryListEvent) {
  compactViewport.value = event.matches
  if (event.matches) dispose()
  else void boot()
}

function displayCell(row: ComposeRow, column: ComposeColumn): string | number {
  if (column.key === 'reference_assets' || column.key === 'source_assets') {
    const count = row[column.key].length
    const uploading = row[column.key].filter((asset) => asset.status === 'uploading').length
    return uploading ? `${count} 个 · 上传中` : count ? `${count} 个文件` : '拖入 / 粘贴'
  }
  if (column.key === 'set_mode_hint') return row.set_mode_hint ? '建议套装' : '单图优先'
  if (column.key === 'structure_type') return row.structure_type === 'three_dimensional' ? '立体' : row.structure_type === 'flat' ? '平面' : ''
  if (column.key === 'slotting') return row.slotting === 'slotted' ? '开槽' : row.slotting === 'not_slotted' ? '不开槽' : ''
  const value = row[column.key as keyof ComposeRow] as string | number | undefined
  if (typeof value === 'number' && Number.isNaN(value)) return ''
  return value ?? ''
}

/** 按容器宽度等比放大列宽，使表格始终铺满编辑区；容器比定义宽度窄时保持原宽并出横向滚动。 */
function scaledColumnWidths(): number[] {
  const defined = columns.value.map((column) => column.width)
  const available = (canvasRef.value?.clientWidth || 0) - GRID_CHROME_WIDTH
  const total = defined.reduce((sum, width) => sum + width, 0)
  if (!Number.isFinite(available) || available <= total || !total) return defined
  const scale = available / total
  return defined.map((width) => Math.floor(width * scale))
}

function designTokenColor(name: string): string {
  const value = getComputedStyle(document.documentElement).getPropertyValue(name).trim()
  if (!value) return 'transparent'
  if (value.startsWith('#') || value.startsWith('rgb') || value.startsWith('hsl')) return value
  const channels = value.split(/[\s,]+/).filter(Boolean)
  return channels.length >= 3 ? `rgb(${channels.slice(0, 3).join(' ')})` : value
}

function workbookData(): IWorkbookData {
  const cellData: NonNullable<IWorkbookData['sheets']>[string]['cellData'] = {}
  cellData[0] = {}
  columns.value.forEach((column, col) => {
    cellData[0]![col] = {
      v: `${column.label}${column.required ? ' *' : ''}`,
      s: { bl: 1, ff: 'Source Han Sans CN', fs: 12, ht: 2, vt: 2, bg: { rgb: designTokenColor('--yb-surface-soft') }, cl: { rgb: designTokenColor(column.required ? '--yb-danger-text' : '--yb-text-muted') } },
    }
  })
  props.rows.forEach((row, rowIndex) => {
    cellData[rowIndex + 1] = {}
    columns.value.forEach((column, col) => {
      cellData[rowIndex + 1]![col] = { v: displayCell(row, column), s: { ff: 'Source Han Sans CN', fs: 11, vt: 2 } }
    })
  })
  const widths = scaledColumnWidths()
  const columnData = Object.fromEntries(widths.map((width, index) => [index, { w: width }]))
  return {
    id: `compose-${props.intent}`,
    name: '统一任务创建工作台',
    appVersion: '0.25.1',
    locale: 'zhCN' as IWorkbookData['locale'],
    styles: {},
    sheetOrder: ['compose-sheet'],
    sheets: {
      'compose-sheet': {
        id: 'compose-sheet',
        name: '任务明细',
        rowCount: Math.max(220, props.rows.length + 8),
        columnCount: columns.value.length,
        cellData,
        columnData,
        rowData: { 0: { h: 38 } },
        defaultRowHeight: 32,
        freeze: { startRow: 1, ySplit: 1, xSplit: 0, startColumn: 0 },
        showGridlines: 1,
      },
    },
    resources: [],
  }
}

async function ensureStyles() {
  if (stylesReady || document.querySelector('style[data-main-univer-compose]')) {
    stylesReady = true
    return
  }
  const [coreCss, drawingCss] = await Promise.all([
    import('@univerjs/preset-sheets-core/lib/index.css?inline'),
    import('@univerjs/preset-sheets-drawing/lib/index.css?inline'),
  ])
  const style = document.createElement('style')
  style.dataset.mainUniverCompose = 'true'
  style.textContent = `${coreCss.default}\n${drawingCss.default}`
  document.head.appendChild(style)
  stylesReady = true
}

async function boot() {
  const sequence = ++bootSequence
  dispose(false)
  await nextTick()
  if (!canvasRef.value) return
  loading.value = true
  errorMessage.value = ''
  canvasRef.value.replaceChildren()
  try {
    await ensureStyles()
    const [{ LogLevel, LocaleType, Univer, mergeLocales }, { FUniver }, { UniverSheetsCorePreset }, { UniverSheetsDrawingPreset }, coreLocale, drawingLocale] = await Promise.all([
      import('@univerjs/core'),
      import('@univerjs/core/facade'),
      import('@univerjs/preset-sheets-core'),
      import('@univerjs/preset-sheets-drawing'),
      import('@univerjs/preset-sheets-core/locales/zh-CN'),
      import('@univerjs/preset-sheets-drawing/locales/zh-CN'),
    ])
    if (sequence !== bootSequence || !canvasRef.value) return
    const univer = new Univer({
      logLevel: LogLevel.WARN,
      locale: LocaleType.ZH_CN,
      locales: { [LocaleType.ZH_CN]: mergeLocales(coreLocale.default, drawingLocale.default) },
    })
    const presets = [
      UniverSheetsCorePreset({
        container: canvasRef.value,
        header: false,
        toolbar: false,
        formulaBar: true,
        footer: false,
        ribbonType: 'collapsed',
        sheets: { disableForceStringAlert: true, disableForceStringMark: true },
      }),
      UniverSheetsDrawingPreset(),
    ]
    for (const preset of presets) {
      for (const entry of preset.plugins as UniverPresetPlugin[]) {
        const [plugin, options] = Array.isArray(entry) ? entry : [entry, undefined]
        univer.registerPlugin(plugin, options)
      }
    }
    const facade = FUniver.newAPI(univer) as unknown as typeof api
    if (!facade) throw new Error('表格初始化失败')
    univerInstance = univer
    api = facade
    workbook = facade.createWorkbook(workbookData())
    const labelUniverCanvases = () => {
      canvasRef.value?.querySelectorAll('canvas').forEach((canvas) => {
        canvas.setAttribute('aria-label', '任务明细表格编辑区域')
      })
    }
    canvasObserver = new MutationObserver(labelUniverCanvases)
    canvasObserver.observe(canvasRef.value, { childList: true, subtree: true })
    labelUniverCanvases()
    resizeObserver = new ResizeObserver(() => {
      if (resizeTimer) clearTimeout(resizeTimer)
      resizeTimer = setTimeout(applyColumnWidths, 160)
    })
    resizeObserver.observe(canvasRef.value)
    eventDisposables.push(facade.addEvent(facade.Event.SheetEditEnded, () => scheduleRead()))
    if (facade.Event.SheetValueChanged) {
      eventDisposables.push(facade.addEvent(facade.Event.SheetValueChanged, () => scheduleRead()))
    }
    eventDisposables.push(facade.addEvent(facade.Event.CommandExecuted, () => scheduleRead()))
    eventDisposables.push(facade.addEvent(facade.Event.CellClicked, (params) => {
      readRowsFromWorkbook()
      const position = { row: Number(params.row), col: Number(params.column) }
      imageBinding?.setActive?.(position)
      const row = props.rows[position.row - 1]
      if (row) {
        activeRowId.value = row.id
        emit('select', row.id)
      }
    }))
    if (facade.Event.SelectionChanged) {
      eventDisposables.push(facade.addEvent(facade.Event.SelectionChanged, (params) => {
        readRowsFromWorkbook()
        const rowIds = composeRowIdsFromSelection(props.rows, params)
        if (!rowIds.length) return
        activeRowId.value = rowIds[rowIds.length - 1]
        emit('selection', rowIds)
      }))
    }
    imageBinding = bindComposeGridImageCells({
      element: canvasRef.value,
      columns: columns.value,
      rows: () => props.rows,
      worksheet: () => facade.getActiveSheet()?.worksheet ?? null,
      hooks: facade.getSheetHooks(),
      onBeforeFiles: readRowsFromWorkbook,
      onFiles(rowId, column, files) {
        const field = column === 'source_assets' ? 'source_assets' : 'reference_assets'
        emit('files', { rowId, field, files })
      },
    }) as typeof imageBinding
    ready.value = true
    highlightedCells = new Set()
    applyViolationHighlights()
  } catch (error) {
    errorMessage.value = error instanceof Error ? error.message : '表格加载失败'
  } finally {
    if (sequence === bootSequence) loading.value = false
  }
}


function scheduleRead() {
  if (applyingVisuals) return
  if (syncTimer) clearTimeout(syncTimer)
  syncTimer = setTimeout(readRowsFromWorkbook, 80)
}

function applyColumnWidths() {
  const worksheet = api?.getActiveSheet()?.worksheet
  if (!worksheet || compactViewport.value) return
  applyingVisuals = true
  try {
    scaledColumnWidths().forEach((width, index) => worksheet.setColumnWidth(index, width))
  } catch { /* 表格尚未就绪时跳过本次布局 */ }
  finally { applyingVisuals = false }
}

/** 校验失败的单元格标红；恢复正常的单元格清回白底。 */
function applyViolationHighlights() {
  if (compactViewport.value || !ready.value) return
  const worksheet = api?.getActiveSheet()?.worksheet
  if (!worksheet) return
  const keyToCol = new Map(columns.value.map((column, index) => [column.key as string, index]))
  const next = new Set<string>()
  for (const issue of props.violations ?? []) {
    if (issue.row_index == null) continue
    const col = keyToCol.get(String(issue.field))
    if (col == null) continue
    next.add(`${issue.row_index + 1}:${col}`)
  }
  const unchanged = next.size === highlightedCells.size && [...next].every((key) => highlightedCells.has(key))
  if (unchanged) return
  applyingVisuals = true
  try {
    for (const key of highlightedCells) {
      if (next.has(key)) continue
      const [row, col] = key.split(':').map(Number)
      worksheet.getRange(row, col, 1, 1).setBackgroundColor(designTokenColor('--yb-surface'))
    }
    for (const key of next) {
      if (highlightedCells.has(key)) continue
      const [row, col] = key.split(':').map(Number)
      worksheet.getRange(row, col, 1, 1).setBackgroundColor(designTokenColor('--yb-danger-soft'))
    }
    highlightedCells = next
  } catch { /* 高亮失败不阻塞编辑 */ }
  finally { applyingVisuals = false }
}

/** 把视图定位并激活到指定行的字段单元格，用于“点击错误跳转”。 */
function focusCell(rowIndex: number, field?: string) {
  if (compactViewport.value) return
  const worksheet = api?.getActiveSheet()?.worksheet
  if (!worksheet) return
  const col = field ? columns.value.findIndex((column) => column.key === field) : 0
  try {
    worksheet.getRange(rowIndex + 1, Math.max(0, col), 1, 1).activate()
    canvasRef.value?.scrollIntoView({ behavior: 'smooth', block: 'center' })
  } catch { /* 激活失败时仅跳过 */ }
}

/** 把单元格原始值写回行模型；数字列填了非数字会存成 NaN，交给校验层提示。 */
function parseCellInto(target: ComposeRow, raw: unknown, column: ComposeColumn): boolean {
  if (column.kind === 'asset') return false
  const record = target as unknown as Record<string, unknown>
  const before = record[column.key]
  if (column.key === 'set_mode_hint') {
    target.set_mode_hint = /^(1|true|是|套装|建议套装)$/i.test(String(raw ?? '').trim())
  } else if (column.key === 'structure_type') {
    const text = String(raw ?? '').trim().toLowerCase()
    record[column.key] = !text ? undefined : /^(立体|3d|three[_ -]?dimensional)$/.test(text) ? 'three_dimensional' : /^(平面|2d|flat)$/.test(text) ? 'flat' : text
  } else if (column.key === 'slotting') {
    const text = String(raw ?? '').trim().toLowerCase()
    record[column.key] = !text ? undefined : /^(开槽|slotted)$/.test(text) ? 'slotted' : /^(不开槽|无槽|not[_ -]?slotted)$/.test(text) ? 'not_slotted' : text
  } else if (column.kind === 'number') {
    const text = String(raw ?? '').trim()
    const numeric = Number(text)
    record[column.key] = !text ? undefined : Number.isFinite(numeric) ? numeric : Number.NaN
  } else {
    record[column.key] = String(raw ?? '').trim()
  }
  const after = record[column.key]
  if (typeof before === 'number' && typeof after === 'number' && Number.isNaN(before) && Number.isNaN(after)) return false
  return after !== before && !(before == null && after === '')
}

function readRowsFromWorkbook() {
  if (compactViewport.value) return
  const worksheet = api?.getActiveSheet()?.worksheet
  if (!worksheet || !columns.value.length) return
  const sheetRows = typeof worksheet.getMaxRows === 'function' ? worksheet.getMaxRows() : props.rows.length + 1
  const scanCount = Math.max(props.rows.length, Math.min(sheetRows - 1, 400))
  if (!scanCount) return
  const values = worksheet.getRange(1, 0, scanCount, columns.value.length).getValues()
  let changed = false
  const next = props.rows.map((row, rowIndex) => {
    const copy = { ...row }
    columns.value.forEach((column, col) => {
      if (parseCellInto(copy, values[rowIndex]?.[col], column)) changed = true
    })
    return copy
  })
  // 用户在模型之外的行粘贴或直接填写时，把这些行并入行模型（中间空行补位，保持行号对齐）。
  for (let rowIndex = props.rows.length; rowIndex < scanCount; rowIndex += 1) {
    const hasContent = columns.value.some((column, col) => column.kind !== 'asset' && String(values[rowIndex]?.[col] ?? '').trim())
    if (!hasContent) continue
    while (next.length < rowIndex) next.push(createComposeRow())
    const fresh = createComposeRow()
    columns.value.forEach((column, col) => parseCellInto(fresh, values[rowIndex]?.[col], column))
    next.push(fresh)
    changed = true
  }
  if (!changed) return
  emit('update:rows', next)
}

function dispose(increment = true) {
  if (increment) bootSequence += 1
  if (syncTimer) clearTimeout(syncTimer)
  if (resizeTimer) clearTimeout(resizeTimer)
  resizeObserver?.disconnect()
  resizeObserver = null
  eventDisposables.forEach((item) => item.dispose())
  eventDisposables = []
  imageBinding?.dispose()
  imageBinding = null
  canvasObserver?.disconnect()
  canvasObserver = null
  workbook?.dispose?.()
  workbook = null
  try { univerInstance?.dispose() } catch { /* navigation must remain safe */ }
  univerInstance = null
  api = null
  ready.value = false
  highlightedCells = new Set()
}

defineExpose({ readRowsFromWorkbook, boot, focusCell })
</script>

<template>
  <section ref="rootRef" class="compose-grid" aria-label="任务明细表格">
    <div class="compose-grid__head">
      <div><p>可以直接从 Excel / WPS 复制粘贴</p><span>支持拖拽选区、Ctrl/Cmd+C/V/Z；选中连续多行后可在上方一次删除。图片可直接拖进“{{ intent === 'planning_sku' ? '产品图片' : '参考图' }}”列。</span></div>
      <span class="compose-grid__engine"><Table2 :size="15" aria-hidden="true" />{{ compactViewport ? '卡片填写' : '在线表格' }}</span>
    </div>
    <div v-if="!compactViewport" class="compose-grid__canvas-shell">
      <div ref="canvasRef" class="compose-grid__canvas" :class="{ 'is-ready': ready }" />
      <div v-if="loading" class="compose-grid__state"><LoaderCircle class="spin" :size="22" />表格加载中…</div>
      <div v-else-if="errorMessage" class="compose-grid__state is-error"><AlertTriangle :size="22" />{{ errorMessage }}，请刷新页面重试。</div>
    </div>
    <p class="compose-grid__tip"><ImagePlus :size="15" aria-hidden="true" />{{ intent === 'planning_sku' ? '每行可以放 1 张产品图片。' : '每行最多放 5 张参考图。' }}</p>
  </section>
</template>

<style scoped>
.compose-grid{display:grid;gap:.65rem}.compose-grid__head{display:flex;justify-content:space-between;gap:1rem;align-items:flex-end}.compose-grid__head p{margin:0;font-weight:760;color:rgb(var(--yb-text-primary))}.compose-grid__head span,.compose-grid__tip{font-size:.78rem;color:rgb(var(--yb-text-secondary))}.compose-grid__head .compose-grid__engine{display:inline-flex;align-items:center;gap:.35rem;padding:.42rem .7rem;border-radius:999px;background:rgb(var(--yb-brand-soft));color:rgb(var(--yb-brand-deep));white-space:nowrap}.compose-grid__canvas-shell{position:relative;min-height:420px;border:1px solid rgb(var(--yb-border-context));border-radius:1rem;overflow:hidden;background:rgb(var(--yb-surface))}.compose-grid__canvas{height:clamp(420px,54vh,720px);opacity:0;transition:opacity .2s ease}.compose-grid__canvas.is-ready{opacity:1}.compose-grid__state{position:absolute;inset:0;display:grid;place-content:center;justify-items:center;gap:.65rem;color:rgb(var(--yb-text-secondary));background:rgb(var(--yb-surface))}.compose-grid__state.is-error{color:rgb(var(--yb-danger));padding:2rem;text-align:center}.compose-grid__tip{margin:0;display:flex;gap:.4rem;align-items:center}.spin{animation:compose-spin 1s linear infinite}@keyframes compose-spin{to{transform:rotate(360deg)}}@media(max-width:760px){.compose-grid__canvas-shell{display:none}.compose-grid__head{align-items:flex-start;flex-direction:column}.compose-grid__tip{display:none}}
</style>
