package monitor

import "github.com/google/uuid"

type EventType string

const (
	EventCreated EventType = "created"
	EventUpdated EventType = "updated"
	EventDeleted EventType = "deleted"
)

type ConfigChangeEvent struct {
	Type      EventType
	MonitorID uuid.UUID
	Job       CheckJob
}
