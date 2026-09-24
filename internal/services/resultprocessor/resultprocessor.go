package resultprocessor

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/vdzhagaev/watchlight/internal/monitor"
)

type ResultProcessor struct {
	checkResultSaver CheckResultSaver
	incidentStore    IncidentStore
}

type CheckResultSaver interface {
	SaveCheckResult(ctx context.Context, r monitor.CheckResult) error
}

type IncidentStore interface {
	GetOpenByMonitor(ctx context.Context, monitorID uuid.UUID) (monitor.Incident, bool, error)
	Save(ctx context.Context, inc monitor.Incident) error
}

type Params struct {
	CheckResultSaver CheckResultSaver
	IncidentStore    IncidentStore
}

func New(p Params) *ResultProcessor {
	return &ResultProcessor{
		checkResultSaver: p.CheckResultSaver,
		incidentStore:    p.IncidentStore,
	}
}

func (rp *ResultProcessor) HandleCheckResult(ctx context.Context, in monitor.CheckResultInput) error {
	cr, err := buildCheckResult(in)
	if err != nil {
		return err
	}

	err = rp.checkResultSaver.SaveCheckResult(ctx, cr)
	if err != nil {
		return err
	}

	inc, exists, err := rp.incidentStore.GetOpenByMonitor(ctx, cr.MonitorID)
	if err != nil {
		return err
	}

	switch {
	case exists && cr.Status == monitor.CheckSuccess: // Need recover reason
		// if reason by config not in reasons - skip
		_, ok := inc.Reasons[cr.ConfigID]
		if !ok {
			return nil
		}
		_, err := inc.RecoverReason(cr.ConfigID, cr.CheckedAt)
		if err != nil {
			return err // maybe need only warning or recononicle
		}
		err = rp.incidentStore.Save(ctx, inc)
		if err != nil {
			return err
		}
		return nil

	case !exists && cr.Status == monitor.CheckSuccess: // Do nothing
		return nil
	case exists && cr.Status == monitor.CheckFailure: // New reason
		r, err := buildReason(cr)
		if err != nil {
			return err
		}
		inc.RecordFailure(r)
		return rp.incidentStore.Save(ctx, inc)
	case !exists && cr.Status == monitor.CheckFailure: // Need open new incident
		r, err := buildReason(cr)
		if err != nil {
			return err
		}
		inc, err := monitor.OpenIncident(cr.MonitorID, r)
		if err != nil {
			return err
		}
		err = rp.incidentStore.Save(ctx, inc)
		if err != nil {
			return err
		}
		return nil

	default:
		panic(fmt.Sprintf("undefined check status: %s", cr.Status))
	}
}

func buildReason(cr monitor.CheckResult) (monitor.Reason, error) {
	var cfgType monitor.ConfigType
	switch cr.CheckType {
	case monitor.CheckPing:
		cfgType = monitor.PingConfigType
	case monitor.CheckHTTP:
		cfgType = monitor.HTTPConfigType
	default:
		return monitor.Reason{}, fmt.Errorf("undefined check type in check result: %s", cr.CheckType)
	}
	return monitor.Reason{
		ConfigID:       cr.ConfigID,
		ConfigType:     cfgType,
		LastError:      cr.Error,
		LatestResultID: cr.ID,
		StartedAt:      cr.CheckedAt,
		LastSeenAt:     cr.CheckedAt,
	}, nil
}

func buildCheckResult(in monitor.CheckResultInput) (monitor.CheckResult, error) {
	status := monitor.CheckFailure
	if in.Reachable && in.Error == nil {
		status = monitor.CheckSuccess
	}

	var errorMessage string
	if in.Error != nil {
		errorMessage = in.Error.Error()
	}

	id, err := uuid.NewV7()
	if err != nil {
		return monitor.CheckResult{}, err
	}

	return monitor.CheckResult{
		ID:            id,
		MonitorID:     in.MonitorID,
		ConfigID:      in.ConfigID,
		CheckType:     in.CheckType,
		Status:        status,
		StatusCode:    in.StatusCode,
		ResponseTime:  in.ResponseTime,
		CheckedAt:     in.CheckedAt,
		Error:         errorMessage,
		FoundKeywords: in.FoundKeywords,
	}, nil
}
