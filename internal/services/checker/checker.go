package checker

import "time"

type CheckRequest struct {
	URL     string        `json:"url"`
	Timeout time.Duration `json:"timeout"`
}
