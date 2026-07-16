import {
  assetWorkbenchApi,
  type DriveFileRow,
  type SystemAssetDownloadInfo,
  type SystemAssetRow,
} from '@aw/shared/api/assetWorkbenchApi'
import { materialAssetKey } from '@aw/shared/materials/systemAssetPreview'
import { downloadIsPreparing, waitForPreparedDownload } from './preparedDownload'
import { useDownloadCenterStore } from './downloadCenter.store'
import { transferDownload, type DownloadTransferMeta, type DownloadTransferProgress, type DownloadTransferResult } from './downloadTransfer'

export function useGlobalDownload() {
  const downloadCenter = useDownloadCenterStore()

  function queueDriveFile(file: DriveFileRow) {
    return downloadCenter.enqueue({
      key: 'drive-file:' + file.id,
      displayName: file.display_name || file.original_filename || '文件-' + file.id,
      sourceLabel: file.upload_directory_name || '我的文件',
      fileSize: file.file_size,
      resolve: async (signal) => {
        const meta = await assetWorkbenchApi.getFileDownload(file.id, signal)
        return {
          downloadUrl: meta.download_url,
          filename: meta.filename || file.display_name || file.original_filename,
          fileSize: meta.file_size || file.file_size,
          mimeType: meta.mime_type || file.mime_type,
        }
      },
    })
  }

  function queueMaterial(asset: SystemAssetRow) {
    const snapshot = { ...asset }
    const materialID = Number(snapshot.material_id || 0)
    const sourceName = snapshot.resource_group_id ? '任务资源组' : snapshot.source_type === 'external' || snapshot.source_type === 'external_asset' ? '外部资源' : '系统资源'
    const initialName = snapshot.original_filename || snapshot.file_name || snapshot.product_name || snapshot.asset_no || '素材文件'
    const knownSize = Number((snapshot as SystemAssetRow & { file_size?: number }).file_size || 0)
    const loadMeta = (signal: AbortSignal) => materialID > 0
      ? assetWorkbenchApi.downloadClientMaterial(materialID, signal)
      : assetWorkbenchApi.downloadMaterialAsset(snapshot, signal)

    return downloadCenter.enqueue({
      key: 'material:' + materialAssetKey(snapshot),
      displayName: initialName,
      sourceLabel: sourceName,
      fileSize: knownSize,
      resolve: async (signal) => {
        let meta = await loadMeta(signal)
        if (downloadIsPreparing(meta)) {
          meta = await waitForPreparedDownload(meta, () => loadMeta(signal), { signal })
        }
        return transferMeta(meta, initialName)
      },
      transfer: snapshot.resource_group_id ? transferResourceGroupBundle : undefined,
    })
  }

  return { downloadCenter, queueDriveFile, queueMaterial }
}

function transferMeta(meta: SystemAssetDownloadInfo, fallbackName: string) {
  const downloadUrl = String(meta.download_url || '').trim()
  if (!downloadUrl) throw new Error('当前文件暂时无法下载，请稍后重试')
  const items = (meta.items || []).map((item) => ({
    downloadUrl: item.download_url,
    filename: item.filename,
    fileSize: Number(item.file_size || 0),
    mimeType: item.mime_type,
  }))
  return {
    downloadUrl,
    filename: items.length > 1 ? `${withoutExtension(fallbackName)}_套装.zip` : meta.filename || fallbackName,
    fileSize: Number(meta.file_size || 0),
    mimeType: meta.mime_type,
    items,
  }
}

type ResourceGroupTransferMeta = DownloadTransferMeta & { items?: DownloadTransferMeta[] }

async function transferResourceGroupBundle(
  rawMeta: DownloadTransferMeta,
  signal: AbortSignal,
  onProgress: (progress: DownloadTransferProgress) => void,
): Promise<DownloadTransferResult> {
  const meta = rawMeta as ResourceGroupTransferMeta
  const items = meta.items || []
  if (items.length <= 1) return transferDownload(items[0] || meta, signal, onProgress)
  const { default: JSZip } = await import('jszip')
  const zip = new JSZip()
  const totalBytes = items.reduce((sum, item) => sum + Number(item.fileSize || 0), 0)
  let receivedBytes = 0
  const startedAt = performance.now()
  for (const [index, item] of items.entries()) {
    const response = await fetch(item.downloadUrl, { cache: 'no-store', credentials: 'same-origin', signal })
    if (!response.ok) throw new Error(`套装第 ${index + 1} 个文件下载失败（${response.status}）`)
    const blob = await response.blob()
    receivedBytes += blob.size
    zip.file(`${String(index + 1).padStart(2, '0')}_${safeBundleFilename(item.filename)}`, blob)
    const elapsed = Math.max(1, performance.now() - startedAt)
    onProgress({ receivedBytes, totalBytes: totalBytes || receivedBytes, speedBytesPerSecond: receivedBytes * 1000 / elapsed, progress: Math.min(90, Math.round(index + 1) / items.length * 90) })
  }
  const archive = await zip.generateAsync({ type: 'blob' }, (metadata) => {
    onProgress({ receivedBytes, totalBytes: totalBytes || receivedBytes, speedBytesPerSecond: 0, progress: 90 + Math.round(metadata.percent / 10) })
  })
  saveBundleBlob(archive, meta.filename)
  const elapsed = Math.max(1, performance.now() - startedAt)
  onProgress({ receivedBytes, totalBytes: totalBytes || receivedBytes, speedBytesPerSecond: receivedBytes * 1000 / elapsed, progress: 100 })
  return { mode: 'tracked', receivedBytes, totalBytes: totalBytes || receivedBytes, speedBytesPerSecond: receivedBytes * 1000 / elapsed }
}

function withoutExtension(filename: string) { return filename.replace(/\.[^.]+$/, '') || '任务资源' }
function safeBundleFilename(filename: string) { return filename.split(/[\\/]/).pop()?.trim() || '成品图' }
function saveBundleBlob(blob: Blob, filename: string) { const url=URL.createObjectURL(blob);const link=document.createElement('a');link.href=url;link.download=safeBundleFilename(filename);document.body.appendChild(link);link.click();link.remove();window.setTimeout(()=>URL.revokeObjectURL(url),30_000) }
