import { useCallback, useEffect, useMemo, useState } from 'react'
import {
  Alert,
  Box,
  Button,
  Chip,
  IconButton,
  Paper,
  Stack,
  Tab,
  Table,
  TableBody,
  TableCell,
  TableContainer,
  TableHead,
  TablePagination,
  TableRow,
  Tabs,
  TextField,
  Tooltip,
  Typography,
} from '@mui/material'
import {
  Add as AddIcon,
  Block as BlockIcon,
  Delete as DeleteIcon,
  Edit as EditIcon,
  History as HistoryIcon,
  Policy as PolicyIcon,
} from '@mui/icons-material'
import { policyApi, riskApi } from '../../api/policy'
import { ipApi } from '../../api/ip'
import { errorMessage } from '../../api/client'
import type { Policy, RiskEvent } from '../../api/types'
import { MessageSnackbar } from '../../components/MessageSnackbar'
import { useMessageSnackbar } from '../../hooks/useMessageSnackbar'
import { ConfirmDialog, useConfirmDialog } from '../../components/ConfirmDialog'
import { EmptyState } from '../../components/EmptyState'
import { RowsPerPageControl } from '../../components/RowsPerPageControl'
import { PolicyEditDialog } from './PolicyEditDialog'
import { formatNumber } from '../../utils/format'

/** 策略匹配条件的摘要文案 */
function matchSummary(p: Policy): string {
  const directionMap = { in: '入站', out: '出站', both: '双向' } as const
  const parts: string[] = [directionMap[p.direction] ?? p.direction]
  if (p.protocol !== 'any') {
    parts.push(p.protocol.toUpperCase())
  } else {
    parts.push('任意协议')
  }
  parts.push(p.port === 0 ? '任意端口' : `端口 ${p.port}`)
  return parts.join(' · ')
}

/** 策略触发条件的摘要文案 */
function triggerSummary(p: Policy): string {
  if (p.action === 'rate_limit') {
    return '由内核 hashlimit 直接限速'
  }
  const parts: string[] = []
  if (p.rate_kbps > 0) parts.push(`速率 ≥ ${p.rate_kbps} KB/s`)
  if (p.total_mb > 0) parts.push(`流量 ≥ ${p.total_mb} MB`)
  if (p.conn_rate > 0) parts.push(`连接 ≥ ${p.conn_rate} 次`)
  if (p.distinct_ports > 0) parts.push(`触碰 ${p.distinct_ports} 个端口`)
  parts.push(`窗口 ${p.window_sec}s`)
  return parts.join('，')
}

/** 策略动作的摘要文案 */
function actionSummary(p: Policy): string {
  switch (p.action) {
    case 'rate_limit':
      return `限速 ${p.limit_kbps} KB/s${p.burst_kbps > 0 ? `（突发 ${p.burst_kbps} KB/s）` : ''}`
    case 'ban':
      return `临时封禁 ${p.ban_sec >= 86400 ? `${Math.round(p.ban_sec / 86400)} 天` : `${Math.round(p.ban_sec / 60)} 分钟`}`
    case 'mark':
      return '仅标记风险'
    default:
      return p.action
  }
}

/** 风险升级的摘要文案 */
function riskSummary(p: Policy): string {
  const parts: string[] = []
  if (p.risk_score > 0) {
    parts.push(`触发 +${p.risk_score} 分`)
  }
  if (p.risk_ban_threshold > 0) {
    parts.push(`满 ${p.risk_ban_threshold} 分自动封禁 ${p.risk_ban_sec >= 86400 ? `${Math.round(p.risk_ban_sec / 86400)} 天` : `${Math.round(p.risk_ban_sec / 60)} 分钟`}`)
  }
  return parts.length > 0 ? parts.join('，') : '—'
}

/** 风险事件动作徽标 */
function eventActionChip(action: RiskEvent['action']) {
  switch (action) {
    case 'ban':
      return <Chip label="已封禁" size="small" color="error" />
    case 'auto_ban':
      return <Chip label="风险升级封禁" size="small" color="error" variant="outlined" />
    case 'mark':
      return <Chip label="风险标记" size="small" color="warning" variant="outlined" />
    default:
      return <Chip label={action} size="small" />
  }
}

