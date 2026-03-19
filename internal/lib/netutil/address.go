package netutil

import (
	"fmt"
	"net"
	"net/url"
	"strings"
)

func PrepareAddress(rawURL string) (string, error) {
	const op = "services.checker.tcp.prepareAddress"
	index := strings.Index(rawURL, "://")

	if index < 0 || index > 6 {
		rawURL = "http://" + rawURL
	}

	u, err := url.Parse(rawURL)
	if err != nil {
		return "", fmt.Errorf("%s: parse url: %w", op, err)
	}

	if u.Host == "" {
		return "", fmt.Errorf("%s: host is empty in url: %s", op, rawURL)
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
