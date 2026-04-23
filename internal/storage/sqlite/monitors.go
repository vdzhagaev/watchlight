package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/vdzhagaev/watchlight/internal/monitor"
	"github.com/vdzhagaev/watchlight/internal/storage"

	"github.com/google/uuid"
	"modernc.org/sqlite"
	sqlite3 "modernc.org/sqlite/lib"
)

func (s *Storage) CreateMonitor(ctx context.Context, m monitor.Monitor) error {
	const op = "storage.sqlite.CreateMonitor"

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}

	defer tx.Rollback()

	_, err = tx.ExecContext(ctx,
		"INSERT INTO monitors (id, name, url, status) VALUES (?, ?)",
		m.ID, m.Name, m.URL, m.Status,
	)
	if err != nil {
		if sqliteErr, ok := err.(*sqlite.Error); ok && sqliteErr.Code() == sqlite3.SQLITE_CONSTRAINT_UNIQUE {
			return fmt.Errorf("%s: insert monitor: %w", op, storage.ErrMonitorExists)
		}
		return fmt.Errorf("%s: %w", op, err)
	}

	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO monitor_check_configs
		(id, monitor_id, check_type,
		is_enabled, check_interval, check_timeout,
		max_attempts, do_error_screenshot, keywords)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`)

	if err != nil {
		return fmt.Errorf("%s: prepare stmt: %w", op, err)
	}
	defer stmt.Close()

	for _, cfg := range m.CheckConfigs {
		keywordsJSON, err := json.Marshal(cfg.Keywords)
		if err != nil {
			return fmt.Errorf("%s: json config keywords: %w", op, err)
		}

		_, err = stmt.ExecContext(ctx,
			cfg.ID,
			m.ID,
			cfg.CheckType,
			cfg.IsEnabled,
			cfg.CheckInterval,
			cfg.CheckTimeout,
			cfg.MaxAttempts,
			cfg.DoErrorScreenshot,
			string(keywordsJSON),
		)
		if err != nil {
			return fmt.Errorf("%s: insert config: %w", op, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("%s: commit: %w", op, err)
	}

	return nil
}

func (s *Storage) UpdateMonitor(ctx context.Context, id uuid.UUID, in monitor.UpdateMonitorInput) (monitor.Monitor, error) {
	const op = "storage.sqlite.UpdateMonitor"

	columns := []string{}
	args := []any{}

	if in.Name != nil {
		columns = append(columns, "name = ?")
		args = append(args, *in.Name)
	}
	if in.URL != nil {
		columns = append(columns, "url = ?")
		args = append(args, *in.URL)
	}
	if in.Status != nil {
		columns = append(columns, "status = ?")
		args = append(args, *in.Status)
	}

	if len(columns) == 0 {
		return s.GetMonitor(ctx, id)
	}

	args = append(args, id)

	query := fmt.Sprintf(
		"UPDATE monitors SET %s WHERE id = ?",
		strings.Join(columns, ", "),
	)

	res, err := s.db.ExecContext(ctx, query, args...)

	if err != nil {
		if sqliteErr, ok := err.(*sqlite.Error); ok && sqliteErr.Code() == sqlite3.SQLITE_CONSTRAINT_UNIQUE {
			return monitor.Monitor{}, fmt.Errorf("%s: %w", op, storage.ErrMonitorExists)
		}
		return monitor.Monitor{}, fmt.Errorf("%s: update monitor: %w", op, err)
	}

	affected, err := res.RowsAffected()
	if err != nil {
		return monitor.Monitor{}, fmt.Errorf("%s: rows affected: %w", op, err)
	}
	if affected == 0 {
		return monitor.Monitor{}, storage.ErrMonitorNotFound
	}
	return s.GetMonitor(ctx, id)
}

func (s *Storage) GetMonitor(ctx context.Context, id uuid.UUID) (monitor.Monitor, error) {
	const op = "storage.sqlite.GetMonitor"

	query := `
		SELECT
			m.id, m.name, m.url, m.status,
			c.id, c.check_type, c.is_enabled, c.check_interval,
			c.check_timeout, c.max_attempts, c.do_error_screenshot,
			c.keywords
		FROM monitors AS m
		LEFT JOIN monitor_check_configs AS c ON m.id = c.monitor_id
		WHERE m.id = ?
		ORDER BY m.id
	`

	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return monitor.Monitor{}, fmt.Errorf("%s: %w", op, err)
	}

	defer tx.Rollback()

	rows, err := tx.QueryContext(ctx, query, id)

	if err != nil {
		return monitor.Monitor{}, fmt.Errorf("%s: %w", op, err)
	}
	defer rows.Close()

	var m monitor.Monitor
	var found bool

	for rows.Next() {
		var (
			mID     uuid.NullUUID
			mName   string
			mURL    string
			mStatus monitor.MonitorStatus

			cID                uuid.NullUUID
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
			return monitor.Monitor{}, fmt.Errorf("%s: scan config: %w", op, err)
		}

		if !found {
			m = monitor.Monitor{
				ID:           mID.UUID,
				Name:         mName,
				URL:          mURL,
				Status:       mStatus,
				CheckConfigs: []monitor.MonitorCheckConfig{},
			}
			found = true
		}

		if cID.Valid {
			cfg := monitor.MonitorCheckConfig{
				ID:                cID.UUID,
				MonitorID:         mID.UUID,
				CheckType:         monitor.CheckType(cType.String),
				IsEnabled:         cEnabled.Bool,
				CheckInterval:     int(cInterval.Int64),
				MaxAttempts:       int(cMaxAttempts.Int64),
				DoErrorScreenshot: cDoErrorScreenshot.Bool,
			}
			if cKeywordsRaw.Valid && cKeywordsRaw.String != "" {
				err := json.Unmarshal([]byte(cKeywordsRaw.String), &cfg.Keywords)
				if err != nil {
					return monitor.Monitor{}, fmt.Errorf("%s: error unmarshal keywords from base: %w", op, err)
				}
			}
			m.CheckConfigs = append(m.CheckConfigs, cfg)
		}
	}

	if err := rows.Err(); err != nil {
		return monitor.Monitor{}, fmt.Errorf("%s: rows iteration error: %w", op, err)
	}

	if !found {
		return monitor.Monitor{}, storage.ErrMonitorNotFound
	}

	if err := tx.Commit(); err != nil {
		return monitor.Monitor{}, fmt.Errorf("%s: commit: %w", op, err)
	}

	return m, nil
}
