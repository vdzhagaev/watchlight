package sqlite

import (
	"context"
	"fmt"
	"time"

	"gitlab.com/l0veme-projects/uptime-monitor/internal/storage"
)

func (s *Storage) GetPendingSites(ctx context.Context, stepType storage.StepType, limit int) ([]storage.Site, error) {
	const op = "sqlite.Storage.GetPendingSites"

	err := checkStepType(stepType)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}

	nextCheckColumn := "next_" + string(stepType) + "_at"
	lastCheckColumn := "last_" + string(stepType) + "_at"
	rows, err := s.db.QueryContext(ctx,
		"SELECT id, name, url, status, "+lastCheckColumn+", "+nextCheckColumn+" FROM sites WHERE "+nextCheckColumn+" <= ? LIMIT ?",
		time.Now(), limit,
	)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}
	defer rows.Close()

	var sites []storage.Site

	for rows.Next() {
		var site storage.Site
		if err := rows.Scan(&site.ID, &site.Name, &site.URL, &site.Status, &site.LastCheckedAt, &site.NextCheckAt); err != nil {
			return nil, fmt.Errorf("%s: %w", op, err)
		}
		sites = append(sites, site)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}
	return sites, nil
}

func checkStepType(st storage.StepType) error {
	switch st {
	case storage.StepPing, storage.StepHTTP, storage.StepBrowser:
		return nil
	default:
		return fmt.Errorf("invalid step type %s", st)
	}
}
