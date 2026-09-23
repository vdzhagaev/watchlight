package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/vdzhagaev/watchlight/internal/monitor"
)

func (s *Storage) GetOpenByMonitor(ctx context.Context, monitorID uuid.UUID) (incident monitor.Incident, found bool, outputErr error) {
	const op = "storage.sqlite.incident.GetOpenByMonitor"

	incidentQuery := `
		SELECT
			id, monitor_id, resolved_reason,
			started_at, resolved_at
		FROM incidents
		WHERE monitor_id = ?
			AND resolved_at IS NULL
	`
	var (
		incID             uuid.UUID
		incMonitorID      uuid.UUID
		incResolvedReason sql.NullString
		incStartedAt      time.Time
		incResolvedAt     sql.NullTime
	)

	err := s.db.QueryRowContext(ctx, incidentQuery, monitorID).Scan(
		&incID, &incMonitorID,
		&incResolvedReason, &incStartedAt,
		&incResolvedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return
		}
		outputErr = fmt.Errorf("%s: %w", op, err)

		return
	}

	reasonsQuery := `
		SELECT
			config_id, config_type,
			last_error, latest_result_id,
			started_at, last_seen_at
		FROM incident_reasons
		WHERE incident_id = ?
	`

	rows, outputErr := s.db.QueryContext(ctx, reasonsQuery, incID)
	if outputErr != nil {
		outputErr = fmt.Errorf("%s: %w", op, outputErr)
		return
	}
	defer rows.Close()

	reasons := make(map[uuid.UUID]monitor.Reason, 0)

	for rows.Next() {
		var (
			rConfigID       uuid.UUID
			rConfigType     string
			rLastError      string
			rLatestResultID uuid.UUID
			rStartedAt      time.Time
			rLastSeenAt     time.Time
		)

		if err := rows.Scan(
			&rConfigID, &rConfigType,
			&rLastError, &rLatestResultID,
			&rStartedAt, &rLastSeenAt,
		); err != nil {
			outputErr = fmt.Errorf("%s: scan reasons: %w", op, err)
			return
		}

		reasons[rConfigID] = monitor.Reason{
			ConfigID:       rConfigID,
			ConfigType:     monitor.ConfigType(rConfigType),
			LastError:      rLastError,
			LatestResultID: rLatestResultID,
			StartedAt:      rStartedAt,
			LastSeenAt:     rLastSeenAt,
		}
	}

	if err := rows.Err(); err != nil {
		outputErr = fmt.Errorf("%s: collect reasons for incident error: %w", op, err)
		return
	}

	var resolvedAt *time.Time

	if incResolvedAt.Valid {
		resolvedAt = &incResolvedAt.Time
	}

	incident = monitor.Incident{
		ID:             incID,
		MonitorID:      incMonitorID,
		Reasons:        reasons,
		ResolvedReason: monitor.ResolvedReason(incResolvedReason.String),
		StartedAt:      incStartedAt,
		ResolvedAt:     resolvedAt,
	}
	found = true
	return
}

func (s *Storage) Save(ctx context.Context, inc monitor.Incident) error {
	const op = "storage.sqlite.incident.Save"

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}
	defer tx.Rollback()

	_, err = tx.ExecContext(ctx,
		`
		INSERT INTO incidents(
			id, monitor_id,
			resolved_reason,
			started_at, resolved_at
		)
		VALUES(?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE
		SET
			resolved_reason = excluded.resolved_reason,
			resolved_at = excluded.resolved_at
		`,
		inc.ID, inc.MonitorID,
		inc.ResolvedReason,
		inc.StartedAt, inc.ResolvedAt,
	)
	if err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}

	_, err = tx.ExecContext(ctx, `
		DELETE FROM incident_reasons
		WHERE incident_id = ?
	`, inc.ID)
	if err != nil {
		return fmt.Errorf("%s: clean incident reasons: %w", op, err)
	}

	stmt, err := tx.PrepareContext(ctx,
		`
		INSERT INTO incident_reasons(
			incident_id, config_id,
			config_type, last_error,
			latest_result_id, started_at,
			last_seen_at
		) VALUES (?, ?, ?, ?, ?, ?, ?)
		`,
	)
	if err != nil {
		return fmt.Errorf("%s: prepare stmt: %w", op, err)
	}
	defer stmt.Close()

	for _, r := range inc.Reasons {
		_, err := stmt.ExecContext(ctx,
			inc.ID, r.ConfigID,
			r.ConfigType, r.LastError,
			r.LatestResultID, r.StartedAt,
			r.LastSeenAt,
		)
		if err != nil {
			return fmt.Errorf("%s: insert incident reason: %w", op, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("%s: commit: %w", op, err)
	}

	return nil
}

func (s *Storage) ListMonitorIDsWithOpenIncident(ctx context.Context) (map[uuid.UUID]struct{}, error) {
	const op = "storage.sqlite.incident.ListMonitorIDsWithOpenIncident"

	query := `
		SELECT
			monitor_id
		FROM incidents
		WHERE resolved_at IS NULL
	`

	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}
	defer rows.Close()

	monitorIDs := make(map[uuid.UUID]struct{}, 0)

	for rows.Next() {
		var monitorID uuid.UUID
		if err := rows.Scan(&monitorID); err != nil {
			return nil, fmt.Errorf("%s: %w", op, err)
		}

		monitorIDs[monitorID] = struct{}{}
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("%s: collect monitor ids error: %w", op, err)
	}

	return monitorIDs, nil
}
