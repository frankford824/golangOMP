import type { MockHandler } from './types'

const MOCK_ACTOR_ID = 'ops_demo'
const MOCK_DISPLAY_NAME = '演示账号'
const MOCK_TOKEN = 'mock-token-ops-demo'
const MOCK_AVATAR_URL = '/v1/me/avatar-files/avatar-aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa.png'
let mockDisplayName = MOCK_DISPLAY_NAME
let mockMobile = '13800000000'
let mockEmail = 'demo@example.com'
let mockAvatarUrl = ''

function buildFrontendAccess() {
  return {
    menus: ['dashboard', 'task_list', 'resource_management', 'cost_rules', 'report_center', 'user_admin'],
    pages: [
      'dashboard',
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
      'resource_management',
      'cost_rules',
      'data_center',
      'user_admin',
    ],
    actions: [
      'task.create',
      'task.view',
      'task.assign',
      'task.upload_source',
      'task.audit',
      'task.reopen',
      'planning_sku.view',
      'planning_sku.create',
      'planning_sku.edit',
      'planning_sku.export',
      'asset.view',
      'asset.download',
      'asset.export',
      'asset.publish',
      'asset.manage',
      'catalog.view',
      'catalog.manage',
      'access.view',
      'access.manage',
      'report.view',
    ],
    scopes: ['global'],
    modules: ['design', 'audit', 'customization', 'assets', 'org'],
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
    account: MOCK_ACTOR_ID,
    username: MOCK_ACTOR_ID,
    display_name: mockDisplayName,
    name: mockDisplayName,
    department: '演示部门',
    team: '演示团队',
    roles: ['super_admin'],
    mobile: mockMobile,
    phone: mockMobile,
    email: mockEmail,
    avatar: mockAvatarUrl,
    avatar_url: mockAvatarUrl,
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
    if (request.body && ('avatar' in request.body || 'avatar_url' in request.body)) {
      return {
        status: 400,
        data: {
          error: {
            code: 'INVALID_REQUEST',
            message: '头像请通过头像上传或移除操作更新',
            deny_code: 'avatar_update_requires_avatar_api',
          },
        },
      }
    }
    mockDisplayName = String(request.body?.display_name ?? mockDisplayName)
    mockMobile = String(request.body?.mobile ?? mockMobile)
    mockEmail = String(request.body?.email ?? mockEmail)
    return {
      status: 200,
      data: {
        data: buildUser(),
      },
    }
  }

  if (request.method === 'POST' && request.path === '/v1/me/change-password') {
    return { status: 200, data: { data: { message: 'ok' } } }
  }

  if (request.method === 'POST' && request.path === '/v1/me/avatar') {
    mockAvatarUrl = MOCK_AVATAR_URL
    return { status: 200, data: { data: buildUser() } }
  }

  if (request.method === 'DELETE' && request.path === '/v1/me/avatar') {
    mockAvatarUrl = ''
    return { status: 200, data: { data: buildUser() } }
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
        roles: ['member'],
      },
    }
  }

  if (request.method === 'POST' && request.path === '/v1/auth/register') {
    return { status: 201, data: buildUser() }
  }

  if (request.method === 'PUT' && request.path === '/v1/auth/password') {
    return { status: 204, data: null }
  }

  return null
}
