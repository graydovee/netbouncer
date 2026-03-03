package web

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"golang.org/x/oauth2"
)

// AuthType 认证类型
const (
	AuthTypeOIDC  = "oidc"
	AuthTypeBasic = "basic"
)

// AuthHandler 认证处理器接口
type AuthHandler interface {
	// IsEnabled 返回认证是否启用
	IsEnabled() bool
	// AuthType 返回认证类型
	AuthType() string
	// RegisterRoutes 注册认证相关路由
	RegisterRoutes(e *echo.Echo)
	// AuthMiddleware 认证中间件
	AuthMiddleware() echo.MiddlewareFunc
	// CleanupSessions 清理过期session
	CleanupSessions()
	// getSession 从请求中获取session（内部使用）
	getSession(c echo.Context) (*Session, error)
}

// Session 存储用户会话信息
type Session struct {
	ID           string
	Token        *oauth2.Token
	IDToken      string
	UserInfo     *UserInfo
	ExpiresAt    time.Time
	State        string // OAuth2 state参数
	CodeVerifier string // PKCE code_verifier
}

// UserInfo 用户信息
type UserInfo struct {
	Subject string `json:"sub"`
	Email   string `json:"email"`
	Name    string `json:"name"`
	Picture string `json:"picture"`
}

// AuthConfig 认证配置
type AuthConfig struct {
	Enabled       bool
	Type          string // "oidc" 或 "basic"
	ClientID      string
	ClientSecret  string
	IssuerURL     string
	RedirectURL   string
	SessionSecret string
	// BasicAuth配置
	BasicUsername string
	BasicPassword string
}

// baseAuthHandler 基础认证处理器，包含通用字段和方法
type baseAuthHandler struct {
	enabled    bool
	sessions   map[string]*Session
	sessionKey string
}

// IsEnabled 返回认证是否启用
func (h *baseAuthHandler) IsEnabled() bool {
	return h.enabled
}

// CleanupSessions 清理过期session
func (h *baseAuthHandler) CleanupSessions() {
	now := time.Now()
	for id, session := range h.sessions {
		if now.After(session.ExpiresAt) {
			delete(h.sessions, id)
		}
	}
}

// getSession 从请求中获取session
func (h *baseAuthHandler) getSession(c echo.Context) (*Session, error) {
	if !h.enabled {
		return nil, nil
	}

	cookie, err := c.Cookie("netbouncer_session")
	if err != nil {
		return nil, err
	}

	session, ok := h.sessions[cookie.Value]
	if !ok {
		return nil, fmt.Errorf("session不存在")
	}

	if time.Now().After(session.ExpiresAt) {
		delete(h.sessions, cookie.Value)
		return nil, fmt.Errorf("session已过期")
	}

	return session, nil
}

// createSessionCookie 创建session cookie
func (h *baseAuthHandler) createSessionCookie(c echo.Context, sessionID string, maxAge int) {
	c.SetCookie(&http.Cookie{
		Name:     "netbouncer_session",
		Value:    sessionID,
		Path:     "/",
		HttpOnly: true,
		Secure:   false,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   maxAge,
	})
}

// clearSessionCookie 清除session cookie
func (h *baseAuthHandler) clearSessionCookie(c echo.Context) {
	c.SetCookie(&http.Cookie{
		Name:     "netbouncer_session",
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		MaxAge:   -1,
	})
}

// isExemptPath 检查是否是豁免路径
func isExemptPath(path string) bool {
	exemptPaths := []string{
		"/auth/login",
		"/auth/callback",
		"/auth/status",
		"/auth/logout",
	}

	for _, exempt := range exemptPaths {
		if path == exempt || strings.HasPrefix(path, exempt+"/") {
			return true
		}
	}

	return false
}

