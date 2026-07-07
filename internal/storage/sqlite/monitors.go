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
		m.PingConfig.Interval.Seconds(),
		m.PingConfig.Timeout.Seconds(),
		m.PingConfig.MaxAttempts.Count(),
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
			string(cfg.Scheme),
			cfg.Path.String(),
			string(cfg.Method),
			cfg.IsEnabled,
			cfg.Interval.Seconds(),
			cfg.Timeout.Seconds(),
			cfg.MaxAttempts.Count(),
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

func (s *Storage) UpdateMonitor(ctx context.Context, id uuid.UUID, in monitor.UpdateMonitorInput) error {
	const op = "storage.sqlite.UpdateMonitor"

	// A nil field -> invalid NullString -> SQL NULL -> COALESCE keeps the column.
	// One atomic UPDATE does the partial merge; no read-modify-write race.
	var status *string
	if in.Status != nil {
		status = (*string)(in.Status)
	}

	res, err := s.db.ExecContext(
		ctx,
		`UPDATE monitors SET
			name = COALESCE(?, name),
			host = COALESCE(?, host),
			status = COALESCE(?, status)
		WHERE id = ?`,
		toNullString(in.Name), toNullString(in.Host), toNullString(status), id,
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

// toNullString maps an optional string to a nullable SQL argument: nil -> NULL.
func toNullString(p *string) sql.NullString {
	if p == nil {
		return sql.NullString{}
	}
	return sql.NullString{String: *p, Valid: true}
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
		mID          uuid.UUID
		mName        string
		mHost        string
		mStatus      string
		pID          uuid.UUID
		pPort        int
		pIsEnabled   bool
		pInterval    int
		pTimeout     int
		pMaxAttempts int
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
		pID,
		mID,
		uint16(pPort),
		pIsEnabled,
		pInterval,
		pTimeout,
		pMaxAttempts,
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
			hID          uuid.UUID
			hScheme      string
			hPath        string
			hMethod      string
			hIsEnabled   bool
			hInterval    int
			hTimeout     int
			hMaxAttempts int
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
			hID,
			mID,
			hScheme,
			hPath,
			hMethod,
			hIsEnabled,
			hInterval,
			hTimeout,
			hMaxAttempts,
			keywords,
		)
		httpConfigs = append(httpConfigs, cfg)

	}

	if err := rows.Err(); err != nil {
		return monitor.Monitor{}, fmt.Errorf("%s: collect http configs error: %w", op, err)
	}

	return monitor.ReconstructMonitor(mID, mName, mHost, mStatus, pingConfig, httpConfigs), nil
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
			p.timeout, p.max_attempts
		FROM monitors AS m
		JOIN ping_configs AS p ON m.id = p.monitor_id
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
			mID          uuid.UUID
			mName        string
			mHost        string
			mStatus      string
			pID          uuid.UUID
			pPort        int
			pIsEnabled   bool
			pInterval    int
			pTimeout     int
			pMaxAttempts int
		)

		if err := monitorPingRows.Scan(
			&mID, &mName, &mHost, &mStatus,
			&pID, &pPort, &pIsEnabled, &pInterval,
			&pTimeout, &pMaxAttempts,
		); err != nil {
			return nil, fmt.Errorf("%s: monitor-ping scan error: %w", op, err)
		}

		pingConfig := monitor.ReconstructPingConfig(
			pID,
			mID,
			uint16(pPort),
			pIsEnabled,
			pInterval,
			pTimeout,
			pMaxAttempts,
		)

		monitors[mID] = monitor.ReconstructMonitor(mID, mName, mHost, mStatus, pingConfig, []monitor.HTTPConfig{})
		order = append(order, mID)
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
			hID          uuid.UUID
			hMonitorID   uuid.UUID
			hScheme      string
			hPath        string
			hMethod      string
			hIsEnabled   bool
			hInterval    int
			hTimeout     int
			hMaxAttempts int
			hKeywordsRaw sql.NullString
		)

		if err := httpConfigRows.Scan(
			&hID, &hMonitorID, &hScheme, &hPath, &hMethod, &hIsEnabled,
			&hInterval, &hTimeout, &hMaxAttempts, &hKeywordsRaw,
		); err != nil {
			return nil, fmt.Errorf("%s: scan http config: %w", op, err)
		}

		m, _ := monitors[hMonitorID]

		keywords := make([]string, 0)

		if hKeywordsRaw.Valid && hKeywordsRaw.String != "" {
			err := json.Unmarshal([]byte(hKeywordsRaw.String), &keywords)
			if err != nil {
				return []monitor.Monitor{}, fmt.Errorf("%s: error unmarshal keywords from base: %w", op, err)
			}
		}

		cfg := monitor.ReconstructHTTPConfig(
			hID,
			hMonitorID,
			hScheme,
			hPath,
			hMethod,
			hIsEnabled,
			hInterval,
			hTimeout,
			hMaxAttempts,
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

	allPingConfigsQuery := `
		SELECT
			p.id, p.monitor_id, p.port, p.is_enabled, p.interval,
			p.timeout, p.max_attempts,
			m.host
		FROM ping_configs AS p
		JOIN monitors AS m ON m.id = p.monitor_id
		WHERE p.is_enabled = 1
		ORDER BY p.monitor_id
	`

	pingRows, err := s.db.QueryContext(ctx, allPingConfigsQuery)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}

	defer pingRows.Close()

	jobs := make([]monitor.CheckJob, 0)

	for pingRows.Next() {
		var (
			mHost        string
			pID          uuid.UUID
			pMonitorID   uuid.UUID
			pPort        int64
			pIsEnabled   bool
			pInterval    int
			pTimeout     int
			pMaxAttempts int
		)

		if err := pingRows.Scan(
			&pID, &pMonitorID, &pPort,
			&pIsEnabled, &pInterval,
			&pTimeout, &pMaxAttempts,
			&mHost,
		); err != nil {
			return nil, fmt.Errorf("%s: ping config scan error: %w", op, err)
		}

		host := monitor.ReconstructHost(mHost)

		pj := monitor.NewPingJob(monitor.CreatePingJobInput{
			MonitorID:   pMonitorID,
			ConfigID:    pID,
			Host:        host,
			Port:        uint16(pPort),
			Interval:    time.Second * time.Duration(pInterval),
			Timeout:     time.Second * time.Duration(pTimeout),
			MaxAttempts: pMaxAttempts,
		})

		jobs = append(jobs, pj)
	}

	if err := pingRows.Err(); err != nil {
		return nil, fmt.Errorf("%s: ping config rows iteration error: %w", op, err)
	}

	allHttpConfQuery := `
		SELECT
			h.id, h.monitor_id, h.scheme, h.path, h.method,
			h.interval, h.timeout, h.max_attempts, h.keywords,
			m.host
		FROM http_configs as h
		JOIN monitors as m ON m.id = h.monitor_id
		WHERE h.is_enabled = 1
		ORDER BY monitor_id
	`

	httpRows, err := s.db.QueryContext(ctx, allHttpConfQuery)
	if err != nil {
		return []monitor.CheckJob{}, fmt.Errorf("%s: %w", op, err)
	}
	defer httpRows.Close()

	for httpRows.Next() {
		var (
			mHost string

			hID          uuid.UUID
			hMonitorID   uuid.UUID
			hScheme      monitor.HTTPScheme
			hPath        string
			hMethod      monitor.HTTPMethod
			hInterval    int
			hTimeout     int
			hMaxAttempts int
			hKeywordsRaw sql.NullString
		)

		if err := httpRows.Scan(
			&hID, &hMonitorID, &hScheme,
			&hPath, &hMethod, &hInterval,
			&hTimeout, &hMaxAttempts,
			&hKeywordsRaw, &mHost,
		); err != nil {
			return []monitor.CheckJob{}, fmt.Errorf("%s: scan http config: %w", op, err)
		}
		var keywords []string

		if hKeywordsRaw.Valid && hKeywordsRaw.String != "" {
			err := json.Unmarshal([]byte(hKeywordsRaw.String), &keywords)
			if err != nil {
				return []monitor.CheckJob{}, fmt.Errorf("%s: error unmarshal keywords from base: %w", op, err)
			}
		}

		host := monitor.ReconstructHost(mHost)
		path := monitor.ReconstructPath(hPath)

		hj := monitor.NewHTTPJob(monitor.CreateHTTPJobInput{
			MonitorID:   hMonitorID,
			ConfigID:    hID,
			Scheme:      hScheme,
			Host:        host,
			Path:        path,
			Method:      hMethod,
			Interval:    time.Second * time.Duration(hInterval),
			Timeout:     time.Second * time.Duration(hTimeout),
			MaxAttempts: hMaxAttempts,
			Keywords:    keywords,
		})

		jobs = append(jobs, hj)
	}

	if err := httpRows.Err(); err != nil {
		return []monitor.CheckJob{}, fmt.Errorf("%s: http configs iteration error: %w", op, err)
	}

	return jobs, nil
}

func (s *Storage) SaveCheckResult(ctx context.Context, r monitor.CheckResult) error {
	const op = "storage.sqlite.SaveCheckResult"

	// check_results keys the config by type: exactly one of ping/http is set
	// (enforced by a table CHECK). CheckType tells us which column owns ConfigID.
	var pingID, httpID uuid.NullUUID
	switch r.CheckType {
	case monitor.CheckPing:
		pingID = uuid.NullUUID{UUID: r.ConfigID, Valid: true}
	case monitor.CheckHTTP:
		httpID = uuid.NullUUID{UUID: r.ConfigID, Valid: true}
	default:
		return fmt.Errorf("%s: cannot persist result for check type %q", op, r.CheckType)
	}

	_, err := s.db.ExecContext(ctx,
		`INSERT INTO check_results (
			id,
			monitor_id,
			ping_config_id,
			http_config_id,
			status,
			status_code,
			response_time_ns,
			checked_at,
			error_message
		)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		r.ID,
		r.MonitorID,
		pingID,
		httpID,
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
