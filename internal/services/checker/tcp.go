package checker

import (
	"context"
	"fmt"
	"net"
	"time"
	"vdzhagev/go-uptime-checker/internal/lib/netutil"
)

func CheckTCP(ctx context.Context, req CheckRequest) (time.Duration, error) {
	const op = "services.checker.tcp.check"

	hostPort, err := netutil.PrepareAddress(req.URL)

	if err != nil {
		return 0, err
	}

	if req.Timeout <= 0 {
		req.Timeout = time.Second * 5
	}

	dialer := net.Dialer{Timeout: req.Timeout}

	start := time.Now()

	conn, err := dialer.DialContext(ctx, "tcp", hostPort)

	if err != nil {
		return 0, fmt.Errorf("%s: connect to %s: %w", op, hostPort, err)
	}
	duration := time.Since(start)
	defer conn.Close()

	return duration, nil
}
