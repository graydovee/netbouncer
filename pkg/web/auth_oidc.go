package web

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/labstack/echo/v4"
	"golang.org/x/oauth2"
)

// oidcLoginStateTTL OIDC登录流程中state会话的有效期
const oidcLoginStateTTL = 10 * time.Minute

// ==================== OIDC Auth Handler ====================

// OidcAuthHandler OIDC认证处理器
type OidcAuthHandler struct {
	baseAuthHandler
	provider     *oidc.Provider
	oauth2Config *oauth2.Config
	verifier     *oidc.IDTokenVerifier
}

// newOidcAuthHandler 创建OIDC处理器
func newOidcAuthHandler(ctx context.Context, cfg *AuthConfig) (*OidcAuthHandler, error) {
	if cfg.ClientID == "" || cfg.IssuerURL == "" || cfg.RedirectURL == "" {
		return nil, fmt.Errorf("OIDC配置不完整: client_id, issuer_url 和 redirect_url 是必需的")
	}

	provider, err := oidc.NewProvider(ctx, cfg.IssuerURL)
	if err != nil {
		return nil, fmt.Errorf("创建OIDC提供者失败: %w", err)
	}

	handler := &OidcAuthHandler{
		baseAuthHandler: baseAuthHandler{
			enabled:  true,
			sessions: newSessionStore(),
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
		return respondError(c, http.StatusInternalServerError, "内部服务器错误")
	}

	codeVerifier, err := generateCodeVerifier()
	if err != nil {
		slog.Error("生成code_verifier失败", "error", err)
		return respondError(c, http.StatusInternalServerError, "内部服务器错误")
	}

	session := &Session{
		ID:           newSessionID(),
		State:        state,
		CodeVerifier: codeVerifier,
		ExpiresAt:    time.Now().Add(oidcLoginStateTTL),
	}
	h.createSession(session)

	h.createSessionCookie(c, session.ID, int(oidcLoginStateTTL.Seconds()))

	opts := []oauth2.AuthCodeOption{
		oauth2.SetAuthURLParam("code_challenge", generateCodeChallenge(codeVerifier)),
		oauth2.SetAuthURLParam("code_challenge_method", "S256"),
	}
	authURL := h.oauth2Config.AuthCodeURL(state, opts...)

	slog.Info("开始OIDC登录")
	return c.Redirect(http.StatusTemporaryRedirect, authURL)
}

// handleCallback 处理OIDC回调
func (h *OidcAuthHandler) handleCallback(c echo.Context) error {
	cookie, err := c.Cookie(sessionCookieName)
	if err != nil {
		slog.Error("获取session cookie失败", "error", err)
		return c.Redirect(http.StatusTemporaryRedirect, "/auth/login")
	}

	session, ok := h.sessions.get(cookie.Value)
	if !ok {
		slog.Error("session不存在")
		return c.Redirect(http.StatusTemporaryRedirect, "/auth/login")
	}

	state := c.QueryParam("state")
	if state != session.State {
		slog.Error("state不匹配")
		h.sessions.delete(cookie.Value)
		return respondError(c, http.StatusBadRequest, "无效的认证请求")
	}

	if errParam := c.QueryParam("error"); errParam != "" {
		slog.Error("OIDC认证失败", "error", errParam)
		h.sessions.delete(cookie.Value)
		return respondError(c, http.StatusBadRequest, fmt.Sprintf("认证失败: %s", errParam))
	}

	code := c.QueryParam("code")
	if code == "" {
		h.sessions.delete(cookie.Value)
		return respondError(c, http.StatusBadRequest, "未收到授权码")
	}

	opts := []oauth2.AuthCodeOption{
		oauth2.SetAuthURLParam("code_verifier", session.CodeVerifier),
	}
	token, err := h.oauth2Config.Exchange(c.Request().Context(), code, opts...)
	if err != nil {
		slog.Error("交换token失败", "error", err)
		h.sessions.delete(cookie.Value)
		return respondError(c, http.StatusInternalServerError, "认证失败")
	}

	idToken, ok := token.Extra("id_token").(string)
	if !ok {
		h.sessions.delete(cookie.Value)
		return respondError(c, http.StatusInternalServerError, "未收到ID Token")
	}

	verifiedIDToken, err := h.verifier.Verify(c.Request().Context(), idToken)
	if err != nil {
		slog.Error("验证ID Token失败", "error", err)
		h.sessions.delete(cookie.Value)
		return respondError(c, http.StatusInternalServerError, "Token验证失败")
	}

	var claims struct {
		Email         string `json:"email"`
		Name          string `json:"name"`
		Picture       string `json:"picture"`
		EmailVerified bool   `json:"email_verified"`
	}
	if err := verifiedIDToken.Claims(&claims); err != nil {
		h.sessions.delete(cookie.Value)
		return respondError(c, http.StatusInternalServerError, "解析用户信息失败")
	}

	session.Token = token
	session.IDToken = idToken
	session.UserInfo = &UserInfo{
		Subject: verifiedIDToken.Subject,
		Email:   claims.Email,
		Name:    claims.Name,
		Picture: claims.Picture,
	}
	session.ExpiresAt = time.Now().Add(sessionTTL)

	slog.Info("用户登录成功", "email", claims.Email)

	h.createSessionCookie(c, session.ID, int(sessionTTL.Seconds()))

	return c.Redirect(http.StatusTemporaryRedirect, "/")
}

// handleStatus 返回认证状态
func (h *OidcAuthHandler) handleStatus(c echo.Context) error {
	result := h.buildStatus(c)
	result["enabled"] = true
	result["type"] = AuthTypeOIDC

	return respondSuccess(c, result)
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
				h.sessions.delete(session.ID)
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
				return respondError(c, http.StatusUnauthorized, "未授权访问，请先登录")
			}

			return c.Redirect(http.StatusTemporaryRedirect, "/auth/login")
		}
	}
}
