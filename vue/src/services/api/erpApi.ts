/**
 * ERP 数据相关 API（MAIN /v1/erp/*，与 OpenAPI 一致）
 */

import http from '@/services/http'
import type { ErpProductsParams } from '@/services/apiTypes'

export const erpApi = {
  /**
   * 查询 ERP 商品列表（用于任务创建时选品）
   * GET /v1/erp/products
   */
  getProducts: (params?: ErpProductsParams, signal?: AbortSignal) =>
    http.get('/v1/erp/products', { params, signal }),

  /**
   * GET /v1/erp/products/{id}
   */
  getProduct: (id: string, signal?: AbortSignal) => http.get(`/v1/erp/products/${id}`, { signal }),

  getProductByCode: (code: string, signal?: AbortSignal) =>
    http.get('/v1/erp/products/by-code', { params: { code }, signal }),

  /**
   * GET /v1/erp/iids
   */
  getIids: (
    params?: { q?: string; keyword?: string; page?: number; page_size?: number },
    signal?: AbortSignal,
  ) => http.get('/v1/erp/iids', { params, signal }),

}
