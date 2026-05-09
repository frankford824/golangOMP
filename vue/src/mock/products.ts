import type { Product } from '@/types'

export const mockProducts: Product[] = [
  { id: 'p1', sku: 'SKU-ERP-1001', name: '夏季主图主款', category: '服饰-上衣', spec: '均码', designHistorySummary: '近30天 3 次设计' },
  { id: 'p2', sku: 'SKU-ERP-2002', name: '促销 Banner', category: '营销素材', spec: '1920x600', designHistorySummary: '近30天 1 次设计' },
  { id: 'p3', sku: 'SKU-ERP-3001', name: '过期主图修改', category: '服饰-裤装', spec: '800x800', designHistorySummary: '近30天 2 次设计' },
  { id: 'p4', sku: 'SKU-ERP-4001', name: '礼盒装外盒', category: '美妆-礼盒', spec: '定制尺寸', designHistorySummary: '无历史' },
]

export const mockProductById = (id: string): Product | undefined =>
  mockProducts.find((p) => p.id === id)
