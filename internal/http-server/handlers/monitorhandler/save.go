package monitorhandler

import (
	"errors"
	"log/slog"
	"net/http"

	resp "github.com/vdzhagaev/watchlight/internal/lib/api/response"
	"github.com/vdzhagaev/watchlight/internal/monitor"

	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/render"
	"github.com/go-playground/validator/v10"

	"github.com/vdzhagaev/watchlight/internal/lib/logger/sl"
)

// PingCheckRequest overrides the intrinsic ping check. Optional: when omitted the
// monitor still gets a default ping config.
type PingCheckRequest struct {
	Port        uint16 `json:"port,omitempty"`
	Interval    int    `json:"interval,omitempty" validate:"omitempty,min=10"`
	Timeout     int    `json:"timeout,omitempty" validate:"omitempty,min=2"`
	MaxAttempts int    `json:"max_attempts,omitempty" validate:"omitempty,min=1"`
	IsEnabled   *bool  `json:"is_enabled,omitempty"`
}

// HTTPCheckRequest is a per-path HTTP check. Scheme and Method are required per
// check because the domain has no default for them.
type HTTPCheckRequest struct {
	Scheme      string   `json:"scheme" validate:"required,oneof=http https"`
	Path        string   `json:"path,omitempty"`
	Method      string   `json:"method" validate:"required,oneof=GET HEAD"`
	Keywords    []string `json:"keywords,omitempty"`
	Interval    int      `json:"interval,omitempty" validate:"omitempty,min=10"`
	Timeout     int      `json:"timeout,omitempty" validate:"omitempty,min=2"`
	MaxAttempts int      `json:"max_attempts,omitempty" validate:"omitempty,min=1"`
	IsEnabled   *bool    `json:"is_enabled,omitempty"`
}

type SaveRequest struct {
	Host       string             `json:"host" validate:"required,hostname_rfc1123|ip"`
	Name       string             `json:"name,omitempty"`
	Ping       *PingCheckRequest  `json:"ping,omitempty"`
	HTTPChecks []HTTPCheckRequest `json:"http_checks,omitempty" validate:"omitempty,dive"`
}

type SaveResponse struct {
	resp.Response
	Monitor monitorView `json:"monitor"`
}

func (h *MonitorHandler) Save(w http.ResponseWriter, r *http.Request) {
	const op = "http-server.handlers.monitor.save.New"

	log := h.log.With(slog.String("op", op), slog.String("request_id", middleware.GetReqID(r.Context())))

	var req SaveRequest

	err := render.DecodeJSON(r.Body, &req)
	if err != nil {
		log.Error("failed to decode request body", sl.Err(err))
		resp.WriteError(w, r, http.StatusBadRequest, "invalid request body")
		return
	}

	log.Info("request body decode successfully", slog.Any("request", req))

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

	m := monitor.CreateMonitorInput{
		Host: req.Host,
		Name: req.Name,
	}

	if req.Ping != nil {
		m.PingConfig = &monitor.CreatePingConfigInput{
			Port:        req.Ping.Port,
			IsEnabled:   req.Ping.IsEnabled,
			Interval:    req.Ping.Interval,
			Timeout:     req.Ping.Timeout,
			MaxAttempts: req.Ping.MaxAttempts,
		}
	}

	for _, c := range req.HTTPChecks {
		m.HTTPConfigs = append(m.HTTPConfigs, monitor.CreateHTTPConfigInput{
			Scheme:      c.Scheme,
			Path:        c.Path,
			Method:      c.Method,
			IsEnabled:   c.IsEnabled,
			Interval:    c.Interval,
			Timeout:     c.Timeout,
			MaxAttempts: c.MaxAttempts,
			Keywords:    c.Keywords,
		})
	}

	createdM, err := h.svc.Create(r.Context(), m)

	if errors.Is(err, monitor.ErrMonitorExists) {
		log.Info("monitor already exists", slog.String("host", req.Host))
		resp.WriteError(w, r, http.StatusConflict, "monitor already exists")
		return
	}
	if errors.Is(err, monitor.ErrValidation) {
		log.Info("invalid monitor input", sl.Err(err))
		resp.WriteError(w, r, http.StatusBadRequest, err.Error())
		return
	}
	if err != nil {
		log.Error("failed to create monitor", sl.Err(err))
		resp.WriteError(w, r, http.StatusInternalServerError, "internal error")
		return
	}

	log.Info("monitor added",
		slog.String("id", createdM.ID.String()),
		slog.String("name", createdM.Name),
		slog.String("host", createdM.Host.String()),
	)

	render.Status(r, http.StatusCreated)
	render.JSON(w, r, SaveResponse{resp.OK(), toMonitorView(createdM)})
}
