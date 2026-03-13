package save

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	resp "gitlab.com/l0veme-projects/uptime-monitor/internal/lib/api/response"

	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/render"
	"github.com/go-playground/validator/v10"

	"gitlab.com/l0veme-projects/uptime-monitor/internal/lib/logger/sl"
	"gitlab.com/l0veme-projects/uptime-monitor/internal/storage"
)

type Request struct {
	MonitorURL string `json:"url" validate:"required,url"`
}

type Response struct {
	resp.Response
}

type MonitorSaver interface {
	SaveMonitor(ctx context.Context, monitorURL string) (int64, error)
}

func New(log *slog.Logger, monitorSaver MonitorSaver) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		const op = "http-server.handlers.monitor.save.New"

		log := log.With(slog.String("op", op), slog.String("request_id", middleware.GetReqID(r.Context())))

		var req Request

		err := render.DecodeJSON(r.Body, &req)
		if err != nil {
			msg := "failed to decode request body"
			log.Error(msg, sl.Err(err))

			render.JSON(w, r, resp.Error(msg))

			return
		}

		log.Info("request body decode successfully", slog.Any("request", req))

		if err := validator.New().Struct(req); err != nil {
			validateErr := err.(validator.ValidationErrors)
			response := resp.ValidationError(validateErr)

			log.Error(response.Error, sl.Err(err))

			render.JSON(w, r, response)
			return
		}

		id, err := monitorSaver.SaveMonitor(r.Context(), req.MonitorURL)

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

		log.Info("monitor added", slog.Int64("id", id))

		render.JSON(w, r, Response{Response: resp.OK()})
	}
}
