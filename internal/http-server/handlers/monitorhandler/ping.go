package monitorhandler

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/render"
	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"
	resp "github.com/vdzhagaev/watchlight/internal/lib/api/response"
	"github.com/vdzhagaev/watchlight/internal/lib/logger/sl"
	"github.com/vdzhagaev/watchlight/internal/monitor"
)

// UpdatePingRequest is the patch body for the intrinsic ping check. A zero port
// is rejected by the domain (ErrInvalidPort); an omitted field is left as is.
type UpdatePingRequest struct {
	Port        *uint16 `json:"port,omitempty"`
	Interval    *int    `json:"interval,omitempty" validate:"omitempty,min=10"`
	Timeout     *int    `json:"timeout,omitempty" validate:"omitempty,min=2"`
	MaxAttempts *int    `json:"max_attempts,omitempty" validate:"omitempty,min=1"`
	IsEnabled   *bool   `json:"is_enabled,omitempty"`
}

func (h *MonitorHandler) UpdatePing(w http.ResponseWriter, r *http.Request) {
	const op = "http-server.handlers.monitor.ping.update"

	log := h.log.With(slog.String("op", op), slog.String("request_id", middleware.GetReqID(r.Context())))

	monitorID, err := uuid.Parse(chi.URLParam(r, "monitorID"))
	if err != nil {
		log.Error("failed to parse monitor id", sl.Err(err))
		resp.WriteError(w, r, http.StatusBadRequest, "invalid monitor id")
		return
	}

	var req UpdatePingRequest
	if err := render.DecodeJSON(r.Body, &req); err != nil {
		log.Error("failed to decode request body", sl.Err(err))
		resp.WriteError(w, r, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := h.val.Struct(req); err != nil {
		var validateErr validator.ValidationErrors
		if errors.As(err, &validateErr) {
			log.Error("validation failed", sl.Err(err))
			resp.WriteValidationError(w, r, validateErr)
			return
		}
		log.Error("invalid request", sl.Err(err))
		resp.WriteError(w, r, http.StatusBadRequest, "invalid request")
		return
	}

	in := monitor.UpdatePingConfigInput{
		Port:        req.Port,
		IsEnabled:   req.IsEnabled,
		Interval:    req.Interval,
		Timeout:     req.Timeout,
		MaxAttempts: req.MaxAttempts,
	}

	if err := h.svc.UpdatePing(r.Context(), monitorID, in); err != nil {
		h.writeDomainError(w, r, log, err)
		return
	}

	log.Info("ping check updated", slog.String("monitor_id", monitorID.String()))

	w.WriteHeader(http.StatusNoContent)
}
