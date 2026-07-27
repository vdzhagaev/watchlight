package monitor

import (
	"net"
	"net/url"
	"slices"
	"strconv"
	"time"

	"github.com/google/uuid"
)

type CheckJob interface {
	isCheckJob()
	Base() jobBase
	Target() string
	CheckType() CheckType
	Equal(other CheckJob) bool
}

type jobBase struct {
	MonitorID   uuid.UUID
	ConfigID    uuid.UUID
	Interval    time.Duration
	Timeout     time.Duration
	MaxAttempts int
}

func (jB jobBase) Equal(other jobBase) bool {
	return jB.MonitorID == other.MonitorID &&
		jB.ConfigID == other.ConfigID &&
		jB.Interval == other.Interval &&
		jB.Timeout == other.Timeout &&
		jB.MaxAttempts == other.MaxAttempts
}

type PingJob struct {
	jobBase
	Host Host
	Port uint16
}

type CreatePingJobInput struct {
	MonitorID   uuid.UUID
	ConfigID    uuid.UUID
	Host        Host
	Port        uint16
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
			Interval:    input.Interval,
			Timeout:     input.Timeout,
			MaxAttempts: input.MaxAttempts,
		},
		Host: input.Host,
		Port: input.Port,
	}
}

func (p PingJob) isCheckJob() {}
func (p PingJob) Target() string {
	return net.JoinHostPort(p.Host.String(), strconv.Itoa(int(p.Port)))
}

func (p PingJob) Equal(other CheckJob) bool {
	if other == nil {
		return false
	}
	o, ok := other.(PingJob)
	return ok &&
		p.Host.Equal(o.Host) &&
		p.Port == o.Port &&
		p.jobBase.Equal(o.jobBase)
}

func (p PingJob) Base() jobBase {
	return p.jobBase
}
func (p PingJob) CheckType() CheckType {
	return CheckPing
}

type HTTPJob struct {
	jobBase
	Scheme   HTTPScheme
	Host     Host
	Path     Path
	Method   HTTPMethod
	Keywords []string
}

type CreateHTTPJobInput struct {
	MonitorID   uuid.UUID
	ConfigID    uuid.UUID
	Scheme      HTTPScheme
	Host        Host
	Path        Path
	Method      HTTPMethod
	Interval    time.Duration
	Timeout     time.Duration
	MaxAttempts int
	Keywords    []string
}

// Reconstruct from DB an HTTPJob from the given input.
func NewHTTPJob(input CreateHTTPJobInput) HTTPJob {
	return HTTPJob{
		jobBase: jobBase{
			MonitorID:   input.MonitorID,
			ConfigID:    input.ConfigID,
			Interval:    input.Interval,
			Timeout:     input.Timeout,
			MaxAttempts: input.MaxAttempts,
		},
		Scheme:   input.Scheme,
		Host:     input.Host,
		Path:     input.Path,
		Method:   input.Method,
		Keywords: input.Keywords,
	}
}

func (HTTPJob) isCheckJob() {}
func (h HTTPJob) Target() string {
	u := url.URL{
		Scheme: string(h.Scheme),
		Host:   h.Host.Authority(),
		Path:   h.Path.String(),
	}
	return u.String()
}
func (h HTTPJob) Base() jobBase {
	return h.jobBase
}
func (HTTPJob) CheckType() CheckType {
	return CheckHTTP
}

func (h HTTPJob) Equal(other CheckJob) bool {
	if other == nil {
		return false
	}
	o, ok := other.(HTTPJob)
	return ok &&
		h.Scheme == o.Scheme &&
		h.Host.Equal(o.Host) &&
		h.Path == o.Path &&
		h.Method == o.Method &&
		slices.Equal(h.Keywords, o.Keywords) &&
		h.jobBase.Equal(o.jobBase)
}

var _ CheckJob = PingJob{}
var _ CheckJob = HTTPJob{}