// NewAuthHandler 创建认证处理器
func NewAuthHandler(ctx context.Context, cfg *AuthConfig) (AuthHandler, error) {
	if !cfg.Enabled {
		return &disabledAuthHandler{
			baseAuthHandler: baseAuthHandler{
				enabled:  false,
				sessions: make(map[string]*Session),
			},
		}, nil
	}

	// 默认认证类型为basic
	authType := cfg.Type
	if authType == "" {
		authType = AuthTypeBasic
	}

	sessionKey := cfg.SessionSecret
	if sessionKey == "" {
		sessionKey = uuid.New().String()
	}

	// 根据认证类型创建具体实现
	switch authType {
	case AuthTypeBasic:
		return newBasicAuthHandler(cfg, sessionKey)
	case AuthTypeOIDC:
		return newOidcAuthHandler(ctx, cfg, sessionKey)
	default:
		return nil, fmt.Errorf("不支持的认证类型: %s", authType)
	}
}

// ==================== Disabled Auth Handler ====================

// disabledAuthHandler 禁用认证的处理器
type disabledAuthHandler struct {
	baseAuthHandler
}

// AuthType 返回认证类型
func (h *disabledAuthHandler) AuthType() string {
	return "disabled"
}

// RegisterRoutes 注册认证相关路由
func (h *disabledAuthHandler) RegisterRoutes(e *echo.Echo) {
	e.GET("/auth/login", func(c echo.Context) error {
		return c.Redirect(http.StatusTemporaryRedirect, "/")
	})
	e.GET("/auth/status", func(c echo.Context) error {
		return c.JSON(http.StatusOK, Success(map[string]interface{}{
			"enabled": false,
		}))
	})
}

// AuthMiddleware 认证中间件
func (h *disabledAuthHandler) AuthMiddleware() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			return next(c)
		}
	}
}

// ==================== Basic Auth Handler ====================

// BasicAuthHandler BasicAuth认证处理器
type BasicAuthHandler struct {
	baseAuthHandler
	username string
	password string
}

// newBasicAuthHandler 创建BasicAuth处理器
func newBasicAuthHandler(cfg *AuthConfig, sessionKey string) (*BasicAuthHandler, error) {
	if cfg.BasicUsername == "" || cfg.BasicPassword == "" {
		return nil, fmt.Errorf("BasicAuth配置不完整: username 和 password 是必需的")
	}

	handler := &BasicAuthHandler{
		baseAuthHandler: baseAuthHandler{
			enabled:    true,
			sessions:   make(map[string]*Session),
			sessionKey: sessionKey,
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
		if authHeader != "" {
			// 验证BasicAuth
			if h.validateBasicAuth(authHeader) {
				return c.Redirect(http.StatusTemporaryRedirect, "/")
			}
		}
		// 返回401要求认证
		c.Response().Header().Set("WWW-Authenticate", `Basic realm="NetBouncer"`)
		return c.JSON(http.StatusUnauthorized, Error(401, "需要认证"))
	}

	// POST请求处理登录
	var loginReq struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := c.Bind(&loginReq); err != nil {
		return c.JSON(http.StatusBadRequest, Error(400, "参数错误"))
	}

	// 验证用户名密码
	if loginReq.Username != h.username || loginReq.Password != h.password {
		slog.Warn("登录失败", "username", loginReq.Username)
		return c.JSON(http.StatusUnauthorized, Error(401, "用户名或密码错误"))
	}

	// 创建session
	sessionID := uuid.New().String()
	h.sessions[sessionID] = &Session{
		ID:        sessionID,
		UserInfo:  &UserInfo{Subject: loginReq.Username, Name: loginReq.Username, Email: loginReq.Username + "@local"},
		ExpiresAt: time.Now().Add(24 * time.Hour),
	}

	// 设置session cookie
	h.createSessionCookie(c, sessionID, 86400)

	slog.Info("用户登录成功", "username", loginReq.Username)
	return c.JSON(http.StatusOK, Success(map[string]interface{}{
		"message": "登录成功",
		"user":    h.sessions[sessionID].UserInfo,
	}))
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

	username := parts[0]
	password := parts[1]

	// 使用恒定时间比较防止时序攻击
	usernameMatch := subtle.ConstantTimeCompare([]byte(username), []byte(h.username)) == 1
	passwordMatch := subtle.ConstantTimeCompare([]byte(password), []byte(h.password)) == 1

	return usernameMatch && passwordMatch
}

// handleStatus 返回认证状态
func (h *BasicAuthHandler) handleStatus(c echo.Context) error {
	result := map[string]interface{}{
		"enabled":  true,
		"type":     AuthTypeBasic,
		"loggedIn": false,
	}

	// 检查session
	session, err := h.getSession(c)
	if err == nil && session != nil {
		result["loggedIn"] = true
		result["user"] = session.UserInfo
	}

	// BasicAuth额外支持浏览器内置认证
	if result["loggedIn"] == false {
		authHeader := c.Request().Header.Get("Authorization")
		if authHeader != "" && h.validateBasicAuth(authHeader) {
			result["loggedIn"] = true
			result["user"] = &UserInfo{
				Subject: h.username,
				Name:    h.username,
				Email:   h.username + "@local",
			}
		}
	}

	return c.JSON(http.StatusOK, Success(result))
}

// handleLogout 处理登出请求
func (h *BasicAuthHandler) handleLogout(c echo.Context) error {
	cookie, err := c.Cookie("netbouncer_session")
	if err == nil {
		delete(h.sessions, cookie.Value)
	}

	h.clearSessionCookie(c)

	slog.Info("用户登出成功")

	if c.Request().Header.Get("Content-Type") == "application/json" ||
		strings.HasPrefix(c.Request().Header.Get("Accept"), "application/json") {
		return c.JSON(http.StatusOK, Success("登出成功"))
	}

	return c.Redirect(http.StatusTemporaryRedirect, "/")
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
				c.Set("user", &UserInfo{
					Subject: h.username,
					Name:    h.username,
					Email:   h.username + "@local",
				})
				return next(c)
			}

			// 未认证
			if strings.HasPrefix(path, "/api/") {
				return c.JSON(http.StatusUnauthorized, Error(401, "未授权访问，请先登录"))
			}

			// 前端请求，BasicAuth返回401让浏览器弹出登录框
			c.Response().Header().Set("WWW-Authenticate", `Basic realm="NetBouncer"`)
			return c.NoContent(http.StatusUnauthorized)
		}
	}
}

