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

type CheckRequest struct {
	Type              string   `json:"type" validate:"required,oneof=http ping headless"`
	Interval          int      `json:"interval" validate:"required,min=10"`
	Timeout           int      `json:"timeout" validate:"required,min=2"`
	MaxAttempts       int      `json:"max_attempts" validate:"required,min=1"`
	DoErrorScreenshot bool     `json:"do_error_screenshot"`
	Keywords          []string `json:"keywords,omitempty"`
	IsEnabled         *bool    `json:"is_enabled,omitempty"`
}

type SaveRequest struct {
	MonitorURL  string         `json:"url" validate:"required,url"`
	MonitorName string         `json:"name,omitempty"`
	Checks      []CheckRequest `json:"checks" validate:"required,dive"`
}

type SaveResponse struct {
	resp.Response
	Monitor monitor.Monitor `json:"monitor"`
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
		URL:  req.MonitorURL,
		Name: req.MonitorName,
	}

	for _, c := range req.Checks {
		m.CheckConfigs = append(m.CheckConfigs, monitor.CreateMonitorCheckConfigInput{
			CheckType:         monitor.CheckType(c.Type),
			CheckInterval:     c.Interval,
			CheckTimeout:      c.Timeout,
			MaxAttempts:       c.MaxAttempts,
			DoErrorScreenshot: c.DoErrorScreenshot,
			Keywords:          c.Keywords,
			IsEnabled:         c.IsEnabled,
		})
	}

	createdM, err := h.svc.Create(r.Context(), m)

	if errors.Is(err, monitor.ErrMonitorExists) {
		log.Info("monitor already exists", slog.String("url", req.MonitorURL))
		resp.WriteError(w, r, http.StatusConflict, "monitor already exists")
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
		slog.String("url", createdM.URL),
	)

	render.Status(r, http.StatusCreated)
	render.JSON(w, r, SaveResponse{resp.OK(), createdM})
}
