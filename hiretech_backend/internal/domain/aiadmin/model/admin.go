package model

import (
	"time"

	"github.com/google/uuid"
)

type Role string

const (
	RoleInterviewer Role = "interviewer"
	RoleEvaluator   Role = "evaluator"
)

type ModelStatus string

const (
	ModelActive   ModelStatus = "active"
	ModelFallback ModelStatus = "fallback"
	ModelDisabled ModelStatus = "disabled"
)

type LatencyClass string

const (
	LatencyFast     LatencyClass = "fast"
	LatencyBalanced LatencyClass = "balanced"
	LatencyDeep     LatencyClass = "deep"
)

type PromptStatus string

const (
	PromptDraft    PromptStatus = "draft"
	PromptActive   PromptStatus = "active"
	PromptArchived PromptStatus = "archived"
)

type Model struct {
	ID            uuid.UUID
	ModelID       string
	DisplayName   string
	ProviderLabel string
	Roles         []Role
	Status        ModelStatus
	LatencyClass  LatencyClass
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

type Prompt struct {
	ID        uuid.UUID
	Role      Role
	Name      string
	Version   int
	Prompt    string
	Status    PromptStatus
	UpdatedAt time.Time
	UpdatedBy uuid.UUID
}

type RubricCriterion struct {
	Name   string  `json:"name"`
	Weight float64 `json:"weight"`
}

type Rubric struct {
	ID        uuid.UUID
	Name      string
	Version   int
	Criteria  []RubricCriterion
	Status    string
	UpdatedAt time.Time
	UpdatedBy uuid.UUID
}

type RoutingRule struct {
	ID              uuid.UUID
	Role            Role
	PrimaryModelID  string
	FallbackModelID string
	TimeoutSeconds  int
	Enabled         bool
	UpdatedAt       time.Time
	UpdatedBy       uuid.UUID
}

type ConfigurationVersion struct {
	ID         uuid.UUID
	Resource   string
	Version    int
	Action     string
	Status     string
	Actor      uuid.UUID
	ApprovedBy *uuid.UUID
	CreatedAt  time.Time
}

const (
	ConfigurationDraft    = "draft"
	ConfigurationActive   = "active"
	ConfigurationArchived = "archived"
)

type AuditEvent struct {
	ID         uuid.UUID
	Action     string
	Actor      uuid.UUID
	Target     string
	Result     string
	OccurredAt time.Time
}

type Workspace struct {
	Models      []*Model
	Prompts     []*Prompt
	Rubric      *Rubric
	Routing     []*RoutingRule
	Versions    []*ConfigurationVersion
	AuditEvents []*AuditEvent
}

type RegisterModelInput struct {
	ModelID       string
	DisplayName   string
	ProviderLabel string
	Roles         []Role
	Status        ModelStatus
	LatencyClass  LatencyClass
}

type CreatePromptInput struct {
	Role   Role
	Name   string
	Prompt string
}

type UpdateRoutingInput struct {
	Role            Role
	PrimaryModelID  string
	FallbackModelID string
	TimeoutSeconds  int
	Enabled         bool
}

type PublishRubricInput struct {
	Name     string
	Criteria []RubricCriterion
}
