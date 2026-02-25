# NetBouncer 配置说明

## 概述

NetBouncer 支持通过 YAML 配置文件和命令行参数进行配置。配置文件提供了更清晰的配置结构，而命令行参数则提供了快速覆盖配置的灵活性。

## 配置优先级

配置的优先级从高到低为：
1. 命令行参数
2. 配置文件
3. 默认值

## 配置文件

### 生成默认配置文件

```bash
# 生成默认配置文件
./netbouncer config generate

# 生成指定名称的配置文件
./netbouncer config generate my-config.yaml
```

### 使用配置文件启动

```bash
# 使用配置文件启动
./netbouncer -c config.yaml

# 或者使用长参数
./netbouncer --config config.yaml
```

## 配置结构

### 监控配置 (monitor)

```yaml
monitor:
  interface: "eth0"  # 网络接口名称
  exclude_subnets: "127.0.0.1/8,10.0.0.0/8"  # 排除的子网
  window: 60  # 监控时间窗口（秒）
  timeout: 86400  # 连接超时时间（秒）
```

### 防火墙配置 (firewall)

```yaml
firewall:
  chain: "NETBOUNCER"  # iptables链名称
  ipset: "netbouncer"  # ipset名称
  type: "ipset"        # 防火墙类型：iptables, ipset, mock
```

### Web服务配置 (web)

```yaml
web:
  listen: "0.0.0.0:8080"  # Web服务监听地址
```

### 数据库配置 (database)

```yaml
database:
  driver: "sqlite"        # 数据库驱动：sqlite, mysql, postgres
  host: ""                # 数据库主机地址
  port: 0                 # 数据库端口号
  username: ""            # 数据库用户名
  password: ""            # 数据库密码
  database: "netbouncer.db"  # 数据库名称或文件路径
  dsn: ""                 # 数据库连接字符串（可选，优先级高于其他配置）
  log_level: "info"       # SQL日志级别: "silent", "error", "warn", "info"
```

### 初始规则配置 (rules)

`rules` 配置项用于在应用启动时自动创建默认的IP分组和规则。这对于预配置常用的封禁列表、白名单等非常有用。

```yaml
rules:
  # 创建一个默认的封禁组
  - group: "blocked"
    groupDescription: "默认封禁组"
    action: "block"
    override: false
    ipNets:
      - "192.168.1.100"
      - "10.0.0.0/24"
  
  # 创建一个白名单组
  - group: "whitelist"
    groupDescription: "白名单组"
    action: "allow"
    override: true
    ipNets:
      - "127.0.0.1"
      - "192.168.1.1"
```

#### 规则配置字段说明

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `group` | string | 是 | 分组名称，用于标识该规则组 |
| `groupDescription` | string | 否 | 分组描述，用于说明该组的用途 |
| `action` | string | 是 | 动作类型：`block`（封禁）或 `allow`（允许） |
| `override` | bool | 否 | 是否覆盖已存在的分组，默认为 `false` |
| `ipNets` | []string | 是 | IP地址或CIDR网段列表 |

#### 规则配置使用场景

1. **预配置封禁列表**: 在启动时自动创建包含已知恶意IP的分组
2. **白名单配置**: 预配置可信IP地址，确保关键服务不受影响
3. **测试环境**: 在开发或测试环境中快速设置测试数据
4. **生产环境**: 根据安全策略预配置必要的IP规则

#### 规则配置注意事项

- 如果 `override` 为 `false` 且分组已存在，则不会覆盖现有分组
- 如果 `override` 为 `true`，则会删除现有分组并重新创建
- `ipNets` 支持单个IP地址（如 `192.168.1.100`）和CIDR网段（如 `10.0.0.0/24`）
- 规则配置在应用启动时执行，如果配置有误可能导致启动失败

## 命令行参数

### 配置文件参数

- `-c, --config`: 指定配置文件路径

### 监控参数

- `-i, --monitor-interface`: 网络接口名称
- `-e, --monitor-exclude-subnets`: 排除的子网（逗号分隔）
- `-w, --monitor-window`: 监控时间窗口（秒）
- `-t, --monitor-timeout`: 连接超时时间（秒）

