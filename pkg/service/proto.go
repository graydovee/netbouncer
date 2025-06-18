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
}