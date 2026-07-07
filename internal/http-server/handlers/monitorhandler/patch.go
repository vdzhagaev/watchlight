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

type UpdateRequest struct {
	Host   *string                `json:"host,omitempty" validate:"omitempty,hostname_rfc1123|ip"`
	Name   *string                `json:"name,omitempty"`
	Status *monitor.MonitorStatus `json:"status,omitempty" validate:"omitempty,oneof=up down unknown"`
}

func (h *MonitorHandler) Patch(w http.ResponseWriter, r *http.Request) {
	const op = "http-server.handlers.monitor.patch"

	log := h.log.With(slog.String("op", op), slog.String("request_id", middleware.GetReqID(r.Context())))

	idStr := chi.URLParam(r, "monitorID")
	id, err := uuid.Parse(idStr)
	if err != nil {
		log.Error("failed to parse id", slog.String("id", idStr), sl.Err(err))
		resp.WriteError(w, r, http.StatusBadRequest, "invalid id")
		return
	}

	var req UpdateRequest

	err = render.DecodeJSON(r.Body, &req)
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

	mUpdateIn := monitor.UpdateMonitorInput{
		Name:   req.Name,
		Host:   req.Host,
		Status: req.Status,
	}

	err = h.svc.Update(r.Context(), id, mUpdateIn)

	if errors.Is(err, monitor.ErrMonitorNotFound) {
		log.Info("monitor not found", slog.String("id", idStr))
		resp.WriteError(w, r, http.StatusNotFound, "monitor not found")
		return
	}
	if errors.Is(err, monitor.ErrMonitorExists) {
		log.Info("monitor already exists")
		resp.WriteError(w, r, http.StatusConflict, "monitor already exists")
		return
	}
	if errors.Is(err, monitor.ErrValidation) {
		log.Info("invalid monitor input", sl.Err(err))
		resp.WriteError(w, r, http.StatusBadRequest, err.Error())
		return
	}
	if err != nil {
		log.Error("failed to update monitor", sl.Err(err))
		resp.WriteError(w, r, http.StatusInternalServerError, "internal error")
		return
	}

	log.Info("monitor updated", slog.String("id", idStr))

	w.WriteHeader(http.StatusNoContent)
}
