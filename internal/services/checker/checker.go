package checker

import (
	"context"
	"time"
)

type CheckRequest struct {
	Target   string
	Method   string
	Timeout  time.Duration
	Keywords []string
}

type CheckResult struct {
	ResponseTime  time.Duration
	StatusCode    int
	FoundKeywords []string
	Reachable     bool
	Err           error
}

type Checker interface {
	Check(ctx context.Context, req CheckRequest) (CheckResult, error)
}
