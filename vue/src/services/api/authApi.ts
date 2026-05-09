/**
 * 认证相关 API
 * 对应文档：v0.4 API 使用说明.md § 2. 认证接口
 */

import http from '@/services/http'
import type {
  LoginResponse,
  BackendUser,
  RegisterPayload,
  ChangePasswordPayload,
} from '@/services/apiTypes'

export const authApi = {
  /**
   * 获取注册所需的下拉选项（部门、团队、角色等）
   * GET /v1/auth/register-options
   * 权限：无需登录
   */
  registerOptions: (signal?: AbortSignal) =>
    http.get<Record<string, unknown>>('/v1/auth/register-options', { signal }),

  /**
   * 注册新账号
   * POST /v1/auth/register
   * 权限：无需登录（或仅管理员，视后端配置而定）
   */
  register: (payload: RegisterPayload, signal?: AbortSignal) =>
    http.post<BackendUser>('/v1/auth/register', payload, { signal }),

  /**
   * 账号密码登录
   * POST /v1/auth/login
   * 权限：无需登录
   * @returns token + user + frontend_access
   */
  login: (payload: { username: string; password: string }, signal?: AbortSignal) =>
    http.post<LoginResponse>('/v1/auth/login', payload, { signal }),

  /**
   * 获取当前登录用户信息（含 frontend_access）
   * GET /v1/auth/me
   * 权限：需要有效 token（Bearer）
   * @example
   *   const { data } = await authApi.me()
   *   // data.user.frontend_access 用于菜单/页面/操作显隐控制
   */
  me: (signal?: AbortSignal) =>
    http.get<BackendUser>('/v1/auth/me', { signal }),

  /**
   * 修改密码
   * PUT /v1/auth/password
   * 权限：登录用户本人
   */
  changePassword: (payload: ChangePasswordPayload, signal?: AbortSignal) =>
    http.put<void>('/v1/auth/password', payload, { signal }),

  /**
   * 路由权限目录
   * GET /v1/access-rules
   */
  getAccessRules: (signal?: AbortSignal) =>
    http.get<Record<string, unknown>>('/v1/access-rules', { signal }),
}
