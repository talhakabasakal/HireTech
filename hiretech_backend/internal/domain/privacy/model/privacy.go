package model

import (
	"time"

	"github.com/google/uuid"
)

type RequestKind string

const (
	RequestDeletion RequestKind = "deletion"
	RequestExport   RequestKind = "export"
)

type RequestStatus string

const (
	StatusPending  RequestStatus = "pending"
	StatusBlocked  RequestStatus = "blocked"
	StatusRunning  RequestStatus = "running"
	StatusComplete RequestStatus = "complete"
	StatusFailed   RequestStatus = "failed"
)

type Request struct {
	ID             uuid.UUID     `json:"id"`
	UserID         uuid.UUID     `json:"user_id"`
	OrganizationID uuid.UUID     `json:"organization_id"`
	Kind           RequestKind   `json:"kind"`
	Status         RequestStatus `json:"status"`
	Reason         string        `json:"reason,omitempty"`
	Manifest       []byte        `json:"manifest,omitempty"`
	CreatedAt      time.Time     `json:"created_at"`
	CompletedAt    *time.Time    `json:"completed_at,omitempty"`
}

type LegalHold struct {
	ID             uuid.UUID  `json:"id"`
	OrganizationID uuid.UUID  `json:"organization_id"`
	UserID         *uuid.UUID `json:"user_id,omitempty"`
	ResourceType   string     `json:"resource_type"`
	ResourceID     string     `json:"resource_id"`
	Reason         string     `json:"reason"`
	CreatedBy      uuid.UUID  `json:"created_by"`
	CreatedAt      time.Time  `json:"created_at"`
	ReleasedAt     *time.Time `json:"released_at,omitempty"`
}
