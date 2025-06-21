import { useState, useEffect, useCallback } from 'react';
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
  TextField,
  Chip,
  IconButton,
  Tooltip,
  Alert,
  CircularProgress,
} from '@mui/material';
import {
  Refresh as RefreshIcon,
  PlayArrow as PlayIcon,
  Stop as StopIcon,
  ArrowUpward as ArrowUpIcon,
  ArrowDownward as ArrowDownIcon,
} from '@mui/icons-material';
import { useMessageSnackbar, MessageSnackbar } from '../components/MessageSnackbar';

// 工具函数
const formatBytes = (bytes) => {
  if (bytes === 0) return '0 B';
  if (bytes < 0) return '0 B';
  
  const units = ['B', 'KB', 'MB', 'GB', 'TB', 'PB'];
  const i = Math.floor(Math.log(bytes) / Math.log(1024));
  
  // 确保i在有效范围内
  if (i < 0 || i >= units.length) {
    return bytes + ' B';
  }
  
  const value = bytes / Math.pow(1024, i);
  return value.toFixed(2) + ' ' + units[i];
};

const formatBytesPerSec = (bytesPerSec) => {
  if (bytesPerSec === 0) return '0 bps';
  if (bytesPerSec < 0) return '0 bps';
  
  const bps = bytesPerSec * 8;
  
  if (bps < 1000) {
    return bps.toFixed(1) + ' bps';
  } else if (bps < 1000000) {
    return (bps / 1000).toFixed(2) + ' Kbps';
  } else if (bps < 1000000000) {
    return (bps / 1000000).toFixed(2) + ' Mbps';
  } else {
    return (bps / 1000000000).toFixed(2) + ' Gbps';
  }
};

