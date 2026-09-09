# NetBouncer API 文档

## 概述

NetBouncer 提供了 RESTful API 接口，用于管理网络流量监控、IP 规则和分组。所有 API 都返回 JSON 格式的响应。

## 响应格式

所有 API 响应都遵循以下格式：

```json
{
  "code": 200,
  "message": "success",
  "data": {}
}
```

- `code`: 业务状态码，与 HTTP 状态码一致（200 表示成功）
- `message`: 响应消息
- `data`: 响应数据

## HTTP 状态码

错误通过标准 HTTP 状态码返回，与 body 中的 `code` 一致：

| 状态码 | 含义 |
|--------|------|
| 200 | 成功 |
| 400 | 请求参数错误 |
| 401 | 未认证（需要登录） |
| 404 | 资源不存在 |
| 409 | 资源冲突（如组名重复） |
| 500 | 服务器内部错误 |
| 502 | URL 导入时上游地址不可达 |

## 认证

启用认证后（配置 `web.auth.enabled: true`），除 `/auth/*` 外的所有接口都需要先登录。

- **BasicAuth**：`POST /auth/login` 携带用户名密码，成功后通过 `netbouncer_session` Cookie 维持会话；同时兼容 HTTP 标准 `Authorization: Basic ...` 头。
- **OIDC**：浏览器访问 `GET /auth/login` 跳转到 IdP，回调 `GET /auth/callback` 完成登录（PKCE + state）。

认证相关接口：

| 方法 | 路径 | 说明 |
|------|------|------|
| GET/POST | `/auth/login` | BasicAuth 登录（GET 时 OIDC 为跳转入口） |
| GET | `/auth/status` | 查询认证状态与当前用户 |
| GET/POST | `/auth/logout` | 登出 |
| GET | `/auth/callback` | OIDC 回调 |

未认证访问 `/api/*` 时返回 `401`，前端据此自动跳转登录页。

## 流量监控 API

### 获取流量统计

获取（已排除监控排除网段后的）实时流量统计信息。

**请求**
```http
GET /api/traffic
```

**响应**
```json
{
  "code": 200,
  "message": "success",
  "data": [
    {
      "remote_ip": "192.168.1.100",
      "local_ip": "192.168.1.1",
      "total_bytes_in": 1024,
      "total_bytes_out": 2048,
      "total_packets_in": 10,
      "total_packets_out": 20,
      "bytes_in_per_sec": 100.5,
      "bytes_out_per_sec": 200.3,
      "connections": 5,
      "first_seen": "2024-01-01T10:00:00Z",
      "last_seen": "2024-01-01T10:05:00Z",
      "is_banned": false,
      "rule_action": "ban",
      "rule_id": 42
    }
  ]
}
```

**字段说明**
- `remote_ip`: 远程IP地址
- `local_ip`: 本地IP地址
- `total_bytes_in` / `total_bytes_out`: 总接收/发送字节数
- `total_packets_in` / `total_packets_out`: 总接收/发送包数
- `bytes_in_per_sec` / `bytes_out_per_sec`: 每秒接收/发送字节数（滑动窗口）
- `connections`: 当前连接数（按 TCP SYN/FIN 估算）
- `first_seen` / `last_seen`: 首次发现/最后活动时间（ISO 8601）
- `is_banned`: 是否命中封禁规则（含被网段规则覆盖的情况）
- `rule_action`: 精确命中该 IP 的规则动作（`""`/`ban`/`allow`），被网段规则覆盖时为空
- `rule_id`: 精确命中规则的 ID，`0` 表示无精确规则

### 查询流量历史趋势

按时间桶聚合查询历史流量（由采样器按 `monitor.history_interval` 周期写入，存区间增量）。

**请求**
```http
GET /api/traffic/history?start=1735689600&end=1735776000&bucket=900&ip=1.2.3.4
```

**查询参数**

| 参数 | 类型 | 说明 |
|------|------|------|
| `start` | int | 开始时间（unix 秒），默认 24 小时前 |
| `end` | int | 结束时间（unix 秒），默认当前时间 |
| `bucket` | int | 聚合桶宽（秒），范围 [10, 86400]，默认 300 |
| `ip` | string | 可选，仅查询该 IP 的曲线；不传则为全服汇总 |

