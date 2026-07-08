package monitor_test

import (
	"errors"
	"testing"
	"time"

	"github.com/vdzhagaev/watchlight/internal/monitor"
)

// Value-object tests: pure construction, normalization and validation. No
// storage, no aggregate — each VO is exercised through its own constructor.

func TestNewHost(t *testing.T) {
	tests := []struct {
		name      string
		raw       string
		wantValue string // expected String() when construction succeeds
		wantErr   bool
	}{
		{"plain domain", "example.com", "example.com", false},
		{"lowercased", "Example.COM", "example.com", false},
		{"trimmed and trailing dot", "  example.com.  ", "example.com", false},
		{"subdomain preserved", "api.example.com", "api.example.com", false},
		{"ipv4", "1.2.3.4", "1.2.3.4", false},
		{"ipv6 bracketed input stored bare", "[::1]", "::1", false},
		{"ipv6 full bracketed", "[fe80::1]", "fe80::1", false},
		{"ipv6 uppercase lowered", "[FE80::1]", "fe80::1", false},

		{"empty", "", "", true},
		{"whitespace only", "   ", "", true},
		{"bare host with port", "example.com:80", "", true},
		{"bare ipv6 without brackets", "::1", "", true},
		{"contains slash", "example.com/path", "", true},
		{"contains scheme", "http://example.com", "", true},
		{"unclosed bracket", "[::1", "", true},
		{"bracketed non-ip", "[notanip]", "", true},
		{"bracketed ipv4 no colon", "[1.2.3.4]", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := monitor.NewHost(tt.raw)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("NewHost(%q) = %q, want error", tt.raw, got.String())
				}
				return
			}
			if err != nil {
				t.Fatalf("NewHost(%q) unexpected error: %v", tt.raw, err)
			}
			if got.String() != tt.wantValue {
				t.Errorf("NewHost(%q).String() = %q, want %q", tt.raw, got.String(), tt.wantValue)
			}
		})
	}
}

func TestNewHost_EmptyErrorIsSentinel(t *testing.T) {
	_, err := monitor.NewHost("")
	if !errors.Is(err, monitor.ErrMonitorEmptyHost) {
		t.Errorf("NewHost(\"\") error = %v, want ErrMonitorEmptyHost", err)
	}
}

