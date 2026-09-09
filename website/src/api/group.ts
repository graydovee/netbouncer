import { del, get, post, put } from './client'
import type { GroupPayload, IpGroup } from './types'

export const groupApi = {
  /** 获取所有组（含组内IP数量） */
  list: (): Promise<IpGroup[]> => get<IpGroup[]>('/api/group'),

  /** 创建组 */
  create: (payload: GroupPayload): Promise<IpGroup> => post<IpGroup>('/api/group', payload),

  /** 更新组信息 */
  update: (id: number, payload: GroupPayload): Promise<IpGroup> =>
    put<IpGroup>('/api/group', { id, ...payload }),

  /** 删除组（组内IP自动归入默认组） */
  remove: (id: number): Promise<void> => del<void>(`/api/group/${id}`),
}