// ==================== OIDC Auth Handler ====================

// OidcAuthHandler OIDC认证处理器
type OidcAuthHandler struct {
	baseAuthHandler
	provider     *oidc.Provider
	oauth2Config *oauth2.Config
	verifier     *oidc.IDTokenVerifier
}

// newOidcAuthHandler 创建OIDC处理器
func newOidcAuthHandler(ctx context.Context, cfg *AuthConfig, sessionKey string) (*OidcAuthHandler, error) {
	if cfg.ClientID == "" || cfg.IssuerURL == "" || cfg.RedirectURL == "" {
		return nil, fmt.Errorf("OIDC配置不完整: client_id, issuer_url 和 redirect_url 是必需的")
	}

	provider, err := oidc.NewProvider(ctx, cfg.IssuerURL)
	if err != nil {
		return nil, fmt.Errorf("创建OIDC提供者失败: %w", err)
	}

	handler := &OidcAuthHandler{
		baseAuthHandler: baseAuthHandler{
			enabled:    true,
			sessions:   make(map[string]*Session),
			sessionKey: sessionKey,
		},
		provider: provider,
		oauth2Config: &oauth2.Config{
			ClientID:     cfg.ClientID,
			ClientSecret: cfg.ClientSecret,
			RedirectURL:  cfg.RedirectURL,
			Endpoint:     provider.Endpoint(),
			Scopes:       []string{oidc.ScopeOpenID, "profile", "email"},
		},
		verifier: provider.Verifier(&oidc.Config{
			ClientID: cfg.ClientID,
		}),
	}

	slog.Info("OIDC认证已启用", "issuer", cfg.IssuerURL)
	return handler, nil
}

// AuthType 返回认证类型
func (h *OidcAuthHandler) AuthType() string {
	return AuthTypeOIDC
}

