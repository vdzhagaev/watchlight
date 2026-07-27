package monitor

import (
	"time"

	"github.com/google/uuid"
)

type CreateMonitorInput struct {
	Name        string
	Host        string
	PingConfig  *CreatePingConfigInput
	HTTPConfigs []CreateHTTPConfigInput
}

type UpdateMonitorInput struct {
	Name *string
	Host *string
}

type CheckResultInput struct {
	MonitorID     uuid.UUID
	ConfigID      uuid.UUID
	CheckType     CheckType
	StatusCode    int
	ResponseTime  time.Duration
	CheckedAt     time.Time
	Reachable     bool
	Error         error
	FoundKeywords []string
}
