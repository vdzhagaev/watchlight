package monitorhandler

import (
	"errors"
	"log/slog"
	"net/http"

	resp "github.com/vdzhagaev/watchlight/internal/lib/api/response"
	"github.com/vdzhagaev/watchlight/internal/lib/logger/sl"
	"github.com/vdzhagaev/watchlight/internal/monitor"
)

// writeDomainError maps a service/domain error to an HTTP response. Specific
// sentinels are matched before the ErrValidation base, which also covers the
// wrapped invariants (ErrInvalidPort, ErrKeywordsRequireGET). Anything unknown
// is a 500 and is the only case worth logging at error level.
func (h *MonitorHandler) writeDomainError(w http.ResponseWriter, r *http.Request, log *slog.Logger, err error) {
	switch {
	case errors.Is(err, monitor.ErrMonitorNotFound):
		resp.WriteError(w, r, http.StatusNotFound, "monitor not found")
	case errors.Is(err, monitor.ErrHTTPConfigNotFound):
		resp.WriteError(w, r, http.StatusNotFound, "http check not found")
	case errors.Is(err, monitor.ErrPingConfigNotFound):
		resp.WriteError(w, r, http.StatusNotFound, "ping check not found")
	case errors.Is(err, monitor.ErrHTTPConfigExists):
		resp.WriteError(w, r, http.StatusConflict, "http check already exists")
	case errors.Is(err, monitor.ErrMonitorExists):
		resp.WriteError(w, r, http.StatusConflict, "monitor already exists")
	case errors.Is(err, monitor.ErrValidation):
		resp.WriteError(w, r, http.StatusBadRequest, err.Error())
	default:
		log.Error("internal error", sl.Err(err))
		resp.WriteError(w, r, http.StatusInternalServerError, "internal error")
	}
}
