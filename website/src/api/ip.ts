import { buildQuery, del, get, post, put } from './client'
import type { BatchResult, CreateIpNetRequest, ImportIpNetRequest, IpNetAction, IpNetListQuery, IpNetListResult } from './types'

export const ipApi = {
  /** 分页获取IP规则列表 */
  list: (query: IpNetListQuery = {}): Promise<IpNetListResult> =>
    get<IpNetListResult>(`/api/ip${buildQuery({ ...query })}`),

  /** 创建（或更新已有IP的）规则 */
  create: (req: CreateIpNetRequest): Promise<void> => post<void>('/api/ip', req),

  /** 删除单条规则 */
  remove: (id: number): Promise<void> => del<void>(`/api/ip/${id}`),

  /** 批量删除规则 */
  batchDelete: (ids: number[]): Promise<BatchResult> =>
    post<BatchResult>('/api/ip/batch-delete', { ids }),

  /** 修改单条规则动作 */
  updateAction: (id: number, action: IpNetAction): Promise<void> =>
    put<void>('/api/ip/action', { id, action }),

  /** 批量修改规则动作 */
  batchUpdateAction: (ids: number[], action: IpNetAction): Promise<BatchResult> =>
    post<BatchResult>('/api/ip/batch-action', { ids, action }),

  /** 修改单条规则所属组 */
  updateGroup: (id: number, groupId: number): Promise<void> =>
    put<void>('/api/ip/group', { id, group_id: groupId }),

  /** 批量修改规则所属组 */
  batchUpdateGroup: (ids: number[], groupId: number): Promise<BatchResult> =>
    post<BatchResult>('/api/ip/batch-group', { ids, group_id: groupId }),

  /** 获取支持的规则动作列表 */
  listActions: (): Promise<string[]> => get<string[]>('/api/ip/action'),

  /** 通过文本或URL批量导入规则 */
  import: (req: ImportIpNetRequest): Promise<BatchResult> =>
    post<BatchResult>('/api/ip/import', req),
}