func TestHost_Authority(t *testing.T) {
	tests := []struct {
		name  string
		value string // trusted stored (bare) value via ReconstructHost
		want  string
	}{
		{"domain unchanged", "example.com", "example.com"},
		{"ipv4 unchanged", "1.2.3.4", "1.2.3.4"},
		{"ipv6 bracketed", "::1", "[::1]"},
		{"ipv6 full bracketed", "fe80::1", "[fe80::1]"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := monitor.ReconstructHost(tt.value)
			if got := h.Authority(); got != tt.want {
				t.Errorf("Authority() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestHost_Equals(t *testing.T) {
	a := monitor.ReconstructHost("example.com")
	b := monitor.ReconstructHost("example.com")
	c := monitor.ReconstructHost("other.com")
	if !a.Equals(b) {
		t.Error("Equals: identical hosts should be equal")
	}
	if a.Equals(c) {
		t.Error("Equals: different hosts should not be equal")
	}
}

func TestNewPath(t *testing.T) {
	tests := []struct {
		name      string
		raw       string
		wantValue string
		wantErr   bool
	}{
		{"leading slash added", "api/v1", "/api/v1", false},
		{"trailing slash stripped", "/api/v1/", "/api/v1", false},
		{"already canonical", "/api/v1", "/api/v1", false},
		{"empty becomes root", "", "/", false},
		{"only slashes become root", "///", "/", false},
		{"trimmed", "  /x  ", "/x", false},
		{"case preserved", "/API/V1", "/API/V1", false},

		{"contains scheme", "http://x/y", "", true},
		{"contains space", "/a b", "", true},
		{"contains tab", "/a\tb", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := monitor.NewPath(tt.raw)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("NewPath(%q) = %q, want error", tt.raw, got.String())
				}
				return
			}
			if err != nil {
				t.Fatalf("NewPath(%q) unexpected error: %v", tt.raw, err)
			}
			if got.String() != tt.wantValue {
				t.Errorf("NewPath(%q).String() = %q, want %q", tt.raw, got.String(), tt.wantValue)
			}
		})
	}
}

func TestPath_Equals(t *testing.T) {
	a := monitor.ReconstructPath("/api")
	b := monitor.ReconstructPath("/api")
	c := monitor.ReconstructPath("/other")
	if !a.Equals(b) {
		t.Error("Equals: identical paths should be equal")
	}
	if a.Equals(c) {
		t.Error("Equals: different paths should not be equal")
	}
}

func TestNewInterval(t *testing.T) {
	tests := []struct {
		name     string
		in       int
		wantSecs int
		wantErr  bool
	}{
		{"zero defaults", 0, monitor.DefaultCheckInterval, false},
		{"at minimum", monitor.MinCheckInterval, monitor.MinCheckInterval, false},
		{"above minimum", 120, 120, false},
		{"below minimum", monitor.MinCheckInterval - 1, 0, true},
		{"negative", -5, 0, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := monitor.NewInterval(tt.in)
			if tt.wantErr {
				if !errors.Is(err, monitor.ErrCheckIntervalTooSmall) {
					t.Fatalf("NewInterval(%d) error = %v, want ErrCheckIntervalTooSmall", tt.in, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("NewInterval(%d) unexpected error: %v", tt.in, err)
			}
			if got.Seconds() != tt.wantSecs {
				t.Errorf("Seconds() = %d, want %d", got.Seconds(), tt.wantSecs)
			}
			if got.Duration() != time.Duration(tt.wantSecs)*time.Second {
				t.Errorf("Duration() = %v, want %ds", got.Duration(), tt.wantSecs)
			}
		})
	}
}

func TestInterval_Equals(t *testing.T) {
	a := monitor.ReconstructInterval(30)
	b := monitor.ReconstructInterval(30)
	c := monitor.ReconstructInterval(60)
	if !a.Equals(b) {
		t.Error("Equals: identical intervals should be equal")
	}
	if a.Equals(c) {
		t.Error("Equals: different intervals should not be equal")
	}
}

func TestNewTimeout(t *testing.T) {
	tests := []struct {
		name     string
		in       int
		wantSecs int
		wantErr  bool
	}{
		{"zero defaults", 0, monitor.DefaultCheckTimeout, false},
		{"at minimum", monitor.MinCheckTimeout, monitor.MinCheckTimeout, false},
		{"above minimum", 30, 30, false},
		{"below minimum", monitor.MinCheckTimeout - 1, 0, true},
		{"negative", -1, 0, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := monitor.NewTimeout(tt.in)
			if tt.wantErr {
				if !errors.Is(err, monitor.ErrCheckTimeoutTooSmall) {
					t.Fatalf("NewTimeout(%d) error = %v, want ErrCheckTimeoutTooSmall", tt.in, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("NewTimeout(%d) unexpected error: %v", tt.in, err)
			}
			if got.Seconds() != tt.wantSecs {
				t.Errorf("Seconds() = %d, want %d", got.Seconds(), tt.wantSecs)
			}
		})
	}
}

func TestTimeout_Equals(t *testing.T) {
	a := monitor.ReconstructTimeout(5)
	b := monitor.ReconstructTimeout(5)
	c := monitor.ReconstructTimeout(10)
	if !a.Equals(b) {
		t.Error("Equals: identical timeouts should be equal")
	}
	if a.Equals(c) {
		t.Error("Equals: different timeouts should not be equal")
	}
}

func TestNewMaxAttempts(t *testing.T) {
	tests := []struct {
		name      string
		in        int
		wantCount int
		wantErr   bool
	}{
		{"zero defaults", 0, monitor.DefaultMaxAttempts, false},
		{"at minimum", monitor.MinMaxAttempts, monitor.MinMaxAttempts, false},
		{"above minimum", 5, 5, false},
		{"negative", -1, 0, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := monitor.NewMaxAttempts(tt.in)
			if tt.wantErr {
				if !errors.Is(err, monitor.ErrMaxAttemptsTooSmall) {
					t.Fatalf("NewMaxAttempts(%d) error = %v, want ErrMaxAttemptsTooSmall", tt.in, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("NewMaxAttempts(%d) unexpected error: %v", tt.in, err)
			}
			if got.Count() != tt.wantCount {
				t.Errorf("Count() = %d, want %d", got.Count(), tt.wantCount)
			}
		})
	}
}
