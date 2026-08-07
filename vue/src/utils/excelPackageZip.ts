import type { AssetExcelPackageItem } from '@/services/api/assetsApi'
import { sanitizeZipEntryName } from '@/utils/batchZipDownload'

type ExcelPackageZipItem = Pick<
  AssetExcelPackageItem,
  | 'asset_id'
  | 'download_url'
  | 'filename'
  | 'order_no'
  | 'package_folder'
  | 'quantity'
  | 'resource_id'
  | 'row_number'
  | 'sku_code'
  | 'sku_name'
  | 'source_type'
>

type FetchLike = (input: RequestInfo | URL, init?: RequestInit) => Promise<Response>

export interface ExcelPackageZipEntry {
  key: string
  filename: string
  zipPath?: string
  downloadURL: string
  failureHint: string
}

export function resolveExcelPackageBusinessName(item: ExcelPackageZipItem): string {
  const fallback = `asset-${item.asset_id}`
  const skuCode = sanitizeZipEntryName(String(item.sku_code ?? '').trim(), fallback)
  const skuName = sanitizeZipEntryName(String(item.sku_name ?? '').trim(), '未命名商品')
  if (skuName.toUpperCase().includes(skuCode.toUpperCase())) return skuName
  return `${skuCode}_${skuName}`
}

export function resolveExcelPackageZipFilename(
  item: ExcelPackageZipItem,
  sequence: number,
  options?: { setComponent?: boolean },
): string {
  const rawFilename = String(item.filename ?? '').trim()
  const extensionMatch = rawFilename.match(/\.[A-Za-z0-9]{1,10}$/)
  const extension = extensionMatch?.[0] ?? '.jpg'
  const base = resolveExcelPackageBusinessName(item)
  if (!options?.setComponent) return `${base}${extension}`
  return `${base}_${String(Math.max(1, Math.trunc(sequence))).padStart(2, '0')}${extension}`
}

export function resolveExcelPackageSetFolders(items: ExcelPackageZipItem[]): string[] {
  const folders = new Array<string>(items.length).fill('')
  const groups = new Map<string, { base: string; indexes: number[] }>()
  items.forEach((item, index) => {
    const rawFolder = String(item.package_folder ?? '').trim()
    if (!rawFolder) return
    const base = resolveExcelPackageBusinessName(item)
    const key = [item.row_number ?? index, item.order_no, item.sku_code, rawFolder].join('\u0000')
    const group = groups.get(key) ?? { base, indexes: [] }
    group.indexes.push(index)
    groups.set(key, group)
  })

  const usedFolders = new Map<string, number>()
  for (const group of groups.values()) {
    const count = (usedFolders.get(group.base) ?? 0) + 1
    usedFolders.set(group.base, count)
    const folder = count === 1 ? group.base : `${group.base} (${count})`
    group.indexes.forEach((index) => {
      folders[index] = folder
    })
  }
  return folders
}

export function buildExcelPackageZipEntries(items: ExcelPackageZipItem[]): ExcelPackageZipEntry[] {
  const groups = new Map<string, ExcelPackageZipItem[]>()
  items.forEach((item, index) => {
    const key = [item.row_number ?? index, item.order_no, item.sku_code, item.sku_name].join('\u0000')
    const group = groups.get(key) ?? []
    group.push(item)
    groups.set(key, group)
  })

  const usedFolders = new Map<string, number>()
  const entries: ExcelPackageZipEntry[] = []
  for (const group of groups.values()) {
    if (!group.length) continue
    const first = group[0]
    const isSet = group.some((item) => String(item.package_folder ?? '').trim() !== '')
    const sourceItems = isSet ? group : [first]
    const quantity = Math.max(1, Math.trunc(Number(first.quantity) || 1))
    for (let copyIndex = 0; copyIndex < quantity; copyIndex += 1) {
      let zipPath: string | undefined
      if (isSet) {
        const base = resolveExcelPackageBusinessName(first)
        const occurrence = (usedFolders.get(base) ?? 0) + 1
        usedFolders.set(base, occurrence)
        zipPath = occurrence === 1 ? base : `${base} (${occurrence})`
      }
      sourceItems.forEach((item, componentIndex) => {
        entries.push({
          key: `${item.resource_id || item.asset_id}-row-${item.row_number || 0}-copy-${copyIndex + 1}-component-${componentIndex + 1}`,
          filename: resolveExcelPackageZipFilename(item, componentIndex + 1, {
            setComponent: isSet,
          }),
          zipPath,
          downloadURL: item.download_url,
          failureHint: `${item.sku_code || item.sku_name}: download_failed`,
        })
      })
    }
  }
  return entries
}

export function countExcelPackageRows(items: Array<{ row_number?: number }>): number {
  const rows = new Set<string>()
  items.forEach((item, index) => {
    const rowNumber = Number(item.row_number)
    rows.add(Number.isInteger(rowNumber) && rowNumber > 0 ? `row:${rowNumber}` : `item:${index}`)
  })
  return rows.size
}

export function excelPackageSourceKey(item: ExcelPackageZipItem): string {
  const resourceID = String(item.resource_id ?? '').trim()
  if (resourceID) return resourceID
  const sourceType = String(item.source_type ?? '').trim() || 'asset'
  const assetID = Number(item.asset_id)
  if (Number.isInteger(assetID) && assetID > 0) return `${sourceType}:${assetID}`
  return String(item.download_url ?? '').trim()
}

export function sumExcelPackageQuantities(items: Array<{ quantity?: number }>): number {
  return items.reduce((total, item) => {
    const quantity = Number(item.quantity)
    return total + (Number.isFinite(quantity) && quantity > 0 ? Math.trunc(quantity) : 1)
  }, 0)
}

function shouldRetryExcelPackageDownload(status: number): boolean {
  return status === 408 || status === 425 || status === 429 || status >= 500
}

async function fetchExcelPackageBlob(url: string, fetcher: FetchLike): Promise<Blob> {
  let lastError: unknown
  for (let attempt = 1; attempt <= 3; attempt += 1) {
    try {
      const response = await fetcher(url, {
        credentials: 'omit',
        mode: 'cors',
      })
      if (response.ok) return response.blob()
      lastError = new Error(`http_${response.status}`)
      if (!shouldRetryExcelPackageDownload(response.status)) throw lastError
    } catch (error) {
      lastError = error
      if (attempt >= 3) break
    }
    await new Promise((resolve) => globalThis.setTimeout(resolve, attempt * 180))
  }
  throw lastError instanceof Error ? lastError : new Error('fetch_failed')
}

export function createExcelPackageBlobLoader(fetcher: FetchLike = fetch): (item: ExcelPackageZipItem) => Promise<Blob> {
  const cache = new Map<string, Promise<Blob>>()
  return (item) => {
    const url = String(item.download_url ?? '').trim()
    if (!url) return Promise.reject(new Error('missing_download_url'))
    const key = excelPackageSourceKey(item) || url
    const cached = cache.get(key)
    if (cached) return cached
    const pending = fetchExcelPackageBlob(url, fetcher)
    cache.set(key, pending)
    return pending
  }
}
