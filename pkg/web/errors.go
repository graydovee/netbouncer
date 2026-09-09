package web

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/labstack/echo/v4"

	"github.com/graydovee/netbouncer/pkg/service"
)

// respondSuccess 输出成功响应（HTTP 200）
func respondSuccess(c echo.Context, data any) error {
	return c.JSON(http.StatusOK, Success(data))
}

// respondError 按给定的 HTTP 状态码输出错误响应
func respondError(c echo.Context, status int, msg string) error {
	return c.JSON(status, Error(status, msg))
}

// respondServiceError 将 service 层错误映射为对应的 HTTP 状态码
func respondServiceError(c echo.Context, err error) error {
	switch {
	case errors.Is(err, service.ErrBadRequest):
		return respondError(c, http.StatusBadRequest, err.Error())
	case errors.Is(err, service.ErrNotFound):
		return respondError(c, http.StatusNotFound, err.Error())
	case errors.Is(err, service.ErrConflict):
		return respondError(c, http.StatusConflict, err.Error())
	default:
		// 内部错误不透出原始错误细节，仅记录日志
		return respondError(c, http.StatusInternalServerError, "内部服务器错误")
	}
}

// bindAndValidate 绑定请求体并执行校验函数，失败时返回 false（响应已写出）
func bindAndValidate(c echo.Context, req any, validate func() error) (bool, error) {
	if err := c.Bind(req); err != nil {
		return false, respondError(c, http.StatusBadRequest, "参数错误")
	}
	if validate != nil {
		if err := validate(); err != nil {
			return false, respondError(c, http.StatusBadRequest, err.Error())
		}
	}
	return true, nil
}

// pathUint 解析路径参数为 uint，失败时输出 400 响应
func pathUint(c echo.Context, name string) (uint, bool, error) {
	id, err := strconv.ParseUint(c.Param(name), 10, 32)
	if err != nil {
		return 0, false, respondError(c, http.StatusBadRequest, "无效的ID参数: "+name)
	}
	return uint(id), true, nil
}

// queryUint 解析查询参数为 uint，未提供时返回 0
func queryUint(c echo.Context, name string) (uint, bool, error) {
	raw := c.QueryParam(name)
	if raw == "" {
		return 0, true, nil
	}
	id, err := strconv.ParseUint(raw, 10, 32)
	if err != nil {
		return 0, false, respondError(c, http.StatusBadRequest, "无效的查询参数: "+name)
	}
	return uint(id), true, nil
}

// queryInt 解析查询参数为 int，未提供时返回默认值
func queryInt(c echo.Context, name string, def int) (int, bool, error) {
	raw := c.QueryParam(name)
	if raw == "" {
		return def, true, nil
	}
	v, err := strconv.Atoi(raw)
	if err != nil {
		return 0, false, respondError(c, http.StatusBadRequest, "无效的查询参数: "+name)
	}
	return v, true, nil
}
