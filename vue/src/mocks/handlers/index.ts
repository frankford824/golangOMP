import { assetsHandler } from './assets'
import { authHandler } from './auth'
import { batchSkuHandler } from './batchSku'
import { designSourcesHandler } from './designSources'
import { erpBridgeHandler } from './erpBridge'
import { notificationsHandler } from './notifications'
import { orgHandler } from './org'
import { costManagementHandler } from './costManagement'
import { searchHandler } from './search'
import { taskDraftsHandler } from './taskDrafts'
import { taskModulesHandler } from './taskModules'
import { tasksHandler } from './tasks'
import { v8Handler } from './v8'
import type { MockHandler, MockHttpResponse, MockRequest } from './types'

const handlers: MockHandler[] = [
  authHandler,
  v8Handler,
  tasksHandler,
  taskModulesHandler,
  taskDraftsHandler,
  erpBridgeHandler,
  designSourcesHandler,
  notificationsHandler,
  orgHandler,
  costManagementHandler,
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
