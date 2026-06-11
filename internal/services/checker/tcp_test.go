package checker

import (
	"errors"
	"net"
	"testing"
	"time"
)

// newTCPListener starts a TCP listener on a random local port that accepts and
// immediately closes any incoming connection.
func newTCPListener(t *testing.T) net.Listener {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to start listener: %v", err)
	}
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			conn.Close()
		}
	}()
	return ln
}

type tcpWant struct {
	reachable bool
	errIs     error // matched against res.Err via errors.Is; nil means expect res.Err == nil
	returnErr bool  // expect a non-nil second return value (PrepareAddress failure)
}

func TestTCPChecker(t *testing.T) {
	tests := []struct {
		name string
		// setup prepares the scenario and returns the target address.
		setup func(t *testing.T) string
		// skipOnUnreachable handles environments with no route to TEST-NET-1,
		// where a connect timeout cannot be simulated.
		skipOnUnreachable bool
		reqTimeout        time.Duration
		want              tcpWant
	}{
		{
			name: "reachable",
			setup: func(t *testing.T) string {
				ln := newTCPListener(t)
				t.Cleanup(func() { ln.Close() })
				return ln.Addr().String()
			},
			want: tcpWant{reachable: true},
		},
		{
			name: "connection refused",
			setup: func(t *testing.T) string {
				ln := newTCPListener(t)
				addr := ln.Addr().String()
				ln.Close() // free the port so nothing is listening on it
				return addr
			},
			want: tcpWant{errIs: ErrUnreachable},
		},
		{
			name: "timeout",
			// 192.0.2.0/24 is TEST-NET-1 (RFC 5737): non-routable, so where a
			// default route exists the connect blackholes and hits the dial
			// timeout.
			setup:             func(t *testing.T) string { return "192.0.2.1:80" },
			skipOnUnreachable: true,
			reqTimeout:        100 * time.Millisecond,
			want:              tcpWant{errIs: ErrTimeout},
		},
		{
			name: "bad address",
			// Empty URL makes PrepareAddress fail, which the checker returns as
			// the second return value rather than inside CheckResult.Err.
			setup: func(t *testing.T) string { return "" },
			want:  tcpWant{returnErr: true},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			url := tc.setup(t)

			res, err := TCPChecker{}.Check(t.Context(), CheckRequest{
				URL:     url,
				Timeout: tc.reqTimeout,
			})

			if tc.want.returnErr {
				if err == nil {
					t.Fatalf("expected returned error, got nil")
				}
			} else if err != nil {
				t.Fatalf("unexpected returned error: %v", err)
			}

			if tc.skipOnUnreachable && errors.Is(res.Err, ErrUnreachable) {
				t.Skip("no route to TEST-NET-1 in this environment; cannot simulate a connect timeout")
			}

			if res.Reachable != tc.want.reachable {
				t.Errorf("Reachable = %t, want %t", res.Reachable, tc.want.reachable)
			}
			if tc.want.errIs != nil && !errors.Is(res.Err, tc.want.errIs) {
				t.Errorf("res.Err = %v, want %v", res.Err, tc.want.errIs)
			}
			if tc.want.errIs == nil && !tc.want.returnErr && res.Err != nil {
				t.Errorf("res.Err = %v, want nil", res.Err)
			}
		})
	}
}
