package checker

import (
	"fmt"
	"net"
	"net/url"
	"strings"
	"time"
)

func CheckTCP(req CheckRequest) (time.Duration, error) {
	const op = "services.checker.tcp.check"

	hostPort, err := PrepareAddress(req)

	if err != nil {
		return 0, err
	}

	start := time.Now()

	conn, err := net.DialTimeout("tcp", hostPort, time.Second*5)

	if err != nil {
		return 0, fmt.Errorf("%s: failed to connect to address: %s\n%w", op, hostPort, err)
	}

	defer conn.Close()

	duration := time.Since(start)

	return duration, nil

}

func PrepareAddress(req CheckRequest) (string, error) {
	const op = "services.checker.tcp.prepareAddress"
	index := strings.Index(req.URL, "://")

	var rawURL string
	if index < 0 || index > 6 {
		rawURL = "http://" + req.URL
	} else {
		rawURL = req.URL
	}

	u, err := url.Parse(rawURL)
	if err != nil {
		return "", err
	}

	if u.Scheme == "" || u.Host == "" {
		return "", fmt.Errorf("%s: wrong url: %s", op, req.URL)
	}

	host, port, err := net.SplitHostPort(u.Host)
	if err != nil {
		host = u.Host
		if u.Scheme == "http" {
			port = "80"

		} else {
			port = "443"
		}
	}
	return net.JoinHostPort(host, port), nil
}
