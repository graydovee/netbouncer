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
  /** 临时封禁到期时间（RFC3339），空 = 永久封禁或未封禁 */
  banned_until: string
  /** 风险分（统计窗口内策略触发累加） */
  risk_score: number
  /** 风险等级: none|low|medium|high */
  risk_level: 'none' | 'low' | 'medium' | 'high'
  /** 协议维度累计统计 */
  protocols: ProtoStat[]
  /** 端口维度累计统计（按流量取前若干条） */
  ports: PortStat[]
}

/** 单协议的累计流量（对应 service.ProtoStat） */
export interface ProtoStat {
  proto: string
  bytes_in: number
  bytes_out: number
  packets_in: number
  packets_out: number
}

/** 单协议+端口的累计流量（对应 service.PortStat） */
export interface PortStat {
  proto: string
  /** 0 表示"其他/未分类" */
  port: number
  bytes_in: number
  bytes_out: number
  packets_in: number
  packets_out: number
  /** 该端口累计新建连接数 */
  conns: number
}

/** 实时端口排行条目（对应 service.PortTraffic） */
export interface PortTraffic {
  proto: string
  port: number
  bytes_in: number
  bytes_out: number
  bytes_in_per_sec: number
  bytes_out_per_sec: number
  ip_count: number
  new_conns: number
  top_clients: string[]
}

/** 端口维度历史聚合点（对应 store.PortHistoryPoint） */
export interface PortHistoryPoint {
  ts: number
  proto: string
  port: number
  bytes_in: number
  bytes_out: number
  packets_in: number
  packets_out: number
}

/** 协议维度历史聚合点（对应 store.ProtoHistoryPoint） */
export interface ProtoHistoryPoint {
  ts: number
  proto: string
  bytes_in: number
  bytes_out: number
  packets_in: number
  packets_out: number
}

/** 端口维度历史 Top 榜条目（对应 store.PortTopEntry） */
export interface PortTopEntry {
  proto: string
  port: number
  bytes_in: number
  bytes_out: number
  bytes_sum: number
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
  /** ban 生效方向: in/out/both */
  direction: 'in' | 'out' | 'both'
  /** 临时封禁到期时间（RFC3339），空 = 永久 */
  expires_at: string
  /** 规则来源: manual / policy:<id> */
  source: string
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

/** 策略（对应 service.Policy） */
export interface Policy {
  id: number
  name: string
  enabled: boolean
  created_at: string
  updated_at: string
  /** 匹配条件 */
  direction: 'in' | 'out' | 'both'
  protocol: 'any' | 'tcp' | 'udp'
  /** 0 = 任意端口 */
  port: number
  /** 触发条件（0 = 不启用） */
  rate_kbps: number
  total_mb: number
  conn_rate: number
  distinct_ports: number
  window_sec: number
  /** 动作: rate_limit|ban|mark */
  action: 'rate_limit' | 'ban' | 'mark'
  limit_kbps: number
  burst_kbps: number
  ban_sec: number
  risk_score: number
  /** 风险升级 */
  risk_ban_threshold: number
  risk_ban_sec: number
  cooldown_sec: number
}

/** 策略创建/更新请求 */
export type PolicyPayload = Omit<Policy, 'id' | 'created_at' | 'updated_at'>

/** 风险事件（对应 service.RiskEvent） */
export interface RiskEvent {
  id: number
  remote_ip: string
  /** unix 秒 */
  ts: number
  policy_id: number
  policy_name: string
  /** 触发时的观测值描述 */
  trigger_value: string
  /** mark|ban|auto_ban */
  action: 'mark' | 'ban' | 'auto_ban'
  score: number
}

/** 风险事件分页列表（对应 service.RiskEventListResult） */
export interface RiskEventListResult {
  items: RiskEvent[]
  total: number
}
