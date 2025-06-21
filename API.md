# NetBouncer API 文档

## 概述

NetBouncer 提供了RESTful API接口，用于管理网络流量监控、IP规则和分组。所有API都返回JSON格式的响应。

## 响应格式

所有API响应都遵循以下格式：

```json
{
  "code": 200,
  "message": "success",
  "data": {}
}
```

- `code`: 状态码，200表示成功
- `message`: 响应消息
- `data`: 响应数据

## 流量监控API

### 获取流量统计

获取所有网络连接的流量统计信息。

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
      "is_banned": false
    }
  ]
}
```

**字段说明**
- `remote_ip`: 远程IP地址
- `local_ip`: 本地IP地址
- `total_bytes_in`: 总接收字节数
- `total_bytes_out`: 总发送字节数
- `total_packets_in`: 总接收包数
- `total_packets_out`: 总发送包数
- `bytes_in_per_sec`: 每秒接收字节数
- `bytes_out_per_sec`: 每秒发送字节数
- `connections`: 连接数
- `first_seen`: 首次发现时间（ISO 8601格式）
- `last_seen`: 最后活动时间（ISO 8601格式）
- `is_banned`: 是否被封禁

## IP管理API

### 获取所有IP列表

获取所有IP规则列表。

**请求**
```http
GET /api/ip
```

**响应**
```json
{
  "code": 200,
  "message": "success",
  "data": [
    {
      "id": 1,
      "ip_net": "192.168.1.100",
      "created_at": "2024-01-01T10:00:00Z",
      "updated_at": "2024-01-01T10:00:00Z",
      "group": {
        "id": 1,
        "name": "默认组",
        "description": "默认IP组",
        "created_at": "2024-01-01T10:00:00Z",
        "updated_at": "2024-01-01T10:00:00Z",
        "is_default": true
      },
      "action": "ban"
    }
  ]
}
```

### 根据组ID获取IP列表

获取指定组下的所有IP规则。

**请求**
```http
GET /api/ip/{groupId}
```

**参数**
- `groupId`: 组ID（路径参数）

**响应**
```json
{
  "code": 200,
  "message": "success",
  "data": [
    {
      "id": 1,
      "ip_net": "192.168.1.100",
      "created_at": "2024-01-01T10:00:00Z",
      "updated_at": "2024-01-01T10:00:00Z",
      "group": {
        "id": 1,
        "name": "默认组",
        "description": "默认IP组",
        "created_at": "2024-01-01T10:00:00Z",
        "updated_at": "2024-01-01T10:00:00Z",
        "is_default": true
      },
      "action": "ban"
    }
  ]
}
```

### 创建IP规则

创建新的IP规则。

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
- `ip_net`: IP地址或CIDR网段（必填）
- `group_id`: 组ID（必填）
- `action`: 行为类型，`ban`（封禁）或 `allow`（允许）（必填）

**响应**
```json
{
  "code": 200,
  "message": "已禁用",
  "data": null
}
```

### 删除IP规则

删除指定的IP规则。

**请求**
```http
DELETE /api/ip/{id}
```

**参数**
- `id`: IP规则ID（路径参数）

**响应**
```json
{
  "code": 200,
  "message": "已解禁",
  "data": null
}
```

### 获取可用操作列表

获取所有可用的操作类型。

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

### 更新IP行为

更新指定IP的行为类型。

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

**字段说明**
- `id`: IP规则ID（必填）
- `action`: 新的行为类型，`ban`（封禁）或 `allow`（允许）（必填）

**响应**
```json
{
  "code": 200,
  "message": "批量禁用成功",
  "data": null
}
```

### 更新IP所属组

更新指定IP的所属组。

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

**字段说明**
- `id`: IP规则ID（必填）
- `group_id`: 新的组ID（必填）

**响应**
```json
{
  "code": 200,
  "message": "IP地址所属组更新成功",
  "data": null
}
```

## 组管理API

### 获取所有组列表

获取所有IP分组列表。

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
      "name": "默认组",
      "description": "默认IP组",
      "created_at": "2024-01-01T10:00:00Z",
      "updated_at": "2024-01-01T10:00:00Z",
      "is_default": true
    }
  ]
}
```

### 创建新组

创建新的IP分组。

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

**字段说明**
- `name`: 组名称（必填）
- `description`: 组描述（可选）

**响应**
```json
{
  "code": 200,
  "message": "success",
  "data": {
    "id": 2,
    "name": "测试组",
    "description": "测试用组",
    "created_at": "2024-01-01T10:00:00Z",
    "updated_at": "2024-01-01T10:00:00Z",
    "is_default": false
  }
}
```

### 更新组信息

更新指定组的信息。

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

**字段说明**
- `id`: 组ID（必填）
- `name`: 新的组名称（必填）
- `description`: 新的组描述（可选）

**响应**
```json
{
  "code": 200,
  "message": "success",
  "data": {
    "id": 1,
    "name": "新组名",
    "description": "新描述",
    "created_at": "2024-01-01T10:00:00Z",
    "updated_at": "2024-01-01T10:05:00Z",
    "is_default": true
  }
}
```

### 删除组

删除指定的IP分组。

**请求**
```http
DELETE /api/group/{id}
```

**参数**
- `id`: 组ID（路径参数）

**响应**
```json
{
  "code": 200,
  "message": "组删除成功",
  "data": null
}
```

## 错误处理

当API调用失败时，会返回相应的错误信息：

```json
{
  "code": 400,
  "message": "参数错误",
  "data": null
}
```

常见错误码：
- `400`: 参数错误
- `500`: 服务器内部错误

## 使用示例

### 使用curl

```bash
# 获取流量统计
curl -X GET http://localhost:8080/api/traffic

# 创建IP规则
curl -X POST http://localhost:8080/api/ip \
  -H "Content-Type: application/json" \
  -d '{"ip_net": "192.168.1.100", "group_id": 1, "action": "ban"}'

# 获取所有组
curl -X GET http://localhost:8080/api/group
```

### 使用JavaScript

```javascript
// 获取流量统计
const response = await fetch('/api/traffic');
const result = await response.json();
if (result.code === 200) {
  console.log(result.data);
}

// 创建IP规则
const createResponse = await fetch('/api/ip', {
  method: 'POST',
  headers: { 'Content-Type': 'application/json' },
  body: JSON.stringify({
    ip_net: '192.168.1.100',
    group_id: 1,
    action: 'ban'
  })
});
const createResult = await createResponse.json();
```

## 注意事项

1. **IP格式**: 支持单个IP地址（如 `192.168.1.100`）或CIDR网段（如 `192.168.1.0/24`）
2. **行为类型**: 目前支持 `ban`（封禁）和 `allow`（允许）两种行为
3. **组管理**: 删除组时，该组下的所有IP会被移动到默认组
4. **时间格式**: 所有时间字段都使用ISO 8601格式
5. **权限要求**: 某些操作（如防火墙规则修改）可能需要root权限 