function PolicyManagement() {
  const [tab, setTab] = useState<'policies' | 'events'>('policies')

  // ---- 策略 ----
  const [policies, setPolicies] = useState<Policy[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [editing, setEditing] = useState<Policy | null>(null)
  const [editorOpen, setEditorOpen] = useState(false)

  // ---- 风险事件 ----
  const [events, setEvents] = useState<RiskEvent[]>([])
  const [eventsTotal, setEventsTotal] = useState(0)
  const [eventsPage, setEventsPage] = useState(0)
  const [eventsPerPage, setEventsPerPage] = useState(20)
  const [eventIpFilter, setEventIpFilter] = useState('')
  const [eventsLoading, setEventsLoading] = useState(false)

  const { snackbar, showMessage, hideMessage } = useMessageSnackbar()
  const { showConfirm, confirmState, handleConfirm, handleCancel } = useConfirmDialog()

  const fetchPolicies = useCallback(async () => {
    setLoading(true)
    setError(null)
    try {
      setPolicies(await policyApi.list())
    } catch (err) {
      setError(errorMessage(err, '获取策略列表失败'))
    } finally {
      setLoading(false)
    }
  }, [])

  const fetchEvents = useCallback(async () => {
    setEventsLoading(true)
    try {
      const result = await riskApi.list({
        ip: eventIpFilter || undefined,
        page: eventsPage + 1,
        page_size: eventsPerPage,
      })
      setEvents(result.items)
      setEventsTotal(result.total)
    } catch (err) {
      console.error('获取风险事件失败:', err)
    } finally {
      setEventsLoading(false)
    }
  }, [eventIpFilter, eventsPage, eventsPerPage])

  useEffect(() => {
    void fetchPolicies()
  }, [fetchPolicies])

  useEffect(() => {
    if (tab === 'events') {
      void fetchEvents()
    }
  }, [tab, fetchEvents])

  const refreshAll = () => {
    void fetchPolicies()
    if (tab === 'events') {
      void fetchEvents()
    }
  }

  // ---- 策略操作 ----
  const openCreate = () => {
    setEditing(null)
    setEditorOpen(true)
  }

  const openEdit = (policy: Policy) => {
    setEditing(policy)
    setEditorOpen(true)
  }

  const toggleEnabled = async (policy: Policy) => {
    try {
      await policyApi.setEnabled(policy.id, !policy.enabled)
      showMessage(`策略「${policy.name}」已${policy.enabled ? '停用' : '启用'}`)
      await fetchPolicies()
    } catch (err) {
      showMessage(errorMessage(err, '操作失败'), 'error')
    }
  }

  const confirmDelete = (policy: Policy) => {
    showConfirm(
      '删除策略',
      `确定删除策略「${policy.name}」吗？其关联的限速规则会一并卸载，历史风险事件保留。`,
      'warning',
      () => {
        void (async () => {
          try {
            await policyApi.remove(policy.id)
            showMessage(`策略「${policy.name}」已删除`)
            await fetchPolicies()
          } catch (err) {
            showMessage(errorMessage(err, '删除失败'), 'error')
          }
        })()
      },
    )
  }

  // ---- 风险事件操作 ----
  const banFromEvent = (event: RiskEvent) => {
    showConfirm(
      '封禁 IP',
      `确定要封禁 ${event.remote_ip} 吗？封禁后将立即阻断该 IP 的入站连接。`,
      'error',
      () => {
        void (async () => {
          try {
            await ipApi.create({ ip_net: event.remote_ip, group_id: 0, action: 'ban' })
            showMessage(`已封禁 ${event.remote_ip}`)
            void fetchEvents()
          } catch (err) {
            showMessage(errorMessage(err, '操作失败'), 'error')
          }
        })()
      },
    )
  }

  const riskyCount = useMemo(
    () => events.filter((e) => e.action !== 'mark').length,
    [events],
  )

  return (
    <Box>
      <Stack direction="row" alignItems="center" justifyContent="space-between" sx={{ mb: 2, flexWrap: 'wrap', gap: 1 }}>
        <Typography variant="h4">策略管理</Typography>
        {tab === 'policies' && (
          <Button variant="contained" startIcon={<AddIcon />} onClick={openCreate}>
            新建策略
          </Button>
        )}
      </Stack>

      {error && (
        <Alert severity="error" sx={{ mb: 2 }} onClose={() => setError(null)}>
          {error}
        </Alert>
      )}

      <Paper sx={{ mb: 2 }}>
        <Tabs value={tab} onChange={(_, value) => setTab(value)} sx={{ px: 2 }}>
          <Tab
            value="policies"
            icon={<PolicyIcon fontSize="small" />}
            iconPosition="start"
            label={`策略 (${policies.length})`}
          />
          <Tab
            value="events"
            icon={<HistoryIcon fontSize="small" />}
            iconPosition="start"
            label={`风险事件${eventsTotal > 0 ? ` (${eventsTotal})` : ''}`}
          />
        </Tabs>
      </Paper>

      {tab === 'policies' ? (
        <Paper>
          <TableContainer>
            <Table size="small">
              <TableHead>
                <TableRow>
                  <TableCell sx={{ fontWeight: 'bold' }}>策略</TableCell>
                  <TableCell sx={{ fontWeight: 'bold' }}>匹配</TableCell>
                  <TableCell sx={{ fontWeight: 'bold', display: { xs: 'none', md: 'table-cell' } }}>触发条件</TableCell>
                  <TableCell sx={{ fontWeight: 'bold' }}>动作</TableCell>
                  <TableCell sx={{ fontWeight: 'bold', display: { xs: 'none', lg: 'table-cell' } }}>风险升级</TableCell>
                  <TableCell sx={{ fontWeight: 'bold' }}>启用</TableCell>
                  <TableCell sx={{ fontWeight: 'bold' }} align="right">
                    操作
                  </TableCell>
                </TableRow>
              </TableHead>
              <TableBody>
                {loading ? (
                  <TableRow>
                    <TableCell colSpan={7} align="center" sx={{ py: 4 }}>
                      加载中...
                    </TableCell>
                  </TableRow>
                ) : policies.length === 0 ? (
                  <TableRow>
                    <TableCell colSpan={7}>
                      <EmptyState
                        icon={<PolicyIcon />}
                        title="暂无策略"
                        description="创建策略后，netbouncer 会按评估周期自动检测并执行限速、临时封禁与风险标记"
                      />
                    </TableCell>
                  </TableRow>
                ) : (
                  policies.map((policy) => (
                    <TableRow key={policy.id} hover sx={{ opacity: policy.enabled ? 1 : 0.55 }}>
                      <TableCell>
                        <Typography variant="body2" sx={{ fontWeight: 600 }}>
                          {policy.name}
                        </Typography>
                      </TableCell>
                      <TableCell>
                        <Typography variant="body2" sx={{ fontFamily: 'monospace' }}>
                          {matchSummary(policy)}
                        </Typography>
                      </TableCell>
                      <TableCell sx={{ display: { xs: 'none', md: 'table-cell' } }}>
                        <Typography variant="body2" color="text.secondary">
                          {triggerSummary(policy)}
                        </Typography>
                      </TableCell>
                      <TableCell>
                        <Chip
                          label={actionSummary(policy)}
                          size="small"
                          color={policy.action === 'ban' || policy.action === 'rate_limit' ? 'primary' : 'warning'}
                          variant="outlined"
                        />
                      </TableCell>
                      <TableCell sx={{ display: { xs: 'none', lg: 'table-cell' } }}>
                        <Typography variant="body2" color="text.secondary">
                          {riskSummary(policy)}
                        </Typography>
                      </TableCell>
                      <TableCell>
                        <Button
                          size="small"
                          variant={policy.enabled ? 'contained' : 'outlined'}
                          color={policy.enabled ? 'success' : 'inherit'}
                          onClick={() => void toggleEnabled(policy)}
                          sx={{ minWidth: 52 }}
                        >
                          {policy.enabled ? '启用' : '停用'}
                        </Button>
                      </TableCell>
                      <TableCell align="right">
                        <Stack direction="row" spacing={0.5} justifyContent="flex-end">
                          <Tooltip title="编辑">
                            <IconButton size="small" onClick={() => openEdit(policy)}>
                              <EditIcon fontSize="small" />
                            </IconButton>
                          </Tooltip>
                          <Tooltip title="删除">
                            <IconButton size="small" color="error" onClick={() => confirmDelete(policy)}>
                              <DeleteIcon fontSize="small" />
                            </IconButton>
                          </Tooltip>
                        </Stack>
                      </TableCell>
                    </TableRow>
                  ))
                )}
              </TableBody>
            </Table>
          </TableContainer>
        </Paper>
      ) : (
        <Paper>
          <Stack direction="row" spacing={2} sx={{ p: 2, flexWrap: 'wrap', gap: 1 }} alignItems="center">
            <TextField
              size="small"
              label="按 IP 过滤"
              placeholder="例如：192.168.1.1"
              value={eventIpFilter}
              onChange={(event) => {
                setEventIpFilter(event.target.value)
                setEventsPage(0)
              }}
              sx={{ width: 220 }}
            />
            <Button size="small" onClick={() => setEventIpFilter('')} disabled={!eventIpFilter}>
              清除过滤
            </Button>
            <Box sx={{ flexGrow: 1 }} />
            <RowsPerPageControl
              value={eventsPerPage}
              onChange={(value) => {
                setEventsPerPage(value)
                setEventsPage(0)
              }}
            />
          </Stack>
          <TableContainer>
            <Table size="small">
              <TableHead>
                <TableRow>
                  <TableCell sx={{ fontWeight: 'bold' }}>时间</TableCell>
                  <TableCell sx={{ fontWeight: 'bold' }}>IP</TableCell>
                  <TableCell sx={{ fontWeight: 'bold' }}>策略</TableCell>
                  <TableCell sx={{ fontWeight: 'bold' }}>触发值</TableCell>
                  <TableCell sx={{ fontWeight: 'bold' }}>动作</TableCell>
                  <TableCell sx={{ fontWeight: 'bold' }} align="right">
                    操作
                  </TableCell>
                </TableRow>
              </TableHead>
              <TableBody>
                {eventsLoading && events.length === 0 ? (
                  <TableRow>
                    <TableCell colSpan={6} align="center" sx={{ py: 4 }}>
                      加载中...
                    </TableCell>
                  </TableRow>
                ) : events.length === 0 ? (
                  <TableRow>
                    <TableCell colSpan={6}>
                      <EmptyState
                        icon={<HistoryIcon />}
                        title={eventIpFilter ? '没有匹配的风险事件' : '暂无风险事件'}
                        description={
                          eventIpFilter
                            ? '试试调整或清除过滤条件'
                            : riskyCount >= 0
                              ? '策略触发或风险升级时，这里会记录对应的事件'
                              : ''
                        }
                      />
                    </TableCell>
                  </TableRow>
                ) : (
                  events.map((event) => (
                    <TableRow key={event.id} hover>
                      <TableCell sx={{ whiteSpace: 'nowrap', color: 'text.secondary', fontSize: '0.8rem' }}>
                        {new Date(event.ts * 1000).toLocaleString('zh-CN', { hour12: false })}
                      </TableCell>
                      <TableCell sx={{ fontFamily: 'monospace' }}>{event.remote_ip}</TableCell>
                      <TableCell>
                        <Typography variant="body2">
                          {event.policy_name}
                          {event.score > 0 && (
                            <Typography component="span" variant="caption" color="warning.main" sx={{ ml: 0.5 }}>
                              +{event.score}
                            </Typography>
                          )}
                        </Typography>
                      </TableCell>
                      <TableCell>
                        <Typography variant="body2" color="text.secondary">
                          {event.trigger_value}
                        </Typography>
                      </TableCell>
                      <TableCell>{eventActionChip(event.action)}</TableCell>
                      <TableCell align="right">
                        <Button
                          variant="outlined"
                          size="small"
                          color="error"
                          startIcon={<BlockIcon />}
                          onClick={() => banFromEvent(event)}
                          sx={{ minWidth: 0, px: 1, whiteSpace: 'nowrap' }}
                        >
                          封禁
                        </Button>
                      </TableCell>
                    </TableRow>
                  ))
                )}
              </TableBody>
            </Table>
          </TableContainer>
          <TablePagination
            component="div"
            count={eventsTotal}
            page={eventsPage}
            onPageChange={(_, newPage) => setEventsPage(newPage)}
            rowsPerPage={eventsPerPage}
            onRowsPerPageChange={() => {}}
            rowsPerPageOptions={[eventsPerPage]}
            labelRowsPerPage="每页显示:"
            labelDisplayedRows={({ from, to, count }) => `${from}-${to} / ${formatNumber(count)}`}
          />
        </Paper>
      )}

      <PolicyEditDialog
        open={editorOpen}
        policy={editing}
        onClose={() => setEditorOpen(false)}
        onMessage={showMessage}
        onSaved={refreshAll}
      />

      <ConfirmDialog confirmState={confirmState} onConfirm={handleConfirm} onCancel={handleCancel} />
      <MessageSnackbar snackbar={snackbar} onClose={hideMessage} />
    </Box>
  )
}

export default PolicyManagement
