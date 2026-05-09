import type { OutsourceOrder } from '@/types'

export const mockOutsourceOrders: OutsourceOrder[] = [
  {
    id: 'out1',
    orderNo: 'OUT-20240306-01',
    taskId: 't5',
    taskNo: 'T20240305003',
    sku: 'SKU-ERP-3001',
    productName: '过期主图修改',
    outsourceType: '二创',
    supplierId: 'sup1',
    supplierName: '协作方A',
    deliveryRequirement: '按终稿二创',
    specNote: '800x800',
    status: 'returned',
    createdAt: '2024-03-05T10:00:00',
    returnedAt: '2024-03-06T09:00:00',
    reviewResult: undefined,
  },
]
