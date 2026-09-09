// 与后端 pkg/service/proto.go、pkg/web/proto.go 对齐的接口类型定义

export type IpNetAction = 'ban' | 'allow'

/** 统一响应包络 */
export interface ApiResponse<T> {
  code: number
  message: string
  data: T
}

/** 认证用户信息（对应 web.UserInfo） */
export interface UserInfo {
  sub: string
  email: string
  name: string
  picture: string
}

/** /auth/status 响应数据 */
export interface AuthStatusData {
  enabled: boolean
  type?: 'basic' | 'oidc'
  loggedIn?: boolean
  user?: UserInfo
}

/** 登录成功响应数据 */
export interface LoginResult {
  message: string
  user: UserInfo
}

/** 流量统计数据（对应 service.TrafficData） */
export interface TrafficData {
  remote_ip: string
  local_ip: string
  total_bytes_in: number
  total_bytes_out: number
  total_packets_in: number
  total_packets_out: number
  bytes_in_per_sec: number
  bytes_out_per_sec: number
  connections: number
  first_seen: string
  last_seen: string
  is_banned: boolean
  /** 精确命中该 IP 的规则动作：'' / ban / allow（被网段规则覆盖时为空） */
  rule_action: IpNetAction | ''
  /** 精确命中规则的 ID，0 表示无精确规则 */
  rule_id: number
}

/** 流量历史聚合点（对应 store.HistoryPoint） */
export interface TrafficHistoryPoint {
  ts: number
  bytes_in: number
  bytes_out: number
  packets_in: number
  packets_out: number
}

/** 流量历史 Top 榜条目（对应 store.TopEntry） */
export interface TrafficTopEntry {
  ip: string
  bytes_in: number
  bytes_out: number
  bytes_sum: number
  last_seen: number
}

/** IP组（对应 service.IpGroup） */
export interface IpGroup {
  id: number
  name: string
  description: string
  created_at: string
  updated_at: string
  is_default: boolean
  ip_count: number
}

/** IP规则（对应 service.IpNet） */
export interface IpNet {
  id: number
  ip_net: string
  created_at: string
  updated_at: string
  group: IpGroup | null
  action: IpNetAction
}

/** IP规则分页列表（对应 service.IpNetListResult） */
export interface IpNetListResult {
  items: IpNet[]
  total: number
}

/** IP规则列表查询参数 */
export interface IpNetListQuery {
  page?: number
  page_size?: number
  group_id?: number
  action?: IpNetAction | ''
  search?: string
}

/** 批量操作/导入结果 */
export interface BatchResult {
  success_count: number
  failed_count: number
}

/** 创建IP规则请求 */
export interface CreateIpNetRequest {
  ip_net: string
  group_id?: number
  action: IpNetAction
}

/** 导入IP规则请求 */
export interface ImportIpNetRequest {
  text?: string
  url?: string
  group_id: number
  action: IpNetAction
}

/** 创建/更新组请求 */
export interface GroupPayload {
  name: string
  description: string
}
