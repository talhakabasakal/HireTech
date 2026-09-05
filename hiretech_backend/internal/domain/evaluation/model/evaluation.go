package model

import (
	"time"

	"github.com/google/uuid"
)

type JobStatus string

const (
	JobQueued    JobStatus = "queued"
	JobRunning   JobStatus = "running"
	JobCompleted JobStatus = "completed"
	JobFailed    JobStatus = "failed"
)

type ReportStatus string

const (
	ReportDraft                 ReportStatus = "draft"
	ReportReviewRequired        ReportStatus = "review_required"
	ReportReadyForHumanDecision ReportStatus = "ready_for_human_decision"
	ReportPublished             ReportStatus = "published"
	ReportRejected              ReportStatus = "rejected"
)

type ReviewUrgency string

const (
	ReviewNone      ReviewUrgency = "none"
	ReviewNormal    ReviewUrgency = "normal"
	ReviewHigh      ReviewUrgency = "high"
	ReviewImmediate ReviewUrgency = "immediate"
)

type ReviewStatus string

const (
	ReviewPending  ReviewStatus = "pending"
	ReviewApproved ReviewStatus = "approved"
	ReviewRejected ReviewStatus = "rejected"
)

type EvaluationJob struct {
	ID               uuid.UUID
	OrganizationID   uuid.UUID
	InterviewID      uuid.UUID
	RequestedBy      uuid.UUID
	Status           JobStatus
	RubricVersion    string
	EvaluatorVersion string
	FailureCode      string
	CreatedAt        time.Time
	StartedAt        *time.Time
	CompletedAt      *time.Time
}

type CriterionScore struct {
	ID                 uuid.UUID
	ReportID           uuid.UUID
	CriterionID        string
	Applicable         bool
	Score              *float64
	MaximumScore       float64
	Weight             float64
	Confidence         float64
	EvidenceReferences []string
	Rationale          string
	Limitations        []string
}

type HumanReview struct {
	ID             uuid.UUID
	ReportID       uuid.UUID
	Required       bool
	Urgency        ReviewUrgency
	ReasonCodes    []string
	Status         ReviewStatus
	ReviewerUserID *uuid.UUID
	Notes          string
	CompletedAt    *time.Time
}

type EvaluationReport struct {
	ID                            uuid.UUID
	OrganizationID                uuid.UUID
	InterviewID                   uuid.UUID
	JobID                         uuid.UUID
	RequestedBy                   uuid.UUID
	Status                        ReportStatus
	RubricID                      string
	RubricVersion                 string
	EvaluatorConfigurationVersion string
	OverallScore                  *float64
	OverallConfidence             float64
	CriterionScores               []*CriterionScore
	Strengths                     []string
	Gaps                          []string
	EvidenceReferences            []string
	Limitations                   []string
	HumanReview                   *HumanReview
	GeneratedAt                   *time.Time
	PublishedAt                   *time.Time
	CreatedAt                     time.Time
	UpdatedAt                     time.Time
}

func (r *EvaluationReport) CanPublish() bool {
	return r != nil && (r.Status == ReportReadyForHumanDecision || r.Status == ReportReviewRequired) &&
		r.HumanReview != nil && r.HumanReview.Status == ReviewApproved
}
