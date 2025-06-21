package service

type TrafficData struct {
	RemoteIP        string  `json:"remote_ip"`         // 远程IP
	LocalIP         string  `json:"local_ip"`          // 本地IP
	TotalBytesIn    uint64  `json:"total_bytes_in"`    // 总接收字节数
	TotalBytesOut   uint64  `json:"total_bytes_out"`   // 总发送字节数
	TotalPacketsIn  uint64  `json:"total_packets_in"`  // 总接收包数
	TotalPacketsOut uint64  `json:"total_packets_out"` // 总发送包数
	BytesInPerSec   float64 `json:"bytes_in_per_sec"`  // 每秒接收字节数
	BytesOutPerSec  float64 `json:"bytes_out_per_sec"` // 每秒发送字节数
	Connections     int     `json:"connections"`       // 连接数
	FirstSeen       string  `json:"first_seen"`        // 首次发现时间
	LastSeen        string  `json:"last_seen"`         // 最后活动时间
	IsBanned        bool    `json:"is_banned"`         // 是否被ban
}

type BannedIpNet struct {
	IpNet     string         `json:"ip_net"`
	CreatedAt string         `json:"created_at"`
	UpdatedAt string         `json:"updated_at"`
	Group     *BannedIpGroup `json:"group"`
}

type BannedIpGroup struct {
	ID          uint   `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
	IsDefault   bool   `json:"is_default"`
}
