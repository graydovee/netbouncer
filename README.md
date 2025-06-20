# NetBouncer

[![Go Version](https://img.shields.io/badge/Go-1.24.3+-blue.svg)](https://golang.org)
[![License](https://img.shields.io/badge/License-MIT-green.svg)](LICENSE)
[![Docker](https://img.shields.io/badge/Docker-Ready-blue.svg)](Dockerfile)

NetBouncer 是一个高性能的网络流量监控工具，提供实时流量统计、IP封禁管理和现代化的Web界面。它能够监控网络接口的流量，识别异常连接，并提供基于iptables的防火墙功能。

## ✨ 主要特性

- 🔍 **实时流量监控**: 基于libpcap的高性能网络包捕获
- 📊 **可视化界面**: 现代化的React Web界面，实时显示流量统计
- 🛡️ **IP封禁管理**: 支持单个IP或CIDR网段的封禁/解封
- ⚡ **高性能**: 使用Go语言开发，支持高并发流量处理
- 🗄️ **多存储后端**: 支持内存存储和数据库存储（SQLite/MySQL/PostgreSQL）
- 🔧 **灵活配置**: 支持配置文件、命令行参数和Docker部署
- 📱 **响应式设计**: 适配桌面和移动设备的Web界面

## 🚀 快速开始

### 使用Docker（推荐）

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

### 从源码构建

#### 前置要求

- Go 1.24.3+
- Node.js 18+ (用于构建前端)
- libpcap-dev (Linux)
- iptables (用于防火墙功能)

#### 构建步骤

```bash
# 克隆仓库
git clone https://github.com/graydovee/netbouncer.git
cd netbouncer

# 构建前端和后端
make all

# 运行应用
./bin/netbouncer
```

## 📖 使用指南

### 基本使用

```bash
# 使用默认配置启动
netbouncer

# 使用配置文件
netbouncer -c config.yaml

# 指定网络接口和监听地址
netbouncer -i eth0 -l 0.0.0.0:9090

# 排除特定网段
netbouncer -e "127.0.0.1/8,192.168.0.0/16"

# 调试模式（不真正设置防护墙）
netbouncer --debug
```

### 配置文件

创建 `config.yaml` 文件：

```yaml
# 监控配置
monitor:
  interface: "eth0"  # 网络接口名称（留空自动选择）
  exclude_subnets: "127.0.0.1/8,10.0.0.0/8,172.16.0.0/12,192.168.0.0/16"
  window: 60  # 监控时间窗口（秒）
  timeout: 86400  # 连接超时时间（秒）

# 防火墙配置
firewall:
  chain: "NETBOUNCER"  # iptables链名称

# Web服务配置
web:
  listen: "0.0.0.0:8080"  # Web服务监听地址

# 存储配置
storage:
  type: "database"  # "memory" 或 "database"
  database:
    driver: "sqlite"
    database: "netbouncer.db"

# 调试模式
debug: false
```

### 命令行参数

| 参数 | 简写 | 说明 | 默认值 |
|------|------|------|--------|
| `--config` | `-c` | 配置文件路径 | - |
| `--monitor-interface` | `-i` | 网络接口名称 | 自动选择 |
| `--monitor-exclude-subnets` | `-e` | 排除的子网 | - |
| `--monitor-window` | `-w` | 监控时间窗口（秒） | 60 |
| `--monitor-timeout` | `-t` | 连接超时时间（秒） | 86400 |
| `--firewall-chain` | `-n` | iptables链名称 | NETBOUNCER |
| `--listen` | `-l` | Web服务监听地址 | 0.0.0.0:8080 |
| `--storage` | `-s` | 存储类型 | memory |
| `--debug` | - | 启用调试模式 | false |

## 🌐 Web界面

NetBouncer 提供了现代化的Web界面，包含以下功能：

### 流量监控页面
- 实时流量统计图表
- 按IP地址的流量详情
- 连接数和数据包统计
- 流量速率监控

### IP封禁管理页面
- 查看已封禁的IP列表
- 添加新的IP封禁规则
- 解除IP封禁
- 支持CIDR网段封禁

### API接口

```bash
# 获取流量统计
GET /api/traffic

# 封禁IP
POST /api/ban
Content-Type: application/json
{"ip": "192.168.1.100"}

# 解封IP
POST /api/unban
Content-Type: application/json
{"ip": "192.168.1.100"}

# 获取已封禁IP列表
GET /api/banned
```

## 🗄️ 存储配置

### 内存存储

```yaml
storage:
  type: "memory"
```

### SQLite数据库（默认）

```yaml
storage:
  type: "database"
  database:
    driver: "sqlite"
    database: "netbouncer.db"
```

### MySQL数据库

```yaml
storage:
  type: "database"
  database:
    driver: "mysql"
    host: "localhost"
    port: 3306
    username: "netbouncer"
    password: "password"
    database: "netbouncer"
```

### PostgreSQL数据库

```yaml
storage:
  type: "database"
  database:
    driver: "postgres"
    host: "localhost"
    port: 5432
    username: "netbouncer"
    password: "password"
    database: "netbouncer"
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

### 多平台构建

```bash
# 构建多平台镜像
make docker-release

# 构建本地镜像
make docker-build
```

## 🔧 开发

### 项目结构

```
netbouncer/
├── cmd/                    # 命令行入口
├── pkg/                    # 核心包
│   ├── config/            # 配置管理
│   ├── core/              # 核心功能（监控、防火墙）
│   ├── service/           # 业务逻辑服务
│   ├── store/             # 数据存储
│   └── web/               # Web服务器
├── website/               # React前端源码
├── web/                   # 构建后的前端文件
└── main.go               # 主程序入口
```

### 开发环境设置

```bash
# 安装Go依赖
go mod tidy

# 启动前端开发服务器
make web-dev

# 运行后端（调试模式）
make debug
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

## 📊 性能特性

- **高并发处理**: 使用Go协程处理网络包
- **内存优化**: 滑动窗口算法减少内存占用
- **实时统计**: 毫秒级的流量统计更新
- **低延迟**: 基于libpcap的高效包捕获

## 🔒 安全特性

- **IP封禁**: 基于iptables的网络层封禁
- **子网过滤**: 支持排除内网和本地网段
- **连接超时**: 自动清理过期连接
- **访问控制**: Web界面访问控制

## 🤝 贡献

欢迎提交Issue和Pull Request！

### 开发流程

1. Fork 项目
2. 创建功能分支 (`git checkout -b feature/AmazingFeature`)
3. 提交更改 (`git commit -m 'Add some AmazingFeature'`)
4. 推送到分支 (`git push origin feature/AmazingFeature`)
5. 打开 Pull Request

## 📄 许可证

本项目采用 MIT 许可证 - 查看 [LICENSE](LICENSE) 文件了解详情。

## 🙏 致谢

- [gopacket](https://github.com/google/gopacket) - 网络包处理
- [Echo](https://echo.labstack.com/) - Web框架
- [Material-UI](https://mui.com/) - React UI组件库
- [React](https://reactjs.org/) - 前端框架

## 📞 支持

如果您遇到问题或有建议，请：

1. 查看 [Issues](https://github.com/graydovee/netbouncer/issues)
2. 创建新的 Issue
3. 联系维护者

---

**NetBouncer** - 让网络监控变得简单高效 🚀 