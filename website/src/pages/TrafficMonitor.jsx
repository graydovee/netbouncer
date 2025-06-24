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
  TablePagination,
  Grid,
  FormControl,
  Select,
  MenuItem,
} from '@mui/material';
import {
  Refresh as RefreshIcon,
  PlayArrow as PlayIcon,
  Stop as StopIcon,
  ArrowUpward as ArrowUpIcon,
  ArrowDownward as ArrowDownIcon,
  FilterList as FilterIcon,
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
  
  // 分页相关状态
  const [page, setPage] = useState(0);
  const [rowsPerPage, setRowsPerPage] = useState(25);
  const [customRowsPerPage, setCustomRowsPerPage] = useState(25);
  
  // 过滤相关状态
  const [filterRemoteIP, setFilterRemoteIP] = useState('');
  const [filterLocalIP, setFilterLocalIP] = useState('');
  const [showFilters, setShowFilters] = useState(false);
  
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

  // 分页处理函数
  const handleChangePage = (event, newPage) => {
    setPage(newPage);
  };

  const handleChangeRowsPerPage = (event) => {
    const newRowsPerPage = parseInt(event.target.value, 10);
    if (newRowsPerPage > 0) {
      setRowsPerPage(newRowsPerPage);
      setCustomRowsPerPage(newRowsPerPage);
      setPage(0);
    }
  };

  // 自定义分页数量处理
  const handleCustomRowsPerPageChange = (event) => {
    const value = parseInt(event.target.value, 10);
    setCustomRowsPerPage(value);
  };

  // 自定义分页数量失去焦点时生效
  const handleCustomRowsPerPageBlur = () => {
    const value = customRowsPerPage;
    if (value > 0 && value <= 10000) {
      setRowsPerPage(value);
      setPage(0);
    } else {
      // 如果输入的值无效，重置为当前有效的分页数量
      setCustomRowsPerPage(rowsPerPage);
    }
  };

  // 自定义分页数量回车键处理
  const handleCustomRowsPerPageKeyDown = (event) => {
    if (event.key === 'Enter') {
      event.target.blur(); // 触发失去焦点事件
    }
  };

  // 过滤数据
  const getFilteredData = () => {
    let filtered = [...trafficData];
    
    // 按远程IP过滤
    if (filterRemoteIP) {
      filtered = filtered.filter(item => 
        item.remote_ip.toLowerCase().includes(filterRemoteIP.toLowerCase())
      );
    }
    
    // 按本地IP过滤
    if (filterLocalIP) {
      filtered = filtered.filter(item => 
        item.local_ip.toLowerCase().includes(filterLocalIP.toLowerCase())
      );
    }
    
    return filtered;
  };

  // 排序数据
  const sortedData = getFilteredData().sort((a, b) => {
    let v1 = a[sortKey], v2 = b[sortKey];
    if (typeof v1 === 'string') {
      return sortAsc ? v1.localeCompare(v2) : v2.localeCompare(v1);
    } else {
      return sortAsc ? v1 - v2 : v2 - v1;
    }
  });

  // 获取当前页的数据
  const getCurrentPageData = () => {
    const startIndex = page * rowsPerPage;
    const endIndex = startIndex + rowsPerPage;
    return sortedData.slice(startIndex, endIndex);
  };

  // 清除过滤条件
  const clearFilters = () => {
    setFilterRemoteIP('');
    setFilterLocalIP('');
    setPage(0);
  };

  // 获取过滤后的数据总数
  const getFilteredDataCount = () => {
    return getFilteredData().length;
  };

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
          <Typography variant="body2" color="text.secondary">
            共 {getFilteredDataCount()} 条记录
            {getFilteredDataCount() > 0 && (
              <span>，当前页 {page + 1}/{Math.ceil(getFilteredDataCount() / rowsPerPage)}</span>
            )}
            {(filterRemoteIP || filterLocalIP) && (
              <span>（已过滤）</span>
            )}
          </Typography>
        </Box>

        {/* 过滤面板 */}
        <Box sx={{ mt: 2, display: 'flex', alignItems: 'center', gap: 2, flexWrap: 'wrap' }}>
          <Button
            variant="outlined"
            onClick={() => setShowFilters(!showFilters)}
            startIcon={<FilterIcon />}
            color={showFilters ? 'primary' : 'default'}
            size="small"
          >
            过滤
          </Button>
          {(filterRemoteIP || filterLocalIP) && (
            <Button
              variant="outlined"
              size="small"
              onClick={clearFilters}
            >
              清除过滤
            </Button>
          )}
        </Box>

        {showFilters && (
          <Box sx={{ mt: 2, p: 2, border: '1px solid', borderColor: 'divider', borderRadius: 1 }}>
            <Typography variant="subtitle2" gutterBottom>
              过滤条件
            </Typography>
            <Grid container spacing={2} alignItems="center">
              <Grid item xs={12} sm={4}>
                <TextField
                  fullWidth
                  size="small"
                  label="远程地址"
                  placeholder="例如：192.168, 10.0"
                  value={filterRemoteIP}
                  onChange={(e) => {
                    setFilterRemoteIP(e.target.value);
                    setPage(0);
                  }}
                />
              </Grid>
              <Grid item xs={12} sm={4}>
                <TextField
                  fullWidth
                  size="small"
                  label="本地地址"
                  placeholder="例如：192.168, 10.0"
                  value={filterLocalIP}
                  onChange={(e) => {
                    setFilterLocalIP(e.target.value);
                    setPage(0);
                  }}
                />
              </Grid>
            </Grid>
          </Box>
        )}

        {/* 分页设置 */}
        <Box sx={{ mt: 2, display: 'flex', alignItems: 'center', gap: 2, flexWrap: 'wrap' }}>
          <Typography variant="body2" color="text.secondary">
            每页显示：
          </Typography>
          <FormControl size="small" sx={{ minWidth: 120 }}>
            <Select
              value={rowsPerPage}
              onChange={handleChangeRowsPerPage}
              displayEmpty
            >
              <MenuItem value={10}>10 条</MenuItem>
              <MenuItem value={25}>25 条</MenuItem>
              <MenuItem value={50}>50 条</MenuItem>
              <MenuItem value={100}>100 条</MenuItem>
            </Select>
          </FormControl>
          <TextField
            size="small"
            type="number"
            label="自定义数量"
            value={customRowsPerPage}
            onChange={handleCustomRowsPerPageChange}
            onBlur={handleCustomRowsPerPageBlur}
            onKeyDown={handleCustomRowsPerPageKeyDown}
            inputProps={{ min: 1, max: 10000 }}
            sx={{ width: 120 }}
          />
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
              ) : getCurrentPageData().length === 0 ? (
                <TableRow>
                  <TableCell colSpan={columns.length} align="center">
                    暂无数据
                  </TableCell>
                </TableRow>
              ) : (
                getCurrentPageData().map((row, index) => (
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
        
        {/* 分页组件 */}
        <TablePagination
          component="div"
          count={getFilteredDataCount()}
          page={page}
          onPageChange={handleChangePage}
          rowsPerPage={rowsPerPage}
          onRowsPerPageChange={handleChangeRowsPerPage}
          rowsPerPageOptions={[10, 25, 50, 100, customRowsPerPage].filter((value, index, self) => self.indexOf(value) === index).sort((a, b) => a - b)}
          labelRowsPerPage="每页显示:"
          labelDisplayedRows={({ from, to, count }) => `${from}-${to} / ${count}`}
          showFirstButton
          showLastButton
        />
      </Paper>
      <MessageSnackbar snackbar={snackbar} onClose={hideMessage} />
    </Box>
  );
}

export default TrafficMonitor; 