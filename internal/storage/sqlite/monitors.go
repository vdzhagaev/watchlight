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
			m.id, m.name, m.host, m.status,
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

	pingConfig := monitor.ReconstructPingConfig(
		pID.UUID,
		mID.UUID,
		uint16(pPort.Int64),
		pIsEnabled.Bool,
		int(pInterval.Int64),
		int(pTimeout.Int64),
		int(pMaxAttempts.Int64),
	)

	httpConfigsQuery := `
		SELECT
			id, scheme, path, method, is_enabled,
			interval, timeout, max_attempts, keywords
		FROM http_configs
		WHERE monitor_id = ?
		ORDER BY monitor_id
	`

	rows, err := s.db.QueryContext(ctx, httpConfigsQuery, id)
	if err != nil {
		return monitor.Monitor{}, fmt.Errorf("%s: %w", op, err)
	}
	defer rows.Close()

	httpConfigs := make([]monitor.HTTPConfig, 0)

	for rows.Next() {
		var (
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
			&hID, &hScheme, &hPath, &hMethod, &hIsEnabled,
			&hInterval, &hTimeout, &hMaxAttempts, &hKeywordsRaw,
		); err != nil {
			return monitor.Monitor{}, fmt.Errorf("%s: scan config: %w", op, err)
		}

		keywords := make([]string, 0)
		if hKeywordsRaw.Valid && hKeywordsRaw.String != "" {
			err := json.Unmarshal([]byte(hKeywordsRaw.String), &keywords)
			if err != nil {
				return monitor.Monitor{}, fmt.Errorf("%s: error unmarshal keywords from base: %w", op, err)
			}
		}
		cfg := monitor.ReconstructHTTPConfig(
			hID.UUID,
			mID.UUID,
			hScheme.String,
			hPath.String,
			hMethod.String,
			hIsEnabled.Bool,
			int(hInterval.Int64),
			int(hTimeout.Int64),
			int(hMaxAttempts.Int64),
			keywords,
		)
		httpConfigs = append(httpConfigs, cfg)

	}

	if err := rows.Err(); err != nil {
		return monitor.Monitor{}, fmt.Errorf("%s: collect http configs error: %w", op, err)
	}

	return monitor.ReconstructMonitor(mID.UUID, mName, mHost, mStatus, pingConfig, httpConfigs), nil
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

	allMonitorsWithPingQuery := `
		SELECT
			m.id, m.name, m.host, m.status,
			p.id, p.port, p.is_enabled, p.interval,
			p.timeout, p.max_attempts,
			h.id, h.scheme, h.path, h.method,
			h.is_enabled, h.interval, h.timeout,
			h.max_attempts, h.keywords
		FROM monitors AS m
		JOIN ping_configs AS p ON m.id = p.monitor_id
		LEFT JOIN http_configs AS h ON m.id = h.monitor_id
		ORDER BY m.id
	`

	monitorPingRows, err := s.db.QueryContext(ctx, allMonitorsWithPingQuery)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}

	defer monitorPingRows.Close()

	monitors := make(map[uuid.UUID]monitor.Monitor)
	order := []uuid.UUID{}

	for monitorPingRows.Next() {
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

		if err := monitorPingRows.Scan(
			&mID, &mName, &mHost, &mStatus,
			&pID, &pPort, &pIsEnabled, &pInterval,
			&pTimeout, &pMaxAttempts,
		); err != nil {
			return nil, fmt.Errorf("%s: monitor-ping scan error: %w", op, err)
		}

		pingConfig := monitor.ReconstructPingConfig(
			pID.UUID,
			mID.UUID,
			uint16(pPort.Int64),
			pIsEnabled.Bool,
			int(pInterval.Int64),
			int(pTimeout.Int64),
			int(pMaxAttempts.Int64),
		)

		monitors[mID.UUID] = monitor.ReconstructMonitor(mID.UUID, mName, mHost, mStatus, pingConfig, []monitor.HTTPConfig{})
		order = append(order, mID.UUID)
	}

	if err := monitorPingRows.Err(); err != nil {
		return nil, fmt.Errorf("%s: monitor-ping rows iteration error: %w", op, err)
	}

	allHttpConfQuery := `
		SELECT
			id, monitor_id, scheme, path, method, is_enabled,
			interval, timeout, max_attempts, keywords
		FROM http_configs
		ORDER BY monitor_id
	`
	httpConfigRows, err := s.db.QueryContext(ctx, allHttpConfQuery)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}

	defer httpConfigRows.Close()

	for httpConfigRows.Next() {
		var (
			hID          uuid.NullUUID
			hMonitorID   uuid.NullUUID
			hScheme      sql.NullString
			hPath        sql.NullString
			hMethod      sql.NullString
			hIsEnabled   sql.NullBool
			hInterval    sql.NullInt64
			hTimeout     sql.NullInt64
			hMaxAttempts sql.NullInt64
			hKeywordsRaw sql.NullString
		)

		if err := httpConfigRows.Scan(
			&hID, &hMonitorID, &hScheme, &hPath, &hMethod, &hIsEnabled,
			&hInterval, &hTimeout, &hMaxAttempts, &hKeywordsRaw,
		); err != nil {
			return nil, fmt.Errorf("%s: scan http config: %w", op, err)
		}

		m, _ := monitors[hMonitorID.UUID]

		keywords := make([]string, 0)

		if hKeywordsRaw.Valid && hKeywordsRaw.String != "" {
			err := json.Unmarshal([]byte(hKeywordsRaw.String), keywords)
			if err != nil {
				return []monitor.Monitor{}, fmt.Errorf("%s: error unmarshal keywords from base: %w", op, err)
			}
		}

		cfg := monitor.ReconstructHTTPConfig(
			hID.UUID,
			hMonitorID.UUID,
			hScheme.String,
			hPath.String,
			hMethod.String,
			hIsEnabled.Bool,
			int(hInterval.Int64),
			int(hTimeout.Int64),
			int(hMaxAttempts.Int64),
			keywords,
		)

		m.HTTPConfigs = append(m.HTTPConfigs, cfg)
		monitors[m.ID] = m
	}

	if err := httpConfigRows.Err(); err != nil {
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
			m.id, m.host,
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
