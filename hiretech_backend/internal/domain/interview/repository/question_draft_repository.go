package repository

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/masterfabric-go/masterfabric/internal/domain/interview/model"
)

type QuestionDraftRepository interface {
	CreateDraft(ctx context.Context, draft *model.QuestionDraft, audit model.AuditEvent) error
	GetDraft(ctx context.Context, organizationID, draftID uuid.UUID) (*model.QuestionDraft, error)
	ApproveDraft(ctx context.Context, organizationID, draftID, reviewerID uuid.UUID, now time.Time, audit model.AuditEvent) (*model.QuestionDraft, *model.Question, error)
	RejectDraft(ctx context.Context, organizationID, draftID, reviewerID uuid.UUID, notes string, now time.Time, audit model.AuditEvent) (*model.QuestionDraft, error)
}
