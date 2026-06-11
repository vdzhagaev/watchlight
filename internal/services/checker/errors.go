package checker

import "errors"

var (
	ErrTimeout     = errors.New("checker: timeout")
	ErrUnreachable = errors.New("checker: unreacheable")
)
