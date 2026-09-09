package web

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"mime"
	"net"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"

	"github.com/graydovee/netbouncer/pkg/service"
	"github.com/graydovee/netbouncer/pkg/store"
)

const (
	// importMaxBodySize 从URL导入时最多读取的数据量
	importMaxBodySize = 100 * 1024 * 1024
	// importTimeout 从URL导入的请求超时时间
	importTimeout = 30 * time.Second
	// sessionCleanupInterval session过期清理周期
	sessionCleanupInterval = 10 * time.Minute
)

type Server struct {
	netService  *service.NetService
	echo        *echo.Echo
	authHandler AuthHandler
	cleanupStop chan struct{}
}

func NewServer(netService *service.NetService, authHandler AuthHandler) *Server {
	e := echo.New()

	// 隐藏Echo框架的banner
	e.HideBanner = true

	svr := &Server{
		netService:  netService,
		echo:        e,
		authHandler: authHandler,
	}

	// 中间件
	e.Use(slogLogger())
	e.Use(middleware.Recover())
	e.Use(middleware.CORS())

	// 注册认证相关路由（不需要认证）
	if authHandler != nil {
		authHandler.RegisterRoutes(e)
		// 添加认证中间件
		e.Use(authHandler.AuthMiddleware())
	}

	svr.registerAPIRoutes(e)
	svr.registerStaticRoutes(e)

	// 统一错误处理
	e.HTTPErrorHandler = svr.handleHTTPError

	return svr
}

func (s *Server) registerAPIRoutes(e *echo.Echo) {
	e.GET("/api/traffic", s.handleGetTraffic)

	e.GET("/api/ip", s.handleListIpNets)
	e.POST("/api/ip", s.handleCreateIpNet)
	e.POST("/api/ip/import", s.handleImportIpNet)
	e.POST("/api/ip/batch-delete", s.handleBatchDeleteIpNet)
	e.POST("/api/ip/batch-action", s.handleBatchUpdateIpNetAction)
	e.POST("/api/ip/batch-group", s.handleBatchUpdateIpNetGroup)
	e.DELETE("/api/ip/:id", s.handleDeleteIpNet)
	e.GET("/api/ip/action", s.handleListAllActions)
	e.PUT("/api/ip/action", s.handleUpdateIpNetAction)
	e.PUT("/api/ip/group", s.handleUpdateIPGroup)

	e.GET("/api/group", s.handleListAllGroups)
	e.POST("/api/group", s.handleCreateGroup)
	e.PUT("/api/group", s.handleUpdateGroup)
	e.DELETE("/api/group/:id", s.handleDeleteGroup)
}

// registerStaticRoutes 注册 SPA 静态资源路由：
// 文件存在则返回原文件，否则回退到 index.html（前端路由）
func (s *Server) registerStaticRoutes(e *echo.Echo) {
	assetFS := webFileSystem()

	e.GET("/", func(c echo.Context) error {
		return serveAsset(c, assetFS, "index.html")
	})
	e.GET("/*", func(c echo.Context) error {
		name := path.Clean("/" + c.Param("*"))[1:]
		if name == "" {
			return serveAsset(c, assetFS, "index.html")
		}
		// API/认证路径不属于前端路由，未匹配到接口时返回 JSON 404
		if strings.HasPrefix(name, "api/") || strings.HasPrefix(name, "auth/") {
			return echo.NewHTTPError(http.StatusNotFound)
		}
		if _, err := fs.Stat(assetFS, name); err != nil {
			// 前端路由回退到 index.html
			return serveAsset(c, assetFS, "index.html")
		}
		return serveAsset(c, assetFS, name)
	})
}

func serveAsset(c echo.Context, fsys fs.FS, name string) error {
	f, err := fsys.Open(name)
	if err != nil {
		return echo.NewHTTPError(http.StatusNotFound)
	}
	defer func() { _ = f.Close() }()

	data, err := io.ReadAll(f)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError)
	}

	contentType := mime.TypeByExtension(path.Ext(name))
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	return c.Blob(http.StatusOK, contentType, data)
}

