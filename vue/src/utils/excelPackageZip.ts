import type { AssetExcelPackageItem } from '@/services/api/assetsApi'
import { sanitizeZipEntryName } from '@/utils/batchZipDownload'

type ExcelPackageZipItem = Pick<
  AssetExcelPackageItem,
  'asset_id' | 'download_url' | 'filename' | 'order_no' | 'resource_id' | 'sku_code' | 'sku_name' | 'source_type'
>

type FetchLike = (input: RequestInfo | URL, init?: RequestInit) => Promise<Response>

function normalizedIncludes(value: string, expected: string): boolean {
  const normalizedValue = value.trim().toUpperCase()
  const normalizedExpected = expected.trim().toUpperCase()
  return normalizedExpected !== '' && normalizedValue.includes(normalizedExpected)
}

export function resolveExcelPackageZipFilename(item: ExcelPackageZipItem, sequence: number): string {
  const rawFilename = String(item.filename ?? '').trim()
  const extensionMatch = rawFilename.match(/\.[A-Za-z0-9]{1,10}$/)
  const extension = extensionMatch?.[0] ?? '.jpg'
  const rawSourceBase = extensionMatch ? rawFilename.slice(0, -extension.length) : rawFilename
  const fallback = `asset-${item.asset_id}`
  const sourceBase = sanitizeZipEntryName(rawSourceBase, fallback)
  const rawSku = String(item.sku_code || item.sku_name || fallback).trim()
  const rawOrder = String(item.order_no ?? '').trim()
  const parts: string[] = []

  if (rawOrder && rawOrder.toUpperCase() !== rawSku.toUpperCase() && !normalizedIncludes(sourceBase, rawOrder)) {
    parts.push(sanitizeZipEntryName(rawOrder, '未知订单'))
  }
  if (rawSku && !normalizedIncludes(sourceBase, rawSku)) {
    parts.push(sanitizeZipEntryName(rawSku, fallback))
  }
  parts.push(sourceBase)

  const base = parts.filter((part, index) => part && parts.indexOf(part) === index).join('_') || fallback
  return `${base}_${Math.max(1, Math.trunc(sequence))}${extension}`
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
      const response = await fetcher(url, { credentials: 'omit', mode: 'cors' })
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
