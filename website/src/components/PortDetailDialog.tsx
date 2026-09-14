import { useEffect, useMemo, useState } from 'react'
import {
  Box,
  CircularProgress,
  Dialog,
  DialogContent,
  DialogTitle,
  Divider,
  IconButton,
  Stack,
  ToggleButton,
  ToggleButtonGroup,
  Typography,
} from '@mui/material'
import { Close as CloseIcon } from '@mui/icons-material'
import { trafficApi } from '../api/traffic'
import type { PortHistoryPoint, PortTraffic } from '../api/types'
import { EChart, type EChartsOption } from './charts/EChart'
import { formatBytes, formatBytesPerSec, formatNumber } from '../utils/format'

interface PortDetailDialogProps {
  detail: { proto: string; port: number } | null
  /** 实时端口排行数据（用于展示活跃客户端） */
  live: PortTraffic[]
  onClose: () => void
}

const RANGES = [
  { key: '1h', label: '1小时', seconds: 3600, bucket: 60 },
  { key: '6h', label: '6小时', seconds: 6 * 3600, bucket: 300 },
  { key: '24h', label: '24小时', seconds: 24 * 3600, bucket: 900 },
  { key: '7d', label: '7天', seconds: 7 * 86400, bucket: 3600 },
] as const

type RangeKey = (typeof RANGES)[number]['key']

/** 端口趋势下钻对话框：该端口的进出流量历史 + 当前活跃客户端 */
export const PortDetailDialog = ({ detail, live, onClose }: PortDetailDialogProps) => {
  const [rangeKey, setRangeKey] = useState<RangeKey>('6h')
  const [points, setPoints] = useState<PortHistoryPoint[]>([])
  const [loading, setLoading] = useState(false)

  const range = RANGES.find((r) => r.key === rangeKey) ?? RANGES[1]

  useEffect(() => {
    if (!detail) {
      return
    }
    setPoints([])
    let cancelled = false
    const fetchTrend = async () => {
      setLoading(true)
      try {
        const end = Math.floor(Date.now() / 1000)
        const data = await trafficApi.portHistory({
          start: end - range.seconds,
          end,
          bucket: range.bucket,
          proto: detail.proto,
          port: detail.port,
        })
        if (!cancelled) {
          setPoints(data)
        }
      } catch (err) {
        console.error('获取端口历史失败:', err)
      } finally {
        if (!cancelled) {
          setLoading(false)
        }
      }
    }
    void fetchTrend()
    return () => {
      cancelled = true
    }
  }, [detail, range])

  const liveEntry = useMemo(
    () =>
      detail
        ? live.find((p) => p.proto === detail.proto && p.port === detail.port) ?? null
        : null,
    [detail, live],
  )

  // 聚合相同时间桶的多协议序列（理论上单端口过滤后只有一个协议）
  const chartOption = useMemo<EChartsOption>(() => {
    const byTs = new Map<number, { in: number; out: number }>()
    for (const p of points) {
      const cur = byTs.get(p.ts) ?? { in: 0, out: 0 }
      cur.in += p.bytes_in
      cur.out += p.bytes_out
      byTs.set(p.ts, cur)
    }
    const sorted = [...byTs.entries()].sort((a, b) => a[0] - b[0])
    return {
      backgroundColor: 'transparent',
      tooltip: {
        trigger: 'axis' as const,
        valueFormatter: (value: unknown) => formatBytes(Number(value)),
      },
      legend: { bottom: 0 },
      grid: { left: 8, right: 8, top: 16, bottom: 40, containLabel: true },
      xAxis: {
        type: 'time' as const,
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
          data: sorted.map(([ts, v]) => [ts * 1000, v.in]),
          smooth: true,
          showSymbol: false,
          areaStyle: { opacity: 0.25 },
        },
        {
          name: '上行',
          type: 'line' as const,
          data: sorted.map(([ts, v]) => [ts * 1000, v.out]),
          smooth: true,
          showSymbol: false,
          areaStyle: { opacity: 0.25 },
        },
      ],
    }
  }, [points])

  if (!detail) {
    return null
  }

  const title = detail.port === 0 ? `${detail.proto}/其他端口` : `${detail.proto}/${detail.port}`

  return (
    <Dialog open onClose={onClose} maxWidth="sm" fullWidth>
      <DialogTitle sx={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
        <Typography component="span" sx={{ fontWeight: 600, fontFamily: 'monospace' }}>
          端口 {title}
        </Typography>
        <IconButton size="small" onClick={onClose}>
          <CloseIcon fontSize="small" />
        </IconButton>
      </DialogTitle>
      <DialogContent dividers>
        {liveEntry && (
          <Stack direction="row" spacing={3} flexWrap="wrap" useFlexGap sx={{ mb: 1.5 }}>
            <Stat label="下行速率" value={`${formatBytesPerSec(liveEntry.bytes_in_per_sec)}`} />
            <Stat label="上行速率" value={`${formatBytesPerSec(liveEntry.bytes_out_per_sec)}`} />
            <Stat label="活跃 IP" value={formatNumber(liveEntry.ip_count)} />
            <Stat label="新连接" value={formatNumber(liveEntry.new_conns)} />
          </Stack>
        )}
        {liveEntry && liveEntry.top_clients.length > 0 && (
          <Typography variant="caption" color="text.secondary" sx={{ display: 'block', mb: 1 }}>
            主要客户端：{liveEntry.top_clients.join('、')}
          </Typography>
        )}
        <Divider sx={{ mb: 1.5 }} />
        <Stack direction="row" alignItems="center" justifyContent="space-between" sx={{ mb: 1 }}>
          <Typography variant="subtitle2">流量趋势</Typography>
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
        {loading ? (
          <Box sx={{ display: 'flex', justifyContent: 'center', py: 6 }}>
            <CircularProgress size={28} />
          </Box>
        ) : points.length === 0 ? (
          <Typography variant="body2" color="text.secondary" sx={{ py: 6, textAlign: 'center' }}>
            该时间段内暂无历史数据（历史采样需服务开启流量历史持久化）
          </Typography>
        ) : (
          <EChart option={chartOption} height={260} />
        )}
      </DialogContent>
    </Dialog>
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
