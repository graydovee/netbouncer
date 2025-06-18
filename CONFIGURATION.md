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
```

### Web服务配置 (web)

```yaml
web:
  listen: "0.0.0.0:8080"  # Web服务监听地址
```

### 存储配置 (storage)

```yaml
storage:
  type: "database"  # 存储类型：memory 或 database
  database:
    driver: "sqlite"        # 数据库驱动
    host: ""                # 数据库主机地址
    port: 0                 # 数据库端口号
    username: ""            # 数据库用户名
    password: ""            # 数据库密码
    database: "netbouncer.db"  # 数据库名称或文件路径
    dsn: ""                 # 数据库连接字符串
```

### 调试配置 (debug)

```yaml
debug: false  # 是否启用调试模式
```

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

### Web服务参数

- `-l, --listen`: Web服务监听地址

### 存储参数

- `-s, --storage`: 存储类型 (memory|database)

### 数据库参数

- `--db-driver`: 数据库驱动 (sqlite|mysql|postgres)
- `--db-host`: 数据库主机地址
- `--db-port`: 数据库端口号
- `--db-username`: 数据库用户名
- `--db-password`: 数据库密码
- `--db-name`: 数据库名称或文件路径
- `--db-dsn`: 数据库连接字符串

### 调试参数

- `--debug`: 启用调试模式

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
# 使用内存存储
./netbouncer -s memory --debug

# 使用数据库存储
./netbouncer -s database --db-driver sqlite --db-name myapp.db

# 指定网络接口和监听地址
./netbouncer -i eth0 -l 0.0.0.0:9090

# 排除特定子网
./netbouncer -e "127.0.0.1/8,192.168.0.0/16"

# 设置防火墙链名称
./netbouncer -n MYCHAIN
```

### 3. 混合使用配置文件和命令行参数

```bash
# 使用配置文件，但覆盖某些参数
./netbouncer -c config.yaml --debug -l 0.0.0.0:9090
```

## 配置结构优化说明

### 主要改进

1. **配置结构简化**: 将网络和监控配置合并为 `MonitorConfig`
2. **数据库配置嵌套**: 将 `DatabaseConfig` 放入 `StorageConfig` 中，使结构更清晰
3. **字段名统一**: 统一使用更直观的字段名，如 `exclude_subnets` 替代 `ignored_subnets`
4. **默认值管理**: 使用 `config.DefaultConfig()` 统一管理默认值
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

storage:
  type: "database"
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
| `--storage-type` | `--storage` | 简化参数名 |
| `--db-type` | `--db-driver` | 更准确的术语 |
| `--db-user` | `--db-username` | 更完整的参数名 |
| `--db-pass` | `--db-password` | 更完整的参数名 |
| `--db-name` | `--db-database` | 更清晰的语义 |
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
| `--listen` | `-l` | 监听地址 |

### 参数分组说明

为了更好地组织参数，我们对参数进行了分组：

- **监控参数**: 以 `monitor-` 开头，包括网络接口、排除子网、监控窗口、超时时间等
- **防火墙参数**: 以 `firewall-` 开头，包括iptables链名称等
- **其他参数**: 保持原有的命名方式，如Web服务、存储、数据库等

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

## 最佳实践

1. **开发环境**: 使用内存存储和调试模式
2. **生产环境**: 使用数据库存储确保数据持久化
3. **配置文件**: 将敏感信息（如数据库密码）通过环境变量或命令行参数传递
4. **版本控制**: 不要将包含敏感信息的配置文件提交到版本控制系统
5. **备份**: 定期备份重要的配置文件 