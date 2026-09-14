import { buildQuery, get } from './client'
import type {
  PortHistoryPoint,
  PortTopEntry,
  PortTraffic,
  ProtoHistoryPoint,
  TrafficData,
  TrafficHistoryPoint,
  TrafficTopEntry,
} from './types'

export interface TrafficHistoryQuery {
  /** unix 秒 */
  start: number
  /** unix 秒 */
  end: number
  /** 聚合桶宽（秒） */
  bucket: number
  /** 可选：仅查询单个 IP 的曲线 */
  ip?: string
}

/** 端口/协议维度历史查询参数（port 传 -1 表示不过滤） */
export interface PortHistoryQuery {
  start: number
  end: number
  bucket: number
  ip?: string
  proto?: string
  port?: number
}

export const trafficApi = {
  /** 获取（排除网段后的）实时流量统计 */
  get: (): Promise<TrafficData[]> => get<TrafficData[]>('/api/traffic'),

  /** 实时端口排行（跨 IP 聚合，由策略引擎评估循环刷新） */
  ports: (): Promise<PortTraffic[]> => get<PortTraffic[]>('/api/traffic/ports'),

  /** 流量历史趋势（不带 ip 为全服汇总） */
  history: (query: TrafficHistoryQuery): Promise<TrafficHistoryPoint[]> =>
    get<TrafficHistoryPoint[]>(`/api/traffic/history${buildQuery({ ...query })}`),

  /** 时间范围内流量最大的 IP 榜 */
  historyTop: (query: { start: number; end: number; limit?: number }): Promise<TrafficTopEntry[]> =>
    get<TrafficTopEntry[]>(`/api/traffic/history/top${buildQuery({ ...query })}`),

  /** 端口维度历史趋势（按 proto+port 分组的分桶序列） */
  portHistory: (query: PortHistoryQuery): Promise<PortHistoryPoint[]> =>
    get<PortHistoryPoint[]>(`/api/traffic/history/ports${buildQuery({ ...query })}`),

  /** 时间范围内流量最大的端口榜 */
  portHistoryTop: (query: PortHistoryQuery & { limit?: number }): Promise<PortTopEntry[]> =>
    get<PortTopEntry[]>(`/api/traffic/history/ports/top${buildQuery({ ...query })}`),

  /** 协议维度历史趋势 */
  protoHistory: (query: PortHistoryQuery): Promise<ProtoHistoryPoint[]> =>
    get<ProtoHistoryPoint[]>(`/api/traffic/history/protocols${buildQuery({ ...query })}`),
}
