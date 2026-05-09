import type { MockHandler } from './types'

const MOCK_ACTOR_ID = 'ops_demo'
const MOCK_DISPLAY_NAME = '演示账号'
const MOCK_TOKEN = 'mock-token-ops-demo'

function buildFrontendAccess() {
  return {
    menus: [
      'dashboard',
      'home',
      'task_list',
      'task_center',
      'task_pool',
      'org_admin',
      'user_admin',
      'org_permission',
      'resource_management',
      'assets_index',
      'report_center',
      'kpi',
      'finance',
      'rules',
      'export_center',
      'audit_log',
      'logs_center',
    ],
    pages: [
      'home',
      'task_list',
      'task_detail',
      'task_create',
      'me',
      'me_security',
      'me_org',
      'me_notifications',
      'me_drafts',
      'assets_index',
      'asset_detail',
      'org_index',
      'org_users',
      'org_departments',
      'org_teams',
      'reports',
      'rules',
      'export_center',
      'audit_log',
      'logs',
    ],
    actions: [
      'task:create',
      'task:assign',
      'task:cancel',
      'task:close',
      'task:audit',
      'task:warehouse',
      'design:work',
      'assets:view',
      'org:manage',
      'kpi:view',
      'finance:view',
      'rules:edit',
      'export:tasks',
      'audit:view',
    ],
    scopes: ['global'],
    modules: ['design', 'audit', 'warehouse', 'customization', 'procurement', 'assets', 'org'],
    roles: ['super_admin'],
    view_all: true,
    is_super_admin: true,
    is_department_admin: true,
    is_group_leader: true,
    department: '演示部门',
    team: '演示团队',
    department_codes: ['DEMO_DEPT'],
    team_codes: ['DEMO_TEAM'],
    managed_departments: ['DEMO_DEPT'],
    managed_teams: ['DEMO_TEAM'],
  }
}

function buildUser() {
  const frontend_access = buildFrontendAccess()
  return {
    id: MOCK_ACTOR_ID,
    username: MOCK_ACTOR_ID,
    display_name: MOCK_DISPLAY_NAME,
    department: '演示部门',
    team: '演示团队',
    roles: ['super_admin'],
    mobile: '13800000000',
    email: 'demo@example.com',
    frontend_access,
  }
}

export const authHandler: MockHandler = (request) => {
  if (request.method === 'POST' && request.path === '/v1/auth/login') {
    const user = buildUser()
    return {
      status: 200,
      data: {
        token: MOCK_TOKEN,
        session: { token: MOCK_TOKEN },
        user,
        frontend_access: user.frontend_access,
      },
    }
  }

  if (request.method === 'GET' && request.path === '/v1/auth/me') {
    return { status: 200, data: buildUser() }
  }

  if (request.method === 'GET' && request.path === '/v1/me') {
    return { status: 200, data: { data: buildUser() } }
  }

  if (request.method === 'PATCH' && request.path === '/v1/me') {
    return {
      status: 200,
      data: {
        data: {
          ...buildUser(),
          nickname: String(request.body?.nickname ?? MOCK_DISPLAY_NAME),
          phone: String(request.body?.phone ?? '13800000000'),
          email: String(request.body?.email ?? 'demo@example.com'),
        },
      },
    }
  }

  if (request.method === 'POST' && request.path === '/v1/me/change-password') {
    return { status: 200, data: { data: { message: 'ok' } } }
  }

  if (request.method === 'GET' && request.path === '/v1/me/org') {
    return {
      status: 200,
      data: {
        data: {
          department: '演示部门',
          teams: ['演示团队'],
          roles: ['super_admin'],
          managed_departments: ['DEMO_DEPT'],
          managed_teams: ['DEMO_TEAM'],
        },
      },
    }
  }

  if (request.method === 'GET' && request.path === '/v1/auth/register-options') {
    return {
      status: 200,
      data: {
        departments: [
          { name: '演示部门', teams: ['演示团队', '备用团队'] },
          { name: '设计部', teams: ['设计一组', '设计二组'] },
          { name: '审核部', teams: ['一审组', '二审组'] },
        ],
        roles: ['member', 'designer', 'auditor', 'warehouse'],
      },
    }
  }

  if (request.method === 'POST' && request.path === '/v1/auth/register') {
    return { status: 201, data: buildUser() }
  }

  if (request.method === 'PUT' && request.path === '/v1/auth/password') {
    return { status: 204, data: null }
  }

  if (request.method === 'GET' && request.path === '/v1/access-rules') {
    return { status: 200, data: { rules: [] } }
  }

  return null
}
