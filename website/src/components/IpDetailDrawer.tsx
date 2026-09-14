import { useCallback, useEffect, useMemo, useState } from 'react'
import {
  Box,
  Button,
  Chip,
  CircularProgress,
  Divider,
  Drawer,
  FormControl,
  InputLabel,
  MenuItem,
  Select,
  Stack,
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableRow,
  ToggleButton,
  ToggleButtonGroup,
  Tooltip,
  Typography,
} from '@mui/material'
import {
  Block as BanIcon,
  Undo as UndoIcon,
  VerifiedUser as AllowIcon,
} from '@mui/icons-material'
import { DonutSmall as ProtoIcon } from '@mui/icons-material'
import { ipApi } from '../api/ip'
import { groupApi } from '../api/group'
import { trafficApi } from '../api/traffic'
import { riskApi } from '../api/policy'
import { errorMessage } from '../api/client'
import type {
  IpGroup,
  IpNetAction,
  PortStat,
  RiskEvent,
  TrafficData,
  TrafficHistoryPoint,
} from '../api/types'
import { EChart } from './charts/EChart'
import { ConfirmDialog, useConfirmDialog } from './ConfirmDialog'
import { formatBytes, formatNumber, formatTimestamp } from '../utils/format'

interface IpDetailDrawerProps {
  ip: string | null
  /** 该 IP 的实时数据（来自流量列表，可能为 null） */
  live: TrafficData | null
  onClose: () => void
  onMessage: (message: string, severity?: 'success' | 'error' | 'warning' | 'info') => void
  /** 规则变更后由父组件刷新列表 */
  onChanged: () => void
}

const RANGES = [
  { key: '6h', label: '6小时', seconds: 6 * 3600, bucket: 300 },
  { key: '24h', label: '24小时', seconds: 24 * 3600, bucket: 900 },
  { key: '7d', label: '7天', seconds: 7 * 86400, bucket: 3600 },
  { key: '30d', label: '30天', seconds: 30 * 86400, bucket: 14400 },
] as const

type RangeKey = (typeof RANGES)[number]['key']

