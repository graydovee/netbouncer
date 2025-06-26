package web

import (
	"io"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/graydovee/netbouncer/pkg/service"
	"github.com/graydovee/netbouncer/pkg/store"
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

	e.GET("/api/ip", svr.handleListAllIpNets)
	e.GET("/api/ip/:groupId", svr.handleListIpNetsByGroup)
	e.POST("/api/ip", svr.handleCreateIpNet)
	e.POST("/api/ip/import", svr.handleImportIpNet)
	e.DELETE("/api/ip/:id", svr.handleDeleteIpNet)
	e.GET("/api/ip/action", svr.handleListAllActions)
	e.PUT("/api/ip/action", svr.handleUpdateIpNetAction)
	e.PUT("/api/ip/group", svr.handleUpdateIPGroup)

	e.GET("/api/group", svr.handleListAllGroups)
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

// handle404 处理404错误，返回React应用的index.html
func (s *Server) handle404(err error, c echo.Context) {
	// 非API请求返回React应用的index.html
	c.File("web/index.html")
}

func (s *Server) handleGetTraffic(c echo.Context) error {
	trafficData, err := s.netService.GetStats()
	if err != nil {
		return c.JSON(http.StatusOK, Error(500, err.Error()))
	}
	return c.JSON(http.StatusOK, Success(trafficData))
}

func (s *Server) handleCreateIpNet(c echo.Context) error {
	var r CreateIPNetRequest
	if err := c.Bind(&r); err != nil || r.IpNet == "" {
		return c.JSON(http.StatusOK, Error(400, "参数错误"))
	}

	// 验证IP或CIDR格式
	if err := validateIpNet(r.IpNet); err != nil {
		return c.JSON(http.StatusOK, Error(400, "无效的IP地址或CIDR格式"))
	}

	err := s.netService.CreateOrUpdateIpNet(r.IpNet, r.GroupId, r.Action)
	if err != nil {
		return c.JSON(http.StatusOK, Error(500, err.Error()))
	}
	return c.JSON(http.StatusOK, Success("已禁用"))
}

func (s *Server) handleImportIpNet(c echo.Context) error {
	var r ImportIPNetRequest
	if err := c.Bind(&r); err != nil {
		return c.JSON(http.StatusOK, Error(400, "参数错误"))
	}

	text := r.Text
	if r.Url != "" {
		slog.Info("从URL导入地址", "url", r.Url)
		response, err := http.Get(r.Url)
		if err != nil {
			return c.JSON(http.StatusOK, Error(500, err.Error()))
		}
		defer response.Body.Close()

		// 避免被攻击，最多只读取100M数据
		body, err := io.ReadAll(io.LimitReader(response.Body, 100*1024*1024))
		if err != nil {
			return c.JSON(http.StatusOK, Error(500, err.Error()))
		}

		text = string(body)
	}

	successCount, errorCount, err := s.netService.ImportIpNet(text, r.GroupId, r.Action)
	if err != nil {
		return c.JSON(http.StatusOK, Error(500, err.Error()))
	}

	return c.JSON(http.StatusOK, Success(ImportIPNetResponse{
		SuccessCount: successCount,
		FailedCount:  errorCount,
	}))
}

func (s *Server) handleListAllActions(c echo.Context) error {
	actions := []string{
		store.ActionBan,
		store.ActionAllow,
	}
	return c.JSON(http.StatusOK, Success(actions))
}

func (s *Server) handleUpdateIpNetAction(c echo.Context) error {
	var r UpdateIPNetActionRequest
	if err := c.Bind(&r); err != nil {
		return c.JSON(http.StatusOK, Error(400, "参数错误"))
	}

	err := s.netService.UpdateIpNetAction(r.ID, r.Action)
	if err != nil {
		return c.JSON(http.StatusOK, Error(500, err.Error()))
	}
	return c.JSON(http.StatusOK, Success("批量禁用成功"))
}

func (s *Server) handleUpdateIPGroup(c echo.Context) error {
	var r UpdateIPNetGroupRequest
	if err := c.Bind(&r); err != nil {
		return c.JSON(http.StatusOK, Error(400, "参数错误"))
	}

	err := s.netService.UpdateIPGroup(r.ID, r.GroupId)
	if err != nil {
		return c.JSON(http.StatusOK, Error(500, err.Error()))
	}
	return c.JSON(http.StatusOK, Success("IP地址所属组更新成功"))
}

func (s *Server) handleDeleteIpNet(c echo.Context) error {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		return c.JSON(http.StatusOK, Error(400, "无效的IP地址ID"))
	}

	err = s.netService.DeleteIpNet(uint(id))
	if err != nil {
		return c.JSON(http.StatusOK, Error(500, err.Error()))
	}
	return c.JSON(http.StatusOK, Success("已解禁"))
}

func (s *Server) handleListAllIpNets(c echo.Context) error {
	ips, err := s.netService.ListAllIpNets()
	if err != nil {
		return c.JSON(http.StatusOK, Error(500, err.Error()))
	}
	return c.JSON(http.StatusOK, Success(ips))
}

// handleGetBannedIPsByGroup 根据组ID获取被封禁的IP列表
func (s *Server) handleListIpNetsByGroup(c echo.Context) error {
	groupIdStr := c.Param("groupId")
	groupId, err := strconv.ParseUint(groupIdStr, 10, 32)
	if err != nil {
		return c.JSON(http.StatusOK, Error(400, "无效的组ID"))
	}

	ips, err := s.netService.ListIpNetsByGroup(uint(groupId))
	if err != nil {
		return c.JSON(http.StatusOK, Error(500, err.Error()))
	}
	return c.JSON(http.StatusOK, Success(ips))
}

// handleGetGroups 获取所有组列表
func (s *Server) handleListAllGroups(c echo.Context) error {
	groups, err := s.netService.ListAllGroups()
	if err != nil {
		return c.JSON(http.StatusOK, Error(500, err.Error()))
	}
	return c.JSON(http.StatusOK, Success(groups))
}

// handleCreateGroup 创建新组
func (s *Server) handleCreateGroup(c echo.Context) error {
	var r CreateGroupRequest
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
