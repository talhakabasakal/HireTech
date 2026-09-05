package evaluation

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	evaluationModel "github.com/masterfabric-go/masterfabric/internal/domain/evaluation/model"
	interviewModel "github.com/masterfabric-go/masterfabric/internal/domain/interview/model"
	interviewPostgres "github.com/masterfabric-go/masterfabric/internal/infrastructure/postgres/interview"
	domainErr "github.com/masterfabric-go/masterfabric/internal/shared/errors"
)

const reportColumns = `id,organization_id,interview_id,job_id,requested_by,status,rubric_id,rubric_version,evaluator_configuration_version,overall_score,overall_confidence,strengths,gaps,evidence_references,limitations,generated_at,published_at,created_at,updated_at`

type Repository struct{ db *pgxpool.Pool }

func NewRepository(db *pgxpool.Pool) *Repository { return &Repository{db: db} }

type scanner interface{ Scan(dest ...any) error }
type queryer interface {
	Query(context.Context, string, ...any) (pgx.Rows, error)
	QueryRow(context.Context, string, ...any) pgx.Row
}

func scanReport(row scanner) (*evaluationModel.EvaluationReport, error) {
	var report evaluationModel.EvaluationReport
	var strengths, gaps, evidence, limitations []byte
	err := row.Scan(&report.ID, &report.OrganizationID, &report.InterviewID, &report.JobID, &report.RequestedBy, &report.Status, &report.RubricID, &report.RubricVersion, &report.EvaluatorConfigurationVersion, &report.OverallScore, &report.OverallConfidence, &strengths, &gaps, &evidence, &limitations, &report.GeneratedAt, &report.PublishedAt, &report.CreatedAt, &report.UpdatedAt)
	if err != nil {
		return nil, err
	}
	if err := unmarshalStrings(strengths, &report.Strengths); err != nil {
		return nil, err
	}
	if err := unmarshalStrings(gaps, &report.Gaps); err != nil {
		return nil, err
	}
	if err := unmarshalStrings(evidence, &report.EvidenceReferences); err != nil {
		return nil, err
	}
	if err := unmarshalStrings(limitations, &report.Limitations); err != nil {
		return nil, err
	}
	return &report, nil
}

