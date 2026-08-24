package storage

import (
	"encoding/json"
	"time"

	"powerpermit/internal/domain"
)

type Event struct {
	Sequence       int64           `json:"sequence"`
	CaseID         string          `json:"caseID"`
	Kind           string          `json:"kind"`
	Actor          string          `json:"actor"`
	IdempotencyKey string          `json:"idempotencyKey"`
	Version        int64           `json:"version"`
	OccurredAt     time.Time       `json:"occurredAt"`
	Payload        json.RawMessage `json:"payload"`
	PreviousHash   string          `json:"previousHash"`
	Hash           string          `json:"hash"`
}

type Snapshot struct {
	Sequence    int64                       `json:"sequence"`
	LastHash    string                      `json:"lastHash"`
	Cases       map[string]domain.Aggregate `json:"cases"`
	Idempotency map[string]CommandResult    `json:"idempotency"`
	Checksum    string                      `json:"checksum"`
}

type CommandResult struct {
	CaseID    string          `json:"caseID"`
	Version   int64           `json:"version"`
	EventKind string          `json:"eventKind"`
	Response  json.RawMessage `json:"response,omitempty"`
}

type Mutation struct {
	CaseID          string
	ExpectedVersion int64
	Kind            string
	Actor           string
	IdempotencyKey  string
	Aggregate       domain.Aggregate
	Response        any
}
