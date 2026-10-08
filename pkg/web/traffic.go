package web

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/graydovee/netbouncer/pkg/history"
	"github.com/labstack/echo/v4"
)

func (s *Server) handleTrafficOverview(c echo.Context) error {
	v, err := s.netService.TrafficOverview()
	if err != nil {
		return respondServiceError(c, err)
	}
	return respondSuccess(c, v)
}
func (s *Server) handleTrafficDetail(c echo.Context) error {
	v, err := s.netService.TrafficDetail(c.Param("ip"))
	if err != nil {
		return respondServiceError(c, err)
	}
	return respondSuccess(c, v)
}
func (s *Server) handleHistoryStatus(c echo.Context) error {
	ctx, cancel := context.WithTimeout(c.Request().Context(), 3*time.Second)
	defer cancel()
	v, err := s.netService.HistoryStatus(ctx)
	if err != nil {
		return respondServiceError(c, err)
	}
	return respondSuccess(c, v)
}
func (s *Server) handleHistoryQuery(c echo.Context, kind string) error {
	p, ok, err := parsePortHistoryParams(c)
	if !ok {
		return err
	}
	limit, ok, err := queryInt(c, "limit", 10)
	if !ok {
		return err
	}
	ctx, cancel := context.WithTimeout(c.Request().Context(), 3*time.Second)
	defer cancel()
	result, err := s.netService.QueryHistory(ctx, history.Query{Start: p.Start, End: p.End, Bucket: p.Bucket, IP: p.IP, Proto: p.Proto, Port: p.Port, Kind: kind, Limit: limit})
	if err != nil {
		if ctx.Err() != nil || errors.Is(err, context.DeadlineExceeded) {
			return respondError(c, http.StatusGatewayTimeout, "历史查询超时，请缩小查询范围")
		}
		return respondServiceError(c, err)
	}
	return respondSuccess(c, result)
}
