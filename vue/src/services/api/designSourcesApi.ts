import http from '@/services/http'

export const designSourcesApi = {
  search: (
    params: {
      keyword: string
      page?: number
      size?: number
    },
    signal?: AbortSignal,
  ) =>
    http.get('/v1/design-sources/search', {
      params: {
        keyword: params.keyword,
        ...(params.page != null ? { page: params.page } : {}),
        ...(params.size != null ? { size: params.size } : {}),
      },
      signal,
    }),
}
