package monitorhandler

import (
	"log/slog"
	"net/http"

	resp "vdzhagev/go-uptime-checker/internal/lib/api/response"
	"vdzhagev/go-uptime-checker/internal/lib/logger/sl"
	"vdzhagev/go-uptime-checker/internal/monitor"

	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/render"
)

type ListResponse struct {
	resp.Response
	MonitorList []monitor.Monitor `json:"monitor_list"`
}

func (h *MonitorHandler) List(w http.ResponseWriter, r *http.Request) {
	const op = "http-server.handlers.monitor.NewList"

	log := h.log.With(slog.String("op", op), slog.String("request_id", middleware.GetReqID(r.Context())))

	monitors, err := h.svc.List(r.Context())

	if err != nil {
		msg := "failed to list monitors"
		log.Error(msg, sl.Err(err))
		render.JSON(w, r, resp.Error(msg))
		return
	}

	render.JSON(w, r, ListResponse{resp.OK(), monitors})
}
