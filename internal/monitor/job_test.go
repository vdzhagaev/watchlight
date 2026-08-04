package monitor

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestPingJob_Equal(t *testing.T) {
	mID, cID := uuid.New(), uuid.New()
	ref := CreatePingJobInput{
		MonitorID:   mID,
		ConfigID:    cID,
		Host:        ReconstructHost("example.com"),
		Port:        443,
		Interval:    60 * time.Second,
		Timeout:     5 * time.Second,
		MaxAttempts: 3,
	}
	// build makes a PingJob from ref, applying an optional mutation first.
	build := func(mut func(*CreatePingJobInput)) PingJob {
		in := ref
		if mut != nil {
			mut(&in)
		}
		return NewPingJob(in)
	}

	subject := build(nil)

	tests := []struct {
		name  string
		other CheckJob
		want  bool
	}{
		{"identical", build(nil), true},
		{"different host", build(func(in *CreatePingJobInput) { in.Host = ReconstructHost("other.com") }), false},
		{"different port", build(func(in *CreatePingJobInput) { in.Port = 80 }), false},
		{"different interval", build(func(in *CreatePingJobInput) { in.Interval = 30 * time.Second }), false},
		{"different timeout", build(func(in *CreatePingJobInput) { in.Timeout = 10 * time.Second }), false},
		{"different max attempts", build(func(in *CreatePingJobInput) { in.MaxAttempts = 5 }), false},
		{"different config id", build(func(in *CreatePingJobInput) { in.ConfigID = uuid.New() }), false},
		{"different monitor id", build(func(in *CreatePingJobInput) { in.MonitorID = uuid.New() }), false},
		{"nil other", nil, false},
		{"cross type (http)", NewHTTPJob(CreateHTTPJobInput{MonitorID: mID, ConfigID: cID}), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := subject.Equal(tt.other); got != tt.want {
				t.Errorf("Equal() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestHTTPJob_Equal(t *testing.T) {
	mID, cID := uuid.New(), uuid.New()
	ref := CreateHTTPJobInput{
		MonitorID:   mID,
		ConfigID:    cID,
		Scheme:      SchemeHTTPS,
		Host:        ReconstructHost("example.com"),
		Path:        ReconstructPath("/health"),
		Method:      MethodGET,
		Interval:    60 * time.Second,
		Timeout:     5 * time.Second,
		MaxAttempts: 3,
		Keywords:    []string{"ok"},
	}
	// build makes an HTTPJob from ref, applying an optional mutation first.
	build := func(mut func(*CreateHTTPJobInput)) HTTPJob {
		in := ref
		if mut != nil {
			mut(&in)
		}
		return NewHTTPJob(in)
	}

	subject := build(nil)

	tests := []struct {
		name  string
		other CheckJob
		want  bool
	}{
		{"identical", build(nil), true},
		{"different scheme", build(func(in *CreateHTTPJobInput) { in.Scheme = SchemeHTTP }), false},
		{"different host", build(func(in *CreateHTTPJobInput) { in.Host = ReconstructHost("other.com") }), false},
		{"different path", build(func(in *CreateHTTPJobInput) { in.Path = ReconstructPath("/other") }), false},
		{"different method", build(func(in *CreateHTTPJobInput) { in.Method = MethodHEAD }), false},
		{"different keywords", build(func(in *CreateHTTPJobInput) { in.Keywords = []string{"nope"} }), false},
		{"different interval", build(func(in *CreateHTTPJobInput) { in.Interval = 30 * time.Second }), false},
		{"different max attempts", build(func(in *CreateHTTPJobInput) { in.MaxAttempts = 5 }), false},
		{"nil other", nil, false},
		{"cross type (ping)", NewPingJob(CreatePingJobInput{MonitorID: mID, ConfigID: cID}), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := subject.Equal(tt.other); got != tt.want {
				t.Errorf("Equal() = %v, want %v", got, tt.want)
			}
		})
	}

	// nil and empty Keywords must compare equal (slices.Equal semantics),
	// otherwise an unchanged config would emit a spurious Updated event.
	t.Run("nil vs empty keywords", func(t *testing.T) {
		withNil := build(func(in *CreateHTTPJobInput) { in.Keywords = nil })
		withEmpty := build(func(in *CreateHTTPJobInput) { in.Keywords = []string{} })
		if !withNil.Equal(withEmpty) {
			t.Error("nil and empty Keywords should compare equal")
		}
	})
}
