package event

import (
	"time"

	"github.com/google/uuid"
)

// Changed is a candidate-safe lifecycle notification. It intentionally
// contains no answer text, prompts, tokens, or provider payloads.
type Changed struct {
	OrganizationID uuid.UUID `json:"organization_id"`
	InterviewID    uuid.UUID `json:"interview_id"`
	ActorID        uuid.UUID `json:"actor_id,omitempty"`
	EventType      string    `json:"event_type"`
	Status         string    `json:"status,omitempty"`
	Version        int       `json:"version,omitempty"`
	Timestamp      time.Time `json:"timestamp"`
}

// EnvelopeType is explicit because the generic struct-name fallback would emit
// "changed", which is not the versioned interview lifecycle topic contract.
func (Changed) EnvelopeType() string { return "interview.changed" }
