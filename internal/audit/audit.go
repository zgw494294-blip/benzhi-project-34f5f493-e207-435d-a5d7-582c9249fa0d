package audit

import (
	"sync"
	"time"
)

type Event struct {
	ID          string    `json:"id"`
	CaseID      string    `json:"caseID"`
	Action      string    `json:"action"`
	Actor       string    `json:"actor"`
	RequestKey  string    `json:"requestKey"`
	FromVersion int64     `json:"fromVersion"`
	ToVersion   int64     `json:"toVersion"`
	FromStatus  string    `json:"fromStatus"`
	ToStatus    string    `json:"toStatus"`
	At          time.Time `json:"at"`
	Detail      string    `json:"detail,omitempty"`
}

type Logger struct {
	mu     sync.RWMutex
	events map[string][]Event
}

func New() *Logger { return &Logger{events: map[string][]Event{}} }

func (l *Logger) Record(event Event) {
	l.events[event.CaseID] = append(l.events[event.CaseID], event)
}

func (l *Logger) Timeline(caseID string) []Event {
	result := append([]Event(nil), l.events[caseID]...)
	return result
}

func (l *Logger) All() []Event {
	var result []Event
	for _, events := range l.events {
		result = append(result, events...)
	}
	return result
}
