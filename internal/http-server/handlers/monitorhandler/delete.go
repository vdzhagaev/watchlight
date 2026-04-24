package monitorhandler

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/render"
	"github.com/google/uuid"
	resp "github.com/vdzhagaev/watchlight/internal/lib/api/response"
	"github.com/vdzhagaev/watchlight/internal/lib/logger/sl"
	"github.com/vdzhagaev/watchlight/internal/monitor"
)

func (h *MonitorHandler) Delete(w http.ResponseWriter, r *http.Request) {
	const op = "http-server.handlers.monitor.delete"
	log := h.log.With(slog.String("op", op), slog.String("request_id", middleware.GetReqID(r.Context())))


	idStr := chi.URLParam(r, "monitorID")
	id, err := uuid.Parse(idStr)
	if err != nil {
		log.Error("failed to parse id", slog.String("id", idStr), sl.Err(err))
		resp.WriteError(w, r, http.StatusBadRequest, "invalid id")
		return
	}

	if err := h.svc.Delete(r.Context(), id); err != nil {
		if errors.Is(err, monitor.ErrMonitorNotFound) {
			resp.WriteError(w, r, http.StatusNotFound, "monitor not found")
			return
		}
		log.Error("failed to delete monitor", sl.Err(err))
		resp.WriteError(w, r, http.StatusInternalServerError, "internal error")
		return
	}

	log.Info("monitor deleted",
		slog.String("id", idStr),
	)

	render.Status(r, 204)
	render.JSON(w, r, nil)
}
