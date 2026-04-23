package monitor

import "github.com/google/uuid"

func New(in CreateMonitorInput) (Monitor, error) {
	id, err := uuid.NewV7()
	if err != nil {
		return Monitor{}, err
	}
	if in.Name == "" {
		return Monitor{}, ErrMonitorEmptyName
	}

	if in.URL == "" {
		return Monitor{}, ErrMonitorEmptyURL
	}

	configs, err := buildConfigs(id, in.CheckConfigs)
	if err != nil {
		return Monitor{}, err
	}

	return Monitor{
		ID:           id,
		Name:         in.Name,
		URL:          in.URL,
		CheckConfigs: configs,
	}, nil
}

func buildConfigs(id uuid.UUID, configs []CreateMonitorCheckConfigInput) ([]MonitorCheckConfig, error) {
	var checks []MonitorCheckConfig
	for _, chk := range configs {
		var checkEnable bool
		if chk.IsEnabled == nil {
			checkEnable = true
		} else {
			checkEnable = *chk.IsEnabled
		}
		checkId, err := uuid.NewV7()
		if err != nil {
			return []MonitorCheckConfig{}, err
		}
		checks = append(checks, MonitorCheckConfig{
			ID:                checkId,
			MonitorID:         id,
			CheckType:         chk.CheckType,
			IsEnabled:         checkEnable,
			CheckInterval:     chk.CheckInterval,
			CheckTimeout:      chk.CheckTimeout,
			MaxAttempts:       chk.MaxAttempts,
			DoErrorScreenshot: chk.DoErrorScreenshot,
			Keywords:          chk.Keywords,
		})
	}
	return checks, nil
}
