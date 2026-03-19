package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"gitlab.com/l0veme-projects/uptime-monitor/internal/domain"
	"gitlab.com/l0veme-projects/uptime-monitor/internal/storage"
	"modernc.org/sqlite"
	sqlite3 "modernc.org/sqlite/lib"
)

func (s *Storage) SaveMonitor(ctx context.Context, m domain.Monitor) (int64, error) {
	const op = "storage.sqlite.SaveMonitor"

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("%s: %w", op, err)
	}

	defer tx.Rollback()

	res, err := tx.ExecContext(ctx,
		"INSERT INTO monitors (name, url) VALUES (?, ?)",
		m.Name, m.URL,
	)
	if err != nil {
		if sqliteErr, ok := err.(*sqlite.Error); ok && sqliteErr.Code() == sqlite3.SQLITE_CONSTRAINT_UNIQUE {
			return 0, fmt.Errorf("%s: insert monitor: %w", op, storage.ErrMonitorExists)
		}
		return 0, fmt.Errorf("%s: %w", op, err)
	}

	id, err := res.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("%s: failed to get last insert id: %w", op, err)
	}

	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO monitor_check_configs
		(monitor_id, check_type, is_enabled, check_interval, max_attempts, do_error_screenshot, keywords)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`)

	if err != nil {
		return 0, fmt.Errorf("%s: prepare stmt: %w", op, err)
	}
	defer stmt.Close()

	for _, cfg := range m.CheckConfigs {
		keywordsJSON, err := json.Marshal(cfg.Keywords)

		if err != nil {
			return 0, fmt.Errorf("%s: json config keywords: %w", op, err)
		}

		_, err = stmt.ExecContext(ctx,
			id,
			cfg.CheckType,
			cfg.IsEnabled,
			cfg.CheckInterval,
			cfg.MaxAttempts,
			cfg.DoErrorScreenshot,
			string(keywordsJSON),
		)
		if err != nil {
			return 0, fmt.Errorf("%s: insert config: %w", op, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("%s: commit: %w", op, err)
	}

	return id, nil
}

func (s *Storage) GetMonitor(ctx context.Context, id int64) (domain.Monitor, error) {
	const op = "storage.sqlite.GetMonitor"

	query := `
		SELECT
			m.id, m.name, m.url, m.status,
			c.id, c.check_type, c.is_enabled, c.check_interval,
			c.max_attempts, c.do_error_screenshot, c.keywords
		FROM monitors AS m
		LEFT JOIN monitor_check_configs AS c ON m.id = c.monitor_id
		WHERE m.id = ?
		ORDER BY m.id
	`

	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return domain.Monitor{}, fmt.Errorf("%s: %w", op, err)
	}

	defer tx.Rollback()

	rows, err := tx.QueryContext(ctx, query, id)

	if err != nil {
		return domain.Monitor{}, fmt.Errorf("%s: %w", op, err)
	}
	defer rows.Close()

	var monitor domain.Monitor
	var found bool

	for rows.Next() {
		var (
			mID     int64
			mName   string
			mURL    string
			mStatus domain.MonitorStatus

			cID                sql.NullInt64
			cType              sql.NullString
			cEnabled           sql.NullBool
			cInterval          sql.NullInt64
			cMaxAttempts       sql.NullInt64
			cDoErrorScreenshot sql.NullBool
			cKeywordsRaw       sql.NullString
		)

		if err := rows.Scan(
			&mID, &mName, &mURL, &mStatus,
			&cID, &cType, &cEnabled, &cInterval,
			&cMaxAttempts, &cDoErrorScreenshot, &cKeywordsRaw,
		); err != nil {
			return domain.Monitor{}, fmt.Errorf("%s: scan config: %w", op, err)
		}

		if !found {
			monitor = domain.Monitor{
				ID:           mID,
				Name:         mName,
				URL:          mURL,
				Status:       mStatus,
				CheckConfigs: []domain.MonitorCheckConfig{},
			}
			found = true
		}

		if cID.Valid {
			cfg := domain.MonitorCheckConfig{
				ID:                int(cID.Int64),
				MonitorID:         int(mID),
				CheckType:         domain.CheckType(cType.String),
				IsEnabled:         cEnabled.Bool,
				CheckInterval:     int(cInterval.Int64),
				MaxAttempts:       int(cMaxAttempts.Int64),
				DoErrorScreenshot: cDoErrorScreenshot.Bool,
			}
			if cKeywordsRaw.Valid && cKeywordsRaw.String != "" {
				err := json.Unmarshal([]byte(cKeywordsRaw.String), &cfg.Keywords)
				if err != nil {
					return domain.Monitor{}, fmt.Errorf("%s: error unmarshal keywords from base: %w", op, err)
				}
			}
			monitor.CheckConfigs = append(monitor.CheckConfigs, cfg)
		}
	}

	fmt.Printf("finded monitor: %v", monitor)

	if err := rows.Err(); err != nil {
		return domain.Monitor{}, fmt.Errorf("%s: rows iteration error: %w", op, err)
	}

	if !found {
		return domain.Monitor{}, storage.ErrMonitorNotFound
	}

	if err := tx.Commit(); err != nil {
		return domain.Monitor{}, fmt.Errorf("%s: commit: %w", op, err)
	}

	return monitor, nil
}
