import { useCallback, useEffect, useMemo, useState } from 'react'
import {
  Alert,
  Box,
  Button,
  Checkbox,
  Chip,
  CircularProgress,
  Grid,
  IconButton,
  Paper,
  Stack,
  Table,
  TableBody,
  TableCell,
  TableContainer,
  TableHead,
  TablePagination,
  TableRow,
  TextField,
  ToggleButton,
  ToggleButtonGroup,
  Tooltip,
  Typography,
} from '@mui/material'
import {
  ArrowDownward as ArrowDownIcon,
  ArrowUpward as ArrowUpIcon,
  Block as BlockIcon,
  FilterList as FilterIcon,
  Info as InfoIcon,
  Language as LanguageIcon,
  PlayArrow as PlayIcon,
  Refresh as RefreshIcon,
  Shield as ShieldIcon,
  Speed as SpeedIcon,
  Stop as StopIcon,
  VerifiedUser as AllowIcon,
} from '@mui/icons-material'
import { trafficApi } from '../api/traffic'
import { ipApi } from '../api/ip'
import { errorMessage } from '../api/client'
import type { TrafficData, TrafficHistoryPoint, TrafficTopEntry } from '../api/types'
import { MessageSnackbar } from '../components/MessageSnackbar'
import { useMessageSnackbar } from '../hooks/useMessageSnackbar'
import { ConfirmDialog, useConfirmDialog } from '../components/ConfirmDialog'
import { EmptyState } from '../components/EmptyState'
import { RowsPerPageControl } from '../components/RowsPerPageControl'
import { StatCard } from '../components/StatCard'
import { ApplyRuleDialog } from '../components/ApplyRuleDialog'
import { IpDetailDrawer } from '../components/IpDetailDrawer'
import { EChart, type EChartsOption } from '../components/charts/EChart'
import { useUrlParams } from '../hooks/useUrlParams'
import { formatBytes, formatBytesPerSec, formatNumber, formatTimestamp } from '../utils/format'
import { groupApi } from '../api/group'
import type { IpNetAction } from '../api/types'

const MIN_REFRESH_SECONDS = 5
const MAX_REFRESH_SECONDS = 3600

// 历史时间范围（秒 → 合理的聚合桶宽）
const RANGES = [
  { key: '1h', label: '1小时', seconds: 3600, bucket: 60 },
  { key: '6h', label: '6小时', seconds: 6 * 3600, bucket: 300 },
  { key: '24h', label: '24小时', seconds: 24 * 3600, bucket: 900 },
  { key: '7d', label: '7天', seconds: 7 * 86400, bucket: 3600 },
  { key: '30d', label: '30天', seconds: 30 * 86400, bucket: 14400 },
] as const

type RangeKey = (typeof RANGES)[number]['key']

/** 行内管控状态徽标 */
function ruleStatus(row: TrafficData): { label: string; color: 'error' | 'success' | 'warning' | 'default' } {
  if (row.rule_action === 'ban') return { label: '已封禁', color: 'error' }
  if (row.rule_action === 'allow') return { label: '已加白', color: 'success' }
  if (row.is_banned) return { label: '被网段封禁', color: 'warning' }
  return { label: '未管控', color: 'default' }
}

