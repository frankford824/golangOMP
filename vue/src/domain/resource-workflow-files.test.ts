// @vitest-environment jsdom
import JSZip from 'jszip'
import { describe, expect, it } from 'vitest'
import { buildSourceBundleFile, expandFinalUploadFiles } from './resource-workflow-files'

function arrayBuffer(blob: Blob): Promise<ArrayBuffer> {
  return new Promise((resolve, reject) => {
    const reader = new FileReader()
    reader.onerror = () => reject(reader.error)
    reader.onload = () => resolve(reader.result as ArrayBuffer)
    reader.readAsArrayBuffer(blob)
  })
}

describe('resource workflow file preparation', () => {
  it('keeps one source unchanged and packages multiple sources in stable order', async () => {
    const first = new File(['psd'], '主图.psd', { type: 'application/octet-stream' })
    const second = new File(['ai'], '刀版.ai', { type: 'application/octet-stream' })

    expect(await buildSourceBundleFile([first], 'SKU-1-设计源文件')).toBe(first)
    const bundle = await buildSourceBundleFile([first, second], 'SKU-1-设计源文件')
    expect(bundle.name).toBe('SKU-1-设计源文件.zip')

    const zip = await JSZip.loadAsync(await arrayBuffer(bundle))
    expect(Object.keys(zip.files)).toEqual(['001_主图.psd', '002_刀版.ai', 'manifest.json'])
    const manifest = JSON.parse(await zip.file('manifest.json')!.async('text'))
    expect(manifest.files.map((item: { original_name: string }) => item.original_name)).toEqual(['主图.psd', '刀版.ai'])
  })

  it('expands image ZIPs, ignores metadata, and preserves deterministic order', async () => {
    const zip = new JSZip()
    zip.file('02-背面.png', 'back')
    zip.file('01-正面.jpg', 'front')
    zip.file('__MACOSX/._01-正面.jpg', 'metadata')
    zip.file('说明.txt', 'not an image')
    const archive = new File([await zip.generateAsync({ type: 'blob' })], '套装成品.zip', { type: 'application/zip' })

    const files = await expandFinalUploadFiles([archive])
    expect(files.map((file) => file.name)).toEqual(['01-正面.jpg', '02-背面.png'])
    expect(files.map((file) => file.type)).toEqual(['image/jpeg', 'image/png'])
  })

  it('rejects an archive without final images', async () => {
    const zip = new JSZip()
    zip.file('README.txt', 'empty')
    const archive = new File([await zip.generateAsync({ type: 'blob' })], 'empty.zip')
    await expect(expandFinalUploadFiles([archive])).rejects.toThrow('没有支持的图片')
  })
})
