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
    const setRowHeight = vi.fn()
    const onFiles = vi.fn()
    const calls: string[] = []
    const columns: ComposeColumn[] = [{ key: 'reference_assets', label: '参考图', width: 120, kind: 'asset' }]
    const binding = bindComposeGridImageCells({
      element,
      columns,
      rows: () => [createComposeRow({ id: 'row-1' })],
      worksheet: () => ({ getRange: () => ({ insertCellImageAsync: insert }), setRowHeight }),
      hooks: {
        onCellDragOver: () => ({ dispose: vi.fn() }),
        onCellDrop: () => ({ dispose: vi.fn() }),
      },
      onBeforeFiles: () => calls.push('flush'),
      onFiles: (...args) => { calls.push('files'); onFiles(...args) },
    })
    binding.setActive({ row: 1, col: 0 })
    const downstreamPaste = vi.fn()
    element.addEventListener('paste', downstreamPaste)
    const file = new File(['image'], 'reference.png', { type: 'image/png' })
    const event = new Event('paste', { bubbles: true, cancelable: true })
    Object.defineProperty(event, 'clipboardData', { value: { files: [file] } })
    element.dispatchEvent(event)
    await Promise.resolve()
    await Promise.resolve()

    expect(insert).toHaveBeenCalledWith(file)
    expect(setRowHeight).toHaveBeenCalledWith(1, 72)
    expect(onFiles).toHaveBeenCalledWith('row-1', 'reference_assets', [file], { previewInserted: true })
    expect(calls).toEqual(['flush', 'files'])
    expect(event.defaultPrevented).toBe(true)
    expect(downstreamPaste).not.toHaveBeenCalled()
    binding.dispose()
  })

  it('does not intercept image paste outside an asset column', async () => {
    const element = document.createElement('div')
    const onFiles = vi.fn()
    const downstreamPaste = vi.fn()
    const binding = bindComposeGridImageCells({
      element,
      columns: [{ key: 'description_spec', label: '产品描述', width: 180, kind: 'text' }],
      rows: () => [createComposeRow({ id: 'row-1' })],
      worksheet: () => null,
      hooks: {
        onCellDragOver: () => ({ dispose: vi.fn() }),
        onCellDrop: () => ({ dispose: vi.fn() }),
      },
      onFiles,
    })
    binding.setActive({ row: 1, col: 0 })
    element.addEventListener('paste', downstreamPaste)
    const event = new Event('paste', { bubbles: true, cancelable: true })
    Object.defineProperty(event, 'clipboardData', { value: { files: [new File(['image'], 'reference.png', { type: 'image/png' })] } })
    element.dispatchEvent(event)
    await Promise.resolve()

    expect(event.defaultPrevented).toBe(false)
    expect(downstreamPaste).toHaveBeenCalledOnce()
    expect(onFiles).not.toHaveBeenCalled()
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

    expect(onFiles).toHaveBeenCalledWith('row-1', 'source_assets', [source], { previewInserted: false })
    binding.dispose()
  })

  it('keeps a forty-file source drop instead of applying the five-image reference limit', async () => {
    const element = document.createElement('div')
    const onFiles = vi.fn()
    const files = Array.from({ length: 40 }, (_, index) => new File(['psd'], `source-${index + 1}.psd`, { type: 'application/octet-stream' }))
    const binding = bindComposeGridImageCells({
      element,
      columns: [{ key: 'source_assets', label: '待修素材', width: 140, kind: 'asset' }],
      rows: () => [createComposeRow({ id: 'row-1' })],
      worksheet: () => null,
      hooks: {
        onCellDragOver: () => ({ dispose: vi.fn() }),
        onCellDrop: () => ({ dispose: vi.fn() }),
      },
      onFiles,
    })
    binding.setActive({ row: 1, col: 0 })
    const event = new Event('drop', { bubbles: true, cancelable: true })
    Object.defineProperty(event, 'dataTransfer', { value: { files, dropEffect: 'none' } })
    element.dispatchEvent(event)
    await Promise.resolve()

    expect(onFiles).toHaveBeenCalledWith('row-1', 'source_assets', files, { previewInserted: false })
    binding.dispose()
  })
})
