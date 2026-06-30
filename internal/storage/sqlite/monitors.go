package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/vdzhagaev/watchlight/internal/monitor"

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

	// Save the monitor
	_, err = tx.ExecContext(ctx,
		"INSERT INTO monitors (id, name, host, status) VALUES (?, ?, ?, ?)",
		m.ID, m.Name, m.Host.String(), m.Status,
	)
	if err != nil {
		if sqliteErr, ok := err.(*sqlite.Error); ok && sqliteErr.Code() == sqlite3.SQLITE_CONSTRAINT_UNIQUE {
			return fmt.Errorf("%s: insert monitor: %w", op, monitor.ErrMonitorExists)
		}
		return fmt.Errorf("%s: %w", op, err)
	}

	// Save the ping config
	_, err = tx.ExecContext(ctx,
		`
			INSERT INTO ping_configs
			(id, monitor_id, port,
			is_enabled, interval,
			timeout, max_attempts)
			VALUES (?, ?, ?, ?, ?, ?, ?)
		`,
		m.PingConfig.ID,
		m.ID,
		m.PingConfig.Port,
		m.PingConfig.IsEnabled,
		m.PingConfig.Interval,
		m.PingConfig.Timeout,
		m.PingConfig.MaxAttempts,
	)
	if err != nil {
		return fmt.Errorf("%s: insert ping config: %w", op, err)
	}

	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO http_configs
		(id, monitor_id, scheme, path, method,
		is_enabled, interval,
		timeout, max_attempts, keywords)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`)

	if err != nil {
		return fmt.Errorf("%s: prepare stmt: %w", op, err)
	}
	defer stmt.Close()

	// Check http configs and save if exists
	for _, cfg := range m.HTTPConfigs {
		keywordsJSON, err := json.Marshal(cfg.Keywords)
		if err != nil {
			return fmt.Errorf("%s: json config keywords: %w", op, err)
		}

		_, err = stmt.ExecContext(ctx,
			cfg.ID,
			m.ID,
			cfg.Scheme,
			cfg.Path,
			cfg.Method,
			cfg.IsEnabled,
			cfg.Interval,
			cfg.Timeout,
			cfg.MaxAttempts,
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

func (s *Storage) SetMonitorStatus(ctx context.Context, id uuid.UUID, status monitor.MonitorStatus) error {
	const op = "storage.sqlite.SetMonitorStatus"

	res, err := s.db.ExecContext(ctx, "UPDATE monitors SET status = ? WHERE id = ?", status, id)
	if err != nil {
		return fmt.Errorf("%s: update status: %w", op, err)
	}

	affected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("%s: rows affected: %w", op, err)
	}
	if affected == 0 {
		return monitor.ErrMonitorNotFound
	}
	return nil
}

func (s *Storage) UpdateMonitor(ctx context.Context, id uuid.UUID, host monitor.Host, name string) error {
	const op = "storage.sqlite.UpdateMonitor"

	res, err := s.db.ExecContext(
		ctx,
		"UPDATE monitors SET name = ?, host = ? WHERE id = ?",
		name, host.String(), id,
	)

	if err != nil {
		if sqliteErr, ok := err.(*sqlite.Error); ok && sqliteErr.Code() == sqlite3.SQLITE_CONSTRAINT_UNIQUE {
			return fmt.Errorf("%s: %w", op, monitor.ErrMonitorExists)
		}
		return fmt.Errorf("%s: update monitor: %w", op, err)
	}

	affected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("%s: rows affected: %w", op, err)
	}
	if affected == 0 {
		return monitor.ErrMonitorNotFound
	}

	return nil
}

func (s *Storage) GetMonitor(ctx context.Context, id uuid.UUID) (monitor.Monitor, error) {
	const op = "storage.sqlite.GetMonitor"

	monitorAndPingQuery := `
		SELECT
			m.id, m.name, m.url, m.status,
			p.id, p.port, p.is_enabled, p.interval,
			p.timeout, p.max_attempts
		FROM monitors AS m
		JOIN ping_configs AS p ON m.id = p.monitor_id
		WHERE m.id = ?
	`

	var (
		mID          uuid.NullUUID
		mName        string
		mHost        string
		mStatus      string
		pID          uuid.NullUUID
		pPort        sql.NullInt64
		pIsEnabled   sql.NullBool
		pInterval    sql.NullInt64
		pTimeout     sql.NullInt64
		pMaxAttempts sql.NullInt64
	)

	err := s.db.QueryRowContext(ctx, monitorAndPingQuery, id).Scan(
		&mID, &mName, &mHost, &mStatus,
		&pID, &pPort, &pIsEnabled, &pInterval,
		&pTimeout, &pMaxAttempts,
	)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return monitor.Monitor{}, fmt.Errorf("%s: %w", op, monitor.ErrMonitorNotFound)
		}
		return monitor.Monitor{}, fmt.Errorf("%s: %w", op, err)
	}

	httpConfigsQuery := `
		SELECT
			h.id, h.scheme, h.path, h.method,  h.is_enabled,
			h.interval, h.timeout, h.max_attempts, h.keywords
		FROM monitors AS m
		LEFT JOIN http_configs AS h ON m.id = h.monitor_id
		WHERE m.id = ?
		ORDER BY m.id
	`

	rows, err := s.db.QueryContext(ctx, query, id)
	if err != nil {
		return monitor.Monitor{}, fmt.Errorf("%s: %w", op, err)
	}
	defer rows.Close()

	var m monitor.Monitor
	var found bool

	for rows.Next() {
		var (
			// Monitor fields
			mID     uuid.NullUUID
			mName   string
			mHost   string
			mStatus string
			// Ping config fields
			pID          uuid.NullUUID
			pPort        sql.NullInt64
			pIsEnabled   sql.NullBool
			pInterval    sql.NullInt64
			pTimeout     sql.NullInt64
			pMaxAttempts sql.NullInt64
			// HTTP config fields
			hID          uuid.NullUUID
			hScheme      sql.NullString
			hPath        sql.NullString
			hMethod      sql.NullString
			hIsEnabled   sql.NullBool
			hInterval    sql.NullInt64
			hTimeout     sql.NullInt64
			hMaxAttempts sql.NullInt64
			hKeywordsRaw sql.NullString
		)

		if err := rows.Scan(
			&mID, &mName, &mHost, &mStatus,
			&pID, &pPort, &pIsEnabled, &pInterval, &pTimeout,
			&pMaxAttempts,
			&hID, &hScheme, &hPath, &hMethod, &hIsEnabled, &hInterval, &hTimeout,
			&hMaxAttempts, &hKeywordsRaw,
		); err != nil {
			return monitor.Monitor{}, fmt.Errorf("%s: scan config: %w", op, err)
		}

		if !found {
			var cfg PingConfig
			if pID.Valid {
				cfg = monitor.ReconstructPingConfig(
					pID.UUID,
					mID.UUID,
					int(pPort.Int64),
					pIsEnabled.Bool,
					int(pInterval.Int64),
					int(pTimeout.Int64),
					int(pMaxAttempts.Int64),
				)
				m.PingConfig = cfg
			} else {
				m.PingConfig = monitor.PingConfig{}
			}
			m = monitor.ReconstructMonitor(
				mID.UUID,
				mName,
				mHost,
				mStatus,
				cfg,
				[]monitor.HTTPConfig{},
			)
			found = true
		}

		if hID.Valid {
			cfg := monitor.HTTPConfig{
				ID:          hID.UUID,
				MonitorID:   mID.UUID,
				Scheme:      hScheme,
				Path:        hPath,
				Method:      hMethod,
				IsEnabled:   hIsEnabled.Bool,
				Interval:    int(hInterval.Int64),
				Timeout:     int(hTimeout.Int64),
				MaxAttempts: int(hMaxAttempts.Int64),
			}
			if cKeywordsRaw.Valid && cKeywordsRaw.String != "" {
				err := json.Unmarshal([]byte(cKeywordsRaw.String), &cfg.Keywords)
				if err != nil {
					return monitor.Monitor{}, fmt.Errorf("%s: error unmarshal keywords from base: %w", op, err)
				}
			}
			m.HTTPConfigs = append(m.HTTPConfigs, cfg)
		}
	}

	if err := rows.Err(); err != nil {
		return monitor.Monitor{}, fmt.Errorf("%s: rows iteration error: %w", op, err)
	}

	if !found {
		return monitor.Monitor{}, monitor.ErrMonitorNotFound
	}

	return m, nil
}

func (s *Storage) DeleteMonitor(ctx context.Context, id uuid.UUID) error {
	const op = "storage.sqlite.DeleteMonitor"
	query := "DELETE FROM monitors WHERE id = ?"
	res, err := s.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("%s: deleting row error: %w", op, err)
	}

	count, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("%s: deleting row error: %w", op, err)
	}

	if count != 1 {
		return monitor.ErrMonitorNotFound
	}

	return nil
}

func (s *Storage) GetMonitorList(ctx context.Context) ([]monitor.Monitor, error) {
	const op = "storage.sqlite.GetMonitorList"

	query := `
		SELECT
			m.id, m.name, m.url, m.status,
			c.id, c.check_type, c.is_enabled, c.check_interval,
			c.check_timeout, c.max_attempts, c.do_error_screenshot,
			c.keywords
		FROM monitors AS m
		LEFT JOIN monitor_check_configs AS c ON m.id = c.monitor_id
		ORDER BY m.id
	`

	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return []monitor.Monitor{}, fmt.Errorf("%s: %w", op, err)
	}
	defer rows.Close()

	monitors := make(map[uuid.UUID]monitor.Monitor)
	order := []uuid.UUID{}

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
			cTimeout           sql.NullInt64
			cMaxAttempts       sql.NullInt64
			cDoErrorScreenshot sql.NullBool
			cKeywordsRaw       sql.NullString
		)

		if err := rows.Scan(
			&mID, &mName, &mURL, &mStatus,
			&cID, &cType, &cEnabled, &cInterval, &cTimeout,
			&cMaxAttempts, &cDoErrorScreenshot, &cKeywordsRaw,
		); err != nil {
			return []monitor.Monitor{}, fmt.Errorf("%s: scan config: %w", op, err)
		}

		m, ok := monitors[mID.UUID]
		if !ok {
			m = monitor.Monitor{
				ID:           mID.UUID,
				Name:         mName,
				URL:          mURL,
				Status:       mStatus,
				CheckConfigs: []monitor.MonitorCheckConfig{},
			}
			order = append(order, m.ID)
		}

		var cfg monitor.MonitorCheckConfig

		if cID.Valid {
			cfg = monitor.MonitorCheckConfig{
				ID:                cID.UUID,
				MonitorID:         mID.UUID,
				CheckType:         monitor.CheckType(cType.String),
				IsEnabled:         cEnabled.Bool,
				CheckInterval:     int(cInterval.Int64),
				CheckTimeout:      int(cTimeout.Int64),
				MaxAttempts:       int(cMaxAttempts.Int64),
				DoErrorScreenshot: cDoErrorScreenshot.Bool,
			}
			if cKeywordsRaw.Valid && cKeywordsRaw.String != "" {
				err := json.Unmarshal([]byte(cKeywordsRaw.String), &cfg.Keywords)
				if err != nil {
					return []monitor.Monitor{}, fmt.Errorf("%s: error unmarshal keywords from base: %w", op, err)
				}
			}
			m.CheckConfigs = append(m.CheckConfigs, cfg)
		}
		monitors[m.ID] = m
	}

	if err := rows.Err(); err != nil {
		return []monitor.Monitor{}, fmt.Errorf("%s: rows iteration error: %w", op, err)
	}

	output := make([]monitor.Monitor, 0, len(monitors))
	for _, id := range order {
		output = append(output, monitors[id])
	}

	return output, nil
}

func (s *Storage) ListEnabledCheckConfigs(ctx context.Context) ([]monitor.CheckJob, error) {
	const op = "storage.sqlite.ListEnabledCheckConfigs"

	query := `
		SELECT
			m.id, m.url,
			c.id, c.check_type, c.check_interval,
			c.check_timeout, c.max_attempts, c.keywords
		FROM monitors AS m
		JOIN monitor_check_configs AS c ON m.id = c.monitor_id
		WHERE c.is_enabled = 1
		ORDER BY m.id
	`

	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return []monitor.CheckJob{}, fmt.Errorf("%s: %w", op, err)
	}
	defer rows.Close()

	var checks []monitor.CheckJob

	for rows.Next() {
		var (
			mID  uuid.NullUUID
			mURL string

			cID          uuid.NullUUID
			cType        monitor.CheckType
			cInterval    int64
			cTimeout     int64
			cMaxAttempts int64
			cKeywordsRaw sql.NullString
		)

		if err := rows.Scan(
			&mID, &mURL,
			&cID, &cType, &cInterval, &cTimeout,
			&cMaxAttempts, &cKeywordsRaw,
		); err != nil {
			return []monitor.CheckJob{}, fmt.Errorf("%s: scan config: %w", op, err)
		}

		check := monitor.CheckJob{
			MonitorID:   mID.UUID,
			ConfigID:    cID.UUID,
			Target:      mURL,
			CheckType:   cType,
			Interval:    time.Duration(int(cInterval)) * time.Second,
			Timeout:     time.Duration(int(cTimeout)) * time.Second,
			MaxAttempts: int(cMaxAttempts),
		}

		if cKeywordsRaw.Valid && cKeywordsRaw.String != "" {
			err := json.Unmarshal([]byte(cKeywordsRaw.String), &check.Keywords)
			if err != nil {
				return []monitor.CheckJob{}, fmt.Errorf("%s: error unmarshal keywords from base: %w", op, err)
			}
		}

		checks = append(checks, check)
	}

	if err := rows.Err(); err != nil {
		return []monitor.CheckJob{}, fmt.Errorf("%s: rows iteration error: %w", op, err)
	}

	return checks, nil
}

func (s *Storage) SaveCheckResult(ctx context.Context, r monitor.CheckResult) error {
	const op = "storage.sqlite.SaveCheckResult"

	_, err := s.db.ExecContext(ctx,
		`INSERT INTO monitor_check_results (
			id,
			monitor_id,
			config_id,
			status,
			status_code,
			response_time_ns,
			checked_at,
			error_message
		)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		r.ID,
		r.MonitorID,
		r.ConfigID,
		r.Status,
		r.StatusCode,
		r.ResponseTime.Nanoseconds(),
		r.CheckedAt,
		r.Error,
	)
	if err != nil {
		if sqliteErr, ok := err.(*sqlite.Error); ok && sqliteErr.Code() == sqlite3.SQLITE_CONSTRAINT_UNIQUE {
			return fmt.Errorf("%s: insert check result: %w", op, monitor.ErrCheckResultExists)
		}
		return fmt.Errorf("%s: %w", op, err)
	}

	return nil
}
