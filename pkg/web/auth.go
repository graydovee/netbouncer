package web

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"golang.org/x/oauth2"
)

// AuthType 认证类型
const (
	AuthTypeOIDC  = "oidc"
	AuthTypeBasic = "basic"
)

// sessionCookieName 会话Cookie名称
const sessionCookieName = "netbouncer_session"

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

// expired 判断会话是否已过期
func (s *Session) expired() bool {
	return time.Now().After(s.ExpiresAt)
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

// sessionStore 并发安全的会话存储
type sessionStore struct {
	mu       sync.RWMutex
	sessions map[string]*Session
}

func newSessionStore() *sessionStore {
	return &sessionStore{sessions: make(map[string]*Session)}
}

func (s *sessionStore) get(id string) (*Session, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	session, ok := s.sessions[id]
	return session, ok
}

func (s *sessionStore) put(session *Session) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sessions[session.ID] = session
}

func (s *sessionStore) delete(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.sessions, id)
}

func (s *sessionStore) deleteExpired() int {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	removed := 0
	for id, session := range s.sessions {
		if now.After(session.ExpiresAt) {
			delete(s.sessions, id)
			removed++
		}
	}
	return removed
}

func (s *sessionStore) len() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.sessions)
}

// baseAuthHandler 基础认证处理器，包含通用字段和方法
type baseAuthHandler struct {
	enabled  bool
	sessions *sessionStore
}

// IsEnabled 返回认证是否启用
func (h *baseAuthHandler) IsEnabled() bool {
	return h.enabled
}

// CleanupSessions 清理过期session
func (h *baseAuthHandler) CleanupSessions() {
	if removed := h.sessions.deleteExpired(); removed > 0 {
		slog.Info("清理过期session", "removed", removed, "remaining", h.sessions.len())
	}
}

// getSession 从请求中获取session
func (h *baseAuthHandler) getSession(c echo.Context) (*Session, error) {
	if !h.enabled {
		return nil, nil
	}

	cookie, err := c.Cookie(sessionCookieName)
	if err != nil {
		return nil, fmt.Errorf("session cookie不存在")
	}

	session, ok := h.sessions.get(cookie.Value)
	if !ok {
		return nil, fmt.Errorf("session不存在")
	}

	if session.expired() {
		h.sessions.delete(cookie.Value)
		return nil, fmt.Errorf("session已过期")
	}

	return session, nil
}

// createSession 创建并存储新会话
func (h *baseAuthHandler) createSession(session *Session) {
	h.sessions.put(session)
}

// buildStatus 构造认证状态响应（session有效时填充用户信息）
func (h *baseAuthHandler) buildStatus(c echo.Context) map[string]any {
	result := map[string]any{
		"loggedIn": false,
	}

	session, err := h.getSession(c)
	if err == nil && session != nil {
		result["loggedIn"] = true
		result["user"] = session.UserInfo
	}

	return result
}

// handleLogout 处理登出请求（所有认证类型通用）
func (h *baseAuthHandler) handleLogout(c echo.Context) error {
	if cookie, err := c.Cookie(sessionCookieName); err == nil {
		h.sessions.delete(cookie.Value)
	}

	h.clearSessionCookie(c)

	slog.Info("用户登出成功")

	if c.Request().Header.Get("Content-Type") == "application/json" ||
		strings.HasPrefix(c.Request().Header.Get("Accept"), "application/json") {
		return c.JSON(http.StatusOK, Success("登出成功"))
	}

	return c.Redirect(http.StatusTemporaryRedirect, "/")
}

// createSessionCookie 创建session cookie
func (h *baseAuthHandler) createSessionCookie(c echo.Context, sessionID string, maxAge int) {
	c.SetCookie(&http.Cookie{
		Name:     sessionCookieName,
		Value:    sessionID,
		Path:     "/",
		HttpOnly: true,
		// 部署在 HTTPS 后时建议启用 Secure；这里默认关闭以兼容纯 HTTP 的内网部署
		Secure:   false,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   maxAge,
	})
}

// clearSessionCookie 清除session cookie
func (h *baseAuthHandler) clearSessionCookie(c echo.Context) {
	c.SetCookie(&http.Cookie{
		Name:     sessionCookieName,
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
				sessions: newSessionStore(),
			},
		}, nil
	}

	// 默认认证类型为basic
	authType := cfg.Type
	if authType == "" {
		authType = AuthTypeBasic
	}

	// 根据认证类型创建具体实现
	switch authType {
	case AuthTypeBasic:
		return newBasicAuthHandler(cfg)
	case AuthTypeOIDC:
		return newOidcAuthHandler(ctx, cfg)
	default:
		return nil, fmt.Errorf("不支持的认证类型: %s", authType)
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

// newSessionID 生成新的会话ID
func newSessionID() string {
	return uuid.New().String()
}