function TrafficMonitor() {
  // ---- 实时数据 ----
  const [trafficData, setTrafficData] = useState<TrafficData[]>([])
  const [loading, setLoading] = useState(false)
  const [initialLoading, setInitialLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [lastUpdate, setLastUpdate] = useState<Date | null>(null)
  const [autoRefresh, setAutoRefresh] = useState(true)
  const [refreshInterval, setRefreshInterval] = useState(30)

  const { getParam, updateParams } = useUrlParams()
  const sortKey = getParam('sort', 'bytes_out_per_sec') as keyof TrafficData
  const sortAsc = getParam('order', 'desc') === 'asc'

  const [page, setPage] = useState(0)
  const [rowsPerPage, setRowsPerPage] = useState(25)

  const [filterRemoteIP, setFilterRemoteIP] = useState('')
  const [filterLocalIP, setFilterLocalIP] = useState('')
  const [showFilters, setShowFilters] = useState(false)

  // ---- 历史图表 ----
  const [rangeKey, setRangeKey] = useState<RangeKey>('24h')
  const [historyPoints, setHistoryPoints] = useState<TrafficHistoryPoint[]>([])
  const [topEntries, setTopEntries] = useState<TrafficTopEntry[]>([])
  const [historyLoading, setHistoryLoading] = useState(false)

  // ---- 交互 ----
  const [selectedIPs, setSelectedIPs] = useState<Set<string>>(new Set())
  const [batchAction, setBatchAction] = useState<IpNetAction | null>(null)
  const [detail, setDetail] = useState<{ ip: string; live: TrafficData | null } | null>(null)

  const { snackbar, showMessage, hideMessage } = useMessageSnackbar()
  const { showConfirm, confirmState, handleConfirm, handleCancel } = useConfirmDialog()

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

  const range = RANGES.find((r) => r.key === rangeKey) ?? RANGES[1]

  const fetchHistory = useCallback(async () => {
    setHistoryLoading(true)
    try {
      const end = Math.floor(Date.now() / 1000)
      const start = end - range.seconds
      const [points, top] = await Promise.all([
        trafficApi.history({ start, end, bucket: range.bucket }),
        trafficApi.historyTop({ start, end, limit: 10 }),
      ])
      setHistoryPoints(points)
      setTopEntries(top)
    } catch (err) {
      console.error('获取流量历史失败:', err)
    } finally {
      setHistoryLoading(false)
    }
  }, [range])

  useEffect(() => {
    void fetchData(true)
  }, [fetchData])

  useEffect(() => {
    void fetchHistory()
  }, [fetchHistory])

  // 自动刷新：实时 + 历史一起刷新
  useEffect(() => {
    if (!autoRefresh) {
      return
    }
    const timer = window.setInterval(() => {
      void fetchData(false)
      void fetchHistory()
    }, Math.max(refreshInterval, 15) * 1000)
    return () => window.clearInterval(timer)
  }, [autoRefresh, refreshInterval, fetchData, fetchHistory])

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

  // ---- 过滤/排序/分页 ----
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

  // ---- 概览统计 ----
  const overview = useMemo(() => {
    let down = 0
    let up = 0
    let connections = 0
    let banned = 0
    for (const item of trafficData) {
      down += item.bytes_in_per_sec
      up += item.bytes_out_per_sec
      connections += item.connections
      if (item.is_banned) {
        banned++
      }
    }
    return { down, up, connections, banned, total: trafficData.length }
  }, [trafficData])

  // ---- 图表配置 ----
  const trendOption = useMemo<EChartsOption>(() => {
    return {
      backgroundColor: 'transparent',
      tooltip: {
        trigger: 'axis',
        axisPointer: { type: 'line' },
        valueFormatter: (value: unknown) => formatBytes(Number(value)),
      },
      legend: { bottom: 0, icon: 'roundRect', itemWidth: 14, itemHeight: 8 },
      grid: { left: 8, right: 16, top: 20, bottom: 44, containLabel: true },
      xAxis: {
        type: 'time',
        axisLabel: { hideOverlap: true },
        axisLine: { lineStyle: { opacity: 0.3 } },
      },
      yAxis: {
        type: 'value',
        axisLabel: { formatter: (v: number) => formatBytes(v) },
        splitLine: { lineStyle: { opacity: 0.2 } },
      },
      series: [
        {
          name: '下行',
          type: 'line',
          data: historyPoints.map((p) => [p.ts * 1000, p.bytes_in]),
          smooth: true,
          showSymbol: false,
          areaStyle: { opacity: 0.22 },
          lineStyle: { width: 2 },
        },
        {
          name: '上行',
          type: 'line',
          data: historyPoints.map((p) => [p.ts * 1000, p.bytes_out]),
          smooth: true,
          showSymbol: false,
          areaStyle: { opacity: 0.22 },
          lineStyle: { width: 2 },
        },
      ],
    }
  }, [historyPoints])

  const topOption = useMemo<EChartsOption>(() => {
    // 横向柱状图倒序显示（最大的在最上面）
    const entries = [...topEntries].reverse()
    return {
      backgroundColor: 'transparent',
      tooltip: {
        trigger: 'axis',
        axisPointer: { type: 'shadow' },
        valueFormatter: (value: unknown) => formatBytes(Number(value)),
      },
      grid: { left: 8, right: 24, top: 8, bottom: 8, containLabel: true },
      xAxis: {
        type: 'value',
        axisLabel: { formatter: (v: number) => formatBytes(v) },
        splitLine: { lineStyle: { opacity: 0.2 } },
      },
      yAxis: {
        type: 'category',
        data: entries.map((e) => e.ip),
        axisLabel: { fontSize: 11 },
      },
      series: [
        {
          name: '总流量',
          type: 'bar',
          data: entries.map((e) => e.bytes_sum),
          barMaxWidth: 18,
          itemStyle: { borderRadius: [0, 4, 4, 0] },
        },
      ],
    }
  }, [topEntries])

  // ---- 操作 ----
  const openDetail = useCallback(
    (ip: string) => {
      const live = trafficData.find((item) => item.remote_ip === ip) ?? null
      setDetail({ ip, live })
    },
    [trafficData],
  )

  const [defaultGroupId, setDefaultGroupId] = useState<number>(0)
  useEffect(() => {
    groupApi
      .list()
      .then((list) => setDefaultGroupId((list.find((g) => g.is_default) ?? list[0])?.id ?? 0))
      .catch(() => setDefaultGroupId(0))
  }, [])

  const applyAction = async (ip: string, action: IpNetAction) => {
    try {
      await ipApi.create({ ip_net: ip, group_id: defaultGroupId, action })
      showMessage(action === 'ban' ? `已封禁 ${ip}` : `已加白 ${ip}`)
      await fetchData(false)
    } catch (err) {
      showMessage(errorMessage(err, '操作失败'), 'error')
    }
  }

  const confirmUnban = (row: TrafficData) => {
    showConfirm(
      '确认解除规则',
      `确定要解除 ${row.remote_ip} 的${row.rule_action === 'ban' ? '封禁' : '加白'}规则吗？`,
      'warning',
      () => {
        void (async () => {
          try {
            await ipApi.remove(row.rule_id)
            showMessage(`已解除 ${row.remote_ip} 的规则`)
            await fetchData(false)
          } catch (err) {
            showMessage(errorMessage(err, '操作失败'), 'error')
          }
        })()
      },
    )
  }

  const requestAction = (row: TrafficData, action: IpNetAction) => {
    if (row.rule_action === action) {
      // 已是目标动作 → 解除
      confirmUnban(row)
      return
    }
    const actionLabel = action === 'ban' ? '封禁' : '加白'
    showConfirm(
      `确认${actionLabel}`,
      `确定要${actionLabel} ${row.remote_ip} 吗？${action === 'ban' ? '\n\n封禁后将立即阻断该 IP 的所有连接。' : ''}`,
      action === 'ban' ? 'error' : 'question',
      () => {
        void applyAction(row.remote_ip, action)
      },
    )
  }

  const hasFilters = Boolean(filterRemoteIP || filterLocalIP)

  const clearFilters = () => {
    setFilterRemoteIP('')
    setFilterLocalIP('')
    setPage(0)
  }

  // ---- 选择 ----
  const currentPageIPs = useMemo(() => currentPageData.map((row) => row.remote_ip), [currentPageData])
  const selectedOnPage = useMemo(
    () => currentPageIPs.filter((ip) => selectedIPs.has(ip)),
    [currentPageIPs, selectedIPs],
  )
  const allOnPageSelected = currentPageIPs.length > 0 && selectedOnPage.length === currentPageIPs.length

  const toggleSelectAllOnPage = () => {
    setSelectedIPs((prev) => {
      const next = new Set(prev)
      if (allOnPageSelected) {
        currentPageIPs.forEach((ip) => next.delete(ip))
      } else {
        currentPageIPs.forEach((ip) => next.add(ip))
      }
      return next
    })
  }

  const toggleSelect = (ip: string) => {
    setSelectedIPs((prev) => {
      const next = new Set(prev)
      if (next.has(ip)) {
        next.delete(ip)
      } else {
        next.add(ip)
      }
      return next
    })
  }

  return (
    <Box>
      <Stack direction="row" alignItems="center" justifyContent="space-between" sx={{ mb: 2, flexWrap: 'wrap', gap: 1 }}>
        <Typography variant="h4">流量监控</Typography>
        <Stack direction="row" alignItems="center" spacing={1.5}>
          {lastUpdate && (
            <Typography variant="caption" color="text.secondary">
              最后更新 {lastUpdate.toLocaleTimeString('zh-CN')}
            </Typography>
          )}
          <Tooltip title="手动刷新">
            <IconButton onClick={() => { void fetchData(false); void fetchHistory() }} disabled={loading} size="small">
              <RefreshIcon />
            </IconButton>
          </Tooltip>
          <Button
            variant={autoRefresh ? 'contained' : 'outlined'}
            size="small"
            startIcon={autoRefresh ? <StopIcon /> : <PlayIcon />}
            color={autoRefresh ? 'primary' : 'inherit'}
            onClick={() => setAutoRefresh(!autoRefresh)}
          >
            {autoRefresh ? '自动刷新中' : '已暂停'}
          </Button>
          <TextField
            label="间隔(秒)"
            type="number"
            value={refreshInterval}
            onChange={(event) => handleRefreshIntervalChange(event.target.value)}
            size="small"
            sx={{ width: 100 }}
            inputProps={{ min: MIN_REFRESH_SECONDS, max: MAX_REFRESH_SECONDS }}
          />
        </Stack>
      </Stack>

      {error && (
        <Alert severity="error" sx={{ mb: 2 }} onClose={() => setError(null)}>
          {error}
        </Alert>
      )}

      {/* 概览卡片 */}
      <Grid container spacing={2} sx={{ mb: 2 }}>
        <Grid size={{ xs: 12, sm: 6, md: 2.4 }}>
          <StatCard icon={<SpeedIcon />} label="下行速率" value={`${formatBytesPerSec(overview.down)}`} color="success" />
        </Grid>
        <Grid size={{ xs: 12, sm: 6, md: 2.4 }}>
          <StatCard icon={<SpeedIcon sx={{ transform: 'rotate(180deg)' }} />} label="上行速率" value={`${formatBytesPerSec(overview.up)}`} color="info" />
        </Grid>
        <Grid size={{ xs: 12, sm: 6, md: 2.4 }}>
          <StatCard icon={<LanguageIcon />} label="活跃 IP" value={formatNumber(overview.total)} sub={`过滤后 ${filteredData.length}`} color="primary" />
        </Grid>
        <Grid size={{ xs: 12, sm: 6, md: 2.4 }}>
          <StatCard icon={<ShieldIcon />} label="封禁中" value={formatNumber(overview.banned)} color="error" />
        </Grid>
        <Grid size={{ xs: 12, sm: 12, md: 2.4 }}>
          <StatCard icon={<BlockIcon />} label="活跃连接" value={formatNumber(overview.connections)} color="warning" />
        </Grid>
      </Grid>

      {/* 图表区 */}
      <Paper sx={{ p: 2, mb: 2 }}>
        <Stack direction="row" alignItems="center" justifyContent="space-between" sx={{ mb: 1, flexWrap: 'wrap', gap: 1 }}>
          <Typography variant="subtitle1" sx={{ fontWeight: 600 }}>
            流量趋势
            <Typography component="span" variant="caption" color="text.secondary" sx={{ ml: 1 }}>
              {historyPoints.length > 0 ? `共 ${historyPoints.length} 个采样点` : '等待采样数据'}
            </Typography>
          </Typography>
          <ToggleButtonGroup
            size="small"
            exclusive
            value={rangeKey}
            onChange={(_, value) => value && setRangeKey(value)}
          >
            {RANGES.map((r) => (
              <ToggleButton key={r.key} value={r.key} sx={{ px: 1.5, py: 0.25, fontSize: 12 }}>
                {r.label}
              </ToggleButton>
            ))}
          </ToggleButtonGroup>
        </Stack>
        {historyLoading && historyPoints.length === 0 ? (
          <Box sx={{ display: 'flex', justifyContent: 'center', py: 8 }}>
            <CircularProgress size={28} />
          </Box>
        ) : (
          <EChart option={trendOption} height={300} />
        )}
      </Paper>

      <Grid container spacing={2} sx={{ mb: 2 }}>
        <Grid size={{ xs: 12, md: 5 }}>
          <Paper sx={{ p: 2, height: '100%' }}>
            <Typography variant="subtitle1" sx={{ fontWeight: 600, mb: 1 }}>
              Top 10 流量 IP
              <Typography component="span" variant="caption" color="text.secondary" sx={{ ml: 1 }}>
                点击查看详情
              </Typography>
            </Typography>
            {topEntries.length === 0 ? (
              <Typography variant="body2" color="text.secondary" sx={{ py: 6, textAlign: 'center' }}>
                暂无历史数据
              </Typography>
            ) : (
              <EChart
                option={topOption}
                height={Math.max(240, topEntries.length * 32)}
                onClick={(params) => {
                  const name = (params as { name?: string }).name
                  if (name) {
                    openDetail(name)
                  }
                }}
              />
            )}
          </Paper>
        </Grid>

        {/* 过滤说明占位：右侧放过滤面板，与 Top 榜同高 */}
        <Grid size={{ xs: 12, md: 7 }}>
          <Paper sx={{ p: 2, height: '100%' }}>
            <Stack direction="row" alignItems="center" justifyContent="space-between" sx={{ mb: 1 }}>
              <Typography variant="subtitle1" sx={{ fontWeight: 600 }}>
                实时列表
              </Typography>
              <Button
                size="small"
                variant={showFilters ? 'contained' : 'outlined'}
                startIcon={<FilterIcon />}
                onClick={() => setShowFilters(!showFilters)}
              >
                过滤
              </Button>
            </Stack>
            {showFilters && (
              <Stack direction="row" spacing={2} sx={{ mb: 2, flexWrap: 'wrap', gap: 1 }}>
                <TextField
                  size="small"
                  label="远程地址"
                  placeholder="例如：192.168"
                  value={filterRemoteIP}
                  onChange={(event) => {
                    setFilterRemoteIP(event.target.value)
                    setPage(0)
                  }}
                  sx={{ width: 200 }}
                />
                <TextField
                  size="small"
                  label="本地地址"
                  placeholder="例如：192.168"
                  value={filterLocalIP}
                  onChange={(event) => {
                    setFilterLocalIP(event.target.value)
                    setPage(0)
                  }}
                  sx={{ width: 200 }}
                />
                {hasFilters && (
                  <Button size="small" onClick={clearFilters}>
                    清除过滤
                  </Button>
                )}
              </Stack>
            )}
            <RowsPerPageControl
              value={rowsPerPage}
              onChange={(value) => {
                setRowsPerPage(value)
                setPage(0)
              }}
            />
          </Paper>
        </Grid>
      </Grid>

      {/* 批量操作栏 */}
      {selectedIPs.size > 0 && (
        <Paper sx={{ p: 1.5, mb: 2, display: 'flex', alignItems: 'center', gap: 1.5, flexWrap: 'wrap' }}>
          <Typography variant="body2" color="primary" sx={{ fontWeight: 600 }}>
            已选择 {selectedIPs.size} 个 IP
          </Typography>
          <Button variant="contained" color="error" size="small" startIcon={<BlockIcon />} onClick={() => setBatchAction('ban')}>
            批量封禁
          </Button>
          <Button variant="contained" color="success" size="small" startIcon={<AllowIcon />} onClick={() => setBatchAction('allow')}>
            批量加白
          </Button>
          <Button variant="text" size="small" onClick={() => setSelectedIPs(new Set())}>
            清除选择
          </Button>
        </Paper>
      )}

      {/* 实时列表 */}
      <Paper>
        <TableContainer>
          <Table size="small">
            <TableHead>
              <TableRow>
                <TableCell padding="checkbox">
                  <Checkbox
                    indeterminate={selectedOnPage.length > 0 && !allOnPageSelected}
                    checked={allOnPageSelected}
                    onChange={toggleSelectAllOnPage}
                    disabled={currentPageData.length === 0}
                  />
                </TableCell>
                {(
                  [
                    { key: 'state', label: '状态', sortable: false, hide: null },
                    { key: 'remote_ip', label: '远程IP', sortable: true, hide: null },
                    { key: 'local_ip', label: '本地IP', sortable: true, hide: 'lg' },
                    { key: 'bytes_in_per_sec', label: '下行速率', sortable: true, hide: null },
                    { key: 'bytes_out_per_sec', label: '上行速率', sortable: true, hide: null },
                    { key: 'total', label: '累计收发', sortable: false, hide: 'md' },
                    { key: 'connections', label: '连接', sortable: true, hide: 'md' },
                    { key: 'last_seen', label: '最后活动', sortable: true, hide: 'sm' },
                    { key: 'actions', label: '操作', sortable: false, hide: null },
                  ] as const
                ).map((column) => (
                  <TableCell
                    key={column.key}
                    sx={{
                      fontWeight: 'bold',
                      cursor: column.sortable ? 'pointer' : 'default',
                      '&:hover': column.sortable ? { backgroundColor: 'action.hover' } : {},
                      ...(column.hide ? { display: { xs: 'none', [column.hide]: 'table-cell' } } : {}),
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
                  <TableCell colSpan={10} align="center">
                    <Box sx={{ display: 'flex', alignItems: 'center', justifyContent: 'center', py: 4 }}>
                      <CircularProgress size={24} />
                      <Typography sx={{ ml: 1 }}>加载中...</Typography>
                    </Box>
                  </TableCell>
                </TableRow>
              ) : currentPageData.length === 0 ? (
                <TableRow>
                  <TableCell colSpan={10}>
                    <EmptyState
                      icon={<LanguageIcon />}
                      title={hasFilters ? '没有匹配的记录' : '暂无流量数据'}
                      description={
                        hasFilters
                          ? '试试调整或清除过滤条件'
                          : '有网络流量经过时，这里会实时显示各远程 IP 的连接情况'
                      }
                    />
                  </TableCell>
                </TableRow>
              ) : (
                currentPageData.map((row) => {
                  const status = ruleStatus(row)
                  return (
                    <TableRow key={`${row.remote_ip}-${row.local_ip}`} hover>
                      <TableCell padding="checkbox">
                        <Checkbox checked={selectedIPs.has(row.remote_ip)} onChange={() => toggleSelect(row.remote_ip)} />
                      </TableCell>
                      <TableCell>
                        <Chip label={status.label} size="small" color={status.color} variant={status.color === 'default' ? 'outlined' : 'filled'} />
                      </TableCell>
                      <TableCell sx={{ fontFamily: 'monospace' }}>
                        <Box
                          onClick={() => openDetail(row.remote_ip)}
                          sx={{ cursor: 'pointer', '&:hover': { textDecoration: 'underline' }, display: 'flex', alignItems: 'center', gap: 0.5 }}
                        >
                          {row.remote_ip}
                          <InfoIcon sx={{ fontSize: 14, color: 'text.disabled' }} />
                        </Box>
                      </TableCell>
                      <TableCell sx={{ fontFamily: 'monospace', display: { xs: 'none', lg: 'table-cell' } }}>
                        {row.local_ip}
                      </TableCell>
                      <TableCell sx={{ fontFamily: 'monospace' }}>{formatBytesPerSec(row.bytes_in_per_sec)}</TableCell>
                      <TableCell sx={{ fontFamily: 'monospace' }}>{formatBytesPerSec(row.bytes_out_per_sec)}</TableCell>
                      <TableCell sx={{ fontFamily: 'monospace', display: { xs: 'none', md: 'table-cell' } }}>
                        {formatBytes(row.total_bytes_in + row.total_bytes_out)}
                      </TableCell>
                      <TableCell sx={{ display: { xs: 'none', md: 'table-cell' } }}>{row.connections}</TableCell>
                      <TableCell
                        sx={{ color: 'text.secondary', fontSize: '0.8rem', display: { xs: 'none', sm: 'table-cell' } }}
                      >
                        {formatTimestamp(row.last_seen)}
                      </TableCell>
                      <TableCell>
                        <Stack direction="row" spacing={0.5}>
                          {row.rule_action === 'ban' || row.rule_action === 'allow' ? (
                            <Tooltip title={`解除${row.rule_action === 'ban' ? '封禁' : '加白'}`}>
                              <span>
                                <Button
                                  variant="outlined"
                                  size="small"
                                  color="warning"
                                  onClick={() => confirmUnban(row)}
                                  disabled={!row.rule_id}
                                  sx={{ minWidth: 0, px: 1, whiteSpace: 'nowrap' }}
                                >
                                  解除
                                </Button>
                              </span>
                            </Tooltip>
                          ) : (
                            <Tooltip title="封禁此 IP">
                              <Button
                                variant="outlined"
                                size="small"
                                color="error"
                                startIcon={<BlockIcon />}
                                onClick={() => requestAction(row, 'ban')}
                                sx={{ minWidth: 0, px: 1, whiteSpace: 'nowrap' }}
                              >
                                封禁
                              </Button>
                            </Tooltip>
                          )}
                          {row.rule_action !== 'allow' && (
                            <Tooltip title="加入白名单">
                              <Button
                                variant="outlined"
                                size="small"
                                color="success"
                                onClick={() => requestAction(row, 'allow')}
                                sx={{ minWidth: 0, px: 1, whiteSpace: 'nowrap' }}
                              >
                                加白
                              </Button>
                            </Tooltip>
                          )}
                          <Tooltip title="查看详情">
                            <IconButton size="small" onClick={() => openDetail(row.remote_ip)}>
                              <InfoIcon fontSize="small" />
                            </IconButton>
                          </Tooltip>
                        </Stack>
                      </TableCell>
                    </TableRow>
                  )
                })
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
          onRowsPerPageChange={() => {}}
          rowsPerPageOptions={[rowsPerPage]}
          labelRowsPerPage="每页显示:"
          labelDisplayedRows={({ from, to, count }) => `${from}-${to} / ${count}`}
          showFirstButton
          showLastButton
        />
      </Paper>

      {/* 批量应用规则 */}
      <ApplyRuleDialog
        open={batchAction !== null}
        ips={Array.from(selectedIPs)}
        defaultAction={batchAction ?? 'ban'}
        onClose={() => setBatchAction(null)}
        onMessage={showMessage}
        onSuccess={(result) => {
          setSelectedIPs(new Set())
          showMessage(
            result.failed_count === 0
              ? `成功应用规则到 ${result.success_count} 个 IP`
              : `应用完成：成功 ${result.success_count} 个，失败 ${result.failed_count} 个`,
            result.failed_count > 0 ? 'warning' : 'success',
          )
          void fetchData(false)
        }}
      />

      {/* IP 详情抽屉 */}
      <IpDetailDrawer
        ip={detail?.ip ?? null}
        live={detail?.live ?? null}
        onClose={() => setDetail(null)}
        onMessage={showMessage}
        onChanged={() => void fetchData(false)}
      />

      {/* 解除规则确认框 */}
      <ConfirmDialog confirmState={confirmState} onConfirm={handleConfirm} onCancel={handleCancel} />
      <MessageSnackbar snackbar={snackbar} onClose={hideMessage} />
    </Box>
  )
}

export default TrafficMonitor
