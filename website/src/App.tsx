import { lazy, Suspense } from 'react'
import { Box, CircularProgress } from '@mui/material'
import { BrowserRouter as Router, Route, Routes } from 'react-router-dom'
import { ColorModeProvider } from './theme'
import Layout from './components/Layout'
import ProtectedRoute from './components/ProtectedRoute'
import { AuthProvider } from './context/AuthContext'
const TrafficMonitor = lazy(() => import('./pages/TrafficMonitor'))
const IPManagement = lazy(() => import('./pages/ip/IPManagement'))
const GroupManagement = lazy(() => import('./pages/GroupManagement'))
const PolicyManagement = lazy(() => import('./pages/policy/PolicyManagement'))
import NotFound from './pages/NotFound'

function App() {
  return (
    <ColorModeProvider>
      <AuthProvider>
        <Router>
          <ProtectedRoute>
            <Layout>
              <Suspense fallback={<Box role="status" aria-label="页面加载中" sx={{ p: 4 }}><CircularProgress size={28} /></Box>}>
              <Routes>
                <Route path="/" element={<TrafficMonitor />} />
                <Route path="/ip-management" element={<IPManagement />} />
                <Route path="/policies" element={<PolicyManagement />} />
                <Route path="/groups" element={<GroupManagement />} />
                <Route path="*" element={<NotFound />} />
              </Routes>
              </Suspense>
            </Layout>
          </ProtectedRoute>
        </Router>
      </AuthProvider>
    </ColorModeProvider>
  )
}

export default App
