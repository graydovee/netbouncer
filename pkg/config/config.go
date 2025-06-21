package config

// Config 主配置结构
type Config struct {
	Monitor  MonitorConfig  `yaml:"monitor"`
	Firewall FirewallConfig `yaml:"firewall"`
	Web      WebConfig      `yaml:"web"`
	Database DatabaseConfig `yaml:"database"`
}

// MonitorConfig 网络和监控配置
type MonitorConfig struct {
	Interface      string `yaml:"interface"`       // 网络接口名称
	ExcludeSubnets string `yaml:"exclude_subnets"` // 排除的子网（逗号分隔）
	Window         int    `yaml:"window"`          // 监控时间窗口（秒）
	Timeout        int    `yaml:"timeout"`         // 连接超时时间（秒）
}

type FirewallType string

const (
	FirewallTypeIptables FirewallType = "iptables"
	FirewallTypeIpSet    FirewallType = "ipset"
	FirewallTypeMock     FirewallType = "mock"
)

// FirewallConfig 防火墙配置
type FirewallConfig struct {
	Chain string `yaml:"chain"` // iptables链名称
	IpSet string `yaml:"ipset"` // ipset名称，如果设置则使用ipset
	Type  string `yaml:"type"`  // 防火墙类型，"iptables" 或 "ipset" 或 "mock"
}

// WebConfig Web服务配置
type WebConfig struct {
	Listen string `yaml:"listen"` // Web服务监听地址
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
