package repository

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/masterfabric-go/masterfabric/internal/domain/interview/model"
	"github.com/masterfabric-go/masterfabric/internal/shared/pagination"
)

type InterviewRepository interface {
	Create(ctx context.Context, interview *model.Interview, audit model.AuditEvent) error
	Get(ctx context.Context, organizationID, interviewID uuid.UUID) (*model.Interview, error)
	List(ctx context.Context, organizationID uuid.UUID, statuses []model.Status, limit int) ([]*model.Interview, error)
	ListPage(ctx context.Context, organizationID uuid.UUID, statuses []model.Status, after *pagination.Cursor, limit int) ([]*model.Interview, error)
	AddQuestion(ctx context.Context, question *model.Question, expectedVersion int, audit model.AuditEvent) error
	ListQuestions(ctx context.Context, organizationID, interviewID uuid.UUID) ([]*model.Question, error)
	GetQuestion(ctx context.Context, organizationID, questionID uuid.UUID) (*model.Question, error)
	Publish(ctx context.Context, organizationID, interviewID uuid.UUID, expectedVersion int, now time.Time, audit model.AuditEvent) (*model.Interview, error)
	CreateInvitation(ctx context.Context, invitation *model.Invitation, expectedVersion int, audit model.AuditEvent) error
	RedeemInvitation(ctx context.Context, tokenHash string, userID, deviceID uuid.UUID, email string, consent *model.Consent, now time.Time, audit model.AuditEvent) (*model.Interview, error)
	Start(ctx context.Context, session *model.Session, expectedVersion int, now time.Time, audit model.AuditEvent) (*model.Interview, error)
	SubmitAnswer(ctx context.Context, answer *model.Answer, audit model.AuditEvent) (*model.Answer, error)
	GetAnswer(ctx context.Context, organizationID, answerID uuid.UUID) (*model.Answer, error)
	ListAnswers(ctx context.Context, organizationID, interviewID uuid.UUID) ([]*model.Answer, error)
	Complete(ctx context.Context, organizationID, interviewID, userID uuid.UUID, expectedVersion int, now time.Time, audit model.AuditEvent) (*model.Interview, error)
	Cancel(ctx context.Context, organizationID, interviewID uuid.UUID, expectedVersion int, now time.Time, audit model.AuditEvent) (*model.Interview, error)
}
