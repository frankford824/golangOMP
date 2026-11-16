import { assetWorkbenchApi, type DriveFileRow } from '@aw/shared/api/assetWorkbenchApi'
import { isVideoMimeOrFilename } from '@aw/shared/materials/systemAssetPreview'

export async function loadWorkbenchFilePreview(file: DriveFileRow) {
  if (isVideoMimeOrFilename(file.mime_type, file.display_name || file.original_filename)) {
    const original = await assetWorkbenchApi.getFileDownload(file.id)
    return { url: original.download_url || '', label: '视频暂无法在线播放，可下载原件查看' }
  }
  const meta = await assetWorkbenchApi.getFilePreview(file.id)
  return {
    url: meta.preview_url || '',
    label: meta.preparing ? '预览生成中，可先下载原件' : '暂无可展示预览，可下载原件查看',
  }
}
