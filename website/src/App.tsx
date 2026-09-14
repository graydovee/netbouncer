import { BrowserRouter as Router, Route, Routes } from 'react-router-dom'
import { ColorModeProvider } from './theme'
import Layout from './components/Layout'
import ProtectedRoute from './components/ProtectedRoute'
import { AuthProvider } from './context/AuthContext'
import TrafficMonitor from './pages/TrafficMonitor'
import IPManagement from './pages/ip/IPManagement'
import GroupManagement from './pages/GroupManagement'
import PolicyManagement from './pages/policy/PolicyManagement'
import NotFound from './pages/NotFound'

function App() {
  return (
    <ColorModeProvider>
      <AuthProvider>
        <Router>
          <ProtectedRoute>
            <Layout>
              <Routes>
                <Route path="/" element={<TrafficMonitor />} />
                <Route path="/ip-management" element={<IPManagement />} />
                <Route path="/policies" element={<PolicyManagement />} />
                <Route path="/groups" element={<GroupManagement />} />
                <Route path="*" element={<NotFound />} />
              </Routes>
            </Layout>
          </ProtectedRoute>
        </Router>
      </AuthProvider>
    </ColorModeProvider>
  )
}

export default App
