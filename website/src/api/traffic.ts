import { buildQuery, get } from './client'
import type { TrafficData, TrafficHistoryPoint, TrafficTopEntry } from './types'

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

export const trafficApi = {
  /** 获取（排除网段后的）实时流量统计 */
  get: (): Promise<TrafficData[]> => get<TrafficData[]>('/api/traffic'),

  /** 流量历史趋势（不带 ip 为全服汇总） */
  history: (query: TrafficHistoryQuery): Promise<TrafficHistoryPoint[]> =>
    get<TrafficHistoryPoint[]>(`/api/traffic/history${buildQuery({ ...query })}`),

  /** 时间范围内流量最大的 IP 榜 */
  historyTop: (query: { start: number; end: number; limit?: number }): Promise<TrafficTopEntry[]> =>
    get<TrafficTopEntry[]>(`/api/traffic/history/top${buildQuery({ ...query })}`),
}
