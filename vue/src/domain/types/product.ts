export interface Product {
  id: string
  sku: string
  name: string
  category: string
  spec: string
  designHistorySummary?: string
  /** ERP 返回的商品主图 URL，可为空 */
  imageUrl?: string
  /** ERP 分类编码，可为空 */
  categoryCode?: string
  /** ERP 产品简称，可为空 */
  shortName?: string
}
