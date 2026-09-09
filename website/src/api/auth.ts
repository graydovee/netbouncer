import { get, post } from './client'
import type { AuthStatusData, LoginResult, UserInfo } from './types'

export const authApi = {
  /** 查询认证状态（无需登录） */
  getStatus: (): Promise<AuthStatusData> => get<AuthStatusData>('/auth/status'),

  /** BasicAuth 登录 */
  login: (username: string, password: string): Promise<LoginResult> =>
    post<LoginResult>('/auth/login', { username, password }),

  /** 跳转 OIDC 登录（整页跳转到后端，由后端重定向到 IdP） */
  loginOIDC: (): void => {
    window.location.href = '/auth/login'
  },

  /** 登出（整页跳转，由后端清理会话后重定向回首页） */
  logout: (): void => {
    window.location.href = '/auth/logout'
  },
}

export type { UserInfo }
