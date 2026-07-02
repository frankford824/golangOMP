export interface GalleryVirtualScrollInput {
  itemCount: number
  scrollTop: number
  viewportHeight: number
  containerWidth: number
  cardMinWidth?: number
  rowHeight?: number
  gridPadding?: number
  gridGap?: number
  overscan?: number
}

export interface GalleryVirtualScrollRange {
  columnCount: number
  rowCount: number
  totalHeight: number
  startIndex: number
  endIndex: number
  top: number
}

export function computeGalleryVirtualScroll(input: GalleryVirtualScrollInput): GalleryVirtualScrollRange {
  const cardMinWidth = input.cardMinWidth ?? 224
  const rowHeight = input.rowHeight ?? 306
  const gridPadding = input.gridPadding ?? 12
  const gridGap = input.gridGap ?? 12
  const overscan = input.overscan ?? 2
  const itemCount = Math.max(0, input.itemCount)

  const contentWidth = Math.max(cardMinWidth, input.containerWidth - gridPadding * 2)
  const columnCount = Math.max(1, Math.floor((contentWidth + gridGap) / (cardMinWidth + gridGap)))
  const rowCount = itemCount > 0 ? Math.ceil(itemCount / columnCount) : 0
  const totalHeight = rowCount > 0 ? rowCount * rowHeight + gridPadding * 2 - gridGap : 0

  if (itemCount === 0 || rowCount === 0) {
    return { columnCount, rowCount, totalHeight, startIndex: 0, endIndex: 0, top: gridPadding }
  }

  const maxScrollTop = Math.max(0, totalHeight - input.viewportHeight)
  const clampedScrollTop = Math.min(Math.max(0, input.scrollTop), maxScrollTop)
  const startRow = Math.max(0, Math.floor(Math.max(0, clampedScrollTop - gridPadding) / rowHeight) - overscan)
  const visibleRows = Math.ceil(input.viewportHeight / rowHeight) + overscan * 2
  const endRow = Math.min(rowCount, startRow + visibleRows)

  return {
    columnCount,
    rowCount,
    totalHeight,
    startIndex: startRow * columnCount,
    endIndex: Math.min(itemCount, endRow * columnCount),
    top: gridPadding + startRow * rowHeight,
  }
}