**响应**
```json
{
  "code": 200,
  "message": "success",
  "data": [
    { "ts": 1735690200, "bytes_in": 1048576, "bytes_out": 262144, "packets_in": 1024, "packets_out": 512 }
  ]
}
```

**字段说明**：`bytes_in/out`、`packets_in/out` 为该桶内的**流量增量**（非累计值）。

### 查询流量 Top 榜

统计时间范围内总流量最大的前 N 个 IP。

**请求**
```http
GET /api/traffic/history/top?start=1735689600&end=1735776000&limit=10
```

**查询参数**：`start`、`end` 同上；`limit` 默认 10，最大 100。

**响应**
```json
{
  "code": 200,
  "message": "success",
  "data": [
    { "ip": "1.2.3.4", "bytes_in": 104857600, "bytes_out": 26214400, "bytes_sum": 131072000, "last_seen": 1735775000 }
  ]
}
```

## IP 管理 API

### 分页获取 IP 规则列表

**请求**
```http
GET /api/ip?page=1&page_size=20&group_id=1&action=ban&search=192.168
```

**查询参数**

| 参数 | 类型 | 说明 |
|------|------|------|
| `page` | int | 页码，从 1 开始，默认 1 |
| `page_size` | int | 每页数量，默认 20，0 表示返回全部 |
| `group_id` | uint | 按组过滤，可选 |
| `action` | string | 按行为过滤（`ban`/`allow`），可选 |
| `search` | string | 按 IP/CIDR 模糊匹配，可选 |

**响应**
```json
{
  "code": 200,
  "message": "success",
  "data": {
    "items": [
      {
        "id": 1,
        "ip_net": "192.168.1.100",
        "created_at": "2024-01-01T10:00:00Z",
        "updated_at": "2024-01-01T10:00:00Z",
        "group": {
          "id": 1,
          "name": "default",
          "description": "系统默认的IP禁用组",
          "created_at": "2024-01-01T10:00:00Z",
          "updated_at": "2024-01-01T10:00:00Z",
          "is_default": true,
          "ip_count": 42
        },
        "action": "ban"
      }
    ],
    "total": 42
  }
}
```

### 创建 IP 规则

创建新规则；如 IP 已存在则仅更新其行为（忽略组信息）。

**请求**
```http
POST /api/ip
Content-Type: application/json
```

**请求体**
```json
{
  "ip_net": "192.168.1.100",
  "group_id": 1,
  "action": "ban"
}
```

**字段说明**
- `ip_net`: IP 地址或 CIDR 网段（必填）
- `group_id`: 组ID，`0` 表示使用默认组
- `action`: 行为类型，`ban`（封禁）或 `allow`（允许）（必填）

**响应**：`200`，`data: null`

**错误**：`400` 参数/格式错误，`404` 组不存在

### 删除 IP 规则

删除规则并撤销对应防火墙条目。

**请求**
```http
DELETE /api/ip/{id}
```

**响应**：`200`，`data: null`；`404` 规则不存在

### 获取可用操作列表

**请求**
```http
GET /api/ip/action
```

**响应**
```json
{
  "code": 200,
  "message": "success",
  "data": ["ban", "allow"]
}
```

### 更新 IP 行为

**请求**
```http
PUT /api/ip/action
Content-Type: application/json
```

**请求体**
```json
{
  "id": 1,
  "action": "ban"
}
```

**响应**：`200`；`404` 规则不存在

### 更新 IP 所属组

**请求**
```http
PUT /api/ip/group
Content-Type: application/json
```

**请求体**
```json
{
  "id": 1,
  "group_id": 2
}
```

**响应**：`200`；`404` 组不存在

### 批量导入规则

从文本或 URL 批量导入 IP/CIDR。URL 方式仅支持 `http/https`，禁止访问内网/回环地址（SSRF 防护），响应超过 100MB 截断。

**请求**
```http
POST /api/ip/import
Content-Type: application/json
```

**请求体**
```json
{
  "text": "192.168.1.1, 10.0.0.0/8",
  "url": "",
  "group_id": 1,
  "action": "ban"
}
```

