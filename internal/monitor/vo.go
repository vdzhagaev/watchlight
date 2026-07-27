package monitor

import (
	"net"
	"strings"
	"time"
)

const (
	DefaultCheckInterval = 60
	DefaultCheckTimeout  = 10
	DefaultMaxAttempts   = 3

	MinCheckInterval = 10
	MinCheckTimeout  = 2
	MinMaxAttempts   = 1
)

type Interval struct {
	duration time.Duration
}

func NewInterval(i int) (Interval, error) {
	if i == 0 {
		return Interval{
			duration: time.Duration(DefaultCheckInterval) * time.Second,
		}, nil
	}
	if i < MinCheckInterval {
		return Interval{}, ErrCheckIntervalTooSmall
	}

	return Interval{
		duration: time.Duration(i) * time.Second,
	}, nil
}

// ReconstructInterval rebuilds an Interval from a trusted, already-validated
// stored value (seconds). It performs no validation or defaulting — use it only
// when rehydrating from storage, never for user input.
func ReconstructInterval(seconds int) Interval {
	return Interval{duration: time.Duration(seconds) * time.Second}
}

func (i Interval) Duration() time.Duration {
	return i.duration
}

func (i Interval) Equal(i2 Interval) bool {
	return i.duration == i2.duration
}

func (i Interval) Seconds() int {
	return int(i.duration / time.Second)
}

type Timeout struct {
	duration time.Duration
}

func NewTimeout(t int) (Timeout, error) {
	if t == 0 {
		return Timeout{
			duration: time.Duration(DefaultCheckTimeout) * time.Second,
		}, nil
	}
	if t < MinCheckTimeout {
		return Timeout{}, ErrCheckTimeoutTooSmall
	}

	return Timeout{
		duration: time.Duration(t) * time.Second,
	}, nil
}

// ReconstructTimeout rebuilds a Timeout from a trusted stored value (seconds).
// No validation — storage rehydration only.
func ReconstructTimeout(seconds int) Timeout {
	return Timeout{duration: time.Duration(seconds) * time.Second}
}

func (t Timeout) Duration() time.Duration {
	return t.duration
}

func (t Timeout) Equal(t2 Timeout) bool {
	return t.duration == t2.duration
}

func (t Timeout) Seconds() int {
	return int(t.duration / time.Second)
}

type MaxAttempts struct {
	count int
}

func NewMaxAttempts(c int) (MaxAttempts, error) {
	if c == 0 {
		return MaxAttempts{count: DefaultMaxAttempts}, nil
	}
	if c < MinMaxAttempts {
		return MaxAttempts{}, ErrMaxAttemptsTooSmall
	}
	return MaxAttempts{count: c}, nil
}

// ReconstructMaxAttempts rebuilds a MaxAttempts from a trusted stored value.
// No validation — storage rehydration only.
func ReconstructMaxAttempts(count int) MaxAttempts {
	return MaxAttempts{count: count}
}

func (ma MaxAttempts) Count() int {
	return ma.count
}

type Host struct {
	value string
}

// NewHost normalizes and validates a host. It lowercases and strips a trailing
// dot, but deliberately preserves subdomains (api.example.com is not
// example.com) and does not strip www. Port and scheme live elsewhere, so a
// bare host carrying ':' or a scheme/path is rejected.
//
// An IPv6 literal must be bracketed on input ("[::1]"): the brackets are the
// marker that ':' belongs to the address rather than a host:port separator. The
// stored value is the bare address ("::1"), because net.JoinHostPort re-brackets
// it when an authority is formed at the dial/URL boundary.
func NewHost(raw string) (Host, error) {
	h := strings.ToLower(strings.TrimSpace(raw))
	h = strings.TrimSuffix(h, ".")
	if h == "" {
		return Host{}, ErrMonitorEmptyHost
	}
	if strings.HasPrefix(h, "[") {
		if !strings.HasSuffix(h, "]") {
			return Host{}, ErrInvalidHost(raw)
		}
		inner := h[1 : len(h)-1]
		if !strings.Contains(inner, ":") || net.ParseIP(inner) == nil {
			return Host{}, ErrInvalidHost(raw)
		}
		return Host{value: inner}, nil
	}
	if strings.ContainsAny(h, " \t/") || strings.Contains(h, ":") {
		return Host{}, ErrInvalidHost(raw)
	}
	return Host{value: h}, nil
}

// ReconstructHost rebuilds a Host from a trusted, already-normalized stored
// value. No normalization or validation — storage rehydration only.
func ReconstructHost(value string) Host {
	return Host{value: value}
}

func (h Host) String() string {
	return h.value
}

func (h Host) Equal(o Host) bool {
	return h.value == o.value
}

func (h Host) Authority() string {
	if strings.Contains(h.value, ":") { // bare IPv6
		return "[" + h.value + "]"
	}
	return h.value
}

type Path struct {
	value string
}

// NewPath normalizes an HTTP path to the canonical form used in the
// (monitor, path) uniqueness key: a leading slash is ensured and trailing
// slashes are stripped (root stays "/"). Paths are case-sensitive, so case is
// preserved (unlike Host). A scheme or whitespace is rejected.
func NewPath(raw string) (Path, error) {
	p := strings.TrimSpace(raw)
	if strings.Contains(p, "://") || strings.ContainsAny(p, " \t") {
		return Path{}, ErrInvalidPath(raw)
	}
	p = "/" + strings.Trim(p, "/")
	return Path{value: p}, nil
}

// ReconstructPath rebuilds a Path from a trusted, already-normalized stored
// value. No normalization or validation — storage rehydration only.
func ReconstructPath(value string) Path {
	return Path{value: value}
}

func (p Path) String() string {
	return p.value
}

func (p Path) Equal(o Path) bool {
	return p.value == o.value
}
