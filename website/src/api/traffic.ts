import { buildQuery, get } from './client'
import type {
  PortHistoryPoint,
  PortTopEntry,
  PortTraffic,
  ProtoHistoryPoint,
  TrafficData,
  TrafficHistoryPoint,
  TrafficTopEntry,
 TrafficPage, TrafficOverview, HistoryResult, StorageStatus,
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
  get: (query: { page?: number; page_size?: number; sort?: string; order?: string; remote_ip?: string; local_ip?: string } = {}, signal?: AbortSignal): Promise<TrafficPage> => get<TrafficPage>(`/api/traffic${buildQuery(query)}`, signal),
  overview: (signal?: AbortSignal): Promise<TrafficOverview> => get<TrafficOverview>('/api/traffic/overview', signal),
  detail: (ip: string, signal?: AbortSignal): Promise<TrafficData> => get<TrafficData>(`/api/traffic/ip/${encodeURIComponent(ip)}`, signal),
  storage: (signal?: AbortSignal): Promise<StorageStatus> => get<StorageStatus>('/api/traffic/storage', signal),

  /** 实时端口排行（跨 IP 聚合，由策略引擎评估循环刷新） */
  ports: (signal?: AbortSignal): Promise<PortTraffic[]> => get<PortTraffic[]>('/api/traffic/ports', signal),

  /** 流量历史趋势（不带 ip 为全服汇总） */
  history: (query: TrafficHistoryQuery, signal?: AbortSignal): Promise<HistoryResult<TrafficHistoryPoint>> =>
    get<HistoryResult<TrafficHistoryPoint>>(`/api/traffic/history${buildQuery({ ...query })}`, signal),

  /** 时间范围内流量最大的 IP 榜 */
  historyTop: (query: { start: number; end: number; limit?: number }, signal?: AbortSignal): Promise<HistoryResult<TrafficTopEntry>> =>
    get<HistoryResult<TrafficTopEntry>>(`/api/traffic/history/top${buildQuery({ ...query })}`, signal),

  /** 端口维度历史趋势（按 proto+port 分组的分桶序列） */
  portHistory: (query: PortHistoryQuery, signal?: AbortSignal): Promise<HistoryResult<PortHistoryPoint>> =>
    get<HistoryResult<PortHistoryPoint>>(`/api/traffic/history/ports${buildQuery({ ...query })}`, signal),

  /** 时间范围内流量最大的端口榜 */
  portHistoryTop: (query: PortHistoryQuery & { limit?: number }, signal?: AbortSignal): Promise<HistoryResult<PortTopEntry>> =>
    get<HistoryResult<PortTopEntry>>(`/api/traffic/history/ports/top${buildQuery({ ...query })}`, signal),

  /** 协议维度历史趋势 */
  protoHistory: (query: PortHistoryQuery, signal?: AbortSignal): Promise<HistoryResult<ProtoHistoryPoint>> =>
    get<HistoryResult<ProtoHistoryPoint>>(`/api/traffic/history/protocols${buildQuery({ ...query })}`, signal),
}
