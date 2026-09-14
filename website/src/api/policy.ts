import { buildQuery, del, get, post, put } from './client'
import type { Policy, PolicyPayload, RiskEventListResult } from './types'

export const policyApi = {
  /** 获取全部策略 */
  list: (): Promise<Policy[]> => get<Policy[]>('/api/policy'),

  /** 创建策略 */
  create: (payload: PolicyPayload): Promise<void> => post<void>('/api/policy', payload),

  /** 更新策略 */
  update: (id: number, payload: PolicyPayload): Promise<void> => put<void>(`/api/policy/${id}`, payload),

  /** 删除策略 */
  remove: (id: number): Promise<void> => del<void>(`/api/policy/${id}`),

  /** 启用/停用策略 */
  setEnabled: (id: number, enabled: boolean): Promise<void> =>
    put<void>(`/api/policy/${id}/enabled`, { enabled }),
}

export const riskApi = {
  /** 分页查询风险事件 */
  list: (query: { ip?: string; page?: number; page_size?: number } = {}): Promise<RiskEventListResult> =>
    get<RiskEventListResult>(`/api/risk/events${buildQuery({ ...query })}`),
}
