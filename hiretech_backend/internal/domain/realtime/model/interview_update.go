package model

import (
	"time"

	"github.com/google/uuid"
)

// InterviewUpdate is the only payload exposed by the GraphQL subscription.
// It carries lifecycle metadata only; candidate answers, prompts, tokens, and
// provider payloads never cross the realtime boundary.
type InterviewUpdate struct {
	OrganizationID uuid.UUID
	InterviewID    uuid.UUID
	Cursor         string
	EventType      string
	Status         string
	Version        int
	OccurredAt     time.Time
}
