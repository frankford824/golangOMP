// @vitest-environment jsdom
import { describe, expect, it, vi } from 'vitest'

import { bindComposeGridImageCells, composeImageColumnAccepts } from './compose-grid-image-cell'
import { createComposeRow, type ComposeColumn } from '@/domain/unified-task-compose'

describe('compose grid image cell bridge', () => {
  it('accepts images for reference cells and any file for source cells', () => {
    const image = { type: 'image/png' } as File
    const archive = { type: 'application/zip' } as File
    expect(composeImageColumnAccepts({ key: 'reference_assets', label: '参考图', width: 120, kind: 'asset' }, image)).toBe(true)
    expect(composeImageColumnAccepts({ key: 'reference_assets', label: '参考图', width: 120, kind: 'asset' }, archive)).toBe(false)
    expect(composeImageColumnAccepts({ key: 'source_assets', label: '素材', width: 120, kind: 'asset' }, archive)).toBe(true)
  })

  it('uses the active Univer cell for clipboard images and inserts an immediate preview', async () => {
    const element = document.createElement('div')
    const insert = vi.fn().mockResolvedValue(true)
    const onFiles = vi.fn()
    const columns: ComposeColumn[] = [{ key: 'reference_assets', label: '参考图', width: 120, kind: 'asset' }]
    const binding = bindComposeGridImageCells({
      element,
      columns,
      rows: () => [createComposeRow({ id: 'row-1' })],
      worksheet: () => ({ getRange: () => ({ insertCellImageAsync: insert }) }),
      hooks: {
        onCellDragOver: () => ({ dispose: vi.fn() }),
        onCellDrop: () => ({ dispose: vi.fn() }),
      },
      onFiles,
    })
    binding.setActive({ row: 1, col: 0 })
    const file = new File(['image'], 'reference.png', { type: 'image/png' })
    const event = new Event('paste', { bubbles: true, cancelable: true })
    Object.defineProperty(event, 'clipboardData', { value: { files: [file] } })
    element.dispatchEvent(event)
    await Promise.resolve()
    await Promise.resolve()

    expect(insert).toHaveBeenCalledWith(file)
    expect(onFiles).toHaveBeenCalledWith('row-1', 'reference_assets', [file])
    binding.dispose()
  })

  it('uses the hovered source column for a drop even when another cell was active', async () => {
    const element = document.createElement('div')
    const onFiles = vi.fn()
    let dragOver: (position: { row: number; col: number } | null) => void = () => undefined
    const columns: ComposeColumn[] = [
      { key: 'reference_assets', label: '参考图', width: 120, kind: 'asset' },
      { key: 'source_assets', label: '源文件', width: 120, kind: 'asset' },
    ]
    const binding = bindComposeGridImageCells({
      element,
      columns,
      rows: () => [createComposeRow({ id: 'row-1' })],
      worksheet: () => null,
      hooks: {
        onCellDragOver: (callback) => { dragOver = callback; return { dispose: vi.fn() } },
        onCellDrop: () => ({ dispose: vi.fn() }),
      },
      onFiles,
    })
    binding.setActive({ row: 1, col: 0 })
    dragOver({ row: 1, col: 1 })
    const source = new File(['psd'], 'design.psd', { type: 'application/octet-stream' })
    const event = new Event('drop', { bubbles: true, cancelable: true })
    Object.defineProperty(event, 'dataTransfer', { value: { files: [source], dropEffect: 'none' } })
    element.dispatchEvent(event)
    await Promise.resolve()

    expect(onFiles).toHaveBeenCalledWith('row-1', 'source_assets', [source])
    binding.dispose()
  })
})
