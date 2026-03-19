package memory

import "vdzhagev/go-uptime-checker/internal/domain"

var monitors = []domain.Monitor{
	{
		ID:     1,
		Name:   "Google Search",
		URL:    "https://google.com",
		Status: domain.MonitorUp,
		CheckConfigs: []domain.MonitorCheckConfig{
			{ID: 101, MonitorID: 1, CheckType: domain.CheckHTTP, IsEnabled: true, CheckInterval: 60, MaxAttempts: 3, Keywords: []string{"google", "search"}},
		},
	},
	{
		ID:     2,
		Name:   "GitHub API",
		URL:    "https://api.github.com",
		Status: domain.MonitorUp,
		CheckConfigs: []domain.MonitorCheckConfig{
			{ID: 102, MonitorID: 2, CheckType: domain.CheckHTTP, IsEnabled: true, CheckInterval: 30, MaxAttempts: 2},
		},
	},
	{
		ID:     3,
		Name:   "Internal DB Gateway",
		URL:    "192.168.1.50",
		Status: domain.MonitorDown,
		CheckConfigs: []domain.MonitorCheckConfig{
			{ID: 103, MonitorID: 3, CheckType: domain.CheckPing, IsEnabled: true, CheckInterval: 15, MaxAttempts: 5},
		},
	},
	{
		ID:     4,
		Name:   "Auth Frontend (Headless)",
		URL:    "https://auth.example.com/login",
		Status: domain.MonitorUp,
		CheckConfigs: []domain.MonitorCheckConfig{
			{ID: 104, MonitorID: 4, CheckType: domain.CheckHeadless, IsEnabled: true, CheckInterval: 300, MaxAttempts: 2, DoErrorScreenshot: true},
		},
	},
	{
		ID:     5,
		Name:   "Legacy CRM",
		URL:    "http://crm.local",
		Status: domain.MonitorUnknown,
		CheckConfigs: []domain.MonitorCheckConfig{
			{ID: 105, MonitorID: 5, CheckType: domain.CheckHTTP, IsEnabled: false, CheckInterval: 600, MaxAttempts: 1},
		},
	},
	{
		ID:     6,
		Name:   "Main Website",
		URL:    "https://my-cool-startup.io",
		Status: domain.MonitorUp,
		CheckConfigs: []domain.MonitorCheckConfig{
			{ID: 106, MonitorID: 6, CheckType: domain.CheckHTTP, IsEnabled: true, CheckInterval: 60, MaxAttempts: 3, Keywords: []string{"Welcome"}},
			{ID: 107, MonitorID: 6, CheckType: domain.CheckPing, IsEnabled: true, CheckInterval: 60, MaxAttempts: 3},
		},
	},
	{
		ID:     7,
		Name:   "Payment Gateway",
		URL:    "https://stripe.com",
		Status: domain.MonitorUp,
		CheckConfigs: []domain.MonitorCheckConfig{
			{ID: 108, MonitorID: 7, CheckType: domain.CheckHTTP, IsEnabled: true, CheckInterval: 30, MaxAttempts: 3},
		},
	},
	{
		ID:     8,
		Name:   "Backup Server",
		URL:    "backup.local.net",
		Status: domain.MonitorDown,
		CheckConfigs: []domain.MonitorCheckConfig{
			{ID: 109, MonitorID: 8, CheckType: domain.CheckPing, IsEnabled: true, CheckInterval: 120, MaxAttempts: 10},
		},
	},
	{
		ID:     9,
		Name:   "Admin Panel",
		URL:    "https://admin.example.com",
		Status: domain.MonitorUp,
		CheckConfigs: []domain.MonitorCheckConfig{
			{ID: 110, MonitorID: 9, CheckType: domain.CheckHeadless, IsEnabled: true, CheckInterval: 600, MaxAttempts: 2, DoErrorScreenshot: true},
		},
	},
	{
		ID:     10,
		Name:   "Mail Server (SMTP)",
		URL:    "smtp.mailtrap.io",
		Status: domain.MonitorUnknown,
		CheckConfigs: []domain.MonitorCheckConfig{
			{ID: 111, MonitorID: 10, CheckType: domain.CheckPing, IsEnabled: true, CheckInterval: 60, MaxAttempts: 3},
		},
	},
}