/** 单 IP 详情抽屉：历史趋势 + 黑白名单规则管理 */
export const IpDetailDrawer = ({ ip, live, onClose, onMessage, onChanged }: IpDetailDrawerProps) => {
  const [rangeKey, setRangeKey] = useState<RangeKey>('24h')
  const [points, setPoints] = useState<TrafficHistoryPoint[]>([])
  const [historyLoading, setHistoryLoading] = useState(false)
  const [groups, setGroups] = useState<IpGroup[]>([])
  const [groupId, setGroupId] = useState<number | ''>('')
  const [acting, setActing] = useState(false)
  const [riskEvents, setRiskEvents] = useState<RiskEvent[]>([])

  const { showConfirm, confirmState, handleConfirm, handleCancel } = useConfirmDialog()

  const range = RANGES.find((r) => r.key === rangeKey) ?? RANGES[1]

  const fetchHistory = useCallback(async () => {
    if (!ip) {
      return
    }
    setHistoryLoading(true)
    try {
      const end = Date.now() / 1000
      const data = await trafficApi.history({
        start: Math.floor(end - range.seconds),
        end: Math.floor(end),
        bucket: range.bucket,
        ip,
      })
      setPoints(data)
    } catch (err) {
      console.error('获取历史失败:', err)
    } finally {
      setHistoryLoading(false)
    }
  }, [ip, range])

  useEffect(() => {
    if (!ip) {
      return
    }
    setPoints([])
    void fetchHistory()
  }, [ip, fetchHistory])

  useEffect(() => {
    if (!ip) {
      return
    }
    setRiskEvents([])
    groupApi
      .list()
      .then((list) => {
        setGroups(list)
        setGroupId((list.find((g) => g.is_default) ?? list[0])?.id ?? '')
      })
      .catch(() => setGroups([]))
    riskApi
      .list({ ip, page_size: 20 })
      .then((result) => setRiskEvents(result.items))
      .catch(() => setRiskEvents([]))
  }, [ip])

  const chartOption = useMemo(() => {
    const xData = points.map((p) => p.ts * 1000)
    return {
      backgroundColor: 'transparent',
      tooltip: {
        trigger: 'axis' as const,
        valueFormatter: (value: number) => formatBytes(value),
      },
      legend: { bottom: 0 },
      grid: { left: 8, right: 8, top: 16, bottom: 40, containLabel: true },
      xAxis: {
        type: 'time' as const,
        data: xData,
        axisLabel: { hideOverlap: true },
      },
      yAxis: {
        type: 'value' as const,
        axisLabel: { formatter: (v: number) => formatBytes(v) },
        splitLine: { lineStyle: { opacity: 0.3 } },
      },
      series: [
        {
          name: '下行',
          type: 'line' as const,
          data: points.map((p) => [p.ts * 1000, p.bytes_in]),
          smooth: true,
          showSymbol: false,
          areaStyle: { opacity: 0.25 },
        },
        {
          name: '上行',
          type: 'line' as const,
          data: points.map((p) => [p.ts * 1000, p.bytes_out]),
          smooth: true,
          showSymbol: false,
          areaStyle: { opacity: 0.25 },
        },
      ],
    }
  }, [points])

  const applyAction = async (action: IpNetAction) => {
    if (!ip) {
      return
    }
    setActing(true)
    try {
      await ipApi.create({ ip_net: ip, group_id: groupId === '' ? 0 : groupId, action })
      onMessage(action === 'ban' ? `已封禁 ${ip}` : `已加白 ${ip}`)
      onChanged()
    } catch (err) {
      onMessage(errorMessage(err, '操作失败'), 'error')
    } finally {
      setActing(false)
    }
  }

  const unban = () => {
    if (!ip || !live?.rule_id) {
      return
    }
    const ruleId = live.rule_id
    showConfirm(
      '确认解除规则',
      `确定要解除 ${ip} 的${live.rule_action === 'ban' ? '封禁' : '加白'}规则吗？`,
      'warning',
      () => {
        void (async () => {
          try {
            await ipApi.remove(ruleId)
            onMessage(`已解除 ${ip} 的规则`)
            onChanged()
          } catch (err) {
            onMessage(errorMessage(err, '操作失败'), 'error')
          }
        })()
      },
    )
  }

  const statusChip = () => {
    if (live?.rule_action === 'ban') {
      return live.banned_until ? (
        <Tooltip title={`临时封禁，${formatTimestamp(live.banned_until)} 自动解封`}>
          <Chip label="临时封禁" color="error" size="small" />
        </Tooltip>
      ) : (
        <Chip label="已封禁" color="error" size="small" />
      )
    }
    if (live?.rule_action === 'allow') {
      return <Chip label="已加白" color="success" size="small" />
    }
    if (live?.is_banned) {
      return (
        <Tooltip title="该 IP 命中了某条网段（CIDR）规则">
          <Chip label="被网段规则覆盖" color="warning" size="small" />
        </Tooltip>
      )
    }
    return <Chip label="未管控" size="small" />
  }

  const riskLabel = () => {
    const score = live?.risk_score ?? 0
    const level = live?.risk_level ?? 'none'
    if (score <= 0 || level === 'none') {
      return null
    }
    const map = {
      low: { label: `低风险 ${score}`, color: 'warning' as const },
      medium: { label: `中风险 ${score}`, color: 'warning' as const },
      high: { label: `高风险 ${score}`, color: 'error' as const },
    }
    const conf = map[level] ?? map.low
    return <Chip label={conf.label} color={conf.color} size="small" variant="outlined" />
  }

  return (
    <Drawer anchor="right" open={Boolean(ip)} onClose={onClose}>
      <Box sx={{ width: { xs: '100vw', sm: 480 }, p: 3 }} role="presentation">
        <Stack direction="row" alignItems="center" justifyContent="space-between" sx={{ mb: 1 }}>
          <Typography variant="h6" sx={{ fontFamily: 'monospace' }}>
            {ip}
          </Typography>
          <Stack direction="row" spacing={0.5} alignItems="center">
            {riskLabel()}
            {statusChip()}
          </Stack>
        </Stack>
        {live && (
          <Typography variant="body2" color="text.secondary" gutterBottom>
            本地地址 {live.local_ip} · 首次发现 {formatTimestamp(live.first_seen)} · 最后活动{' '}
            {formatTimestamp(live.last_seen)}
          </Typography>
        )}

        <Divider sx={{ my: 2 }} />

        <Typography variant="subtitle2" gutterBottom>
          实时统计
        </Typography>
        <Box sx={{ display: 'flex', gap: 3, flexWrap: 'wrap', mb: 1 }}>
          <Stat label="下行速率" value={live ? `${formatBytes(live.bytes_in_per_sec)}/s` : '-'} />
          <Stat label="上行速率" value={live ? `${formatBytes(live.bytes_out_per_sec)}/s` : '-'} />
          <Stat label="累计收发" value={live ? formatBytes(live.total_bytes_in + live.total_bytes_out) : '-'} />
          <Stat label="连接数" value={live ? formatNumber(live.connections) : '-'} />
        </Box>

        {(live?.protocols?.length ?? 0) > 0 && (
          <>
            <Typography variant="caption" color="text.secondary" gutterBottom sx={{ display: 'block' }}>
              协议分布（累计流量）
            </Typography>
            <Stack direction="row" spacing={0.75} flexWrap="wrap" useFlexGap sx={{ mb: 1 }}>
              {(live?.protocols ?? []).map((p) => (
                <Tooltip key={p.proto} title={`${formatBytes(p.bytes_in)} ↓ / ${formatBytes(p.bytes_out)} ↑`}>
                  <Chip
                    label={`${p.proto} ${formatBytes(p.bytes_in + p.bytes_out)}`}
                    size="small"
                    variant="outlined"
                    icon={<ProtoIcon sx={{ fontSize: 13 }} />}
                  />
                </Tooltip>
              ))}
            </Stack>
          </>
        )}

        {(live?.ports?.length ?? 0) > 0 && (
          <>
            <Typography variant="caption" color="text.secondary" gutterBottom sx={{ display: 'block' }}>
              端口明细（累计流量前 8）
            </Typography>
            <Box sx={{ maxHeight: 150, overflow: 'auto', mb: 1 }}>
              <PortTable ports={live?.ports ?? []} />
            </Box>
          </>
        )}

        <Divider sx={{ my: 2 }} />

        <Stack direction="row" alignItems="center" justifyContent="space-between" sx={{ mb: 1 }}>
          <Typography variant="subtitle2">历史趋势</Typography>
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
        {historyLoading ? (
          <Box sx={{ display: 'flex', justifyContent: 'center', py: 6 }}>
            <CircularProgress size={28} />
          </Box>
        ) : points.length === 0 ? (
          <Typography variant="body2" color="text.secondary" sx={{ py: 6, textAlign: 'center' }}>
            该时间段内暂无历史数据（采样可能尚未覆盖）
          </Typography>
        ) : (
          <EChart option={chartOption} height={240} />
        )}

        <Divider sx={{ my: 2 }} />

        <Typography variant="subtitle2" gutterBottom>
          风险事件
          <Typography component="span" variant="caption" color="text.secondary" sx={{ ml: 1 }}>
            最近 20 条
          </Typography>
        </Typography>
        {riskEvents.length === 0 ? (
          <Typography variant="body2" color="text.secondary" sx={{ pb: 1 }}>
            该 IP 暂无风险事件
          </Typography>
        ) : (
          <Box sx={{ maxHeight: 200, overflow: 'auto', mb: 1 }}>
            <Stack spacing={0.75}>
              {riskEvents.map((event) => (
                <Stack key={event.id} direction="row" spacing={1} alignItems="baseline" flexWrap="wrap" useFlexGap>
                  <Typography variant="caption" color="text.secondary" sx={{ whiteSpace: 'nowrap' }}>
                    {formatTimestamp(new Date(event.ts * 1000).toISOString())}
                  </Typography>
                  <Chip
                    label={
                      event.action === 'auto_ban'
                        ? '风险升级封禁'
                        : event.action === 'ban'
                          ? '触发封禁'
                          : '风险标记'
                    }
                    size="small"
                    color={event.action === 'mark' ? 'warning' : 'error'}
                    variant={event.action === 'mark' ? 'outlined' : 'filled'}
                    sx={{ height: 20, fontSize: 11 }}
                  />
                  <Typography variant="caption" sx={{ fontFamily: 'monospace' }}>
                    {event.policy_name}
                  </Typography>
                  <Typography variant="caption" color="text.secondary">
                    {event.trigger_value}
                  </Typography>
                </Stack>
              ))}
            </Stack>
          </Box>
        )}

        <Divider sx={{ my: 2 }} />

        <Typography variant="subtitle2" gutterBottom>
          规则管理
        </Typography>
        <Stack direction="row" spacing={2} alignItems="center" sx={{ mb: 2 }}>
          <FormControl size="small" sx={{ minWidth: 160 }}>
            <InputLabel>目标组</InputLabel>
            <Select
              value={groupId}
              label="目标组"
              onChange={(event) => setGroupId(Number(event.target.value))}
            >
              {groups.map((group) => (
                <MenuItem key={group.id} value={group.id}>
                  {group.name}
                </MenuItem>
              ))}
            </Select>
          </FormControl>
          <Typography variant="body2" color="text.secondary">
            对下方操作生效
          </Typography>
        </Stack>
        <Stack direction="row" spacing={1.5} flexWrap="wrap" useFlexGap>
          {live?.rule_action === 'ban' ? (
            <>
              <Button
                variant="outlined"
                color="warning"
                startIcon={<UndoIcon />}
                onClick={unban}
                disabled={acting || !live.rule_id}
              >
                解除封禁
              </Button>
              <Button
                variant="outlined"
                color="success"
                startIcon={<AllowIcon />}
                onClick={() => void applyAction('allow')}
                disabled={acting}
              >
                改为加白
              </Button>
            </>
          ) : live?.rule_action === 'allow' ? (
            <>
              <Button
                variant="outlined"
                color="warning"
                startIcon={<UndoIcon />}
                onClick={unban}
                disabled={acting || !live.rule_id}
              >
                解除加白
              </Button>
              <Button
                variant="outlined"
                color="error"
                startIcon={<BanIcon />}
                onClick={() => void applyAction('ban')}
                disabled={acting}
              >
                改为封禁
              </Button>
            </>
          ) : (
            <>
              <Button
                variant="contained"
                color="error"
                startIcon={<BanIcon />}
                onClick={() => void applyAction('ban')}
                disabled={acting}
              >
                封禁
              </Button>
              <Button
                variant="contained"
                color="success"
                startIcon={<AllowIcon />}
                onClick={() => void applyAction('allow')}
                disabled={acting}
              >
                加白
              </Button>
            </>
          )}
        </Stack>
      </Box>
      <ConfirmDialog confirmState={confirmState} onConfirm={handleConfirm} onCancel={handleCancel} />
    </Drawer>
  )
}

