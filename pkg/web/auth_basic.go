package web

import (
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/labstack/echo/v4"
)

// sessionTTL BasicAuth会话有效期
const sessionTTL = 24 * time.Hour

// ==================== Basic Auth Handler ====================

// BasicAuthHandler BasicAuth认证处理器
type BasicAuthHandler struct {
	baseAuthHandler
	username string
	password string
}

// newBasicAuthHandler 创建BasicAuth处理器
func newBasicAuthHandler(cfg *AuthConfig) (*BasicAuthHandler, error) {
	if cfg.BasicUsername == "" || cfg.BasicPassword == "" {
		return nil, fmt.Errorf("BasicAuth配置不完整: username 和 password 是必需的")
	}

	handler := &BasicAuthHandler{
		baseAuthHandler: baseAuthHandler{
			enabled:  true,
			sessions: newSessionStore(),
		},
		username: cfg.BasicUsername,
		password: cfg.BasicPassword,
	}

	slog.Info("BasicAuth认证已启用", "username", cfg.BasicUsername)
	return handler, nil
}

// AuthType 返回认证类型
func (h *BasicAuthHandler) AuthType() string {
	return AuthTypeBasic
}

// RegisterRoutes 注册认证相关路由
func (h *BasicAuthHandler) RegisterRoutes(e *echo.Echo) {
	e.GET("/auth/login", h.handleLogin)
	e.POST("/auth/login", h.handleLogin)
	e.GET("/auth/logout", h.handleLogout)
	e.POST("/auth/logout", h.handleLogout)
	e.GET("/auth/status", h.handleStatus)
}

// handleLogin 处理登录请求
func (h *BasicAuthHandler) handleLogin(c echo.Context) error {
	// 如果是GET请求，返回登录页面提示（前端会处理）
	if c.Request().Method == http.MethodGet {
		// 检查是否已经通过BasicAuth认证
		authHeader := c.Request().Header.Get("Authorization")
		if authHeader != "" && h.validateBasicAuth(authHeader) {
			return c.Redirect(http.StatusTemporaryRedirect, "/")
		}
		// 返回401要求认证
		c.Response().Header().Set("WWW-Authenticate", `Basic realm="NetBouncer"`)
		return respondError(c, http.StatusUnauthorized, "需要认证")
	}

	// POST请求处理登录
	var loginReq struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	ok, err := bindAndValidate(c, &loginReq, nil)
	if !ok {
		return err
	}

	// 验证用户名密码
	if !h.validateCredentials(loginReq.Username, loginReq.Password) {
		slog.Warn("登录失败", "username", loginReq.Username)
		return respondError(c, http.StatusUnauthorized, "用户名或密码错误")
	}

	// 创建session
	session := &Session{
		ID:        newSessionID(),
		UserInfo:  &UserInfo{Subject: loginReq.Username, Name: loginReq.Username, Email: loginReq.Username + "@local"},
		ExpiresAt: time.Now().Add(sessionTTL),
	}
	h.createSession(session)

	// 设置session cookie
	h.createSessionCookie(c, session.ID, int(sessionTTL.Seconds()))

	slog.Info("用户登录成功", "username", loginReq.Username)
	return respondSuccess(c, map[string]any{
		"message": "登录成功",
		"user":    session.UserInfo,
	})
}

// validateCredentials 校验用户名密码（恒定时间比较，防止时序攻击）
func (h *BasicAuthHandler) validateCredentials(username, password string) bool {
	usernameMatch := subtle.ConstantTimeCompare([]byte(username), []byte(h.username)) == 1
	passwordMatch := subtle.ConstantTimeCompare([]byte(password), []byte(h.password)) == 1
	return usernameMatch && passwordMatch
}

// validateBasicAuth 验证BasicAuth头
func (h *BasicAuthHandler) validateBasicAuth(authHeader string) bool {
	if !strings.HasPrefix(authHeader, "Basic ") {
		return false
	}

	decoded, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(authHeader, "Basic "))
	if err != nil {
		return false
	}

	parts := strings.SplitN(string(decoded), ":", 2)
	if len(parts) != 2 {
		return false
	}

	return h.validateCredentials(parts[0], parts[1])
}

// localUserInfo 本地用户信息
func (h *BasicAuthHandler) localUserInfo() *UserInfo {
	return &UserInfo{
		Subject: h.username,
		Name:    h.username,
		Email:   h.username + "@local",
	}
}

// handleStatus 返回认证状态
func (h *BasicAuthHandler) handleStatus(c echo.Context) error {
	result := h.buildStatus(c)
	result["enabled"] = true
	result["type"] = AuthTypeBasic

	// BasicAuth额外支持浏览器内置认证（Authorization头）
	if result["loggedIn"] == false {
		authHeader := c.Request().Header.Get("Authorization")
		if authHeader != "" && h.validateBasicAuth(authHeader) {
			result["loggedIn"] = true
			result["user"] = h.localUserInfo()
		}
	}

	return respondSuccess(c, result)
}

// AuthMiddleware 认证中间件
func (h *BasicAuthHandler) AuthMiddleware() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			path := c.Request().URL.Path
			if isExemptPath(path) {
				return next(c)
			}

			// 先检查session
			session, err := h.getSession(c)
			if err == nil && session != nil {
				c.Set("user", session.UserInfo)
				c.Set("session", session)
				return next(c)
			}

			// BasicAuth额外支持浏览器内置认证和Authorization头
			authHeader := c.Request().Header.Get("Authorization")
			if authHeader != "" && h.validateBasicAuth(authHeader) {
				c.Set("user", h.localUserInfo())
				return next(c)
			}

			// 未认证
			if strings.HasPrefix(path, "/api/") {
				return respondError(c, http.StatusUnauthorized, "未授权访问，请先登录")
			}

			// 前端请求，BasicAuth返回401让浏览器弹出登录框
			c.Response().Header().Set("WWW-Authenticate", `Basic realm="NetBouncer"`)
			return c.NoContent(http.StatusUnauthorized)
		}
	}
}
