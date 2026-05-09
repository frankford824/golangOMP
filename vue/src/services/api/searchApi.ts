import type { V1GlobalSearchResponse, V1GlobalSearchScope } from '@/domain/global-search'
import http from '@/services/http'

export const searchApi = {
  query: (
    params: {
      keyword: string
      scope?: V1GlobalSearchScope
      limit?: number
    },
    signal?: AbortSignal,
  ) =>
    http.get<V1GlobalSearchResponse>('/v1/search', {
      params: {
        q: params.keyword,
        scope: params.scope ?? 'all',
        ...(params.limit != null ? { limit: params.limit } : {}),
      },
      signal,
    }),
}
