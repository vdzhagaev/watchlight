package monitor

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/render"
	"gitlab.com/l0veme-projects/uptime-monitor/internal/domain"
	resp "gitlab.com/l0veme-projects/uptime-monitor/internal/lib/api/response"
	"gitlab.com/l0veme-projects/uptime-monitor/internal/lib/logger/sl"
)

type ListResponse struct {
	resp.Response
	MonitorList []domain.Monitor `json:"monitor_list"`
}

type MonitorList interface {
	GetMonitorList(ctx context.Context) ([]domain.Monitor, error)
}

func (h *MonitorHandler) List(w http.ResponseWriter, r *http.Request) {
	const op = "http-server.handlers.monitor.NewList"

	log := h.log.With(slog.String("op", op), slog.String("request_id", middleware.GetReqID(r.Context())))

	monitors, err := h.finder.GetMonitorList(r.Context())

	if err != nil {
		msg := "failed to list monitors"
		log.Error(msg, sl.Err(err))
		render.JSON(w, r, resp.Error(msg))
		return
	}

	render.JSON(w, r, ListResponse{resp.OK(), monitors})
}
