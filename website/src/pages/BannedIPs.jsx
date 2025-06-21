import { useState, useEffect } from 'react';
import {
  Box,
  Typography,
  Paper,
  Table,
  TableBody,
  TableCell,
  TableContainer,
  TableHead,
  TableRow,
  Button,
  Alert,
  CircularProgress,
  Tooltip,
  TextField,
  Grid,
} from '@mui/material';
import {
  Refresh as RefreshIcon,
  Block as BlockIcon,
  Add as AddIcon,
} from '@mui/icons-material';
import { useMessageSnackbar, MessageSnackbar } from '../components/MessageSnackbar';

// 格式化时间戳
const formatTimestamp = (timestamp) => {
  if (!timestamp) return '-';
  const date = new Date(timestamp);
  return date.toLocaleString('zh-CN', {
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit'
  });
};

function BannedIPs() {
  const [bannedIPs, setBannedIPs] = useState([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState(null);
  const [newIP, setNewIP] = useState('');
  const [banLoading, setBanLoading] = useState(false);
  
  // 使用消息提示Hook
  const { snackbar, showMessage, hideMessage } = useMessageSnackbar();

  // 获取禁用IP列表
  const fetchBannedIPs = async () => {
    setLoading(true);
    setError(null);
    try {
      const response = await fetch('/api/banned');
      const result = await response.json();
      if (result.code === 200) {
        // 新的响应格式是BannedIpNet数组
        const bannedIpNets = result.data || [];
        setBannedIPs(bannedIpNets);
      } else {
        setError('获取数据失败: ' + result.message);
      }
    } catch (error) {
      setError('网络请求失败');
      console.error('获取禁用IP列表失败:', error);
    } finally {
      setLoading(false);
    }
  };

  // 验证IP或CIDR格式
  const validateIPOrCIDR = (input) => {
    // 简单的IP或CIDR验证
    const ipRegex = /^(\d{1,3}\.){3}\d{1,3}$/;
    const cidrRegex = /^(\d{1,3}\.){3}\d{1,3}\/\d{1,2}$/;
    
    if (ipRegex.test(input)) {
      // 验证IP地址的每个段是否在0-255范围内
      const parts = input.split('.');
      return parts.every(part => parseInt(part) >= 0 && parseInt(part) <= 255);
    } else if (cidrRegex.test(input)) {
      // 验证CIDR格式
      const [ip, prefix] = input.split('/');
      const prefixNum = parseInt(prefix);
      if (prefixNum < 0 || prefixNum > 32) return false;
      
      const parts = ip.split('.');
      return parts.every(part => parseInt(part) >= 0 && parseInt(part) <= 255);
    }
    return false;
  };

  // 手动禁用IP或CIDR
  const banIPOrCIDR = async () => {
    if (!newIP.trim()) {
      showMessage('请输入IP地址或CIDR', 'warning');
      return;
    }

    if (!validateIPOrCIDR(newIP.trim())) {
      showMessage('请输入有效的IP地址或CIDR格式（例如：192.168.1.1 或 192.168.1.0/24）', 'error');
      return;
    }

    setBanLoading(true);
    try {
      const response = await fetch('/api/ban', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ ip_net: newIP.trim() })
      });
      const result = await response.json();
      if (result.code === 200) {
        // 禁用成功后重新获取完整的列表，确保包含时间信息
        await fetchBannedIPs();
        setNewIP(''); // 清空输入框
        showMessage(`成功禁用 ${newIP.trim()}`);
      } else {
        showMessage('禁用失败: ' + result.message, 'error');
      }
    } catch (error) {
      showMessage('禁用失败: 网络错误', 'error');
    } finally {
      setBanLoading(false);
    }
  };

  // 解禁IP
  const unbanIP = async (ip) => {
    try {
      const response = await fetch('/api/unban', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ ip_net: ip })
      });
      const result = await response.json();
      if (result.code === 200) {
        // 从列表中移除该IP
        setBannedIPs(prev => prev.filter(bannedIP => bannedIP.ip_net !== ip));
        showMessage(`成功解禁 ${ip}`);
      } else {
        showMessage('解禁失败: ' + result.message, 'error');
      }
    } catch (error) {
      showMessage('解禁失败: 网络错误', 'error');
    }
  };

  // 处理回车键
  const handleKeyDown = (event) => {
    if (event.key === 'Enter') {
      banIPOrCIDR();
    }
  };

  // 初始化
  useEffect(() => {
    fetchBannedIPs();
  }, []);

  return (
    <Box>
      <Typography variant="h4" gutterBottom>
        已禁用IP管理
      </Typography>

      {error && (
        <Alert severity="error" sx={{ mb: 2 }}>
          {error}
        </Alert>
      )}

      {/* 手动禁用操作栏 */}
      <Paper sx={{ p: 2, mb: 2 }}>
        <Typography variant="h6" gutterBottom>
          手动禁用IP或CIDR
        </Typography>
        <Grid container spacing={2} alignItems="center">
          <Grid item xs={12} sm={6} md={4}>
            <TextField
              fullWidth
              label="IP地址或CIDR"
              placeholder="例如10.0.0.1或10.0.0.0/8"
              value={newIP}
              onChange={(e) => setNewIP(e.target.value)}
              onKeyDown={handleKeyDown}
              size="small"
              disabled={banLoading}
            />
          </Grid>
          <Grid item>
            <Button
              variant="contained"
              onClick={banIPOrCIDR}
              startIcon={<AddIcon />}
              disabled={banLoading || !newIP.trim()}
              color="error"
            >
              {banLoading ? '禁用中...' : '禁用'}
            </Button>
          </Grid>
        </Grid>
        <Typography variant="body2" color="text.secondary" sx={{ mt: 1 }}>
          支持单个IP地址（如：192.168.1.1）或CIDR网段（如：192.168.1.0/24）
        </Typography>
      </Paper>

      {/* 操作栏 */}
      <Paper sx={{ p: 2, mb: 2 }}>
        <Box sx={{ display: 'flex', alignItems: 'center', gap: 2 }}>
          <Button
            variant="outlined"
            onClick={fetchBannedIPs}
            startIcon={<RefreshIcon />}
            disabled={loading}
          >
            刷新列表
          </Button>
          <Typography variant="body2" color="text.secondary">
            共 {bannedIPs.length} 个已禁用IP
          </Typography>
        </Box>
      </Paper>

      {/* IP列表表格 */}
      <Paper>
        <TableContainer>
          <Table>
            <TableHead>
              <TableRow>
                <TableCell sx={{ fontWeight: 'bold' }}>IP地址或CIDR</TableCell>
                <TableCell sx={{ fontWeight: 'bold' }}>禁用时间</TableCell>
                <TableCell sx={{ fontWeight: 'bold' }}>更新时间</TableCell>
                <TableCell sx={{ fontWeight: 'bold' }}>操作</TableCell>
              </TableRow>
            </TableHead>
            <TableBody>
              {loading ? (
                <TableRow>
                  <TableCell colSpan={4} align="center">
                    <CircularProgress size={24} />
                    <Typography sx={{ ml: 1 }}>加载中...</Typography>
                  </TableCell>
                </TableRow>
              ) : bannedIPs.length === 0 ? (
                <TableRow>
                  <TableCell colSpan={4} align="center">
                    暂无已禁用IP
                  </TableCell>
                </TableRow>
              ) : (
                bannedIPs.map((bannedIP, index) => (
                  <TableRow key={index} hover>
                    <TableCell sx={{ fontFamily: 'monospace', fontSize: '1rem' }}>
                      {bannedIP.ip_net}
                    </TableCell>
                    <TableCell sx={{ color: 'text.secondary', fontSize: '0.875rem' }}>
                      {formatTimestamp(bannedIP.created_at)}
                    </TableCell>
                    <TableCell sx={{ color: 'text.secondary', fontSize: '0.875rem' }}>
                      {formatTimestamp(bannedIP.updated_at)}
                    </TableCell>
                    <TableCell>
                      <Tooltip title="解禁此IP或CIDR">
                        <Button
                          variant="outlined"
                          size="small"
                          color="primary"
                          startIcon={<BlockIcon />}
                          onClick={() => unbanIP(bannedIP.ip_net)}
                        >
                          解禁
                        </Button>
                      </Tooltip>
                    </TableCell>
                  </TableRow>
                ))
              )}
            </TableBody>
          </Table>
        </TableContainer>
      </Paper>
      <MessageSnackbar snackbar={snackbar} onClose={hideMessage} />
    </Box>
  );
}

export default BannedIPs; 