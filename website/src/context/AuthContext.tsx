import { createContext, useCallback, useContext, useEffect, useState, type ReactNode } from 'react'
import { authApi } from '../api/auth'
import { ApiError, setUnauthorizedHandler } from '../api/client'
import type { UserInfo } from '../api/types'

export type AuthType = 'basic' | 'oidc'

interface AuthContextValue {
  user: UserInfo | null
  loading: boolean
  authEnabled: boolean
  authType: AuthType | null
  isAuthenticated: boolean
  login: () => void
  basicLogin: (username: string, password: string) => Promise<{ success: boolean; message?: string }>
  logout: () => void
  checkAuthStatus: () => Promise<void>
}

const AuthContext = createContext<AuthContextValue | null>(null)

// eslint-disable-next-line react-refresh/only-export-components
export const useAuth = (): AuthContextValue => {
  const context = useContext(AuthContext)
  if (!context) {
    throw new Error('useAuth must be used within an AuthProvider')
  }
  return context
}

export const AuthProvider = ({ children }: { children: ReactNode }) => {
  const [user, setUser] = useState<UserInfo | null>(null)
  const [loading, setLoading] = useState(true)
  const [authEnabled, setAuthEnabled] = useState(false)
  const [authType, setAuthType] = useState<AuthType | null>(null)

  const checkAuthStatus = useCallback(async () => {
    try {
      const data = await authApi.getStatus()
      setAuthEnabled(data.enabled)
      setAuthType(data.type ?? null)
      setUser(data.enabled && data.loggedIn ? (data.user ?? null) : null)
    } catch (error) {
      // 状态查询失败时保留当前状态，避免误踢用户
      console.error('检查认证状态失败:', error)
    } finally {
      setLoading(false)
    }
  }, [])

  // 任意接口返回 401 时立即重新检查认证状态，驱动界面回到登录页
  useEffect(() => {
    setUnauthorizedHandler(() => {
      void checkAuthStatus()
    })
    return () => setUnauthorizedHandler(null)
  }, [checkAuthStatus])

  useEffect(() => {
    void checkAuthStatus()
    const interval = window.setInterval(() => {
      void checkAuthStatus()
    }, 60_000)
    return () => window.clearInterval(interval)
  }, [checkAuthStatus])

  const login = () => {
    authApi.loginOIDC()
  }

  const basicLogin = async (
    username: string,
    password: string,
  ): Promise<{ success: boolean; message?: string }> => {
    try {
      const data = await authApi.login(username, password)
      setUser(data.user)
      setAuthEnabled(true)
      setAuthType('basic')
      return { success: true }
    } catch (error) {
      return { success: false, message: error instanceof ApiError ? error.message : '网络错误' }
    }
  }

  const logout = () => {
    authApi.logout()
  }

  const value: AuthContextValue = {
    user,
    loading,
    authEnabled,
    authType,
    isAuthenticated: !!user,
    login,
    basicLogin,
    logout,
    checkAuthStatus,
  }

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>
}
