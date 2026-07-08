package monitor

import "net/http"

type MonitorStatus string

const (
	MonitorUp      MonitorStatus = "up"
	MonitorDown    MonitorStatus = "down"
	MonitorUnknown MonitorStatus = "unknown"
)

type CheckStatus string

const (
	CheckSuccess CheckStatus = "success"
	CheckFailure CheckStatus = "failure"
)

type CheckType string

const (
	CheckPing     CheckType = "ping"
	CheckHTTP     CheckType = "http"
	CheckHeadless CheckType = "headless"
)

type HTTPMethod string

const (
	MethodHEAD HTTPMethod = http.MethodHead
	MethodGET  HTTPMethod = http.MethodGet
)

type HTTPScheme string

const (
	SchemeHTTP  HTTPScheme = "http"
	SchemeHTTPS HTTPScheme = "https"
)
