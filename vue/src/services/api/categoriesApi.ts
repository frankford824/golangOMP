/**
 * 分类与品类映射 API
 * 汇总表：款式分类（本地）GET /v1/categories、GET /v1/categories/search
 * 材质选项：GET /v1/category-mappings (secondary_condition_key=material)
 * 材质分类：GET /v1/categories (category_type=material)
 */
import http from '@/services/http'

export interface Category {
  id?: number
  /** OpenAPI 与部分后端使用 category_id */
  category_id?: number
  category_code?: string
  category_name?: string
  display_name?: string
  category_type?: string
  search_entry_code?: string
  is_search_entry?: boolean
  parent_id?: number
  level?: number
  is_active?: boolean
  sort_order?: number
}

export interface CategoryMapping {
  id?: number
  category_id?: number
  category_code?: string
  search_entry_code?: string
  secondary_condition_key?: string
  secondary_condition_value?: string
  tertiary_condition_key?: string
  tertiary_condition_value?: string
  is_active?: boolean
  is_primary?: boolean
}

export interface CategoriesListParams {
  keyword?: string
  category_type?: string
  parent_id?: number
  level?: number
  is_active?: boolean
  page?: number
  page_size?: number
}

export interface CategoriesSearchParams {
  keyword?: string
  category_type?: string
  is_active?: boolean
  limit?: number
}

export interface CategoryMappingsListParams {
  keyword?: string
  category_id?: number
  category_code?: string
  search_entry_code?: string
  secondary_condition_key?: string
  is_active?: boolean
  is_primary?: boolean
  page?: number
  page_size?: number
}

export const categoriesApi = {
  /** 分类列表 GET /v1/categories */
  list: (params?: CategoriesListParams, signal?: AbortSignal) =>
    http.get<{ data?: Category[]; pagination?: { total: number; page: number; page_size: number } }>(
      '/v1/categories',
      { params, signal },
    ),

  /** 分类搜索 GET /v1/categories/search（轻量选择，适合下拉） */
  search: (params?: CategoriesSearchParams, signal?: AbortSignal) =>
    http.get<{ data?: Category[] }>('/v1/categories/search', { params, signal }),

  create: (payload: Category, signal?: AbortSignal) =>
    http.post('/v1/categories', payload, { signal }),

  update: (id: number | string, payload: Partial<Category>, signal?: AbortSignal) =>
    http.patch(`/v1/categories/${encodeURIComponent(String(id))}`, payload, { signal }),

  /** 品类映射列表 GET /v1/category-mappings（材质选项：secondary_condition_key=material） */
  listMappings: (params?: CategoryMappingsListParams, signal?: AbortSignal) =>
    http.get<{ data?: CategoryMapping[]; pagination?: unknown }>('/v1/category-mappings', {
      params,
      signal,
    }),

  createMapping: (payload: CategoryMapping, signal?: AbortSignal) =>
    http.post('/v1/category-mappings', payload, { signal }),

  updateMapping: (id: number | string, payload: Partial<CategoryMapping>, signal?: AbortSignal) =>
    http.patch(`/v1/category-mappings/${encodeURIComponent(String(id))}`, payload, { signal }),

  listCostRules: (params?: Record<string, unknown>, signal?: AbortSignal) =>
    http.get('/v1/cost-rules', { params, signal }),

  createCostRule: (payload: Record<string, unknown>, signal?: AbortSignal) =>
    http.post('/v1/cost-rules', payload, { signal }),

  updateCostRule: (id: number | string, payload: Record<string, unknown>, signal?: AbortSignal) =>
    http.patch(`/v1/cost-rules/${encodeURIComponent(String(id))}`, payload, { signal }),
}
