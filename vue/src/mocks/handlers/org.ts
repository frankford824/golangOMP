import type { MockHandler } from './types'

export const orgHandler: MockHandler = (request) => {
  if (request.method === 'GET' && request.path === '/v1/users') {
    return {
      status: 200,
      data: {
        items: [
          { id: 'u_1', name: '王小明', department: '运营部', team: 'ops_core', role: 'member', is_active: true },
          { id: 'u_2', name: '李四', department: '设计部', team: 'design_standard', role: 'team_lead', is_active: true },
          { id: 'u_3', name: '张三', department: '审核部', team: 'audit_standard', role: 'dept_admin', is_active: true },
        ],
      },
    }
  }

  if (request.method === 'POST' && request.path.match(/^\/v1\/users\/[^/]+\/(activate|deactivate)$/)) {
    return { status: 204, data: undefined }
  }

  if (request.method === 'POST' && request.path.match(/^\/v1\/users\/[^/]+\/roles$/)) {
    return { status: 200, data: { success: true } }
  }

  if (request.method === 'GET' && request.path === '/v1/departments') {
    return {
      status: 200,
      data: {
        items: [
          { id: 'd_ops', name: '运营部' },
          { id: 'd_design', name: '设计部' },
          { id: 'd_audit', name: '审核部' },
        ],
      },
    }
  }

  if (request.method === 'GET' && request.path === '/v1/org/options') {
    return {
      status: 200,
      data: {
        data: {
          departments: [
            {
              id: 'd_ops',
              name: '运营部',
              teams: [{ id: 't_ops', name: '运营核心组' }],
            },
            {
              id: 'd_design',
              name: '设计部',
              teams: [{ id: 't_design', name: '设计标准组' }],
            },
          ],
        },
      },
    }
  }

  if (request.method === 'GET' && request.path === '/v1/teams') {
    return {
      status: 200,
      data: {
        items: [
          { id: 't_ops', code: 'ops_core', name: '运营核心组' },
          { id: 't_design', code: 'design_standard', name: '设计标准组' },
          { id: 't_audit', code: 'audit_standard', name: '审核标准组' },
        ],
      },
    }
  }

  if (request.method === 'GET' && request.path === '/v1/org-move-requests') {
    return {
      status: 200,
      data: {
        items: [
          {
            id: 'mr_1',
            user_name: '王小明',
            from_department: '运营部',
            to_department: '设计部',
            status: 'pending_super_admin_confirm',
          },
        ],
      },
    }
  }

  if (request.method === 'POST' && request.path.match(/^\/v1\/departments\/[^/]+\/org-move-requests$/)) {
    return { status: 201, data: { data: { id: `mr_${Date.now()}`, ...request.body, status: 'pending' } } }
  }

  if (request.method === 'POST' && request.path.match(/^\/v1\/org-move-requests\/[^/]+\/(approve|reject)$/)) {
    return { status: 200, data: { success: true } }
  }

  return null
}