// RegisterRoutes 注册认证相关路由
func (h *OidcAuthHandler) RegisterRoutes(e *echo.Echo) {
	e.GET("/auth/login", h.handleLogin)
	e.POST("/auth/login", h.handleLogin)
	e.GET("/auth/callback", h.handleCallback)
	e.GET("/auth/logout", h.handleLogout)
	e.POST("/auth/logout", h.handleLogout)
	e.GET("/auth/status", h.handleStatus)
}

// handleLogin 处理OIDC登录
func (h *OidcAuthHandler) handleLogin(c echo.Context) error {
	state, err := generateRandomState()
	if err != nil {
		slog.Error("生成state失败", "error", err)
		return c.JSON(http.StatusInternalServerError, Error(500, "内部服务器错误"))
	}

	codeVerifier, err := generateCodeVerifier()
	if err != nil {
		slog.Error("生成code_verifier失败", "error", err)
		return c.JSON(http.StatusInternalServerError, Error(500, "内部服务器错误"))
	}

	sessionID := uuid.New().String()
	h.sessions[sessionID] = &Session{
		ID:           sessionID,
		State:        state,
		CodeVerifier: codeVerifier,
		ExpiresAt:    time.Now().Add(10 * time.Minute),
	}

	h.createSessionCookie(c, sessionID, 600)

	opts := []oauth2.AuthCodeOption{
		oauth2.SetAuthURLParam("code_challenge", generateCodeChallenge(codeVerifier)),
		oauth2.SetAuthURLParam("code_challenge_method", "S256"),
	}
	authURL := h.oauth2Config.AuthCodeURL(state, opts...)

	slog.Info("开始OIDC登录", "state", state)
	return c.Redirect(http.StatusTemporaryRedirect, authURL)
}

// handleCallback 处理OIDC回调
func (h *OidcAuthHandler) handleCallback(c echo.Context) error {
	cookie, err := c.Cookie("netbouncer_session")
	if err != nil {
		slog.Error("获取session cookie失败", "error", err)
		return c.Redirect(http.StatusTemporaryRedirect, "/auth/login")
	}

	session, ok := h.sessions[cookie.Value]
	if !ok {
		slog.Error("session不存在")
		return c.Redirect(http.StatusTemporaryRedirect, "/auth/login")
	}

	state := c.QueryParam("state")
	if state != session.State {
		slog.Error("state不匹配")
		delete(h.sessions, cookie.Value)
		return c.JSON(http.StatusBadRequest, Error(400, "无效的认证请求"))
	}

	if errParam := c.QueryParam("error"); errParam != "" {
		slog.Error("OIDC认证失败", "error", errParam)
		delete(h.sessions, cookie.Value)
		return c.JSON(http.StatusBadRequest, Error(400, fmt.Sprintf("认证失败: %s", errParam)))
	}

	code := c.QueryParam("code")
	if code == "" {
		delete(h.sessions, cookie.Value)
		return c.JSON(http.StatusBadRequest, Error(400, "未收到授权码"))
	}

	opts := []oauth2.AuthCodeOption{
		oauth2.SetAuthURLParam("code_verifier", session.CodeVerifier),
	}
	token, err := h.oauth2Config.Exchange(c.Request().Context(), code, opts...)
	if err != nil {
		slog.Error("交换token失败", "error", err)
		delete(h.sessions, cookie.Value)
		return c.JSON(http.StatusInternalServerError, Error(500, "认证失败"))
	}

	idToken, ok := token.Extra("id_token").(string)
	if !ok {
		delete(h.sessions, cookie.Value)
		return c.JSON(http.StatusInternalServerError, Error(500, "未收到ID Token"))
	}

	verifiedIDToken, err := h.verifier.Verify(c.Request().Context(), idToken)
	if err != nil {
		slog.Error("验证ID Token失败", "error", err)
		delete(h.sessions, cookie.Value)
		return c.JSON(http.StatusInternalServerError, Error(500, "Token验证失败"))
	}

	var claims struct {
		Email         string `json:"email"`
		Name          string `json:"name"`
		Picture       string `json:"picture"`
		EmailVerified bool   `json:"email_verified"`
	}
	if err := verifiedIDToken.Claims(&claims); err != nil {
		delete(h.sessions, cookie.Value)
		return c.JSON(http.StatusInternalServerError, Error(500, "解析用户信息失败"))
	}

	session.Token = token
	session.IDToken = idToken
	session.UserInfo = &UserInfo{
		Subject: verifiedIDToken.Subject,
		Email:   claims.Email,
		Name:    claims.Name,
		Picture: claims.Picture,
	}
	session.ExpiresAt = time.Now().Add(24 * time.Hour)

	slog.Info("用户登录成功", "email", claims.Email)

	h.createSessionCookie(c, session.ID, 86400)

	return c.Redirect(http.StatusTemporaryRedirect, "/")
}

