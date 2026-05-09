import { assetsHandler } from './assets'
import { authHandler } from './auth'
import { batchSkuHandler } from './batchSku'
import { designSourcesHandler } from './designSources'
import { erpBridgeHandler } from './erpBridge'
import { notificationsHandler } from './notifications'
import { orgHandler } from './org'
import { reportsHandler } from './reports'
import { searchHandler } from './search'
import { taskDraftsHandler } from './taskDrafts'
import { taskModulesHandler } from './taskModules'
import { tasksHandler } from './tasks'
import type { MockHandler, MockHttpResponse, MockRequest } from './types'

const handlers: MockHandler[] = [
  authHandler,
  tasksHandler,
  taskModulesHandler,
  taskDraftsHandler,
  erpBridgeHandler,
  designSourcesHandler,
  notificationsHandler,
  orgHandler,
  reportsHandler,
  searchHandler,
  assetsHandler,
  batchSkuHandler,
]

export function dispatchMockRequest(request: MockRequest): MockHttpResponse {
  for (const handler of handlers) {
    const response = handler(request)
    if (response) return response
  }

  return {
    status: 404,
    data: {
      code: 'mock_not_found',
      message: `No mock handler for ${request.method} ${request.path}`,
    },
  }
}
