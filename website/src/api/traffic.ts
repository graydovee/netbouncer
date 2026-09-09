import { get } from './client'
import type { TrafficData } from './types'

export const trafficApi = {
  /** 获取（排除网段后的）实时流量统计 */
  get: (): Promise<TrafficData[]> => get<TrafficData[]>('/api/traffic'),
}
