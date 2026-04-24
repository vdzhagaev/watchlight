package monitorhandler

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/vdzhagaev/watchlight/internal/lib/logger/sl"
	"github.com/vdzhagaev/watchlight/internal/monitor"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/render"
	"github.com/google/uuid"

	resp "github.com/vdzhagaev/watchlight/internal/lib/api/response"
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
		log.Error("failed to parse id", slog.String("id", idStr), sl.Err(err))
		resp.WriteError(w, r, http.StatusBadRequest, "invalid id")
		return
	}

	m, err := h.svc.Get(r.Context(), id)

	if errors.Is(err, monitor.ErrMonitorNotFound) {
		resp.WriteError(w, r, http.StatusNotFound, "monitor not found")
		return
	}

	if err != nil {
		log.Error("failed to find monitor", sl.Err(err))
		resp.WriteError(w, r, http.StatusInternalServerError, "internal error")
		return
	}

	render.JSON(w, r, FindResponse{resp.OK(), m})
}
