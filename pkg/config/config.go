package config

// Config 主配置结构
type Config struct {
	Monitor  MonitorConfig  `yaml:"monitor"`
	Firewall FirewallConfig `yaml:"firewall"`
	Web      WebConfig      `yaml:"web"`
	Storage  StorageConfig  `yaml:"storage"`
	Debug    bool           `yaml:"debug"`
}

// MonitorConfig 网络和监控配置
type MonitorConfig struct {
	Interface      string `yaml:"interface"`       // 网络接口名称
	ExcludeSubnets string `yaml:"exclude_subnets"` // 排除的子网（逗号分隔）
	Window         int    `yaml:"window"`          // 监控时间窗口（秒）
	Timeout        int    `yaml:"timeout"`         // 连接超时时间（秒）
}

// FirewallConfig 防火墙配置
type FirewallConfig struct {
	Chain        string `yaml:"chain"`         // iptables链名称
	IpSet        string `yaml:"ipset"`         // ipset名称，如果设置则使用ipset
	DisableIpSet bool   `yaml:"disable_ipset"` // 是否禁用ipset
}

// WebConfig Web服务配置
type WebConfig struct {
	Listen string `yaml:"listen"` // Web服务监听地址
}

// StorageConfig 存储配置
type StorageConfig struct {
	Type     string         `yaml:"type"`     // "memory" 或 "database"
	Database DatabaseConfig `yaml:"database"` // 数据库配置
}

// DatabaseConfig 数据库配置
type DatabaseConfig struct {
	Driver   string `yaml:"driver"`   // "sqlite", "mysql", "postgres" 等
	Host     string `yaml:"host"`     // 数据库主机地址
	Port     int    `yaml:"port"`     // 数据库端口号
	Username string `yaml:"username"` // 数据库用户名
	Password string `yaml:"password"` // 数据库密码
	Database string `yaml:"database"` // 数据库名称或文件路径
	DSN      string `yaml:"dsn"`      // 数据库连接字符串
}

// GetStorageType 获取存储类型，兼容旧版本
func (c *Config) GetStorageType() string {
	return c.Storage.Type
}

// GetDatabaseConfig 获取数据库配置，兼容旧版本
func (c *Config) GetDatabaseConfig() *DatabaseConfig {
	return &c.Storage.Database
}
