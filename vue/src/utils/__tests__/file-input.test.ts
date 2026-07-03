// @vitest-environment jsdom

import { describe, expect, it } from 'vitest'
import { snapshotAndResetFileInput } from '../file-input'

function installLiveFiles(input: HTMLInputElement, files: File[]) {
  let cleared = false
  const liveFiles = {
    get length() {
      return cleared ? 0 : files.length
    },
    item(index: number) {
      return cleared ? null : files[index] ?? null
    },
    *[Symbol.iterator]() {
      if (!cleared) yield* files
    },
  } as unknown as FileList

  Object.defineProperty(input, 'files', {
    get: () => liveFiles,
    configurable: true,
  })
  Object.defineProperty(input, 'value', {
    get: () => (cleared ? '' : 'C:\\fakepath\\upload.png'),
    set: (value: string) => {
      if (value === '') cleared = true
    },
    configurable: true,
  })

  return { liveFiles }
}

describe('snapshotAndResetFileInput', () => {
  it('keeps selected files before clearing the browser live FileList', () => {
    const input = document.createElement('input')
    input.type = 'file'
    const file = new File(['image'], 'upload.png', { type: 'image/png' })
    const { liveFiles } = installLiveFiles(input, [file])

    const snapshot = snapshotAndResetFileInput(input)

    expect(snapshot).toEqual([file])
    expect(liveFiles.length).toBe(0)
  })
})
