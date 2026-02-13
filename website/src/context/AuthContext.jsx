import { createContext, useContext, useState, useEffect, useCallback } from 'react';

const AuthContext = createContext(null);

export const useAuth = () => {
  const context = useContext(AuthContext);
  if (!context) {
    throw new Error('useAuth must be used within an AuthProvider');
  }
  return context;
};

export const AuthProvider = ({ children }) => {
  const [user, setUser] = useState(null);
  const [loading, setLoading] = useState(true);
  const [authEnabled, setAuthEnabled] = useState(false);
  const [authType, setAuthType] = useState(null);

  const checkAuthStatus = useCallback(async () => {
    try {
      const response = await fetch('/auth/status');
      const result = await response.json();
      
      if (result.code === 200) {
        setAuthEnabled(result.data.enabled || false);
        setAuthType(result.data.type || null);
        if (result.data.loggedIn) {
          setUser(result.data.user);
        } else {
          setUser(null);
        }
      }
    } catch (error) {
      console.error('Failed to check auth status:', error);
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    checkAuthStatus();
    // 定期检查认证状态
    const interval = setInterval(checkAuthStatus, 60000); // 每分钟检查一次
    return () => clearInterval(interval);
  }, [checkAuthStatus]);

  // OIDC登录 - 重定向到OIDC提供者
  const login = () => {
    window.location.href = '/auth/login';
  };

  // BasicAuth登录 - 发送用户名密码
  const basicLogin = async (username, password) => {
    try {
      const response = await fetch('/auth/login', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({ username, password }),
      });
      
      const result = await response.json();
      
      if (result.code === 200) {
        setUser(result.data.user);
        return { success: true };
      } else {
        return { success: false, message: result.message || '登录失败' };
      }
    } catch (error) {
      console.error('Login failed:', error);
      return { success: false, message: '网络错误' };
    }
  };

  const logout = () => {
    // 使用 window.location.href 直接跳转到登出端点
    // 这会避免 CORS 问题，因为浏览器会正确处理重定向
    window.location.href = '/auth/logout';
  };

  const value = {
    user,
    loading,
    authEnabled,
    authType,
    isAuthenticated: !!user,
    login,
    basicLogin,
    logout,
    checkAuthStatus,
  };

  return (
    <AuthContext.Provider value={value}>
      {children}
    </AuthContext.Provider>
  );
};

export default AuthContext;
