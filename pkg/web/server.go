package web

import (
	"net/http"

	"github.com/graydovee/netbouncer/pkg/service"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)

type Server struct {
	netService *service.NetService
	echo       *echo.Echo
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
	e.POST("/api/ban", svr.handleBanIpNet)
	e.POST("/api/batchBan", svr.handleBanIpNets)
	e.POST("/api/unban", svr.handleUnbanIpNet)
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

func (s *Server) handleBanIpNet(c echo.Context) error {
	var r IPRequest
	if err := c.Bind(&r); err != nil || r.IpNet == "" {
		return c.JSON(http.StatusOK, Error(400, "参数错误"))
	}

	// 验证IP或CIDR格式
	if err := validateIpNet(r.IpNet); err != nil {
		return c.JSON(http.StatusOK, Error(400, "无效的IP地址或CIDR格式"))
	}

	err := s.netService.BanIpNet(r.IpNet)
	if err != nil {
		return c.JSON(http.StatusOK, Error(500, err.Error()))
	}
	return c.JSON(http.StatusOK, Success("已禁用"))
}

func (s *Server) handleBanIpNets(c echo.Context) error {
	var r BatchIPRequest
	if err := c.Bind(&r); err != nil || len(r.IpNets) == 0 {
		return c.JSON(http.StatusOK, Error(400, "参数错误"))
	}

	// 验证所有IP或CIDR格式
	for _, ipNet := range r.IpNets {
		if err := validateIpNet(ipNet); err != nil {
			return c.JSON(http.StatusOK, Error(400, "无效的IP地址或CIDR格式: "+ipNet))
		}
	}

	err := s.netService.BanIpNets(r.IpNets)
	if err != nil {
		return c.JSON(http.StatusOK, Error(500, err.Error()))
	}
	return c.JSON(http.StatusOK, Success("批量禁用成功"))
}

func (s *Server) handleUnbanIpNet(c echo.Context) error {
	var r IPRequest
	if err := c.Bind(&r); err != nil || r.IpNet == "" {
		return c.JSON(http.StatusOK, Error(400, "参数错误"))
	}

	// 验证IP或CIDR格式
	if err := validateIpNet(r.IpNet); err != nil {
		return c.JSON(http.StatusOK, Error(400, "无效的IP地址或CIDR格式"))
	}

	err := s.netService.UnbanIpNet(r.IpNet)
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
