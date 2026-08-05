package monitor

import (
	"fmt"
	"time"

	"github.com/google/uuid"
)

type ResolvedReason string

const (
	ResolveByRecovered     ResolvedReason = "recovered"
	ResolveByConfigRemoved ResolvedReason = "config_removed"
)

type Incident struct {
	ID             uuid.UUID
	MonitorID      uuid.UUID
	Reasons        map[uuid.UUID]Reason // uuid = configID
	ResolvedReason ResolvedReason
	StartedAt      time.Time
	ResolvedAt     *time.Time
}

type Reason struct {
	ConfigID       uuid.UUID
	LastError      string
	LatestResultID uuid.UUID
	StartedAt      time.Time
	LastSeenAt     time.Time
}

func OpenIncident(monitorID uuid.UUID, first Reason) (Incident, error) {
	id, err := uuid.NewV7()
	if err != nil {
		return Incident{}, err
	}

	// TODO: append validation

	return Incident{
		ID:        id,
		MonitorID: monitorID,
		Reasons: map[uuid.UUID]Reason{
			first.ConfigID: first,
		},
		StartedAt: first.StartedAt,
	}, nil
}

func (inc *Incident) RecordFailure(r Reason) {
	if oldR, ok := inc.Reasons[r.ConfigID]; ok {
		r.StartedAt = oldR.StartedAt
	}
	inc.Reasons[r.ConfigID] = r
}

// RecoverReason removes the reason for configID after a successful check and
// reports whether that closed the incident (no reasons left). It requires the
// reason to exist: the caller must only invoke it on a down→up transition,
// i.e. after checking inc.Reasons holds configID. A missing reason means the
// caller's transition logic disagrees with incident state and is returned as
// an error, not silently ignored.
func (inc *Incident) RecoverReason(configID uuid.UUID, at time.Time) (bool, error) {
	_, ok := inc.Reasons[configID]
	if !ok {
		return false, fmt.Errorf("reason not found or already closed")
	}
	delete(inc.Reasons, configID)
	return inc.tryClose(ResolveByRecovered, at), nil
}

// RemoveReason drops configID's reason when its config is deleted or disabled,
// and reports whether that closed the incident. Unlike RecoverReason it is a
// no-op for an absent reason: a removed config may never have been failing, so
// its absence is normal rather than an error.
func (inc *Incident) RemoveReason(configID uuid.UUID, at time.Time) bool {
	if _, ok := inc.Reasons[configID]; !ok {
		return false
	}
	delete(inc.Reasons, configID)
	return inc.tryClose(ResolveByConfigRemoved, at)
}

func (inc *Incident) tryClose(reason ResolvedReason, at time.Time) bool {
	if len(inc.Reasons) == 0 {
		inc.close(reason, at)
		return true
	}
	return false
}

func (inc *Incident) close(reason ResolvedReason, at time.Time) {
	if inc.ResolvedAt != nil {
		return
	}
	inc.ResolvedReason = reason
	inc.ResolvedAt = &at

}
