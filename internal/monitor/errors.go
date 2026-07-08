package monitor

import (
	"errors"
	"fmt"
)

// ErrValidation is the base for all bad-input errors. Every validation error
// wraps it, so the transport layer can map the whole class to HTTP 400 with a
// single errors.Is check while keeping each error's specific identity.
var ErrValidation = errors.New("validation failed")

var (
	ErrMonitorEmptyName = fmt.Errorf("%w: monitor name can not be empty", ErrValidation)
	ErrMonitorEmptyHost = fmt.Errorf("%w: monitor host can not be empty", ErrValidation)
	ErrMonitorNotFound  = errors.New("monitor not found")
	ErrMonitorExists    = errors.New("monitor already exists")

	ErrCheckResultExists = errors.New("result with same id already exists")

	ErrCheckIntervalTooSmall = fmt.Errorf("%w: check interval below minimum", ErrValidation)
	ErrCheckTimeoutTooSmall  = fmt.Errorf("%w: check timeout below minimum", ErrValidation)
	ErrMaxAttemptsTooSmall   = fmt.Errorf("%w: max attempts below minimum", ErrValidation)

	ErrInvalidPort        = fmt.Errorf("%w: invalid port for ping config", ErrValidation)
	ErrKeywordsRequireGET = fmt.Errorf("%w: keywords can not passed with the HEAD method", ErrValidation)

	ErrHTTPConfigNotFound = errors.New("http config not found")
	ErrPingConfigNotFound = errors.New("ping config not found")

	ErrHTTPConfigExists = errors.New("http config already exists")
)

func ErrMethodNotAllowed(method string) error {
	return fmt.Errorf("%w: method not allowed: %s", ErrValidation, method)
}

func ErrInvalidHTTPScheme(scheme string) error {
	return fmt.Errorf("%w: invalid http scheme: %s", ErrValidation, scheme)
}

func ErrInvalidHost(host string) error {
	return fmt.Errorf("%w: invalid host: %s", ErrValidation, host)
}

func ErrInvalidPath(path string) error {
	return fmt.Errorf("%w: invalid path: %s", ErrValidation, path)
}