// Start 启动Web服务（阻塞），并开启 session 定期清理
func (s *Server) Start(addr string) error {
	if s.authHandler != nil && s.authHandler.IsEnabled() {
		s.cleanupStop = make(chan struct{})
		go s.sessionCleanupLoop()
	}
	return s.echo.Start(addr)
}

// Shutdown 优雅停止Web服务
func (s *Server) Shutdown(ctx context.Context) error {
	if s.cleanupStop != nil {
		close(s.cleanupStop)
		s.cleanupStop = nil
	}
	return s.echo.Shutdown(ctx)
}

// sessionCleanupLoop 定期清理过期session，避免内存泄漏
func (s *Server) sessionCleanupLoop() {
	ticker := time.NewTicker(sessionCleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			s.authHandler.CleanupSessions()
		case <-s.cleanupStop:
			return
		}
	}
}

// handleHTTPError 统一错误处理：API 返回 JSON，其余回退到前端页面
func (s *Server) handleHTTPError(err error, c echo.Context) {
	if c.Response().Committed {
		return
	}

	path := c.Request().URL.Path
	if strings.HasPrefix(path, "/api/") || strings.HasPrefix(path, "/auth/") {
		code := http.StatusInternalServerError
		msg := "内部服务器错误"
		var httpErr *echo.HTTPError
		if errors.As(err, &httpErr) {
			code = httpErr.Code
			if m, ok := httpErr.Message.(string); ok {
				msg = m
			}
		}
		_ = respondError(c, code, msg)
		return
	}

	// 前端路由回退到 index.html
	if serr := serveAsset(c, webFileSystem(), "index.html"); serr != nil {
		_ = c.String(http.StatusInternalServerError, "front-end assets not available")
	}
}

func (s *Server) handleGetTraffic(c echo.Context) error {
	trafficData, err := s.netService.GetStats()
	if err != nil {
		return respondServiceError(c, err)
	}
	return respondSuccess(c, trafficData)
}

func (s *Server) handleCreateIpNet(c echo.Context) error {
	var r CreateIPNetRequest
	ok, err := bindAndValidate(c, &r, func() error {
		if r.IpNet == "" {
			return fmt.Errorf("ip_net 不能为空")
		}
		if r.Action == "" {
			return fmt.Errorf("action 不能为空")
		}
		return nil
	})
	if !ok {
		return err
	}

	// 验证IP或CIDR格式
	if err := validateIpNet(r.IpNet); err != nil {
		return respondError(c, http.StatusBadRequest, "无效的IP地址或CIDR格式")
	}

	if err := s.netService.CreateOrUpdateIpNet(r.IpNet, r.GroupId, r.Action); err != nil {
		return respondServiceError(c, err)
	}
	return respondSuccess(c, nil)
}

func (s *Server) handleImportIpNet(c echo.Context) error {
	var r ImportIPNetRequest
	ok, err := bindAndValidate(c, &r, func() error {
		if r.Text == "" && r.Url == "" {
			return fmt.Errorf("text 和 url 至少提供一个")
		}
		if r.Action == "" {
			return fmt.Errorf("action 不能为空")
		}
		return nil
	})
	if !ok {
		return err
	}

	text := r.Text
	if r.Url != "" {
		body, fetchErr := fetchURLText(c.Request().Context(), r.Url)
		if fetchErr != nil {
			return respondError(c, http.StatusBadGateway, fetchErr.Error())
		}
		text = string(body)
	}

	successCount, errorCount, err := s.netService.ImportIpNet(text, r.GroupId, r.Action)
	if err != nil {
		return respondServiceError(c, err)
	}

	return respondSuccess(c, ImportIPNetResponse{
		SuccessCount: successCount,
		FailedCount:  errorCount,
	})
}