// handleStatus 返回认证状态
func (h *OidcAuthHandler) handleStatus(c echo.Context) error {
	result := map[string]interface{}{
		"enabled":  true,
		"type":     AuthTypeOIDC,
		"loggedIn": false,
	}

	// 检查session
	session, err := h.getSession(c)
	if err == nil && session != nil {
		result["loggedIn"] = true
		result["user"] = session.UserInfo
	}

	return c.JSON(http.StatusOK, Success(result))
}

// handleLogout 处理登出请求
func (h *OidcAuthHandler) handleLogout(c echo.Context) error {
	cookie, err := c.Cookie("netbouncer_session")
	if err == nil {
		delete(h.sessions, cookie.Value)
	}

	h.clearSessionCookie(c)

	slog.Info("用户登出成功")

	if c.Request().Header.Get("Content-Type") == "application/json" ||
		strings.HasPrefix(c.Request().Header.Get("Accept"), "application/json") {
		return c.JSON(http.StatusOK, Success("登出成功"))
	}

	return c.Redirect(http.StatusTemporaryRedirect, "/")
}

// getSession 重写getSession以支持token刷新
func (h *OidcAuthHandler) getSession(c echo.Context) (*Session, error) {
	session, err := h.baseAuthHandler.getSession(c)
	if err != nil || session == nil {
		return session, err
	}

	// OIDC token刷新
	if session.Token != nil && !session.Token.Valid() {
		if session.Token.RefreshToken != "" {
			newToken, err := h.oauth2Config.TokenSource(c.Request().Context(), session.Token).Token()
			if err != nil {
				delete(h.sessions, session.ID)
				return nil, fmt.Errorf("刷新token失败: %w", err)
			}
			session.Token = newToken
		}
	}

	return session, nil
}

// AuthMiddleware 认证中间件
func (h *OidcAuthHandler) AuthMiddleware() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			path := c.Request().URL.Path
			if isExemptPath(path) {
				return next(c)
			}

			// 检查session
			session, err := h.getSession(c)
			if err == nil && session != nil {
				c.Set("user", session.UserInfo)
				c.Set("session", session)
				return next(c)
			}

			// 未认证
			if strings.HasPrefix(path, "/api/") {
				return c.JSON(http.StatusUnauthorized, Error(401, "未授权访问，请先登录"))
			}

			return c.Redirect(http.StatusTemporaryRedirect, "/auth/login")
		}
	}
}

// ==================== Helper Functions ====================

// generateRandomState 生成随机state参数
func generateRandomState() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(b), nil
}

// generateCodeVerifier 生成PKCE code_verifier
func generateCodeVerifier() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.URLEncoding.WithPadding(base64.NoPadding).EncodeToString(b), nil
}

// generateCodeChallenge 从code_verifier生成code_challenge (S256)
func generateCodeChallenge(verifier string) string {
	h := sha256.Sum256([]byte(verifier))
	return base64.URLEncoding.WithPadding(base64.NoPadding).EncodeToString(h[:])
}

// GetUserInfo 从context获取用户信息
func GetUserInfo(c echo.Context) *UserInfo {
	user, ok := c.Get("user").(*UserInfo)
	if !ok {
		return nil
	}
	return user
}

// MarshalJSON 实现自定义JSON序列化
func (s *Session) MarshalJSON() ([]byte, error) {
	return json.Marshal(map[string]interface{}{
		"id":        s.ID,
		"expiresAt": s.ExpiresAt,
		"userInfo":  s.UserInfo,
	})
}
