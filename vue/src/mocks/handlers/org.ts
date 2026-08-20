import type { MockHandler } from './types'

const mockUsers = [
  {
    id: 'u_1',
    username: 'ops_demo',
    display_name: '王小明',
    department: '运营部',
    team: '运营核心组',
    roles: ['super_admin'],
    status: 'active',
  },
  {
    id: 'u_2',
    username: 'design_lead',
    display_name: '李四',
    department: '设计部',
    team: '设计标准组',
    roles: ['designer'],
    status: 'active',
  },
  {
    id: 'u_3',
    username: 'audit_admin',
    display_name: '张三',
    department: '审核部',
    team: '审核标准组',
    roles: ['dept_admin'],
    status: 'disabled',
  },
]

export const orgHandler: MockHandler = (request) => {
  if (request.method === 'GET' && request.path === '/v1/users') {
    return {
      status: 200,
      data: {
        data: mockUsers,
        pagination: { total: mockUsers.length, page: 1, page_size: 20 },
      },
    }
  }

  if (request.method === 'GET' && request.path.startsWith('/v1/users/designers')) {
    return {
      status: 200,
      data: {
        data: [
          { id: 2, username: 'design_lead', display_name: '李四' },
          { id: 4, username: 'designer_wu', display_name: '吴设计' },
          { id: 5, username: 'designer_li_xiaoyu', display_name: '李晓雨' },
        ],
        pagination: { total: 3, page: 1, page_size: 20 },
      },
    }
  }

  if (request.method === 'GET' && request.path.match(/^\/v1\/users\/[^/]+$/)) {
    const id = request.path.split('/').pop()
    return {
      status: 200,
      data: { data: mockUsers.find((user) => user.id === id) ?? mockUsers[0] },
    }
  }

  if (request.method === 'POST' && request.path.match(/^\/v1\/users\/[^/]+\/(activate|deactivate)$/)) {
    return { status: 204, data: undefined }
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

  return null
}
