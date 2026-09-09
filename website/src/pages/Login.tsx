import { useState, type FormEvent } from 'react'
import {
  Alert,
  Box,
  Button,
  Card,
  CardContent,
  CircularProgress,
  TextField,
  Typography,
} from '@mui/material'
import { useAuth } from '../context/AuthContext'

const Login = () => {
  const { basicLogin, authType } = useAuth()
  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState('')

  const isOIDC = authType === 'oidc'

  const handleSubmit = async (event: FormEvent) => {
    event.preventDefault()
    setLoading(true)
    setError('')

    const result = await basicLogin(username, password)
    if (!result.success) {
      setError(result.message ?? '登录失败')
    }
    setLoading(false)
  }

  return (
    <Box
      sx={{
        display: 'flex',
        flexDirection: 'column',
        justifyContent: 'center',
        alignItems: 'center',
        minHeight: '100vh',
        backgroundColor: '#f5f5f5',
      }}
    >
      <Card sx={{ maxWidth: 400, width: '100%', mx: 2 }}>
        <CardContent sx={{ p: 4 }}>
          <Typography variant="h5" component="h1" gutterBottom align="center">
            NetBouncer 登录
          </Typography>

          {error && (
            <Alert severity="error" sx={{ mb: 2 }}>
              {error}
            </Alert>
          )}

          {isOIDC ? (
            <Button
              variant="contained"
              color="primary"
              fullWidth
              sx={{ mt: 2 }}
              onClick={() => {
                window.location.href = '/auth/login'
              }}
            >
              通过 OIDC 登录
            </Button>
          ) : (
            <form onSubmit={(event) => void handleSubmit(event)}>
              <TextField
                label="用户名"
                variant="outlined"
                fullWidth
                margin="normal"
                value={username}
                onChange={(event) => setUsername(event.target.value)}
                required
                autoFocus
              />
              <TextField
                label="密码"
                type="password"
                variant="outlined"
                fullWidth
                margin="normal"
                value={password}
                onChange={(event) => setPassword(event.target.value)}
                required
              />
              <Button
                type="submit"
                variant="contained"
                color="primary"
                fullWidth
                sx={{ mt: 3 }}
                disabled={loading}
              >
                {loading ? <CircularProgress size={24} /> : '登录'}
              </Button>
            </form>
          )}
        </CardContent>
      </Card>
    </Box>
  )
}

export default Login