func (r *Repository) Create(ctx context.Context, job *evaluationModel.EvaluationJob, report *evaluationModel.EvaluationReport, audit interviewModel.AuditEvent) (err error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer rollback(ctx, tx, &err)
	_, err = tx.Exec(ctx, `INSERT INTO evaluation_jobs (id,organization_id,interview_id,requested_by,status,rubric_version,evaluator_version,failure_code,created_at,started_at,completed_at) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)`, job.ID, job.OrganizationID, job.InterviewID, job.RequestedBy, job.Status, job.RubricVersion, job.EvaluatorVersion, job.FailureCode, job.CreatedAt, job.StartedAt, job.CompletedAt)
	if err != nil {
		return mapWriteError("create evaluation job", err)
	}
	strengths, err := json.Marshal(report.Strengths)
	if err != nil {
		return err
	}
	gaps, err := json.Marshal(report.Gaps)
	if err != nil {
		return err
	}
	evidence, err := json.Marshal(report.EvidenceReferences)
	if err != nil {
		return err
	}
	limitations, err := json.Marshal(report.Limitations)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `INSERT INTO evaluation_reports (`+reportColumns+`) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19)`, report.ID, report.OrganizationID, report.InterviewID, report.JobID, report.RequestedBy, report.Status, report.RubricID, report.RubricVersion, report.EvaluatorConfigurationVersion, report.OverallScore, report.OverallConfidence, strengths, gaps, evidence, limitations, report.GeneratedAt, report.PublishedAt, report.CreatedAt, report.UpdatedAt)
	if err != nil {
		return mapWriteError("create evaluation report", err)
	}
	for _, criterion := range report.CriterionScores {
		criterionEvidence, marshalErr := json.Marshal(criterion.EvidenceReferences)
		if marshalErr != nil {
			return marshalErr
		}
		criterionLimitations, marshalErr := json.Marshal(criterion.Limitations)
		if marshalErr != nil {
			return marshalErr
		}
		_, err = tx.Exec(ctx, `INSERT INTO evaluation_criterion_scores (id,report_id,criterion_id,applicable,score,maximum_score,weight,confidence,evidence_references,rationale,limitations) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)`, criterion.ID, report.ID, criterion.CriterionID, criterion.Applicable, criterion.Score, criterion.MaximumScore, criterion.Weight, criterion.Confidence, criterionEvidence, criterion.Rationale, criterionLimitations)
		if err != nil {
			return mapWriteError("create criterion score", err)
		}
	}
	reasons, err := json.Marshal(report.HumanReview.ReasonCodes)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `INSERT INTO evaluation_human_reviews (id,report_id,required,urgency,reason_codes,status,reviewer_user_id,notes,completed_at) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)`, report.HumanReview.ID, report.ID, report.HumanReview.Required, report.HumanReview.Urgency, reasons, report.HumanReview.Status, report.HumanReview.ReviewerUserID, report.HumanReview.Notes, report.HumanReview.CompletedAt)
	if err != nil {
		return mapWriteError("create human review", err)
	}
	if err = interviewPostgres.InsertAudit(ctx, tx, audit); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (r *Repository) GetReport(ctx context.Context, organizationID, interviewID uuid.UUID) (*evaluationModel.EvaluationReport, error) {
	return r.getReport(ctx, r.db, `WHERE organization_id=$1 AND interview_id=$2`, organizationID, interviewID)
}
func (r *Repository) GetReportByID(ctx context.Context, organizationID, reportID uuid.UUID) (*evaluationModel.EvaluationReport, error) {
	return r.getReport(ctx, r.db, `WHERE organization_id=$1 AND id=$2`, organizationID, reportID)
}
func (r *Repository) getReport(ctx context.Context, source queryer, predicate string, args ...any) (*evaluationModel.EvaluationReport, error) {
	report, err := scanReport(source.QueryRow(ctx, `SELECT `+reportColumns+` FROM evaluation_reports `+predicate, args...))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domainErr.New(domainErr.ErrNotFound, "evaluation report not found", nil)
	}
	if err != nil {
		return nil, domainErr.New(domainErr.ErrInternal, "failed to get evaluation report", err)
	}
	if err := loadChildren(ctx, source, report); err != nil {
		return nil, err
	}
	return report, nil
}

func loadChildren(ctx context.Context, source queryer, report *evaluationModel.EvaluationReport) error {
	rows, err := source.Query(ctx, `SELECT id,report_id,criterion_id,applicable,score,maximum_score,weight,confidence,evidence_references,rationale,limitations FROM evaluation_criterion_scores WHERE report_id=$1 ORDER BY criterion_id`, report.ID)
	if err != nil {
		return domainErr.New(domainErr.ErrInternal, "failed to get criterion scores", err)
	}
	defer rows.Close()
	report.CriterionScores = make([]*evaluationModel.CriterionScore, 0)
	for rows.Next() {
		var criterion evaluationModel.CriterionScore
		var evidence, limitations []byte
		if err := rows.Scan(&criterion.ID, &criterion.ReportID, &criterion.CriterionID, &criterion.Applicable, &criterion.Score, &criterion.MaximumScore, &criterion.Weight, &criterion.Confidence, &evidence, &criterion.Rationale, &limitations); err != nil {
			return domainErr.New(domainErr.ErrInternal, "failed to scan criterion score", err)
		}
		if err := unmarshalStrings(evidence, &criterion.EvidenceReferences); err != nil {
			return domainErr.New(domainErr.ErrInternal, "failed to decode evidence references", err)
		}
		if err := unmarshalStrings(limitations, &criterion.Limitations); err != nil {
			return domainErr.New(domainErr.ErrInternal, "failed to decode criterion limitations", err)
		}
		report.CriterionScores = append(report.CriterionScores, &criterion)
	}
	if err := rows.Err(); err != nil {
		return domainErr.New(domainErr.ErrInternal, "failed to read criterion scores", err)
	}
	var review evaluationModel.HumanReview
	var reasons []byte
	err = source.QueryRow(ctx, `SELECT id,report_id,required,urgency,reason_codes,status,reviewer_user_id,notes,completed_at FROM evaluation_human_reviews WHERE report_id=$1`, report.ID).Scan(&review.ID, &review.ReportID, &review.Required, &review.Urgency, &reasons, &review.Status, &review.ReviewerUserID, &review.Notes, &review.CompletedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return domainErr.New(domainErr.ErrInternal, "failed to get human review", err)
	}
	if err := unmarshalStrings(reasons, &review.ReasonCodes); err != nil {
		return domainErr.New(domainErr.ErrInternal, "failed to decode review reasons", err)
	}
	report.HumanReview = &review
	return nil
}

