import { useAuth } from '../context/AuthContext';
import { CircularProgress, Box } from '@mui/material';
import Login from '../pages/Login';

const ProtectedRoute = ({ children }) => {
  const { loading, authEnabled, authType, isAuthenticated, login } = useAuth();

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
    );
  }

  // 如果认证未启用，直接渲染子组件
  if (!authEnabled) {
    return children;
  }

  // 如果认证已启用但用户未登录
  if (!isAuthenticated) {
    // BasicAuth显示登录页面
    if (authType === 'basic') {
      return <Login />;
    }
    // OIDC重定向到登录页面
    window.location.href = '/auth/login';
    return null;
  }

  return children;
};

export default ProtectedRoute;
