package monitor

import (
	"errors"
	"fmt"
)

var (
	ErrMonitorEmptyName = errors.New("monitor name can not be empty")
	ErrMonitorEmptyHost = errors.New("monitor host can not be empty")
	ErrMonitorNotFound  = errors.New("monitor not found")
	ErrMonitorExists    = errors.New("monitor already exists")

	ErrCheckResultExists = errors.New("result with same id already exists")

	ErrCheckIntervalTooSmall = errors.New("check interval below minimum")
	ErrCheckTimeoutTooSmall  = errors.New("check timeout below minimum")
	ErrMaxAttemptsTooSmall   = errors.New("max attempts below minimum")

	ErrInvalidPort        = errors.New("invalid port for ping config")
	ErrKeywordsRequireGET = errors.New("keywords can not passed with the HEAD method")
)

func ErrMethodNotAllowed(method string) error {
	return fmt.Errorf("method not allowed: %s", method)
}

func ErrInvalidHTTPScheme(scheme string) error {
	return fmt.Errorf("invalid http scheme: %s", scheme)
}

func ErrInvalidHost(host string) error {
	return fmt.Errorf("invalid host: %s", host)
}

func ErrInvalidPath(path string) error {
	return fmt.Errorf("invalid path: %s", path)
}
