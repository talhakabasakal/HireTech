package model

import (
	"time"

	"github.com/google/uuid"
)

type Status string

const (
	StatusDraft      Status = "draft"
	StatusReady      Status = "ready"
	StatusInvited    Status = "invited"
	StatusInProgress Status = "in_progress"
	StatusCompleted  Status = "completed"
	StatusCancelled  Status = "cancelled"
	StatusExpired    Status = "expired"
)

type InterviewLanguage string

const (
	LanguageTurkish InterviewLanguage = "tr"
	LanguageEnglish InterviewLanguage = "en"
)

type QuestionSource string

const (
	QuestionSourceHuman QuestionSource = "human"
	QuestionSourceAI    QuestionSource = "ai"
)

type Mode string

const (
	ModeAIDisabled      Mode = "ai_disabled"
	ModeGuidedAI        Mode = "guided_ai"
	ModeAICollaboration Mode = "ai_collaboration"
)

type Interview struct {
	ID                   uuid.UUID
	OrganizationID       uuid.UUID
	CreatedBy            uuid.UUID
	CandidateUserID      uuid.UUID
	CandidateEmail       string
	CandidateDisplayName string
	Title                string
	PositionTitle        string
	Seniority            string
	TechnologyTags       []string
	Mode                 Mode
	Language             InterviewLanguage
	QuestionSource       QuestionSource
	RubricVersion        string
	Status               Status
	StartsAt             *time.Time
	ExpiresAt            time.Time
	StartedAt            *time.Time
	CompletedAt          *time.Time
	CancelledAt          *time.Time
	Version              int
	CreatedAt            time.Time
	UpdatedAt            time.Time
	Questions            []*Question
	Answers              []*Answer
}

func (i *Interview) CanAddQuestion() bool {
	return i != nil && (i.Status == StatusDraft || i.Status == StatusReady)
}
func (i *Interview) CanPublish() bool {
	return i != nil && (i.Status == StatusReady || i.Status == StatusInvited)
}
func (i *Interview) IsTerminal() bool {
	return i == nil || i.Status == StatusCompleted || i.Status == StatusCancelled || i.Status == StatusExpired
}

type QuestionType string

const (
	QuestionTechnicalDiscussion QuestionType = "technical_discussion"
	QuestionCoding              QuestionType = "coding"
	QuestionSystemDesign        QuestionType = "system_design"
	QuestionDebugging           QuestionType = "debugging"
)

type Question struct {
	ID               uuid.UUID
	OrganizationID   uuid.UUID
	InterviewID      uuid.UUID
	Sequence         int
	Type             QuestionType
	Prompt           string
	CompetencyIDs    []string
	Difficulty       int
	TimeLimitSeconds *int
	CreatedAt        time.Time
}

type Invitation struct {
	ID             uuid.UUID
	OrganizationID uuid.UUID
	InterviewID    uuid.UUID
	TokenHash      string
	ExpiresAt      time.Time
	UsedAt         *time.Time
	RevokedAt      *time.Time
	CreatedAt      time.Time
}

type Consent struct {
	ID             uuid.UUID
	OrganizationID uuid.UUID
	InterviewID    uuid.UUID
	UserID         uuid.UUID
	PolicyVersion  string
	Locale         string
	Purpose        string
	AcceptedAt     time.Time
}

type SessionStatus string

const (
	SessionReady     SessionStatus = "ready"
	SessionActive    SessionStatus = "active"
	SessionCompleted SessionStatus = "completed"
	SessionExpired   SessionStatus = "expired"
)

type Session struct {
	ID             uuid.UUID
	OrganizationID uuid.UUID
	InterviewID    uuid.UUID
	UserID         uuid.UUID
	DeviceID       uuid.UUID
	Status         SessionStatus
	StartedAt      *time.Time
	CompletedAt    *time.Time
	LastSeenAt     time.Time
	Version        int
	CreatedAt      time.Time
}

type AnswerStatus string

const (
	AnswerSubmitted  AnswerStatus = "submitted"
	AnswerSuperseded AnswerStatus = "superseded"
)

type Answer struct {
	ID             uuid.UUID
	OrganizationID uuid.UUID
	InterviewID    uuid.UUID
	QuestionID     uuid.UUID
	SessionID      uuid.UUID
	UserID         uuid.UUID
	Status         AnswerStatus
	Text           *string
	CodeLanguage   *string
	CodeContent    *string
	ContentHash    string
	IdempotencyKey uuid.UUID
	SupersedesID   *uuid.UUID
	SubmittedAt    time.Time
	CreatedAt      time.Time
}

type AuditEvent struct {
	ID             uuid.UUID
	OrganizationID uuid.UUID
	ActorID        uuid.UUID
	SessionID      string
	DeviceID       uuid.UUID
	RequestID      string
	Action         string
	ResourceType   string
	ResourceID     uuid.UUID
	ParentID       uuid.UUID
	FromStatus     string
	ToStatus       string
	OccurredAt     time.Time
}
