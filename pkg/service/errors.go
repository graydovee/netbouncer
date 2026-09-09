package service

import (
	"errors"
	"fmt"
)

// 业务错误哨兵，web 层据此映射 HTTP 状态码
var (
	// ErrBadRequest 请求参数不合法
	ErrBadRequest = errors.New("请求参数不合法")
	// ErrNotFound 请求的资源不存在
	ErrNotFound = errors.New("资源不存在")
	// ErrConflict 资源状态冲突（如重复创建）
	ErrConflict = errors.New("资源已存在")
	// ErrInternal 内部错误（存储/防火墙操作失败等）
	ErrInternal = errors.New("内部错误")
)

func Invalidf(format string, args ...any) error {
	return fmt.Errorf("%w: %s", ErrBadRequest, fmt.Sprintf(format, args...))
}

func NotFoundf(format string, args ...any) error {
	return fmt.Errorf("%w: %s", ErrNotFound, fmt.Sprintf(format, args...))
}

func Conflictf(format string, args ...any) error {
	return fmt.Errorf("%w: %s", ErrConflict, fmt.Sprintf(format, args...))
}

func Internalf(format string, args ...any) error {
	return fmt.Errorf("%w: %s", ErrInternal, fmt.Sprintf(format, args...))
}