### 防火墙参数

- `-n, --firewall-chain`: iptables链名称
- `-p, --firewall-ipset`: ipset名称
- `-f, --firewall-type`: 防火墙类型 (iptables|ipset|mock)

### Web服务参数

- `-l, --listen`: Web服务监听地址

### 数据库参数

- `--db-driver`: 数据库驱动 (sqlite|mysql|postgres)
- `--db-host`: 数据库主机地址
- `--db-port`: 数据库端口号
- `--db-username`: 数据库用户名
- `--db-password`: 数据库密码
- `--db-name`: 数据库名称或文件路径
- `--db-dsn`: 数据库连接字符串
- `--db-log-level`: SQL日志级别 (silent|error|warn|info)

## 使用示例

### 1. 使用配置文件启动

```bash
# 生成配置文件
./netbouncer config generate

# 编辑配置文件
vim config.yaml

# 使用配置文件启动
./netbouncer -c config.yaml
```

### 2. 使用命令行参数启动

```bash
# 使用SQLite数据库
./netbouncer --db-driver sqlite --db-name myapp.db

# 使用MySQL数据库
./netbouncer --db-driver mysql --db-host localhost --db-port 3306 --db-username netbouncer --db-password password --db-name netbouncer

# 指定网络接口和监听地址
./netbouncer -i eth0 -l 0.0.0.0:9090

# 排除特定子网
./netbouncer -e "127.0.0.1/8,192.168.0.0/16"

# 使用iptables防火墙
./netbouncer -f iptables -n MYCHAIN

# 使用mock防火墙（调试模式）
./netbouncer -f mock
```

### 3. 混合使用配置文件和命令行参数

```bash
# 使用配置文件，但覆盖某些参数
./netbouncer -c config.yaml -f mock -l 0.0.0.0:9090
```

## 防火墙类型说明

### ipset模式（默认）

使用ipset进行IP封禁，性能更好，适合高并发场景：

```yaml
firewall:
  type: "ipset"
  ipset: "netbouncer"  # ipset名称
```

### iptables模式

使用iptables进行IP封禁，适合大多数Linux系统：

```yaml
firewall:
  type: "iptables"
  chain: "NETBOUNCER"  # 自定义iptables链名称
```

### mock模式

模拟防火墙操作，不实际执行封禁，适合开发和测试：

```yaml
firewall:
  type: "mock"
```

## 数据库配置详解

### SQLite（默认）

适合单机部署，无需额外配置：

```yaml
database:
  driver: "sqlite"
  database: "netbouncer.db"  # SQLite数据库文件路径
```

### MySQL

适合生产环境，支持高并发：

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

适合复杂查询场景：

```yaml
database:
  driver: "postgres"
  host: "localhost"
  port: 5432
  username: "netbouncer"
  password: "password"
  database: "netbouncer"
```

### 使用DSN连接字符串

DSN连接字符串的优先级高于其他配置：

```yaml
database:
  dsn: "mysql://user:password@localhost:3306/netbouncer"
```

## 配置结构优化说明

### 主要改进

1. **配置结构简化**: 将网络和监控配置合并为 `MonitorConfig`
2. **数据库配置独立**: 将 `DatabaseConfig` 独立出来，使结构更清晰
3. **防火墙类型支持**: 新增防火墙类型配置，支持iptables、ipset和mock
4. **字段名统一**: 统一使用更直观的字段名
5. **参数分组**: Monitor和Firewall参数使用前缀进行分组

### 配置结构对比

#### 旧结构
```yaml
network:
  interface: "eth0"
  exclude_subnets: "..."

monitor:
  window: 60
  timeout: 86400

storage:
  type: "database"

database:
  driver: "sqlite"
  # ...
```

#### 新结构
```yaml
monitor:
  interface: "eth0"
  exclude_subnets: "..."
  window: 60
  timeout: 86400

firewall:
  type: "ipset"
  chain: "NETBOUNCER"
  ipset: "netbouncer"

database:
  driver: "sqlite"
  # ...
```

