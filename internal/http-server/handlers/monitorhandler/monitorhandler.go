package monitorhandler

import (
	"log/slog"

	"github.com/vdzhagaev/watchlight/internal/monitor"

	"github.com/go-playground/validator/v10"
)

type MonitorHandler struct {
	log *slog.Logger
	val *validator.Validate
	svc *monitor.Service
}

func NewHandler(log *slog.Logger, v *validator.Validate, svc *monitor.Service) *MonitorHandler {
	return &MonitorHandler{
		log: log,
		val: v,
		svc: svc,
	}
}
