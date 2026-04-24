package monitorhandler

import (
	"log/slog"
	"net/http"

	resp "github.com/vdzhagaev/watchlight/internal/lib/api/response"
	"github.com/vdzhagaev/watchlight/internal/lib/logger/sl"
	"github.com/vdzhagaev/watchlight/internal/monitor"

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
		log.Error("failed to list monitors", sl.Err(err))
		resp.WriteError(w, r, http.StatusInternalServerError, "internal error")
		return
	}

	render.JSON(w, r, ListResponse{resp.OK(), monitors})
}
