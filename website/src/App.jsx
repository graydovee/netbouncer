import { BrowserRouter as Router, Routes, Route } from 'react-router-dom';
import { ThemeProvider, createTheme } from '@mui/material/styles';
import CssBaseline from '@mui/material/CssBaseline';
import Layout from './components/Layout';
import TrafficMonitor from './pages/TrafficMonitor';
import IPManagement from './pages/IPManagement';
import GroupManagement from './pages/GroupManagement';
import NotFound from './pages/NotFound';

const theme = createTheme({
  palette: {
    mode: 'light',
    primary: {
      main: '#2c3e50',
    },
    secondary: {
      main: '#3498db',
    },
    error: {
      main: '#e74c3c',
    },
    success: {
      main: '#27ae60',
    },
  },
  typography: {
    fontFamily: '"Segoe UI", Arial, sans-serif',
  },
});

function App() {
  return (
    <ThemeProvider theme={theme}>
      <CssBaseline />
      <Router>
        <Layout>
          <Routes>
            <Route path="/" element={<TrafficMonitor />} />
            <Route path="/ip-management" element={<IPManagement />} />
            <Route path="/groups" element={<GroupManagement />} />
            <Route path="*" element={<NotFound />} />
          </Routes>
        </Layout>
      </Router>
    </ThemeProvider>
  );
}

export default App;
