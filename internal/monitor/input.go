package monitor

type CreateMonitorInput struct {
	Name         string
	URL          string
	CheckConfigs []CreateMonitorCheckConfigInput
}

type CreateMonitorCheckConfigInput struct {
	CheckType         CheckType
	IsEnabled         *bool
	CheckInterval     int
	CheckTimeout      int
	MaxAttempts       int
	DoErrorScreenshot bool
	Keywords          []string
}

type UpdateMonitorInput struct {
	Name   *string
	URL    *string
	Status *MonitorStatus
}

type UpdateMonitorCheckConfigInput struct {
	CheckType         *CheckType
	IsEnabled         *bool
	CheckInterval     *int
	CheckTimeout      *int
	MaxAttempts       *int
	DoErrorScreenshot *bool
	Keywords          *[]string
}
