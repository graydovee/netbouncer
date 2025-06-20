package web

import (
	"log/slog"
	"net"
	"net/http"
	"time"

	"github.com/graydovee/netbouncer/pkg/service"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)

type Server struct {
	netService *service.NetService
	echo       *echo.Echo
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

// slogLogger 自定义日志中间件，使用Go的slog
func slogLogger() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			start := time.Now()

			err := next(c)

			duration := time.Since(start)

			req := c.Request()
			res := c.Response()

			status := res.Status
			var logFunc func(msg string, args ...any)

			switch {
			case status >= 500:
				logFunc = slog.Error
			case status >= 400:
				logFunc = slog.Warn
			default:
				logFunc = slog.Info
			}

			args := []any{
				"method", req.Method,
				"uri", req.RequestURI,
				"status", status,
				"duration", duration.String(),
				"remote_ip", c.RealIP(),
				"user_agent", req.UserAgent(),
			}

			if err != nil {
				args = append(args, "error", err.Error())
			}

			logFunc("HTTP Request", args...)

			return err
		}
	}
}

// validateIPOrCIDR 验证输入是否为有效的IP地址或CIDR格式
func validateIPOrCIDR(input string) error {
	// 首先尝试解析为IP地址
	if ip := net.ParseIP(input); ip != nil {
		return nil
	}

	// 如果不是单个IP，尝试解析为CIDR格式
	if _, _, err := net.ParseCIDR(input); err == nil {
		return nil
	}

	return echo.NewHTTPError(http.StatusBadRequest, "无效的IP地址或CIDR格式")
}

func NewServer(netService *service.NetService) *Server {
	e := echo.New()

	// 隐藏Echo框架的banner
	e.HideBanner = true

	svr := &Server{
		netService: netService,
		echo:       e,
	}

	// 中间件
	e.Use(slogLogger())
	e.Use(middleware.Recover())
	e.Use(middleware.CORS())

	// API路由
	e.GET("/api/traffic", svr.handleGetTraffic)
	e.POST("/api/ban", svr.handleBanIP)
	e.POST("/api/unban", svr.handleUnbanIP)
	e.GET("/api/banned", svr.handleGetBannedIPs)

	// 静态文件服务
	e.Static("/", "web")

	// 404处理 - 返回React应用的index.html
	e.HTTPErrorHandler = svr.handle404

	return svr
}

func (s *Server) Start(addr string) error {
	return s.echo.Start(addr)
}

func (s *Server) handleGetTraffic(c echo.Context) error {
	trafficData := s.netService.GetStats()
	return c.JSON(http.StatusOK, Success(trafficData))
}

func (s *Server) handleBanIP(c echo.Context) error {
	var r IPRequest
	if err := c.Bind(&r); err != nil || r.IP == "" {
		return c.JSON(http.StatusOK, Error(400, "参数错误"))
	}

	// 验证IP或CIDR格式
	if err := validateIPOrCIDR(r.IP); err != nil {
		return c.JSON(http.StatusOK, Error(400, "无效的IP地址或CIDR格式"))
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

	// 验证IP或CIDR格式
	if err := validateIPOrCIDR(r.IP); err != nil {
		return c.JSON(http.StatusOK, Error(400, "无效的IP地址或CIDR格式"))
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

// handle404 处理404错误，返回React应用的index.html
func (s *Server) handle404(err error, c echo.Context) {
	// 非API请求返回React应用的index.html
	c.File("web/index.html")
}
