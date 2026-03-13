package storage

import "errors"

var (
	ErrMonitorNotFound = errors.New("monitor not found")
	ErrMonitorExists   = errors.New("monitor already exists")
)
