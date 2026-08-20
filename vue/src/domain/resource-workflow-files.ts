import JSZip from 'jszip'

const SOURCE_BUNDLE_MAX_BYTES = 299 * 1024 * 1024
const FIXED_ZIP_DATE = new Date('1980-01-01T00:00:00.000Z')
const IMAGE_EXTENSIONS = new Set(['avif', 'bmp', 'gif', 'heic', 'heif', 'jpeg', 'jpg', 'png', 'svg', 'tif', 'tiff', 'webp'])
const FINAL_DESIGN_EXTENSIONS = new Set(['ai', 'cdr', 'plt', 'psb', 'psd'])
const FINAL_DOCUMENT_EXTENSIONS = new Set(['pdf'])
const FINAL_DIRECT_EXTENSIONS = new Set([...IMAGE_EXTENSIONS, ...FINAL_DESIGN_EXTENSIONS, ...FINAL_DOCUMENT_EXTENSIONS])

export const FINAL_UPLOAD_ACCEPT_ATTRIBUTE = [
  'image/*',
  ...[...FINAL_DIRECT_EXTENSIONS].sort().map((extension) => `.${extension}`),
  'application/pdf',
  '.zip',
  'application/zip',
].join(',')

function safeName(name: string, fallback: string) {
  const normalized = name
    .trim()
    .replace(/[\\/:*?"<>|\u0000-\u001f]/g, '_')
    .replace(/\.\./g, '_')
  return normalized || fallback
}

function fileExtension(name: string) {
  const normalized = name.trim().toLowerCase()
  const index = normalized.lastIndexOf('.')
  return index >= 0 ? normalized.slice(index + 1) : ''
}

function isArchiveMetadata(path: string) {
  const normalized = path.replace(/\\/g, '/').replace(/^\/+/, '')
  return normalized === '__MACOSX' ||
    normalized.startsWith('__MACOSX/') ||
    normalized.split('/').pop() === '.DS_Store'
}

function isFinalEntry(path: string) {
  return FINAL_DIRECT_EXTENSIONS.has(fileExtension(path))
}

function readBlobAsArrayBuffer(blob: Blob): Promise<ArrayBuffer> {
  if (typeof blob.arrayBuffer === 'function') return blob.arrayBuffer()
  return new Promise((resolve, reject) => {
    const reader = new FileReader()
    reader.onerror = () => reject(reader.error || new Error('file_read_failed'))
    reader.onload = () => resolve(reader.result as ArrayBuffer)
    reader.readAsArrayBuffer(blob)
  })
}

function uniqueFileName(name: string, used: Map<string, number>) {
  const count = (used.get(name) ?? 0) + 1
  used.set(name, count)
  if (count === 1) return name
  const dot = name.lastIndexOf('.')
  return dot <= 0 ? `${name} (${count})` : `${name.slice(0, dot)} (${count})${name.slice(dot)}`
}

export async function buildSourceBundleFile(files: File[], bundleName: string): Promise<File> {
  if (!files.length) throw new Error('请至少选择一个设计源文件。')
  if (files.length === 1) return files[0]

  const sourceBytes = files.reduce((total, file) => total + file.size, 0)
  if (sourceBytes > SOURCE_BUNDLE_MAX_BYTES) {
    throw new Error('多源文件打包后不能超过 299MB，请拆分或压缩后重试。')
  }

  const zip = new JSZip()
  const manifest = files.map((file, index) => {
    const order = index + 1
    const name = safeName(file.name, `source-${order}`)
    const archivePath = `${String(order).padStart(3, '0')}_${name}`
    zip.file(archivePath, file, {
      binary: true,
      compression: 'STORE',
      date: FIXED_ZIP_DATE,
      unixPermissions: 0o100644,
    })
    return {
      order,
      archive_path: archivePath,
      original_name: file.name,
      size: file.size,
      mime_type: file.type || 'application/octet-stream',
    }
  })
  zip.file('manifest.json', JSON.stringify({ version: 1, files: manifest }, null, 2) + '\n', {
    compression: 'STORE',
    date: FIXED_ZIP_DATE,
    unixPermissions: 0o100644,
  })

  const blob = await zip.generateAsync({
    type: 'blob',
    compression: 'STORE',
    platform: 'UNIX',
    streamFiles: true,
  })
  if (blob.size > 300 * 1024 * 1024) {
    throw new Error('多源文件打包后超过 300MB 上传上限，请拆分或压缩后重试。')
  }
  const filename = safeName(bundleName.replace(/\.zip$/i, ''), '设计源文件包') + '.zip'
  return new File([blob], filename, { type: 'application/zip', lastModified: FIXED_ZIP_DATE.getTime() })
}

export async function expandFinalUploadFiles(files: File[]): Promise<File[]> {
  const expanded: File[] = []
  const usedNames = new Map<string, number>()

  for (const file of files) {
    if (fileExtension(file.name) !== 'zip') {
      if (!isFinalEntry(file.name)) {
        throw new Error(`成品只支持图片、PSD/PSB/AI/CDR/PLT、PDF 或包含这些格式的 ZIP：${file.name}`)
      }
      const name = uniqueFileName(safeName(file.name, 'final-file'), usedNames)
      expanded.push(name === file.name ? file : new File([file], name, { type: file.type, lastModified: file.lastModified }))
      continue
    }

    let zip: JSZip
    try {
      zip = await JSZip.loadAsync(await readBlobAsArrayBuffer(file), { checkCRC32: true })
    } catch {
      throw new Error(`无法读取成品压缩包，请确认 ZIP 未加密且文件完整：${file.name}`)
    }
    const entries = Object.values(zip.files)
      .filter((entry) => !entry.dir && !isArchiveMetadata(entry.name) && isFinalEntry(entry.name))
      .sort((left, right) => left.name.localeCompare(right.name, 'zh-CN', { numeric: true }))
    if (!entries.length) {
      throw new Error(`成品压缩包内没有支持的图片、设计文件或 PDF：${file.name}`)
    }
    for (const entry of entries) {
      const baseName = safeName(entry.name.replace(/\\/g, '/').split('/').pop() || '', 'final-file')
      const name = uniqueFileName(baseName, usedNames)
      const bytes = await entry.async('uint8array')
      const buffer = bytes.buffer.slice(bytes.byteOffset, bytes.byteOffset + bytes.byteLength) as ArrayBuffer
      expanded.push(new File([buffer], name, { type: finalMimeType(name), lastModified: FIXED_ZIP_DATE.getTime() }))
    }
  }
  return expanded
}

function finalMimeType(name: string) {
  const extension = fileExtension(name)
  if (extension === 'pdf') return 'application/pdf'
  if (extension === 'psd') return 'image/vnd.adobe.photoshop'
  if (extension === 'ai') return 'application/postscript'
  if (extension === 'plt') return 'application/vnd.hp-hpgl'
  if (extension === 'jpg' || extension === 'jpeg') return 'image/jpeg'
  if (extension === 'svg') return 'image/svg+xml'
  if (extension === 'tif' || extension === 'tiff') return 'image/tiff'
  if (IMAGE_EXTENSIONS.has(extension)) return `image/${extension}`
  return 'application/octet-stream'
}
