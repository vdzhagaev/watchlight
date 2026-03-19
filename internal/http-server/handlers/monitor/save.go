package monitor

import (
	"errors"
	"log/slog"
	"net/http"

	"gitlab.com/l0veme-projects/uptime-monitor/internal/domain"
	resp "gitlab.com/l0veme-projects/uptime-monitor/internal/lib/api/response"

	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/render"
	"github.com/go-playground/validator/v10"

	"gitlab.com/l0veme-projects/uptime-monitor/internal/lib/logger/sl"
	"gitlab.com/l0veme-projects/uptime-monitor/internal/storage"
)

type CheckRequest struct {
	Type              string   `json:"type" validate:"required,oneof=http ping custom"`
	Interval          int      `json:"interval" validate:"required,min=10"`
	MaxAttempts       int      `json:"max_attempts" validate:"required,min=1"`
	DoErrorScreenshot bool     `json:"do_error_screenshot"`
	Keywords          []string `json:"keywords,omitempty"`
}

type SaveRequest struct {
	MonitorURL  string         `json:"url" validate:"required,url"`
	MonitorName string         `json:"name,omitempty"`
	Checks      []CheckRequest `json:"checks" validate:"required,dive"`
}

type SaveResponse struct {
	resp.Response
	ID int64 `json:"id"`
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

	m := domain.Monitor{
		URL:  req.MonitorURL,
		Name: req.MonitorName,
	}

	for _, c := range req.Checks {
		m.CheckConfigs = append(m.CheckConfigs, domain.MonitorCheckConfig{
			CheckType:         domain.CheckType(c.Type),
			CheckInterval:     c.Interval,
			MaxAttempts:       c.MaxAttempts,
			DoErrorScreenshot: c.DoErrorScreenshot,
			Keywords:          c.Keywords,
			IsEnabled:         true, // on creation default true
		})
	}

	err = h.saver.SaveMonitor(r.Context(), &m)

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

	log.Info("monitor added", slog.Int64("id", m.ID))

	render.JSON(w, r, SaveResponse{resp.OK(), m.ID})
}
