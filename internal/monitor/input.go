package monitor

import (
	"time"

	"github.com/google/uuid"
)

type CreateMonitorInput struct {
	Name         string
	URL          string
	CheckConfigs []CreateMonitorCheckConfigInput
}

type CreateMonitorCheckConfigInput struct {
	CheckType         CheckType
	IsEnabled         *bool
	CheckInterval     int
	CheckTimeout      int
	MaxAttempts       int
	DoErrorScreenshot bool
	Keywords          []string
}

type UpdateMonitorInput struct {
	Name   *string
	URL    *string
	Status *MonitorStatus
}

type UpdateMonitorCheckConfigInput struct {
	CheckType         *CheckType
	IsEnabled         *bool
	CheckInterval     *int
	CheckTimeout      *int
	MaxAttempts       *int
	DoErrorScreenshot *bool
	Keywords          *[]string
}

type CheckResultInput struct {
	MonitorID      uuid.UUID
	ConfigID       uuid.UUID
	StatusCode     int
	ResponseTime   time.Duration
	CheckedAt      time.Time
	Reachable      bool
	Error          error
	ScreenshotPath string
	FoundKeywords  []string
}