`text` 与 `url` 至少提供一个；已存在的 IP 仅在其行为与目标不一致时更新。

**响应**
```json
{
  "code": 200,
  "message": "success",
  "data": {
    "success_count": 100,
    "failed_count": 2
  }
}
```

**错误**：`400` 参数错误，`404` 组不存在，`502` URL 拉取失败

### 批量删除

**请求**
```http
POST /api/ip/batch-delete
Content-Type: application/json
```

**请求体**
```json
{
  "ids": [1, 2, 3]
}
```

**响应**
```json
{
  "code": 200,
  "message": "success",
  "data": { "success_count": 3, "failed_count": 0 }
}
```

### 批量更新行为

**请求**
```http
POST /api/ip/batch-action
Content-Type: application/json
```

**请求体**
```json
{
  "ids": [1, 2, 3],
  "action": "allow"
}
```

**响应**：同批量删除。

### 批量更新所属组

**请求**
```http
POST /api/ip/batch-group
Content-Type: application/json
```

**请求体**
```json
{
  "ids": [1, 2, 3],
  "group_id": 2
}
```

**响应**：同批量删除；`404` 组不存在。

## 组管理 API

### 获取所有组列表

**请求**
```http
GET /api/group
```

**响应**
```json
{
  "code": 200,
  "message": "success",
  "data": [
    {
      "id": 1,
      "name": "default",
      "description": "系统默认的IP禁用组",
      "created_at": "2024-01-01T10:00:00Z",
      "updated_at": "2024-01-01T10:00:00Z",
      "is_default": true,
      "ip_count": 42
    }
  ]
}
```

**字段说明**：`ip_count` 为该组下的 IP 规则数量（前端组 Tab 徽标数据来源）。

### 创建新组

**请求**
```http
POST /api/group
Content-Type: application/json
```

**请求体**
```json
{
  "name": "测试组",
  "description": "测试用组"
}
```

**响应**：`200`，`data` 为创建后的组对象；`409` 组名已存在

### 更新组信息

**请求**
```http
PUT /api/group
Content-Type: application/json
```

**请求体**
```json
{
  "id": 1,
  "name": "新组名",
  "description": "新描述"
}
```

**响应**：`200`，`data` 为更新后的组对象；`404` 组不存在

### 删除组

删除指定组，组内 IP 自动归入默认组。默认组不可删除（返回 `400`）。

**请求**
```http
DELETE /api/group/{id}
```

**响应**：`200`，`data: null`；`400` 目标为默认组；`404` 组不存在

## 使用示例

### 使用 curl

```bash
# 获取流量统计
curl -s http://localhost:8080/api/traffic

# 创建IP规则
curl -s -X POST http://localhost:8080/api/ip \
  -H "Content-Type: application/json" \
  -d '{"ip_net": "192.168.1.100", "group_id": 0, "action": "ban"}'

# 分页查询规则
curl -s "http://localhost:8080/api/ip?page=1&page_size=20&search=192.168"

# 批量删除
curl -s -X POST http://localhost:8080/api/ip/batch-delete \
  -H "Content-Type: application/json" \
  -d '{"ids": [1, 2, 3]}'
```

### 使用 JavaScript（与前端 `src/api/client.ts` 一致）

```javascript
const response = await fetch('/api/ip?page=1&page_size=20');
if (!response.ok) {
  // 401 会话过期、404 不存在等统一走 HTTP 状态码
  throw new Error(`请求失败 (HTTP ${response.status})`);
}
const result = await response.json();
console.log(result.data.items, result.data.total);
```

## 注意事项

1. **IP格式**: 支持单个 IP 地址（如 `192.168.1.100`）或 CIDR 网段（如 `192.168.1.0/24`）
2. **行为类型**: 目前支持 `ban`（封禁）和 `allow`（允许）两种行为
3. **组管理**: 删除组时，该组下的所有 IP 会被移动到默认组
4. **时间格式**: 所有时间字段都使用 ISO 8601 格式
5. **权限要求**: 防火墙规则修改需要 root 权限或 `CAP_NET_ADMIN`/`CAP_NET_RAW` 能力
6. **URL 导入安全**: 仅支持公网 http/https 地址，禁止内网/回环目标，超时 30 秒
