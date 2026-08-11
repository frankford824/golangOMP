import type { ComposeColumn, ComposeColumnKey, ComposeRow } from '@/domain/unified-task-compose'

export interface ComposeGridCellPosition {
  row: number
  col: number
}

export interface ComposeImageCellBinding {
  dispose(): void
  setActive(position: ComposeGridCellPosition): void
}

interface UniverRangeLike {
  insertCellImageAsync(file: File | string): Promise<boolean>
}

interface UniverWorksheetLike {
  getRange(row: number, column: number, numRows?: number, numColumns?: number): UniverRangeLike
  setRowHeight?(rowPosition: number, height: number): unknown
}

interface SheetHookLike {
  onCellDragOver(callback: (position: ComposeGridCellPosition | null) => void): { dispose(): void }
  onCellDrop(callback: (position: ComposeGridCellPosition | null) => void): { dispose(): void }
}

export interface ComposeImageCellOptions {
  element: HTMLElement
  columns: ComposeColumn[]
  rows: () => ComposeRow[]
  worksheet: () => UniverWorksheetLike | null
  hooks: SheetHookLike
  onBeforeFiles?(): void
  onFiles(rowId: string, column: ComposeColumnKey, files: File[]): void
  onActiveCell?(position: ComposeGridCellPosition): void
}

export function isComposeImageColumn(column: ComposeColumn | undefined): boolean {
  return column?.kind === 'asset'
}

export function composeImageColumnAccepts(column: ComposeColumn | undefined, file: File): boolean {
  if (!isComposeImageColumn(column)) return false
  if (column?.key === 'source_assets') return true
  return isComposeImageFile(file)
}

const COMPOSE_IMAGE_FILE_NAME = /\.(?:avif|bmp|gif|heic|heif|jpe?g|png|svg|tiff?|webp)$/i
const COMPOSE_IMAGE_ROW_HEIGHT = 72

export function isComposeImageFile(file: File): boolean {
  return file.type.startsWith('image/') || COMPOSE_IMAGE_FILE_NAME.test(file.name)
}

/**
 * Bridges native File drag/paste data with Univer's cell hit-testing. This owns
 * the image event so Univer cannot also create a duplicate floating drawing.
 * The business row model remains authoritative for upload state and references.
 */
export function bindComposeGridImageCells(options: ComposeImageCellOptions): ComposeImageCellBinding {
  let hover: ComposeGridCellPosition | null = null
  let active: ComposeGridCellPosition | null = null
  const dragDisposable = options.hooks.onCellDragOver((position) => {
    hover = position
  })
  const dropDisposable = options.hooks.onCellDrop((position) => {
    if (position) hover = position
  })

  const resolveTarget = () => hover ?? active
  const handleFiles = async (files: File[]) => {
    const target = resolveTarget()
    if (!target || target.row < 1) return
    const column = options.columns[target.col]
    if (!isComposeImageColumn(column)) return
    const row = options.rows()[target.row - 1]
    if (!row) return
    const limit = column.key === 'source_assets' ? 50 : 5
    const accepted = files.filter((file) => composeImageColumnAccepts(column, file)).slice(0, limit)
    if (!accepted.length) return
    // SheetEditEnded is debounced. If the operator types a requirement and
    // immediately pastes an image, rebuilding the workbook for the new asset
    // can otherwise restore the older row model and erase the fresh text.
    options.onBeforeFiles?.()
    const worksheet = options.worksheet()
    if (worksheet && isComposeImageFile(accepted[0])) {
      worksheet.setRowHeight?.(target.row, COMPOSE_IMAGE_ROW_HEIGHT)
      await worksheet.getRange(target.row, target.col, 1, 1).insertCellImageAsync(accepted[0]).catch(() => false)
    }
    options.onFiles(row.id, column.key, accepted)
  }

  const onDragOver = (event: DragEvent) => {
    const target = resolveTarget()
    if (target && isComposeImageColumn(options.columns[target.col])) {
      event.preventDefault()
      if (event.dataTransfer) event.dataTransfer.dropEffect = 'copy'
    }
  }
  const onDrop = (event: DragEvent) => {
    const files = Array.from(event.dataTransfer?.files ?? [])
    if (!files.length) return
    const target = resolveTarget()
    if (!target || !files.some((file) => composeImageColumnAccepts(options.columns[target.col], file))) return
    event.preventDefault()
    event.stopImmediatePropagation()
    void handleFiles(files)
  }
  const onPaste = (event: ClipboardEvent) => {
    const files = Array.from(event.clipboardData?.files ?? [])
    if (!files.length) return
    const target = resolveTarget()
    if (!target || !files.some((file) => composeImageColumnAccepts(options.columns[target.col], file))) return
    event.preventDefault()
    event.stopImmediatePropagation()
    void handleFiles(files)
  }
  const setActive = (position: ComposeGridCellPosition) => {
    active = position
    options.onActiveCell?.(position)
  }

  options.element.addEventListener('dragover', onDragOver)
  options.element.addEventListener('drop', onDrop, true)
  options.element.addEventListener('paste', onPaste, true)

  return {
    dispose() {
      dragDisposable.dispose()
      dropDisposable.dispose()
      options.element.removeEventListener('dragover', onDragOver)
      options.element.removeEventListener('drop', onDrop, true)
      options.element.removeEventListener('paste', onPaste, true)
    },
    setActive,
  }
}
