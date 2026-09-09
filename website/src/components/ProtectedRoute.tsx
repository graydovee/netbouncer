import { useEffect, type ReactNode } from 'react'
import { Box, CircularProgress } from '@mui/material'
import { useAuth } from '../context/AuthContext'
import Login from '../pages/Login'

/** 整页跳转到后端路由（/auth/* 由后端处理，不能走前端路由） */
const RedirectExternal = ({ to }: { to: string }) => {
  useEffect(() => {
    window.location.href = to
  }, [to])
  return null
}

const ProtectedRoute = ({ children }: { children: ReactNode }) => {
  const { loading, authEnabled, authType, isAuthenticated } = useAuth()

  if (loading) {
    return (
      <Box
        sx={{
          display: 'flex',
          justifyContent: 'center',
          alignItems: 'center',
          minHeight: '100vh',
        }}
      >
        <CircularProgress />
      </Box>
    )
  }

  // 认证未启用，直接放行
  if (!authEnabled) {
    return children
  }

  if (!isAuthenticated) {
    // BasicAuth 展示登录页；OIDC 整页跳转到后端登录入口
    if (authType === 'basic') {
      return <Login />
    }
    return <RedirectExternal to="/auth/login" />
  }

  return children
}

export default ProtectedRoute
