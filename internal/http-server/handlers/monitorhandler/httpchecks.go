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

// UpdateHTTPCheckRequest is the patch body for an existing HTTP check: every
// field is optional, an omitted field leaves the stored value untouched.
type UpdateHTTPCheckRequest struct {
	Scheme      *string   `json:"scheme,omitempty" validate:"omitempty,oneof=http https"`
	Path        *string   `json:"path,omitempty"`
	Method      *string   `json:"method,omitempty" validate:"omitempty,oneof=GET HEAD"`
	Keywords    *[]string `json:"keywords,omitempty"`
	Interval    *int      `json:"interval,omitempty" validate:"omitempty,min=10"`
	Timeout     *int      `json:"timeout,omitempty" validate:"omitempty,min=2"`
	MaxAttempts *int      `json:"max_attempts,omitempty" validate:"omitempty,min=1"`
	IsEnabled   *bool     `json:"is_enabled,omitempty"`
}

func (h *MonitorHandler) AddHTTPCheck(w http.ResponseWriter, r *http.Request) {
	const op = "http-server.handlers.monitor.httpcheck.add"

	log := h.log.With(slog.String("op", op), slog.String("request_id", middleware.GetReqID(r.Context())))

	monitorID, err := uuid.Parse(chi.URLParam(r, "monitorID"))
	if err != nil {
		log.Error("failed to parse monitor id", sl.Err(err))
		resp.WriteError(w, r, http.StatusBadRequest, "invalid monitor id")
		return
	}

	var req HTTPCheckRequest
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

	in := monitor.CreateHTTPConfigInput{
		Scheme:      req.Scheme,
		Path:        req.Path,
		Method:      req.Method,
		IsEnabled:   req.IsEnabled,
		Interval:    req.Interval,
		Timeout:     req.Timeout,
		MaxAttempts: req.MaxAttempts,
		Keywords:    req.Keywords,
	}

	cfg, err := h.svc.AddHTTPCheck(r.Context(), monitorID, in)
	if err != nil {
		h.writeDomainError(w, r, log, err)
		return
	}

	log.Info("http check added",
		slog.String("monitor_id", monitorID.String()),
		slog.String("config_id", cfg.ID.String()),
	)

	render.Status(r, http.StatusCreated)
	render.JSON(w, r, toHTTPCheckView(cfg))
}

func (h *MonitorHandler) UpdateHTTPCheck(w http.ResponseWriter, r *http.Request) {
	const op = "http-server.handlers.monitor.httpcheck.update"

	log := h.log.With(slog.String("op", op), slog.String("request_id", middleware.GetReqID(r.Context())))

	monitorID, err := uuid.Parse(chi.URLParam(r, "monitorID"))
	if err != nil {
		log.Error("failed to parse monitor id", sl.Err(err))
		resp.WriteError(w, r, http.StatusBadRequest, "invalid monitor id")
		return
	}

	configID, err := uuid.Parse(chi.URLParam(r, "configID"))
	if err != nil {
		log.Error("failed to parse config id", sl.Err(err))
		resp.WriteError(w, r, http.StatusBadRequest, "invalid config id")
		return
	}

	var req UpdateHTTPCheckRequest
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

	in := monitor.UpdateHTTPConfigInput{
		Scheme:      req.Scheme,
		Path:        req.Path,
		Method:      req.Method,
		IsEnabled:   req.IsEnabled,
		Interval:    req.Interval,
		Timeout:     req.Timeout,
		MaxAttempts: req.MaxAttempts,
		Keywords:    req.Keywords,
	}

	if err := h.svc.UpdateHTTPCheck(r.Context(), monitorID, configID, in); err != nil {
		h.writeDomainError(w, r, log, err)
		return
	}

	log.Info("http check updated",
		slog.String("monitor_id", monitorID.String()),
		slog.String("config_id", configID.String()),
	)

	w.WriteHeader(http.StatusNoContent)
}

func (h *MonitorHandler) RemoveHTTPCheck(w http.ResponseWriter, r *http.Request) {
	const op = "http-server.handlers.monitor.httpcheck.remove"

	log := h.log.With(slog.String("op", op), slog.String("request_id", middleware.GetReqID(r.Context())))

	monitorID, err := uuid.Parse(chi.URLParam(r, "monitorID"))
	if err != nil {
		log.Error("failed to parse monitor id", sl.Err(err))
		resp.WriteError(w, r, http.StatusBadRequest, "invalid monitor id")
		return
	}

	configID, err := uuid.Parse(chi.URLParam(r, "configID"))
	if err != nil {
		log.Error("failed to parse config id", sl.Err(err))
		resp.WriteError(w, r, http.StatusBadRequest, "invalid config id")
		return
	}

	if err := h.svc.RemoveHTTPCheck(r.Context(), monitorID, configID); err != nil {
		h.writeDomainError(w, r, log, err)
		return
	}

	log.Info("http check removed",
		slog.String("monitor_id", monitorID.String()),
		slog.String("config_id", configID.String()),
	)

	w.WriteHeader(http.StatusNoContent)
}