// fetchURLText 从指定URL拉取文本内容，限制大小并阻止访问内网地址（SSRF防护）
func fetchURLText(ctx context.Context, rawURL string) ([]byte, error) {
	u, err := url.Parse(rawURL)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") {
		return nil, fmt.Errorf("仅支持 http/https 地址")
	}

	client := &http.Client{
		Timeout: importTimeout,
		Transport: &http.Transport{
			// Dial 阶段校验解析出的目标IP，阻止重定向/域名解析绕过
			DialContext: ssrfGuardedDialContext,
		},
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 5 {
				return fmt.Errorf("重定向次数过多")
			}
			if req.URL.Scheme != "http" && req.URL.Scheme != "https" {
				return fmt.Errorf("仅支持 http/https 地址")
			}
			return nil
		},
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("无效的URL地址")
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("获取URL内容失败: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("URL返回异常状态码: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, importMaxBodySize))
	if err != nil {
		return nil, fmt.Errorf("读取URL内容失败: %v", err)
	}
	return body, nil
}

// ssrfGuardedDialContext 在建立连接前解析目标IP，
// 拒绝回环、内网、链路本地等地址，并直连已校验的IP
func ssrfGuardedDialContext(ctx context.Context, network, addr string) (net.Conn, error) {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return nil, err
	}

	// 本身就是IP字面量时直接校验
	if ip := net.ParseIP(host); ip != nil {
		if isForbiddenIP(ip) {
			return nil, fmt.Errorf("禁止访问内网地址")
		}
	}

	ips, err := net.DefaultResolver.LookupIPAddr(ctx, host)
	if err != nil {
		return nil, err
	}

	var allowed net.IP
	for _, candidate := range ips {
		if !isForbiddenIP(candidate.IP) {
			allowed = candidate.IP
			break
		}
	}
	if allowed == nil {
		return nil, fmt.Errorf("禁止访问内网地址")
	}

	var dialer net.Dialer
	return dialer.DialContext(ctx, network, net.JoinHostPort(allowed.String(), port))
}

func isForbiddenIP(ip net.IP) bool {
	return ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() ||
		ip.IsLinkLocalMulticast() || ip.IsUnspecified()
}

func (s *Server) handleListAllActions(c echo.Context) error {
	actions := []string{
		store.ActionBan,
		store.ActionAllow,
	}
	return respondSuccess(c, actions)
}

func (s *Server) handleUpdateIpNetAction(c echo.Context) error {
	var r UpdateIPNetActionRequest
	ok, err := bindAndValidate(c, &r, func() error {
		if r.ID == 0 {
			return fmt.Errorf("id 不能为空")
		}
		if r.Action == "" {
			return fmt.Errorf("action 不能为空")
		}
		return nil
	})
	if !ok {
		return err
	}

	if err := s.netService.UpdateIpNetAction(r.ID, r.Action); err != nil {
		return respondServiceError(c, err)
	}
	return respondSuccess(c, nil)
}

func (s *Server) handleBatchUpdateIpNetAction(c echo.Context) error {
	var r BatchUpdateIPNetActionRequest
	ok, err := bindAndValidate(c, &r, func() error {
		if len(r.IDs) == 0 {
			return fmt.Errorf("ids 不能为空")
		}
		if r.Action == "" {
			return fmt.Errorf("action 不能为空")
		}
		return nil
	})
	if !ok {
		return err
	}

	success, err := s.netService.UpdateIpNetActions(r.IDs, r.Action)
	if err != nil {
		return respondServiceError(c, err)
	}
	return respondSuccess(c, BatchOperationResponse{
		SuccessCount: success,
		FailedCount:  len(r.IDs) - success,
	})
}

func (s *Server) handleBatchDeleteIpNet(c echo.Context) error {
	var r BatchDeleteIPNetRequest
	ok, err := bindAndValidate(c, &r, func() error {
		if len(r.IDs) == 0 {
			return fmt.Errorf("ids 不能为空")
		}
		return nil
	})
	if !ok {
		return err
	}

	success, err := s.netService.DeleteIpNets(r.IDs)
	if err != nil {
		return respondServiceError(c, err)
	}
	return respondSuccess(c, BatchOperationResponse{
		SuccessCount: success,
		FailedCount:  len(r.IDs) - success,
	})
}

