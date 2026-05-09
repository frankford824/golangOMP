export interface MockRequest {
  method: string
  path: string
  query: Record<string, unknown>
  body: Record<string, unknown> | null
}

export interface MockHttpResponse<T = unknown> {
  status: number
  data: T
}

export type MockHandler = (request: MockRequest) => MockHttpResponse | null
