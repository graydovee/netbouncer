package web

import (
	"net/http"
	"strconv"

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
	e.GET("/api/banned/:groupId", svr.handleGetBannedIPsByGroup)
	e.GET("/api/groups", svr.handleGetGroups)
	e.POST("/api/group", svr.handleCreateGroup)
	e.PUT("/api/group", svr.handleUpdateGroup)
	e.DELETE("/api/group/:id", svr.handleDeleteGroup)

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
	trafficData, err := s.netService.GetStats()
	if err != nil {
		return c.JSON(http.StatusOK, Error(500, err.Error()))
	}
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

	err := s.netService.BanIpNet(r.IpNet, r.GroupId)
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

	err := s.netService.BanIpNets(r.IpNets, r.GroupId)
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

// handleGetBannedIPsByGroup 根据组ID获取被封禁的IP列表
func (s *Server) handleGetBannedIPsByGroup(c echo.Context) error {
	groupIdStr := c.Param("groupId")
	groupId, err := strconv.ParseUint(groupIdStr, 10, 32)
	if err != nil {
		return c.JSON(http.StatusOK, Error(400, "无效的组ID"))
	}

	ips, err := s.netService.GetBannedIPsByGroup(uint(groupId))
	if err != nil {
		return c.JSON(http.StatusOK, Error(500, err.Error()))
	}
	return c.JSON(http.StatusOK, Success(ips))
}

// handleGetGroups 获取所有组列表
func (s *Server) handleGetGroups(c echo.Context) error {
	groups, err := s.netService.GetGroups()
	if err != nil {
		return c.JSON(http.StatusOK, Error(500, err.Error()))
	}
	return c.JSON(http.StatusOK, Success(groups))
}

// handleCreateGroup 创建新组
func (s *Server) handleCreateGroup(c echo.Context) error {
	var r GroupRequest
	if err := c.Bind(&r); err != nil {
		return c.JSON(http.StatusOK, Error(400, "参数错误"))
	}

	if r.Name == "" {
		return c.JSON(http.StatusOK, Error(400, "组名称不能为空"))
	}

	group, err := s.netService.CreateGroup(r.Name, r.Description)
	if err != nil {
		return c.JSON(http.StatusOK, Error(500, err.Error()))
	}
	return c.JSON(http.StatusOK, Success(group))
}

// handleUpdateGroup 更新组信息
func (s *Server) handleUpdateGroup(c echo.Context) error {
	var r UpdateGroupRequest
	if err := c.Bind(&r); err != nil {
		return c.JSON(http.StatusOK, Error(400, "参数错误"))
	}

	if r.ID == 0 {
		return c.JSON(http.StatusOK, Error(400, "组ID不能为空"))
	}

	if r.Name == "" {
		return c.JSON(http.StatusOK, Error(400, "组名称不能为空"))
	}

	group, err := s.netService.UpdateGroup(r.ID, r.Name, r.Description)
	if err != nil {
		return c.JSON(http.StatusOK, Error(500, err.Error()))
	}
	return c.JSON(http.StatusOK, Success(group))
}

// handleDeleteGroup 删除组
func (s *Server) handleDeleteGroup(c echo.Context) error {
	groupIdStr := c.Param("id")
	groupId, err := strconv.ParseUint(groupIdStr, 10, 32)
	if err != nil {
		return c.JSON(http.StatusOK, Error(400, "无效的组ID"))
	}

	err = s.netService.DeleteGroup(uint(groupId))
	if err != nil {
		return c.JSON(http.StatusOK, Error(500, err.Error()))
	}
	return c.JSON(http.StatusOK, Success("组删除成功"))
}