const formatTimestamp = (timestamp) => {
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

const columns = [
  { key: 'remote_ip', label: '远程IP', sortable: true },
  { key: 'local_ip', label: '本地IP', sortable: true },
  { key: 'total_bytes_in', label: '总接收流量', sortable: true, formatter: formatBytes },
  { key: 'total_bytes_out', label: '总发送流量', sortable: true, formatter: formatBytes },
  { key: 'total_packets_in', label: '接收包数', sortable: true, formatter: (val) => val.toLocaleString() },
  { key: 'total_packets_out', label: '发送包数', sortable: true, formatter: (val) => val.toLocaleString() },
  { key: 'bytes_in_per_sec', label: '接收速率', sortable: true, formatter: formatBytesPerSec },
  { key: 'bytes_out_per_sec', label: '发送速率', sortable: true, formatter: formatBytesPerSec },
  { key: 'connections', label: '连接数', sortable: true },
  { key: 'first_seen', label: '首次发现', sortable: true, formatter: formatTimestamp },
  { key: 'last_seen', label: '最后活动', sortable: true, formatter: formatTimestamp },
  { key: 'actions', label: '操作', sortable: false },
];

function TrafficMonitor() {
  const [trafficData, setTrafficData] = useState([]);
  const [loading, setLoading] = useState(false);
  const [initialLoading, setInitialLoading] = useState(true);
  const [error, setError] = useState(null);
  const [sortKey, setSortKey] = useState('bytes_out_per_sec');
  const [sortAsc, setSortAsc] = useState(false);
  const [refreshInterval, setRefreshInterval] = useState(30);
  const [isRefreshing, setIsRefreshing] = useState(false);
  const [refreshTimer, setRefreshTimer] = useState(null);
  const [lastUpdate, setLastUpdate] = useState(null);
  
  // 使用消息提示Hook
  const { snackbar, showMessage, hideMessage } = useMessageSnackbar();

  // 获取流量数据
  const fetchTrafficData = useCallback(async (showLoading = false) => {
    if (showLoading) {
      setLoading(true);
    }
    setError(null);
    try {
      const response = await fetch('/api/traffic');
      const result = await response.json();
      if (result.code === 200) {
        setTrafficData(result.data);
        setLastUpdate(new Date());
      } else {
        setError('获取数据失败: ' + result.message);
      }
    } catch (error) {
      setError('网络请求失败');
      console.error('获取流量数据失败:', error);
    } finally {
      setLoading(false);
      setInitialLoading(false);
    }
  }, []);

  // 禁用IP
  const banIP = async (ip) => {
    try {
      const response = await fetch('/api/ip', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ 
          ip_net: ip,
          group_id: 0, // 使用默认组
          action: 'ban' // 明确指定行为为ban
        })
      });
      const result = await response.json();
      if (result.code === 200) {
        // 更新本地数据，将对应IP标记为已禁用
        setTrafficData(prev => prev.map(item => 
          item.remote_ip === ip ? { ...item, is_banned: true } : item
        ));
        showMessage(`成功禁用 ${ip}`);
      } else {
        showMessage('禁用失败: ' + result.message, 'error');
      }
    } catch (error) {
      showMessage('禁用失败: 网络错误', 'error');
    }
  };

  // 排序数据
  const sortedData = [...trafficData].sort((a, b) => {
    let v1 = a[sortKey], v2 = b[sortKey];
    if (typeof v1 === 'string') {
      return sortAsc ? v1.localeCompare(v2) : v2.localeCompare(v1);
    } else {
      return sortAsc ? v1 - v2 : v2 - v1;
    }
  });

  // 处理排序
  const handleSort = (key) => {
    if (sortKey === key) {
      setSortAsc(!sortAsc);
    } else {
      setSortKey(key);
      setSortAsc(false);
    }
  };

  // 开始自动刷新
  const startRefresh = useCallback(() => {
    if (refreshTimer) {
      clearInterval(refreshTimer);
    }
    const timer = setInterval(() => {
      fetchTrafficData(false); // 自动刷新时不显示loading状态
    }, refreshInterval * 1000);
    setRefreshTimer(timer);
    setIsRefreshing(true);
  }, [refreshInterval, fetchTrafficData]);

  // 停止自动刷新
  const stopRefresh = useCallback(() => {
    if (refreshTimer) {
      clearInterval(refreshTimer);
      setRefreshTimer(null);
    }
    setIsRefreshing(false);
  }, [refreshTimer]);

  // 手动刷新
  const handleManualRefresh = async () => {
    await fetchTrafficData(false); // 手动刷新时不显示loading状态
  };

  // 切换刷新状态
  const toggleRefresh = () => {
    if (isRefreshing) {
      stopRefresh();
    } else {
      startRefresh();
    }
  };

  // 初始化
  useEffect(() => {
    fetchTrafficData(true); // 首次加载时显示loading状态
    startRefresh();
    
    return () => {
      if (refreshTimer) {
        clearInterval(refreshTimer);
      }
    };
  }, []);

  // 刷新间隔变化时重新启动
  useEffect(() => {
    if (isRefreshing) {
      startRefresh();
    }
  }, [refreshInterval, startRefresh]);

  return (
    <Box>
      <Typography variant="h4" gutterBottom>
        网络流量监控
      </Typography>

      {error && (
        <Alert severity="error" sx={{ mb: 2 }}>
          {error}
        </Alert>
      )}

      {/* 刷新控制 */}
      <Paper sx={{ p: 2, mb: 2 }}>
        <Box sx={{ display: 'flex', alignItems: 'center', gap: 2, flexWrap: 'wrap' }}>
          <TextField
            label="刷新间隔 (秒)"
            type="number"
            value={refreshInterval}
            onChange={(e) => setRefreshInterval(Math.max(1, parseInt(e.target.value) || 30))}
            size="small"
            sx={{ width: 150 }}
          />
          <Button
            variant="contained"
            onClick={toggleRefresh}
            startIcon={isRefreshing ? <StopIcon /> : <PlayIcon />}
            color={isRefreshing ? 'error' : 'success'}
          >
            {isRefreshing ? '停止刷新' : '开始刷新'}
          </Button>
          <IconButton
            onClick={handleManualRefresh}
            disabled={loading}
            color="primary"
            size="small"
            sx={{ 
              border: '1px solid',
              borderColor: 'primary.main',
              '&:hover': {
                backgroundColor: 'primary.main',
                color: 'white'
              }
            }}
          >
            <RefreshIcon />
          </IconButton>
          <Chip
            label={`状态: ${isRefreshing ? '自动刷新中' : '已停止'}`}
            color={isRefreshing ? 'success' : 'default'}
            size="small"
          />
          {lastUpdate && (
            <Typography variant="body2" color="text.secondary">
              最后更新: {lastUpdate.toLocaleString('zh-CN')}
            </Typography>
          )}
        </Box>
      </Paper>

      {/* 数据表格 */}
      <Paper>
        <TableContainer>
          <Table>
            <TableHead>
              <TableRow>
                {columns.map((column) => (
                  <TableCell
                    key={column.key}
                    sx={{
                      fontWeight: 'bold',
                      cursor: column.sortable ? 'pointer' : 'default',
                      '&:hover': column.sortable ? { backgroundColor: 'action.hover' } : {},
                    }}
                    onClick={() => column.sortable && handleSort(column.key)}
                  >
                    <Box sx={{ display: 'flex', alignItems: 'center' }}>
                      {column.label}
                      {column.sortable && sortKey === column.key && (
                        <Box component="span" sx={{ ml: 0.5 }}>
                          {sortAsc ? <ArrowUpIcon fontSize="small" /> : <ArrowDownIcon fontSize="small" />}
                        </Box>
                      )}
                    </Box>
                  </TableCell>
                ))}
              </TableRow>
            </TableHead>
            <TableBody>
              {initialLoading ? (
                <TableRow>
                  <TableCell colSpan={columns.length} align="center">
                    <CircularProgress size={24} />
                    <Typography sx={{ ml: 1 }}>加载中...</Typography>
                  </TableCell>
                </TableRow>
              ) : sortedData.length === 0 ? (
                <TableRow>
                  <TableCell colSpan={columns.length} align="center">
                    暂无数据
                  </TableCell>
                </TableRow>
              ) : (
                sortedData.map((row, index) => (
                  <TableRow key={index} hover>
                    <TableCell sx={{ fontFamily: 'monospace' }}>{row.remote_ip}</TableCell>
                    <TableCell sx={{ fontFamily: 'monospace' }}>{row.local_ip}</TableCell>
                    <TableCell sx={{ fontFamily: 'monospace' }}>
                      {formatBytes(row.total_bytes_in)}
                    </TableCell>
                    <TableCell sx={{ fontFamily: 'monospace' }}>
                      {formatBytes(row.total_bytes_out)}
                    </TableCell>
                    <TableCell sx={{ fontFamily: 'monospace' }}>
                      {row.total_packets_in.toLocaleString()}
                    </TableCell>
                    <TableCell sx={{ fontFamily: 'monospace' }}>
                      {row.total_packets_out.toLocaleString()}
                    </TableCell>
                    <TableCell sx={{ fontFamily: 'monospace' }}>
                      {formatBytesPerSec(row.bytes_in_per_sec)}
                    </TableCell>
                    <TableCell sx={{ fontFamily: 'monospace' }}>
                      {formatBytesPerSec(row.bytes_out_per_sec)}
                    </TableCell>
                    <TableCell sx={{ fontFamily: 'monospace' }}>
                      {row.connections}
                    </TableCell>
                    <TableCell sx={{ color: 'text.secondary', fontSize: '0.875rem' }}>
                      {formatTimestamp(row.first_seen)}
                    </TableCell>
                    <TableCell sx={{ color: 'text.secondary', fontSize: '0.875rem' }}>
                      {formatTimestamp(row.last_seen)}
                    </TableCell>
                    <TableCell>
                      <Tooltip title={row.is_banned ? '已禁用' : '禁用此IP'}>
                        <Button
                          variant="outlined"
                          size="small"
                          color="error"
                          disabled={row.is_banned}
                          onClick={() => banIP(row.remote_ip)}
                        >
                          {row.is_banned ? '已禁用' : '禁用'}
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

export default TrafficMonitor; 