### 参数名改进

我们对参数名进行了优化，使其更加直观和易用：

| 旧参数名 | 新参数名 | 改进说明 |
|---------|---------|---------|
| `--device` | `--monitor-interface` | 更准确地描述网络接口，添加分组前缀 |
| `--ignored-subnets` | `--monitor-exclude-subnets` | 更清晰的语义表达，添加分组前缀 |
| `--window-size` | `--monitor-window` | 简化参数名，添加分组前缀 |
| `--connection-timeout` | `--monitor-timeout` | 简化参数名，添加分组前缀 |
| `--netbouncer-chain` | `--firewall-chain` | 简化参数名，添加分组前缀 |
| `--addr` | `--listen` | 更准确地描述监听行为 |
| `--storage-type` | `--db-driver` | 更准确的术语 |
| `--db-type` | `--db-driver` | 更准确的术语 |
| `--db-user` | `--db-username` | 更完整的参数名 |
| `--db-pass` | `--db-password` | 更完整的参数名 |
| `--db-name` | `--db-name` | 保持一致性 |
| `--db-conn` | `--db-dsn` | 更标准的缩写 |

### 短参数优化

我们也优化了短参数，使其更加合理：

| 参数 | 短参数 | 说明 |
|------|--------|------|
| `--monitor-interface` | `-i` | 网络接口 |
| `--monitor-exclude-subnets` | `-e` | 排除子网 |
| `--monitor-window` | `-w` | 监控窗口 |
| `--monitor-timeout` | `-t` | 超时时间 |
| `--firewall-chain` | `-n` | 防火墙链 |
| `--firewall-ipset` | `-p` | ipset名称 |
| `--firewall-type` | `-f` | 防火墙类型 |
| `--listen` | `-l` | 监听地址 |

### 参数分组说明

为了更好地组织参数，我们对参数进行了分组：

- **监控参数**: 以 `monitor-` 开头，包括网络接口、排除子网、监控窗口、超时时间等
- **防火墙参数**: 以 `firewall-` 开头，包括iptables链名称、ipset名称、防火墙类型等
- **数据库参数**: 以 `db-` 开头，包括数据库连接相关配置
- **其他参数**: 保持原有的命名方式，如Web服务等

## 配置验证

配置文件使用 YAML 格式，支持以下特性：

- 注释：使用 `#` 开始注释
- 嵌套结构：使用缩进表示层级关系
- 数据类型：自动识别字符串、数字、布尔值等

## 故障排除

### 配置文件格式错误

如果配置文件格式有误，程序会显示详细的错误信息：

```bash
./netbouncer -c invalid-config.yaml
# 错误: 加载配置文件失败: failed to parse config file invalid-config.yaml: yaml: line 2: did not find expected key
```

### 配置文件不存在

```bash
./netbouncer -c not-exist.yaml
# 错误: 加载配置文件失败: failed to read config file not-exist.yaml: open not-exist.yaml: no such file or directory
```

### 权限问题

确保程序有权限读取配置文件：

```bash
# 检查文件权限
ls -la config.yaml

# 修改权限（如果需要）
chmod 644 config.yaml
```

### 防火墙权限问题

使用iptables或ipset需要root权限：

```bash
# 使用sudo运行
sudo ./netbouncer

# 或者使用mock模式进行测试
./netbouncer -f mock
```

## 最佳实践

1. **开发环境**: 使用mock防火墙模式和SQLite数据库
2. **生产环境**: 使用iptables或ipset防火墙和MySQL/PostgreSQL数据库
3. **配置文件**: 将敏感信息（如数据库密码）通过环境变量或命令行参数传递
4. **版本控制**: 不要将包含敏感信息的配置文件提交到版本控制系统
5. **备份**: 定期备份重要的配置文件和数据
6. **防火墙选择**: 
   - 单机部署：使用iptables
   - 高并发场景：使用ipset
   - 开发和测试：使用mock 