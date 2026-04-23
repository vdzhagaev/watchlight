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
	"github.com/vdzhagaev/watchlight/internal/storage"
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
		msg := "failed to decode request body"
		log.Error(msg, sl.Err(err))

		render.JSON(w, r, resp.Error(msg))

		return
	}

	log.Info("request body decode successfully", slog.Any("request", req))

	if err := h.val.Struct(req); err != nil {
		validateErr := err.(validator.ValidationErrors)
		response := resp.ValidationError(validateErr)

		log.Error(response.Error, sl.Err(err))

		render.JSON(w, r, response)
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

	if errors.Is(err, storage.ErrMonitorExists) {
		msg := "monitor already exists"
		log.Info(msg, slog.String("url", req.MonitorURL))
		render.JSON(w, r, resp.Error(msg))
		return
	}
	if err != nil {
		msg := "failed to add monitor"
		log.Error(msg, sl.Err(err))
		render.JSON(w, r, resp.Error(msg))
		return
	}

	log.Info("monitor added",
		slog.String("id", createdM.ID.String()),
		slog.String("name", createdM.Name),
		slog.String("url", createdM.URL),
	)

	render.JSON(w, r, SaveResponse{resp.OK(), createdM})
}
