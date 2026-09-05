package model

import (
	"time"

	"github.com/google/uuid"
)

type DraftStatus string

const (
	DraftPending  DraftStatus = "pending"
	DraftApproved DraftStatus = "approved"
	DraftRejected DraftStatus = "rejected"
)

type QuestionDraft struct {
	ID               uuid.UUID
	OrganizationID   uuid.UUID
	InterviewID      uuid.UUID
	RequestedBy      uuid.UUID
	ReviewedBy       *uuid.UUID
	Type             QuestionType
	Prompt           string
	CompetencyIDs    []string
	Difficulty       int
	TimeLimitSeconds *int
	Language         InterviewLanguage
	Status           DraftStatus
	ModelID          string
	ModelVersion     string
	ReviewNotes      string
	CreatedAt        time.Time
	ReviewedAt       *time.Time
}
