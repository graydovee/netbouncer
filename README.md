# NetBouncer

[![Go Version](https://img.shields.io/badge/Go-1.24.3+-blue.svg)](https://golang.org)
[![License](https://img.shields.io/badge/License-MIT-green.svg)](LICENSE)
[![Docker](https://img.shields.io/badge/Docker-Ready-blue.svg)](Dockerfile)

[English](README_en.md) | [中文](README.md)

NetBouncer 是一个高性能的网络流量监控工具，提供实时流量统计、IP管理和现代化的Web界面。支持iptables/ipset防火墙、IP分组管理、批量操作等功能。

## ✨ 主要特性

- 🔍 **实时流量监控**: 基于libpcap的高性能网络包捕获
- 📊 **可视化界面**: 现代化的React Web界面，实时显示流量统计
- 🛡️ **IP管理**: 支持单个IP或CIDR网段的封禁/允许管理
- 📁 **分组管理**: 支持IP分组管理，便于批量操作
- ⚡ **高性能**: 使用Go语言开发，支持高并发流量处理
- 🗄️ **多存储后端**: 支持SQLite、MySQL、PostgreSQL数据库
- 🔧 **灵活配置**: 支持配置文件、命令行参数和Docker部署
- 📱 **响应式设计**: 适配桌面和移动设备的Web界面
- 🛡️ **多种防火墙**: 支持iptables、ipset和mock模式

## 🚀 快速开始

### 方式一：使用Docker（推荐）

```bash
# 拉取最新镜像
docker pull graydovee/netbouncer:latest

# 运行容器
docker run -d \
  --name netbouncer \
  --network host \
  --cap-add=NET_ADMIN \
  --cap-add=NET_RAW \
  graydovee/netbouncer:latest

# 访问Web界面
# http://localhost:8080
```

### 方式二：从源码构建

#### 前置要求

- Go 1.24.3+
- Node.js 18+ (用于构建前端)
- libpcap-dev (Linux)
- iptables/ipset (用于防火墙功能)

#### 构建步骤

```bash
# 1. 克隆仓库
git clone https://github.com/graydovee/netbouncer.git
cd netbouncer

# 2. 构建前端和后端
make all

# 3. 运行应用
./bin/netbouncer
```

## 📖 使用指南

### 基本使用

```bash
# 使用默认配置启动（ipset模式）
netbouncer

# 使用配置文件启动
netbouncer -c config.yaml

# 指定网络接口和监听地址
netbouncer -i eth0 -l 0.0.0.0:9090

# 使用iptables防火墙模式
netbouncer --firewall-type iptables

# 使用mock模式（调试用）
netbouncer --firewall-type mock
```

### 配置文件设置

> 💡 详细配置说明请参考 [CONFIGURATION.md](doc/CONFIGURATION.md)

创建 `config.yaml` 文件：

```yaml
# 监控配置
monitor:
  interface: "eth0"  # 网络接口名称（留空自动选择）
  exclude_subnets: "127.0.0.1/8,10.0.0.0/8,172.16.0.0/12,192.168.0.0/16"
  window: 60  # 监控时间窗口（秒）
  timeout: 86400  # 监控清理不活跃连接的时间（秒）

# 防火墙配置
firewall:
  chain: "NETBOUNCER"  # iptables链名称
  ipset: "netbouncer"  # ipset名称
  type: "ipset"        # 防火墙类型：iptables, ipset, mock

# Web服务配置
web:
  listen: "0.0.0.0:8080"  # Web服务监听地址

# 数据库配置
database:
  driver: "sqlite"        # "sqlite", "mysql", "postgres"
  host: ""                # 数据库主机地址
  port: 0                 # 数据库端口号
  username: ""            # 数据库用户名
  password: ""            # 数据库密码
  database: "netbouncer.db"  # 数据库名称或文件路径
  dsn: ""                 # 数据库连接字符串（可选）
  log_level: "info"       # SQL日志级别: "silent", "error", "warn", "info"

# 初始规则配置
rules:
  # 示例：创建一个默认的封禁组
  - group: "blocked"
    groupDescription: "默认封禁组"
    action: "block"
    override: false
    ipNets:
      - "192.168.1.100"
      - "10.0.0.0/24"
  
  # 示例：创建一个白名单组
  - group: "whitelist"
    groupDescription: "白名单组"
    action: "allow"
    override: true
    ipNets:
      - "127.0.0.1"
      - "192.168.1.1"
```

### 规则配置说明

`rules` 配置项允许您在应用启动时预配置IP分组和规则，这些规则将自动创建。这对于设置默认封禁列表、白名单和其他常用配置非常有用。

#### 规则配置字段

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `group` | string | 是 | 分组名称，用于标识该规则组 |
| `groupDescription` | string | 否 | 分组描述，用于说明该组的用途 |
| `action` | string | 是 | 动作类型：`block`（封禁）或 `allow`（允许） |
| `override` | bool | 否 | 是否覆盖已存在的分组（默认：false） |
| `ipNets` | []string | 是 | IP地址或CIDR网段列表 |

#### 使用场景

- **预配置封禁列表**: 自动创建包含已知恶意IP的分组
- **白名单配置**: 预配置可信IP地址
- **测试环境**: 在开发或测试环境中快速设置测试数据
- **生产环境**: 根据安全策略预配置必要的IP规则

### 常用命令行参数

| 参数 | 简写 | 说明 | 默认值 |
|------|------|------|--------|
| `--config` | `-c` | 配置文件路径 | - |
| `--monitor-interface` | `-i` | 网络接口名称 | 自动选择 |
| `--monitor-exclude-subnets` | `-e` | 排除的子网 | - |
| `--firewall-type` | `-f` | 防火墙类型 (iptables\|ipset\|mock) | ipset |
| `--listen` | `-l` | Web服务监听地址 | 0.0.0.0:8080 |
| `--db-driver` | - | 数据库驱动 (sqlite\|mysql\|postgres) | sqlite |
| `--db-name` | - | 数据库名称或文件路径 | netbouncer.db |
| `--db-log-level` | - | SQL日志级别 (silent\|error\|warn\|info) | info |

## 🌐 Web界面使用

启动应用后，访问 `http://localhost:8080` 进入Web界面：

### 流量监控页面
- 实时查看网络连接流量统计
- 支持按流量、连接数等字段排序
- 可配置自动刷新间隔
- 一键封禁IP功能

### IP管理页面
- 查看所有IP或按组查看
- 添加新的IP地址或CIDR网段
- 修改IP行为（封禁/允许）
- 修改IP所属组
- 批量操作和批量导入

### 组管理页面
- 创建、编辑、删除IP分组
- 查看组列表和组信息

## 🗄️ 数据库配置

### SQLite（默认，推荐）

```yaml
database:
  driver: "sqlite"
  database: "netbouncer.db"
```

### MySQL

```yaml
database:
  driver: "mysql"
  host: "localhost"
  port: 3306
  username: "netbouncer"
  password: "password"
  database: "netbouncer"
```

### PostgreSQL

```yaml
database:
  driver: "postgres"
  host: "localhost"
  port: 5432
  username: "netbouncer"
  password: "password"
  database: "netbouncer"
```

## 🛡️ 防火墙配置

### ipset模式（默认）

```yaml
firewall:
  type: "ipset"
  ipset: "netbouncer"
```

### iptables模式

```yaml
firewall:
  type: "iptables"
  chain: "NETBOUNCER"
```

### mock模式（调试用）

```yaml
firewall:
  type: "mock"
```

## 🐳 Docker部署

### 使用Docker Compose

创建 `docker-compose.yml`：

```yaml
version: '3.8'

services:
  netbouncer:
    image: graydovee/netbouncer:latest
    container_name: netbouncer
    network_mode: host
    cap_add:
      - NET_ADMIN
      - NET_RAW
    volumes:
      - ./config.yaml:/app/config.yaml
      - ./data:/app/data
    command: ["-c", "/app/config.yaml"]
    restart: unless-stopped
```

运行：
```bash
docker-compose up -d
```

## 🔧 开发

### 开发环境设置

```bash
# 安装Go依赖
go mod tidy

# 启动前端开发服务器
make web-dev

# 运行后端（mock模式）
./bin/netbouncer --firewall-type mock
```

### 构建

```bash
# 构建所有组件
make all

# 仅构建Go程序
make build-go

# 仅构建前端
make build-web
```

## 📊 API接口

详细的API文档请参考 [API.md](doc/API.md)

主要API端点：
- `GET /api/traffic` - 获取流量统计
- `GET /api/ip` - 获取IP列表
- `POST /api/ip` - 创建IP规则
- `GET /api/group` - 获取组列表
- `POST /api/group` - 创建组

## 🔐 认证配置

NetBouncer 支持两种认证方式：**BasicAuth**（简单用户名密码）和 **OIDC**（OpenID Connect）。

### BasicAuth 认证（推荐简单部署）

BasicAuth 是最简单的认证方式，只需配置用户名和密码：

```yaml
web:
  listen: "0.0.0.0:8080"
  auth:
    enabled: true                      # 启用认证
    type: "basic"                      # 认证类型
    basic:
      username: "admin"                # 用户名
      password: "your-secure-password" # 密码
```

#### BasicAuth 特点
- 简单易用，无需外部依赖
- 支持浏览器弹出登录框
- 支持前端登录页面
- 支持API请求使用 Authorization 头

#### API 调用示例
```bash
curl -u admin:your-password http://localhost:8080/api/traffic
```

### OIDC 认证（推荐企业部署）

OIDC 支持与外部身份提供商集成：

```yaml
web:
  listen: "0.0.0.0:8080"
  auth:
    enabled: true                      # 启用认证
    type: "oidc"                       # 认证类型
    oidc:
      client_id: "your-client-id"        # OIDC客户端ID
      client_secret: "your-client-secret" # OIDC客户端密钥
      issuer_url: "https://accounts.google.com"  # OIDC提供者URL
      redirect_url: "http://localhost:8080/auth/callback"  # 回调地址
```

#### 常用OIDC提供者配置

**Google:**
```yaml
auth:
  enabled: true
  type: "oidc"
  oidc:
    client_id: "xxx.apps.googleusercontent.com"
    client_secret: "xxx"
    issuer_url: "https://accounts.google.com"
    redirect_url: "http://your-domain:8080/auth/callback"
```

**Keycloak:**
```yaml
auth:
  enabled: true
  type: "oidc"
  oidc:
    client_id: "netbouncer"
    client_secret: "xxx"
    issuer_url: "https://keycloak.example.com/realms/your-realm"
    redirect_url: "http://your-domain:8080/auth/callback"
```

**Authentik:**
```yaml
auth:
  enabled: true
  type: "oidc"
  oidc:
    client_id: "netbouncer"
    client_secret: "xxx"
    issuer_url: "https://authentik.example.com/application/o/netbouncer/"
    redirect_url: "http://your-domain:8080/auth/callback"
```

### 认证流程

1. 用户访问Web界面
2. 如果未登录：
   - BasicAuth: 显示登录页面或浏览器弹窗
   - OIDC: 重定向到OIDC提供者登录页面
3. 认证成功后访问应用
4. Session保持24小时有效

### API认证

启用认证后，API请求需要认证：
- **BasicAuth**: 使用 `Authorization: Basic base64(username:password)` 头
- **OIDC**: 需要有效的Session Cookie

未认证的API请求将返回401错误。
