package repository

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/masterfabric-go/masterfabric/internal/domain/evaluation/model"
	interviewModel "github.com/masterfabric-go/masterfabric/internal/domain/interview/model"
)

type EvaluationRepository interface {
	Create(ctx context.Context, job *model.EvaluationJob, report *model.EvaluationReport, audit interviewModel.AuditEvent) error
	GetReport(ctx context.Context, organizationID, interviewID uuid.UUID) (*model.EvaluationReport, error)
	GetReportByID(ctx context.Context, organizationID, reportID uuid.UUID) (*model.EvaluationReport, error)
	RecordHumanReview(ctx context.Context, organizationID, reportID, reviewerID uuid.UUID, approved bool, notes string, now time.Time, audit interviewModel.AuditEvent) (*model.EvaluationReport, error)
}
