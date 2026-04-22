package memory

import "vdzhagev/go-uptime-checker/internal/monitor"

var monitors = []monitor.Monitor{
	{
		ID:     1,
		Name:   "Google Search",
		URL:    "https://google.com",
		Status: monitor.MonitorUp,
		CheckConfigs: []monitor.MonitorCheckConfig{
			{ID: 101, MonitorID: 1, CheckType: monitor.CheckHTTP, IsEnabled: true, CheckInterval: 60, CheckTimeout: 5, MaxAttempts: 3, Keywords: []string{"google", "search"}},
		},
	},
	{
		ID:     2,
		Name:   "GitHub API",
		URL:    "https://api.github.com",
		Status: monitor.MonitorUp,
		CheckConfigs: []monitor.MonitorCheckConfig{
			{ID: 102, MonitorID: 2, CheckType: monitor.CheckHTTP, IsEnabled: true, CheckInterval: 30, CheckTimeout: 10, MaxAttempts: 2},
		},
	},
	{
		ID:     3,
		Name:   "Internal DB Gateway",
		URL:    "192.168.1.50",
		Status: monitor.MonitorDown,
		CheckConfigs: []monitor.MonitorCheckConfig{
			{ID: 103, MonitorID: 3, CheckType: monitor.CheckPing, IsEnabled: true, CheckInterval: 15, CheckTimeout: 2, MaxAttempts: 5},
		},
	},
	{
		ID:     4,
		Name:   "Auth Frontend (Headless)",
		URL:    "https://auth.example.com/login",
		Status: monitor.MonitorUp,
		CheckConfigs: []monitor.MonitorCheckConfig{
			{ID: 104, MonitorID: 4, CheckType: monitor.CheckHeadless, IsEnabled: true, CheckInterval: 300, CheckTimeout: 30, MaxAttempts: 2, DoErrorScreenshot: true},
		},
	},
	{
		ID:     5,
		Name:   "Legacy CRM",
		URL:    "http://crm.local",
		Status: monitor.MonitorUnknown,
		CheckConfigs: []monitor.MonitorCheckConfig{
			{ID: 105, MonitorID: 5, CheckType: monitor.CheckHTTP, IsEnabled: false, CheckInterval: 600, CheckTimeout: 15, MaxAttempts: 1},
		},
	},
	{
		ID:     6,
		Name:   "Main Website",
		URL:    "https://my-cool-startup.io",
		Status: monitor.MonitorUp,
		CheckConfigs: []monitor.MonitorCheckConfig{
			{ID: 106, MonitorID: 6, CheckType: monitor.CheckHTTP, IsEnabled: true, CheckInterval: 60, CheckTimeout: 7, MaxAttempts: 3, Keywords: []string{"Welcome"}},
			{ID: 107, MonitorID: 6, CheckType: monitor.CheckPing, IsEnabled: true, CheckInterval: 60, CheckTimeout: 3, MaxAttempts: 3},
		},
	},
	{
		ID:     7,
		Name:   "Payment Gateway",
		URL:    "https://stripe.com",
		Status: monitor.MonitorUp,
		CheckConfigs: []monitor.MonitorCheckConfig{
			{ID: 108, MonitorID: 7, CheckType: monitor.CheckHTTP, IsEnabled: true, CheckInterval: 30, CheckTimeout: 5, MaxAttempts: 3},
		},
	},
	{
		ID:     8,
		Name:   "Backup Server",
		URL:    "backup.local.net",
		Status: monitor.MonitorDown,
		CheckConfigs: []monitor.MonitorCheckConfig{
			{ID: 109, MonitorID: 8, CheckType: monitor.CheckPing, IsEnabled: true, CheckInterval: 120, CheckTimeout: 10, MaxAttempts: 10},
		},
	},
	{
		ID:     9,
		Name:   "Admin Panel",
		URL:    "https://admin.example.com",
		Status: monitor.MonitorUp,
		CheckConfigs: []monitor.MonitorCheckConfig{
			{ID: 110, MonitorID: 9, CheckType: monitor.CheckHeadless, IsEnabled: true, CheckInterval: 600, CheckTimeout: 45, MaxAttempts: 2, DoErrorScreenshot: true},
		},
	},
	{
		ID:     10,
		Name:   "Mail Server (SMTP)",
		URL:    "smtp.mailtrap.io",
		Status: monitor.MonitorUnknown,
		CheckConfigs: []monitor.MonitorCheckConfig{
			{ID: 111, MonitorID: 10, CheckType: monitor.CheckPing, IsEnabled: true, CheckInterval: 60, CheckTimeout: 5, MaxAttempts: 3},
		},
	},
}
