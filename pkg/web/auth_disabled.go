package web

import (
	"net/http"

	"github.com/labstack/echo/v4"
)

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
		return c.JSON(http.StatusOK, Success(map[string]any{
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
