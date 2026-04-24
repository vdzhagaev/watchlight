package memory

import (
	"github.com/vdzhagaev/watchlight/internal/monitor"

	"github.com/google/uuid"
)

var newID = uuid.MustParse

var (
	idMon1  = newID("01931d4f-0000-7000-8000-000000000001")
	idMon2  = newID("01931d4f-0000-7000-8000-000000000002")
	idMon3  = newID("01931d4f-0000-7000-8000-000000000003")
	idMon4  = newID("01931d4f-0000-7000-8000-000000000004")
	idMon5  = newID("01931d4f-0000-7000-8000-000000000005")
	idMon6  = newID("01931d4f-0000-7000-8000-000000000006")
	idMon7  = newID("01931d4f-0000-7000-8000-000000000007")
	idMon8  = newID("01931d4f-0000-7000-8000-000000000008")
	idMon9  = newID("01931d4f-0000-7000-8000-000000000009")
	idMon10 = newID("01931d4f-0000-7000-8000-00000000000a")
)

var SampleMonitors = []monitor.Monitor{
	{
		ID:     idMon1,
		Name:   "Google Search",
		URL:    "https://google.com",
		Status: monitor.MonitorUp,
		CheckConfigs: []monitor.MonitorCheckConfig{
			{ID: newID("01931d4f-0000-7000-8000-000000000101"), MonitorID: idMon1, CheckType: monitor.CheckHTTP, IsEnabled: true, CheckInterval: 60, CheckTimeout: 5, MaxAttempts: 3, Keywords: []string{"google", "search"}},
		},
	},
	{
		ID:     idMon2,
		Name:   "GitHub API",
		URL:    "https://api.github.com",
		Status: monitor.MonitorUp,
		CheckConfigs: []monitor.MonitorCheckConfig{
			{ID: newID("01931d4f-0000-7000-8000-000000000102"), MonitorID: idMon2, CheckType: monitor.CheckHTTP, IsEnabled: true, CheckInterval: 30, CheckTimeout: 10, MaxAttempts: 2},
		},
	},
	{
		ID:     idMon3,
		Name:   "Internal DB Gateway",
		URL:    "192.168.1.50",
		Status: monitor.MonitorDown,
		CheckConfigs: []monitor.MonitorCheckConfig{
			{ID: newID("01931d4f-0000-7000-8000-000000000103"), MonitorID: idMon3, CheckType: monitor.CheckPing, IsEnabled: true, CheckInterval: 15, CheckTimeout: 2, MaxAttempts: 5},
		},
	},
	{
		ID:     idMon4,
		Name:   "Auth Frontend (Headless)",
		URL:    "https://auth.example.com/login",
		Status: monitor.MonitorUp,
		CheckConfigs: []monitor.MonitorCheckConfig{
			{ID: newID("01931d4f-0000-7000-8000-000000000104"), MonitorID: idMon4, CheckType: monitor.CheckHeadless, IsEnabled: true, CheckInterval: 300, CheckTimeout: 30, MaxAttempts: 2, DoErrorScreenshot: true},
		},
	},
	{
		ID:     idMon5,
		Name:   "Legacy CRM",
		URL:    "http://crm.local",
		Status: monitor.MonitorUnknown,
		CheckConfigs: []monitor.MonitorCheckConfig{
			{ID: newID("01931d4f-0000-7000-8000-000000000105"), MonitorID: idMon5, CheckType: monitor.CheckHTTP, IsEnabled: false, CheckInterval: 600, CheckTimeout: 15, MaxAttempts: 1},
		},
	},
	{
		ID:     idMon6,
		Name:   "Main Website",
		URL:    "https://my-cool-startup.io",
		Status: monitor.MonitorUp,
		CheckConfigs: []monitor.MonitorCheckConfig{
			{ID: newID("01931d4f-0000-7000-8000-000000000106"), MonitorID: idMon6, CheckType: monitor.CheckHTTP, IsEnabled: true, CheckInterval: 60, CheckTimeout: 7, MaxAttempts: 3, Keywords: []string{"Welcome"}},
			{ID: newID("01931d4f-0000-7000-8000-000000000107"), MonitorID: idMon6, CheckType: monitor.CheckPing, IsEnabled: true, CheckInterval: 60, CheckTimeout: 3, MaxAttempts: 3},
		},
	},
	{
		ID:     idMon7,
		Name:   "Payment Gateway",
		URL:    "https://stripe.com",
		Status: monitor.MonitorUp,
		CheckConfigs: []monitor.MonitorCheckConfig{
			{ID: newID("01931d4f-0000-7000-8000-000000000108"), MonitorID: idMon7, CheckType: monitor.CheckHTTP, IsEnabled: true, CheckInterval: 30, CheckTimeout: 5, MaxAttempts: 3},
		},
	},
	{
		ID:     idMon8,
		Name:   "Backup Server",
		URL:    "backup.local.net",
		Status: monitor.MonitorDown,
		CheckConfigs: []monitor.MonitorCheckConfig{
			{ID: newID("01931d4f-0000-7000-8000-000000000109"), MonitorID: idMon8, CheckType: monitor.CheckPing, IsEnabled: true, CheckInterval: 120, CheckTimeout: 10, MaxAttempts: 10},
		},
	},
	{
		ID:     idMon9,
		Name:   "Admin Panel",
		URL:    "https://admin.example.com",
		Status: monitor.MonitorUp,
		CheckConfigs: []monitor.MonitorCheckConfig{
			{ID: newID("01931d4f-0000-7000-8000-00000000010a"), MonitorID: idMon9, CheckType: monitor.CheckHeadless, IsEnabled: true, CheckInterval: 600, CheckTimeout: 45, MaxAttempts: 2, DoErrorScreenshot: true},
		},
	},
	{
		ID:     idMon10,
		Name:   "Mail Server (SMTP)",
		URL:    "smtp.mailtrap.io",
		Status: monitor.MonitorUnknown,
		CheckConfigs: []monitor.MonitorCheckConfig{
			{ID: newID("01931d4f-0000-7000-8000-00000000010b"), MonitorID: idMon10, CheckType: monitor.CheckPing, IsEnabled: true, CheckInterval: 60, CheckTimeout: 5, MaxAttempts: 3},
		},
	},
}
