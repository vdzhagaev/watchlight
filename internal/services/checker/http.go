package checker

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"
)

type HTTPChecker struct{}

func (h HTTPChecker) Check(ctx context.Context, command CheckRequest) (CheckResult, error) {
	const op = "services.checker.http.Check"
	if command.Timeout == 0 {
		command.Timeout = 5 * time.Second
	}
	ctx, cancel := context.WithTimeout(ctx, command.Timeout)
	defer cancel()

	method := http.MethodHead

	if len(command.Keywords) > 0 {
		method = http.MethodGet
	}

	req, err := http.NewRequestWithContext(ctx, method, command.URL, nil)
	if err != nil {
		return CheckResult{}, fmt.Errorf("%s: failed building request: %w", op, err)
	}
	client := &http.Client{Timeout: command.Timeout}
	start := time.Now()
	resp, err := client.Do(req)
	if err != nil {
		var netErr net.Error

		if ok := errors.As(err, &netErr); ok && netErr.Timeout() {
			return CheckResult{
				ResponseTime: time.Since(start),
				Reachable:    false,
				Err:          fmt.Errorf("%s: %w: %w", op, ErrTimeout, err),
			}, nil
		}
		return CheckResult{
			ResponseTime: time.Since(start),
			Reachable:    false,
			Err:          fmt.Errorf("%s: %w: %w", op, ErrUnreachable, err),
		}, nil
	}

	if method == http.MethodGet {
		foundKeywords := make([]string, 0, len(command.Keywords))
		defer func() {
			_, _ = io.Copy(io.Discard, resp.Body)
			resp.Body.Close()
		}()

		const maxBodySize = 5 * 1024 * 1024
		limitReader := io.LimitReader(resp.Body, maxBodySize)
		var buf bytes.Buffer
		_, err = io.Copy(&buf, limitReader)
		if err != nil {
			return CheckResult{
				ResponseTime: time.Since(start),
				StatusCode:   resp.StatusCode,
				Reachable:    true,
				Err:          err,
			}, nil
		}

		responseTime := time.Since(start)

		bodyText := strings.ToLower(buf.String())

		for _, kw := range command.Keywords {
			if strings.Contains(bodyText, strings.ToLower(kw)) {
				foundKeywords = append(foundKeywords, kw)
			}
		}

		return CheckResult{
			ResponseTime:  responseTime,
			StatusCode:    resp.StatusCode,
			Reachable:     true,
			FoundKeywords: foundKeywords,
		}, nil

	} else {
		defer resp.Body.Close()
	}

	return CheckResult{
		ResponseTime: time.Since(start),
		StatusCode:   resp.StatusCode,
		Reachable:    true,
	}, nil
}

var _ Checker = HTTPChecker{}
