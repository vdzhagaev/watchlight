package monitor

import "errors"

var (
	ErrMonitorEmptyName = errors.New("monitor name can not be empty")
	ErrMonitorEmptyURL  = errors.New("monitor url can not be empty")
	ErrMonitorNoChecks  = errors.New("monitor must have at least one check")
	ErrMonitorNotFound  = errors.New("monitor not found")
	ErrMonitorExists    = errors.New("monitor already exists")

	ErrCheckIntervalTooSmall = errors.New("check interval below minimum")
	ErrCheckTimeoutTooSmall  = errors.New("check timeout below minimum")
	ErrMaxAttemptsTooSmall   = errors.New("max attempts below minimum")
)
