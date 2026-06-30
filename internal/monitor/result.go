package monitor

import (
	"time"

	"github.com/google/uuid"
)

type CheckResult struct {
	ID            uuid.UUID
	MonitorID     uuid.UUID
	ConfigID      uuid.UUID
	Status        CheckStatus
	StatusCode    int
	ResponseTime  time.Duration
	CheckedAt     time.Time
	Error         string
	FoundKeywords []string
}
