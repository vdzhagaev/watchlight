package storage

import "time"

type SiteStatus string

const (
	StatusUp      SiteStatus = "up"
	StatusDown    SiteStatus = "down"
	StatusUnknown SiteStatus = "unknown"
)

type Interval struct {
	Ping    int `json:"ping"`
	Http    int `json:"http"`
	Browser int `json:"browser"`
}

type CheckAt struct {
	PingAt    time.Time `json:"ping"`
	HttpAt    time.Time `json:"http"`
	BrowserAt time.Time `json:"browser"`
}

type Site struct {
	ID            int64      `json:"id"`
	Name          string     `json:"name"`
	URL           string     `json:"url"`
	Interval      Interval   `json:"interval"`
	Status        SiteStatus `json:"status"`
	LastCheckedAt CheckAt    `json:"last_checked_at"`
	NextCheckAt   CheckAt    `json:"next_check_at"`
}

type CheckStatus string

const (
	CheckStatusSuccess CheckStatus = "success"
	CheckStatusFailure CheckStatus = "failure"
)

type StepType string

const (
	StepPing    StepType = "ping"
	StepHTTP    StepType = "http"
	StepBrowser StepType = "browser"
)

type Check struct {
	ID             int           `json:"id"`
	SiteID         int           `json:"site_id"`
	StepType       StepType      `json:"step_type"`
	Status         CheckStatus   `json:"status"`
	StatusCode     int           `json:"status_code"`
	ResponseTime   time.Duration `json:"response_time"`
	ErrorMessage   string        `json:"error_message,omitempty"`
	ScreenshotPath string        `json:"screenshot_path,omitempty"`
	Attempt        int           `json:"attempt"`
	StartedAt      time.Time     `json:"started_at"`
	FinishedAt     time.Time     `json:"finished_at"`
}

type IncidentStatus string

const (
	IncidentStatusOpen   IncidentStatus = "open"
	IncidentStatusClosed IncidentStatus = "closed"
)

type Incident struct {
	ID             int            `json:"id"`
	SiteID         int            `json:"site_id"`
	Status         IncidentStatus `json:"status"`
	OpenedAt       time.Time      `json:"opened_at"`
	ResolvedAt     *time.Time     `json:"resolved_at"`
	ScreenshotPath string         `json:"screenshot_path"`
}
