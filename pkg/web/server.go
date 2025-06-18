package web

import (
	"net/http"

	"github.com/graydovee/netbouncer/pkg/service"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)

type Server struct {
	netService *service.NetService

	echo *echo.Echo
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

func NewServer(netService *service.NetService) *Server {
	e := echo.New()
	svr := &Server{
		netService: netService,
		echo:       e,
	}

	// 中间件
	e.Use(middleware.Logger())
	e.Use(middleware.Recover())
	e.Use(middleware.CORS())

	// 静态文件
	e.Static("/static", "static")

	// 路由
	e.GET("/", svr.handleIndex)
	e.GET("/api/traffic", svr.handleGetTraffic)
	e.POST("/api/ban", svr.handleBanIP)
	e.POST("/api/unban", svr.handleUnbanIP)
	e.GET("/api/banned", svr.handleGetBannedIPs)
	e.GET("/banned", svr.handleBannedPage)

	return svr
}

func (s *Server) Start(addr string) error {
	return s.echo.Start(addr)
}

func (s *Server) handleIndex(c echo.Context) error {
	return c.File("view/monitor.html")
}

func (s *Server) handleGetTraffic(c echo.Context) error {
	trafficData := s.netService.GetAllRemoteIPStats()
	return c.JSON(http.StatusOK, Success(trafficData))
}

func (s *Server) handleBanIP(c echo.Context) error {
	var r IPRequest
	if err := c.Bind(&r); err != nil || r.IP == "" {
		return c.JSON(http.StatusOK, Error(400, "参数错误"))
	}
	err := s.netService.BanIP(r.IP)
	if err != nil {
		return c.JSON(http.StatusOK, Error(500, err.Error()))
	}
	return c.JSON(http.StatusOK, Success("已禁用"))
}

func (s *Server) handleUnbanIP(c echo.Context) error {
	var r IPRequest
	if err := c.Bind(&r); err != nil || r.IP == "" {
		return c.JSON(http.StatusOK, Error(400, "参数错误"))
	}
	err := s.netService.UnbanIP(r.IP)
	if err != nil {
		return c.JSON(http.StatusOK, Error(500, err.Error()))
	}
	return c.JSON(http.StatusOK, Success("已解禁"))
}

func (s *Server) handleGetBannedIPs(c echo.Context) error {
	ips, err := s.netService.GetBannedIPs()
	if err != nil {
		return c.JSON(http.StatusOK, Error(500, err.Error()))
	}
	return c.JSON(http.StatusOK, Success(ips))
}

func (s *Server) handleBannedPage(c echo.Context) error {
	return c.File("view/banned.html")
}