func (s *Server) handleBatchUpdateIpNetGroup(c echo.Context) error {
	var r BatchUpdateIPNetGroupRequest
	ok, err := bindAndValidate(c, &r, func() error {
		if len(r.IDs) == 0 {
			return fmt.Errorf("ids 不能为空")
		}
		if r.GroupId == 0 {
			return fmt.Errorf("group_id 不能为空")
		}
		return nil
	})
	if !ok {
		return err
	}

	success, err := s.netService.UpdateIPGroups(r.IDs, r.GroupId)
	if err != nil {
		return respondServiceError(c, err)
	}
	return respondSuccess(c, BatchOperationResponse{
		SuccessCount: success,
		FailedCount:  len(r.IDs) - success,
	})
}

func (s *Server) handleUpdateIPGroup(c echo.Context) error {
	var r UpdateIPNetGroupRequest
	ok, err := bindAndValidate(c, &r, func() error {
		if r.ID == 0 {
			return fmt.Errorf("id 不能为空")
		}
		if r.GroupId == 0 {
			return fmt.Errorf("group_id 不能为空")
		}
		return nil
	})
	if !ok {
		return err
	}

	if err := s.netService.UpdateIPGroup(r.ID, r.GroupId); err != nil {
		return respondServiceError(c, err)
	}
	return respondSuccess(c, nil)
}

func (s *Server) handleDeleteIpNet(c echo.Context) error {
	id, ok, err := pathUint(c, "id")
	if !ok {
		return err
	}

	if err := s.netService.DeleteIpNet(id); err != nil {
		return respondServiceError(c, err)
	}
	return respondSuccess(c, nil)
}

// handleListIpNets 分页获取IP规则列表
// 查询参数: page, page_size, group_id, action, search
func (s *Server) handleListIpNets(c echo.Context) error {
	page, ok, err := queryInt(c, "page", 1)
	if !ok {
		return err
	}
	pageSize, ok, err := queryInt(c, "page_size", 20)
	if !ok {
		return err
	}
	groupId, ok, err := queryUint(c, "group_id")
	if !ok {
		return err
	}

	action := c.QueryParam("action")
	if action != "" && action != store.ActionBan && action != store.ActionAllow {
		return respondError(c, http.StatusBadRequest, "无效的action参数")
	}

	result, err := s.netService.ListIpNets(service.IpNetListParams{
		Page:     page,
		PageSize: pageSize,
		GroupID:  groupId,
		Action:   action,
		Search:   c.QueryParam("search"),
	})
	if err != nil {
		return respondServiceError(c, err)
	}
	return respondSuccess(c, result)
}

// handleListAllGroups 获取所有组列表（含组内IP数量）
func (s *Server) handleListAllGroups(c echo.Context) error {
	groups, err := s.netService.ListAllGroups()
	if err != nil {
		return respondServiceError(c, err)
	}
	return respondSuccess(c, groups)
}

// handleCreateGroup 创建新组
func (s *Server) handleCreateGroup(c echo.Context) error {
	var r CreateGroupRequest
	ok, err := bindAndValidate(c, &r, func() error {
		if strings.TrimSpace(r.Name) == "" {
			return fmt.Errorf("组名称不能为空")
		}
		return nil
	})
	if !ok {
		return err
	}

	group, err := s.netService.CreateGroup(r.Name, r.Description)
	if err != nil {
		return respondServiceError(c, err)
	}
	return respondSuccess(c, group)
}

// handleUpdateGroup 更新组信息
func (s *Server) handleUpdateGroup(c echo.Context) error {
	var r UpdateGroupRequest
	ok, err := bindAndValidate(c, &r, func() error {
		if r.ID == 0 {
			return fmt.Errorf("组ID不能为空")
		}
		if strings.TrimSpace(r.Name) == "" {
			return fmt.Errorf("组名称不能为空")
		}
		return nil
	})
	if !ok {
		return err
	}

	group, err := s.netService.UpdateGroup(r.ID, r.Name, r.Description)
	if err != nil {
		return respondServiceError(c, err)
	}
	return respondSuccess(c, group)
}

// handleDeleteGroup 删除组
func (s *Server) handleDeleteGroup(c echo.Context) error {
	groupId, ok, err := pathUint(c, "id")
	if !ok {
		return err
	}

	if err := s.netService.DeleteGroup(groupId); err != nil {
		return respondServiceError(c, err)
	}
	return respondSuccess(c, nil)
}