const Stat = ({ label, value }: { label: string; value: string }) => (
  <Box>
    <Typography variant="caption" color="text.secondary" display="block">
      {label}
    </Typography>
    <Typography variant="body2" sx={{ fontWeight: 600 }}>
      {value}
    </Typography>
  </Box>
)

const PortTable = ({ ports }: { ports: PortStat[] }) => (
  <Table size="small">
    <TableHead>
      <TableRow>
        <TableCell sx={{ py: 0.25, fontSize: 11 }}>协议</TableCell>
        <TableCell sx={{ py: 0.25, fontSize: 11 }}>端口</TableCell>
        <TableCell sx={{ py: 0.25, fontSize: 11 }} align="right">下行</TableCell>
        <TableCell sx={{ py: 0.25, fontSize: 11 }} align="right">上行</TableCell>
        <TableCell sx={{ py: 0.25, fontSize: 11 }} align="right">连接</TableCell>
      </TableRow>
    </TableHead>
    <TableBody>
      {ports.map((p) => (
        <TableRow key={`${p.proto}-${p.port}`}>
          <TableCell sx={{ py: 0.25, fontSize: 12 }}>{p.proto}</TableCell>
          <TableCell sx={{ py: 0.25, fontSize: 12, fontFamily: 'monospace' }}>
            {p.port === 0 ? '其他' : p.port}
          </TableCell>
          <TableCell sx={{ py: 0.25, fontSize: 12 }} align="right">{formatBytes(p.bytes_in)}</TableCell>
          <TableCell sx={{ py: 0.25, fontSize: 12 }} align="right">{formatBytes(p.bytes_out)}</TableCell>
          <TableCell sx={{ py: 0.25, fontSize: 12 }} align="right">{formatNumber(p.conns)}</TableCell>
        </TableRow>
      ))}
    </TableBody>
  </Table>
)
