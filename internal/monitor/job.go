package monitor

import (
	"time"

	"github.com/google/uuid"
)

type CheckJob interface {
	isCheckJob()
}

type jobBase struct {
	MonitorID   uuid.UUID
	ConfigID    uuid.UUID
	Target      string
	Interval    time.Duration
	Timeout     time.Duration
	MaxAttempts int
}

type PingJob struct {
	jobBase // target is a host:port
}

type CreatePingJobInput struct {
	MonitorID   uuid.UUID
	ConfigID    uuid.UUID
	Target      string
	Interval    time.Duration
	Timeout     time.Duration
	MaxAttempts int
}

// Reconstruct from DB a PingJob from the given input.
func NewPingJob(input CreatePingJobInput) PingJob {
	return PingJob{
		jobBase: jobBase{
			MonitorID:   input.MonitorID,
			ConfigID:    input.ConfigID,
			Target:      input.Target,
			Interval:    input.Interval,
			Timeout:     input.Timeout,
			MaxAttempts: input.MaxAttempts,
		},
	}
}

func (p PingJob) isCheckJob() {}

type HTTPJob struct {
	jobBase  // target is a URL
	Method   HTTPMethod
	Keywords []string
}

type CreateHTTPJobInput struct {
	MonitorID   uuid.UUID
	ConfigID    uuid.UUID
	Target      string
	Interval    time.Duration
	Timeout     time.Duration
	MaxAttempts int
	Method      HTTPMethod
	Keywords    []string
}

// Reconstruct from DB an HTTPJob from the given input.
func NewHTTPJob(input CreateHTTPJobInput) HTTPJob {
	return HTTPJob{
		jobBase: jobBase{
			MonitorID:   input.MonitorID,
			ConfigID:    input.ConfigID,
			Target:      input.Target,
			Interval:    input.Interval,
			Timeout:     input.Timeout,
			MaxAttempts: input.MaxAttempts,
		},
		Method:   input.Method,
		Keywords: input.Keywords,
	}
}

func (HTTPJob) isCheckJob() {}
