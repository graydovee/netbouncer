package web

import (
	"log/slog"
	"net"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
)

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

// validateIpNet验证输入是否为有效的IP地址或CIDR格式
func validateIpNet(input string) error {
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
