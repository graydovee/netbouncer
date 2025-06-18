package config

type Config struct {
	Device  string
	WebAddr string

	MonitorWindowSize        int
	MonitorConnectionTimeout int

	IptablesChainName string

	Debug bool
}
