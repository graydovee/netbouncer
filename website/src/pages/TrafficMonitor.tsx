import { useCallback, useEffect, useMemo, useState } from 'react'
import {
  Alert,
  Box,
  Button,
  Chip,
  CircularProgress,
  IconButton,
  Paper,
  Table,
  TableBody,
  TableCell,
  TableContainer,
  TableHead,
  TablePagination,
  TableRow,
  TextField,
  Tooltip,
  Typography,
} from '@mui/material'
import {
  ArrowDownward as ArrowDownIcon,
  ArrowUpward as ArrowUpIcon,
  FilterList as FilterIcon,
  PlayArrow as PlayIcon,
  Refresh as RefreshIcon,
  Stop as StopIcon,
} from '@mui/icons-material'
import { trafficApi } from '../api/traffic'
import { ipApi } from '../api/ip'
import { errorMessage } from '../api/client'
import type { TrafficData } from '../api/types'
import { MessageSnackbar } from '../components/MessageSnackbar'
import { useMessageSnackbar } from '../hooks/useMessageSnackbar'
import { EmptyState } from '../components/EmptyState'
import { RowsPerPageControl } from '../components/RowsPerPageControl'
import { useUrlParams } from '../hooks/useUrlParams'
import { formatBytes, formatBytesPerSec, formatNumber, formatTimestamp } from '../utils/format'
import SearchOffIcon from '@mui/icons-material/SearchOff'

const MIN_REFRESH_SECONDS = 1
const MAX_REFRESH_SECONDS = 3600

interface Column {
  key: keyof TrafficData | 'actions'
  label: string
  sortable: boolean
  render?: (row: TrafficData) => string
  /** 在中屏以下隐藏次要列，避免横向滚动 */
  hideBelow?: 'sm' | 'md' | 'lg'
}

const columns: Column[] = [
  { key: 'remote_ip', label: '远程IP', sortable: true },
  { key: 'local_ip', label: '本地IP', sortable: true, hideBelow: 'lg' },
  { key: 'total_bytes_in', label: '总接收流量', sortable: true, render: (row) => formatBytes(row.total_bytes_in) },
  { key: 'total_bytes_out', label: '总发送流量', sortable: true, render: (row) => formatBytes(row.total_bytes_out) },
  {
    key: 'total_packets_in',
    label: '接收包数',
    sortable: true,
    render: (row) => formatNumber(row.total_packets_in),
    hideBelow: 'md',
  },
  {
    key: 'total_packets_out',
    label: '发送包数',
    sortable: true,
    render: (row) => formatNumber(row.total_packets_out),
    hideBelow: 'md',
  },
  {
    key: 'bytes_in_per_sec',
    label: '接收速率',
    sortable: true,
    render: (row) => formatBytesPerSec(row.bytes_in_per_sec),
  },
  {
    key: 'bytes_out_per_sec',
    label: '发送速率',
    sortable: true,
    render: (row) => formatBytesPerSec(row.bytes_out_per_sec),
  },
  { key: 'connections', label: '连接数', sortable: true, hideBelow: 'md' },
  {
    key: 'first_seen',
    label: '首次发现',
    sortable: true,
    render: (row) => formatTimestamp(row.first_seen),
    hideBelow: 'lg',
  },
  {
    key: 'last_seen',
    label: '最后活动',
    sortable: true,
    render: (row) => formatTimestamp(row.last_seen),
    hideBelow: 'sm',
  },
  { key: 'actions', label: '操作', sortable: false },
]

const hideBelowSx = (hideBelow?: Column['hideBelow']) => {
  if (!hideBelow) return {}
  return { display: { xs: 'none', [hideBelow]: 'table-cell' } }
}