func (r *Repository) RecordHumanReview(ctx context.Context, organizationID, reportID, reviewerID uuid.UUID, approved bool, notes string, now time.Time, audit interviewModel.AuditEvent) (result *evaluationModel.EvaluationReport, err error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer rollback(ctx, tx, &err)
	var interviewID uuid.UUID
	var currentStatus evaluationModel.ReportStatus
	err = tx.QueryRow(ctx, `SELECT interview_id,status FROM evaluation_reports WHERE organization_id=$1 AND id=$2 FOR UPDATE`, organizationID, reportID).Scan(&interviewID, &currentStatus)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domainErr.New(domainErr.ErrNotFound, "evaluation report not found", nil)
	}
	if err != nil {
		return nil, err
	}
	if currentStatus == evaluationModel.ReportPublished || currentStatus == evaluationModel.ReportRejected {
		return nil, domainErr.New(domainErr.ErrConflict, "evaluation report has already been decided", nil)
	}
	reviewStatus, reportStatus := evaluationModel.ReviewRejected, evaluationModel.ReportRejected
	var publishedAt *time.Time
	if approved {
		reviewStatus, reportStatus, publishedAt = evaluationModel.ReviewApproved, evaluationModel.ReportPublished, &now
	}
	tag, err := tx.Exec(ctx, `UPDATE evaluation_reports SET status=$3,published_at=$4::timestamptz,updated_at=$5 WHERE organization_id=$1 AND id=$2 AND status IN ('review_required','ready_for_human_decision','draft')`, organizationID, reportID, reportStatus, publishedAt, now)
	if err != nil {
		return nil, err
	}
	if tag.RowsAffected() != 1 {
		return nil, domainErr.New(domainErr.ErrConflict, "evaluation report changed concurrently", nil)
	}
	tag, err = tx.Exec(ctx, `UPDATE evaluation_human_reviews SET status=$2,reviewer_user_id=$3,notes=$4,completed_at=$5 WHERE report_id=$1 AND status='pending'`, reportID, reviewStatus, reviewerID, notes, now)
	if err != nil {
		return nil, err
	}
	if tag.RowsAffected() != 1 {
		return nil, domainErr.New(domainErr.ErrConflict, "human review is no longer pending", nil)
	}
	audit.OrganizationID, audit.ResourceID, audit.ParentID = organizationID, reportID, interviewID
	if err = interviewPostgres.InsertAudit(ctx, tx, audit); err != nil {
		return nil, err
	}
	if err = tx.Commit(ctx); err != nil {
		return nil, err
	}
	return r.GetReport(ctx, organizationID, interviewID)
}

func unmarshalStrings(raw []byte, target *[]string) error {
	if len(raw) == 0 {
		*target = []string{}
		return nil
	}
	if err := json.Unmarshal(raw, target); err != nil {
		return err
	}
	if *target == nil {
		*target = []string{}
	}
	return nil
}
func rollback(ctx context.Context, tx pgx.Tx, err *error) {
	if *err != nil {
		_ = tx.Rollback(ctx)
	}
}
func mapWriteError(action string, err error) error {
	if strings.Contains(err.Error(), "duplicate key") {
		return domainErr.New(domainErr.ErrConflict, action+" conflicts with existing state", nil)
	}
	return domainErr.New(domainErr.ErrInternal, "failed to "+action, err)
}
