/**
 * 任务动态中的资源类型来自后端稳定 code；这里只做用户可读的中文转换。
 */
const ASSET_KIND_CN: Record<string, string> = {
  reference: '运营参考图',
  source: '设计源文件 / 审核修订源文件',
  delivery: '最终成品图',
  preview: '预览辅助',
  design_thumb: '预览辅助',
}

export function assetKindLabelCn(raw: string | undefined | null): string {
  const text = (raw ?? '').trim()
  if (!text) return '—'
  return ASSET_KIND_CN[text.toLowerCase()] ?? text
}
