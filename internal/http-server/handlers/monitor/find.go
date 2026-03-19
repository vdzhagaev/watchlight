package monitor

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/render"
	"gitlab.com/l0veme-projects/uptime-monitor/internal/domain"
	"gitlab.com/l0veme-projects/uptime-monitor/internal/lib/logger/sl"
	"gitlab.com/l0veme-projects/uptime-monitor/internal/storage"

	resp "gitlab.com/l0veme-projects/uptime-monitor/internal/lib/api/response"
)

type FindResponse struct {
	resp.Response
	Monitor domain.Monitor `json:"monitor"`
}

func NewFind(log *slog.Logger, monitorFinder MonitorFinder) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		const op = "http-server.handlers.monitor.NewFind"

		log := log.With(slog.String("op", op), slog.String("request_id", middleware.GetReqID(r.Context())))

		idStr := chi.URLParam(r, "monitorID")
		id, err := strconv.ParseInt(idStr, 10, 64)

		monitor, err := monitorFinder.GetMonitor(r.Context(), id)

		if errors.Is(err, storage.ErrMonitorNotFound) {
			render.Status(r, http.StatusNotFound)
			render.JSON(w, r, resp.Error("not found"))
			return
		}

		if err != nil {
			msg := fmt.Sprintf("failed to find monitor by id: %d", id)
			log.Error(msg, sl.Err(err))
			render.JSON(w, r, resp.Error(msg))
			return
		}

		render.JSON(w, r, FindResponse{resp.OK(), monitor})
	}
}
