package web

import (
	"net/http"
	"time"

	"github.com/graydovee/netbouncer/pkg/monitor"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)

type Server struct {
	monitor *monitor.Monitor
	echo    *echo.Echo
}

// TrafficData 用于API响应的数据结构
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

func NewServer(mon *monitor.Monitor) *Server {
	e := echo.New()

	// 中间件
	e.Use(middleware.Logger())
	e.Use(middleware.Recover())
	e.Use(middleware.CORS())

	// 静态文件
	e.Static("/static", "static")

	// 路由
	e.GET("/", handleIndex)
	e.GET("/api/traffic", handleGetTraffic(mon))

	return &Server{
		monitor: mon,
		echo:    e,
	}
}

func (s *Server) Start(addr string) error {
	return s.echo.Start(addr)
}

func handleIndex(c echo.Context) error {
	return c.File("view/monitor.html")
}

func handleGetTraffic(mon *monitor.Monitor) echo.HandlerFunc {
	return func(c echo.Context) error {
		stats := mon.GetAllRemoteIPStats()
		trafficData := make([]TrafficData, 0, len(stats))

		for _, stat := range stats {
			trafficData = append(trafficData, TrafficData{
				RemoteIP:        stat.RemoteIP,
				LocalIP:         stat.LocalIP,
				TotalBytesIn:    stat.BytesRecv,
				TotalBytesOut:   stat.BytesSent,
				TotalPacketsIn:  stat.PacketsRecv,
				TotalPacketsOut: stat.PacketsSent,
				BytesInPerSec:   stat.BytesRecvPerSec,
				BytesOutPerSec:  stat.BytesSentPerSec,
				Connections:     stat.Connections,
				FirstSeen:       stat.FirstSeen.Format(time.RFC3339),
				LastSeen:        stat.LastSeen.Format(time.RFC3339),
			})
		}

		return c.JSON(http.StatusOK, trafficData)
	}
}