function TrafficMonitor() {
  const [trafficData, setTrafficData] = useState<TrafficData[]>([])
  const [loading, setLoading] = useState(false)
  const [initialLoading, setInitialLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [lastUpdate, setLastUpdate] = useState<Date | null>(null)

  // 排序状态同步到 URL，刷新/分享不丢失
  const { getParam, updateParams } = useUrlParams()
  const sortKey = getParam('sort', 'bytes_out_per_sec') as keyof TrafficData
  const sortAsc = getParam('order', 'desc') === 'asc'

  const [refreshInterval, setRefreshInterval] = useState(30)
  const [autoRefresh, setAutoRefresh] = useState(true)
  const [page, setPage] = useState(0)
  const [rowsPerPage, setRowsPerPage] = useState(25)

  const [filterRemoteIP, setFilterRemoteIP] = useState('')
  const [filterLocalIP, setFilterLocalIP] = useState('')
  const [showFilters, setShowFilters] = useState(false)

  const { snackbar, showMessage, hideMessage } = useMessageSnackbar()

  const fetchData = useCallback(async (showLoading: boolean) => {
    if (showLoading) {
      setLoading(true)
    }
    setError(null)
    try {
      const data = await trafficApi.get()
      setTrafficData(data)
      setLastUpdate(new Date())
    } catch (err) {
      setError(errorMessage(err, '获取流量数据失败'))
    } finally {
      setLoading(false)
      setInitialLoading(false)
    }
  }, [])

  useEffect(() => {
    void fetchData(true)
  }, [fetchData])

  // 自动刷新定时器由 effect 持有：间隔或开关变化时重建，卸载时必然清理
  useEffect(() => {
    if (!autoRefresh) {
      return
    }
    const timer = window.setInterval(() => {
      void fetchData(false)
    }, refreshInterval * 1000)
    return () => window.clearInterval(timer)
  }, [autoRefresh, refreshInterval, fetchData])

  const handleRefreshIntervalChange = (raw: string) => {
    const parsed = parseInt(raw, 10) || MIN_REFRESH_SECONDS
    setRefreshInterval(Math.min(Math.max(parsed, MIN_REFRESH_SECONDS), MAX_REFRESH_SECONDS))
  }

  const handleSort = (key: string) => {
    if (sortKey === key) {
      updateParams({ order: sortAsc ? 'desc' : 'asc' })
    } else {
      updateParams({ sort: key, order: 'desc' })
    }
  }

  // 过滤 + 排序仅在数据或条件变化时计算一次
  const filteredData = useMemo(() => {
    let filtered = trafficData
    if (filterRemoteIP) {
      const needle = filterRemoteIP.toLowerCase()
      filtered = filtered.filter((item) => item.remote_ip.toLowerCase().includes(needle))
    }
    if (filterLocalIP) {
      const needle = filterLocalIP.toLowerCase()
      filtered = filtered.filter((item) => item.local_ip.toLowerCase().includes(needle))
    }
    return filtered
  }, [trafficData, filterRemoteIP, filterLocalIP])

  const sortedData = useMemo(() => {
    const sorted = [...filteredData]
    sorted.sort((a, b) => {
      const v1 = a[sortKey]
      const v2 = b[sortKey]
      let compared: number
      if (typeof v1 === 'string' || typeof v2 === 'string') {
        compared = String(v1).localeCompare(String(v2))
      } else {
        compared = (v1 as number) - (v2 as number)
      }
      return sortAsc ? compared : -compared
    })
    return sorted
  }, [filteredData, sortKey, sortAsc])

  const currentPageData = useMemo(
    () => sortedData.slice(page * rowsPerPage, (page + 1) * rowsPerPage),
    [sortedData, page, rowsPerPage],
  )

  const hasFilters = Boolean(filterRemoteIP || filterLocalIP)

  const clearFilters = () => {
    setFilterRemoteIP('')
    setFilterLocalIP('')
    setPage(0)
  }

  // 一键封禁当前远程IP
  const banIP = async (ip: string) => {
    try {
      await ipApi.create({ ip_net: ip, group_id: 0, action: 'ban' })
      setTrafficData((prev) =>
        prev.map((item) => (item.remote_ip === ip ? { ...item, is_banned: true } : item)),
      )
      showMessage(`成功禁用 ${ip}`)
    } catch (err) {
      showMessage(errorMessage(err, '禁用失败'), 'error')
    }
  }

  return (
    <Box>
      <Typography variant="h4" gutterBottom>
        流量监控
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
            onChange={(event) => handleRefreshIntervalChange(event.target.value)}
            size="small"
            sx={{ width: 150 }}
            inputProps={{ min: MIN_REFRESH_SECONDS, max: MAX_REFRESH_SECONDS }}
          />
          <Button
            variant="contained"
            onClick={() => setAutoRefresh(!autoRefresh)}
            startIcon={autoRefresh ? <StopIcon /> : <PlayIcon />}
            color={autoRefresh ? 'error' : 'success'}
          >
            {autoRefresh ? '停止刷新' : '开始刷新'}
          </Button>
          <IconButton
            onClick={() => void fetchData(false)}
            disabled={loading}
            color="primary"
            size="small"
            sx={{
              border: '1px solid',
              borderColor: 'primary.main',
              '&:hover': {
                backgroundColor: 'primary.main',
                color: 'white',
              },
            }}
          >
            <RefreshIcon />
          </IconButton>
          <Chip
            label={`状态: ${autoRefresh ? '自动刷新中' : '已停止'}`}
            color={autoRefresh ? 'success' : 'default'}
            size="small"
          />
          {lastUpdate && (
            <Typography variant="body2" color="text.secondary">
              最后更新: {lastUpdate.toLocaleTimeString('zh-CN')}
            </Typography>
          )}
          <Typography variant="body2" color="text.secondary">
            共 {formatNumber(filteredData.length)} 条记录
            {hasFilters && <span>（已过滤）</span>}
          </Typography>
        </Box>

        {/* 过滤面板 */}
        <Box sx={{ mt: 2, display: 'flex', alignItems: 'center', gap: 2, flexWrap: 'wrap' }}>
          <Button
            variant="outlined"
            onClick={() => setShowFilters(!showFilters)}
            startIcon={<FilterIcon />}
            color={showFilters ? 'primary' : 'inherit'}
            size="small"
          >
            过滤
          </Button>
          {hasFilters && (
            <Button variant="outlined" size="small" onClick={clearFilters}>
              清除过滤
            </Button>
          )}
        </Box>

        {showFilters && (
          <Box sx={{ mt: 2, p: 2, border: '1px solid', borderColor: 'divider', borderRadius: 1 }}>
            <Typography variant="subtitle2" gutterBottom>
              过滤条件
            </Typography>
            <Box sx={{ display: 'flex', gap: 2, flexWrap: 'wrap' }}>
              <TextField
                size="small"
                label="远程地址"
                placeholder="例如：192.168, 10.0"
                value={filterRemoteIP}
                onChange={(event) => {
                  setFilterRemoteIP(event.target.value)
                  setPage(0)
                }}
                sx={{ minWidth: 220 }}
              />
              <TextField
                size="small"
                label="本地地址"
                placeholder="例如：192.168, 10.0"
                value={filterLocalIP}
                onChange={(event) => {
                  setFilterLocalIP(event.target.value)
                  setPage(0)
                }}
                sx={{ minWidth: 220 }}
              />
            </Box>
          </Box>
        )}

        {/* 分页设置 */}
        <Box sx={{ mt: 2 }}>
          <RowsPerPageControl
            value={rowsPerPage}
            onChange={(value) => {
              setRowsPerPage(value)
              setPage(0)
            }}
          />
        </Box>
      </Paper>

      {/* 数据表格 */}
      <Paper>
        <TableContainer>
          <Table size="small">
            <TableHead>
              <TableRow>
                {columns.map((column) => (
                  <TableCell
                    key={column.key}
                    sx={{
                      fontWeight: 'bold',
                      cursor: column.sortable ? 'pointer' : 'default',
                      '&:hover': column.sortable ? { backgroundColor: 'action.hover' } : {},
                      ...hideBelowSx(column.hideBelow),
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
                    <Box sx={{ display: 'flex', alignItems: 'center', justifyContent: 'center', py: 3 }}>
                      <CircularProgress size={24} />
                      <Typography sx={{ ml: 1 }}>加载中...</Typography>
                    </Box>
                  </TableCell>
                </TableRow>
              ) : currentPageData.length === 0 ? (
                <TableRow>
                  <TableCell colSpan={columns.length}>
                    <EmptyState
                      icon={<SearchOffIcon />}
                      title={hasFilters ? '没有匹配的记录' : '暂无流量数据'}
                      description={
                        hasFilters
                          ? '试试调整或清除过滤条件'
                          : '有网络流量经过时，这里会实时显示各远程IP的连接情况'
                      }
                    />
                  </TableCell>
                </TableRow>
              ) : (
                currentPageData.map((row) => (
                  <TableRow key={`${row.remote_ip}-${row.local_ip}`} hover>
                    {columns.map((column) => {
                      if (column.key === 'actions') {
                        return (
                          <TableCell key="actions">
                            <Tooltip title={row.is_banned ? '已禁用' : '禁用此IP'}>
                              <span>
                                <Button
                                  variant="outlined"
                                  size="small"
                                  color="error"
                                  disabled={row.is_banned}
                                  onClick={() => void banIP(row.remote_ip)}
                                >
                                  {row.is_banned ? '已禁用' : '禁用'}
                                </Button>
                              </span>
                            </Tooltip>
                          </TableCell>
                        )
                      }
                      const text = column.render
                        ? column.render(row)
                        : String(row[column.key as keyof TrafficData] ?? '')
                      const isAddress = column.key === 'remote_ip' || column.key === 'local_ip'
                      return (
                        <TableCell
                          key={column.key}
                          sx={{
                            fontFamily: isAddress ? 'monospace' : undefined,
                            color: column.key === 'first_seen' || column.key === 'last_seen'
                              ? 'text.secondary'
                              : undefined,
                            fontSize: '0.875rem',
                            ...hideBelowSx(column.hideBelow),
                          }}
                        >
                          {text}
                        </TableCell>
                      )
                    })}
                  </TableRow>
                ))
              )}
            </TableBody>
          </Table>
        </TableContainer>

        <TablePagination
          component="div"
          count={filteredData.length}
          page={page}
          onPageChange={(_, newPage) => setPage(newPage)}
          rowsPerPage={rowsPerPage}
          onRowsPerPageChange={() => {
            // 每页条数由上方 RowsPerPageControl 控制
          }}
          rowsPerPageOptions={[rowsPerPage]}
          labelRowsPerPage="每页显示:"
          labelDisplayedRows={({ from, to, count }) => `${from}-${to} / ${count}`}
          showFirstButton
          showLastButton
        />
      </Paper>
      <MessageSnackbar snackbar={snackbar} onClose={hideMessage} />
    </Box>
  )
}

export default TrafficMonitor
