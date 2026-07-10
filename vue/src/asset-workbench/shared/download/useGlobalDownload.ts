import {
  assetWorkbenchApi,
  type DriveFileRow,
  type SystemAssetDownloadInfo,
  type SystemAssetRow,
} from '@aw/shared/api/assetWorkbenchApi'
import { materialAssetKey } from '@aw/shared/materials/systemAssetPreview'
import { downloadIsPreparing, waitForPreparedDownload } from './preparedDownload'
import { useDownloadCenterStore } from './downloadCenter.store'

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
    const sourceName = snapshot.source_type === 'external' ? '外部资源' : '系统资源'
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
    })
  }

  return { downloadCenter, queueDriveFile, queueMaterial }
}

function transferMeta(meta: SystemAssetDownloadInfo, fallbackName: string) {
  const downloadUrl = String(meta.download_url || '').trim()
  if (!downloadUrl) throw new Error('当前文件暂时无法下载，请稍后重试')
  return {
    downloadUrl,
    filename: meta.filename || fallbackName,
    fileSize: Number(meta.file_size || 0),
    mimeType: meta.mime_type,
  }
}
