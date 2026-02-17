package storage

import (
	"context"
	"time"
)

type UpdateSiteDto struct {
	Name     *string   `json:"name"`
	URL      *string   `json:"url"`
	Interval *Interval `json:"interval"`
}

type Store interface {
	// Scheduler
	GetPendingSites(ctx context.Context, stepType StepType, limit int) ([]Site, error)
	UpdateNextCheck(ctx context.Context, siteID int, stepType StepType, nextCheck time.Time) error

	// Checker
	AddCheck(ctx context.Context, check Check) (int, error)

	// Incidents
	OpenIncident(ctx context.Context, siteID int, screenshotPath string) (int, error)
	ResolveIncident(ctx context.Context, siteID int) error
	GetOpenIncident(ctx context.Context, siteID int) (*Incident, error)

	// Web
	AddSite(ctx context.Context, url string, interval time.Duration) (int, error)
	UpdateSite(ctx context.Context, id int, dto UpdateSiteDto) error
	DeleteSite(ctx context.Context, id int) error
	GetSite(ctx context.Context, id int) (*Site, error)
	ListSites(ctx context.Context) ([]Site, error)
	GetCheckHistory(ctx context.Context, siteID int, limit int, offset int) ([]Check, error)
}
