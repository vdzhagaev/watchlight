package monitorhandler

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"

	"vdzhagev/go-uptime-checker/internal/lib/logger/sl"
	"vdzhagev/go-uptime-checker/internal/monitor"
	"vdzhagev/go-uptime-checker/internal/storage"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/render"
	"github.com/google/uuid"

	resp "vdzhagev/go-uptime-checker/internal/lib/api/response"
)

type FindResponse struct {
	resp.Response
	Monitor monitor.Monitor `json:"monitor"`
}

func (h *MonitorHandler) Find(w http.ResponseWriter, r *http.Request) {
	const op = "http-server.handlers.monitor.NewFind"

	log := h.log.With(slog.String("op", op), slog.String("request_id", middleware.GetReqID(r.Context())))

	idStr := chi.URLParam(r, "monitorID")
	id, err := uuid.Parse(idStr)
	if err != nil {
		msg := fmt.Sprintf("failed to parse id: %s", idStr)
		log.Error(msg, sl.Err(err))
		render.JSON(w, r, resp.Error(msg))
		return

	}

	m, err := h.svc.Get(r.Context(), id)

	if errors.Is(err, storage.ErrMonitorNotFound) {
		render.Status(r, http.StatusNotFound)
		render.JSON(w, r, resp.Error("not found"))
		return
	}

	if err != nil {
		msg := fmt.Sprintf("failed to find monitor by id: %s", idStr)
		log.Error(msg, sl.Err(err))
		render.JSON(w, r, resp.Error(msg))
		return
	}

	render.JSON(w, r, FindResponse{resp.OK(), m})
}
