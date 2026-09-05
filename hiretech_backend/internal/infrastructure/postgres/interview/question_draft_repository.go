package interview

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	interviewModel "github.com/masterfabric-go/masterfabric/internal/domain/interview/model"
	domainErr "github.com/masterfabric-go/masterfabric/internal/shared/errors"
)

const questionDraftColumns = `id,organization_id,interview_id,requested_by,reviewed_by,type,prompt,competency_ids,difficulty,time_limit_seconds,language,status,model_id,model_version,review_notes,created_at,reviewed_at`

type draftScanner interface{ Scan(dest ...any) error }

func scanQuestionDraft(row draftScanner) (*interviewModel.QuestionDraft, error) {
	var draft interviewModel.QuestionDraft
	var reviewedBy *uuid.UUID
	err := row.Scan(&draft.ID, &draft.OrganizationID, &draft.InterviewID, &draft.RequestedBy, &reviewedBy, &draft.Type, &draft.Prompt, &draft.CompetencyIDs, &draft.Difficulty, &draft.TimeLimitSeconds, &draft.Language, &draft.Status, &draft.ModelID, &draft.ModelVersion, &draft.ReviewNotes, &draft.CreatedAt, &draft.ReviewedAt)
	if reviewedBy != nil {
		draft.ReviewedBy = reviewedBy
	}
	return &draft, err
}

func (r *Repository) CreateDraft(ctx context.Context, draft *interviewModel.QuestionDraft, audit interviewModel.AuditEvent) (err error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer rollback(ctx, tx, &err)
	_, err = tx.Exec(ctx, `INSERT INTO interview_question_drafts (`+questionDraftColumns+`) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17)`, draft.ID, draft.OrganizationID, draft.InterviewID, draft.RequestedBy, nullableUUIDPtr(draft.ReviewedBy), draft.Type, draft.Prompt, draft.CompetencyIDs, draft.Difficulty, draft.TimeLimitSeconds, draft.Language, draft.Status, draft.ModelID, draft.ModelVersion, draft.ReviewNotes, draft.CreatedAt, draft.ReviewedAt)
	if err != nil {
		return mapWriteError("create question draft", err)
	}
	if err = InsertAudit(ctx, tx, audit); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (r *Repository) GetDraft(ctx context.Context, organizationID, draftID uuid.UUID) (*interviewModel.QuestionDraft, error) {
	draft, err := scanQuestionDraft(r.db.QueryRow(ctx, `SELECT `+questionDraftColumns+` FROM interview_question_drafts WHERE organization_id=$1 AND id=$2`, organizationID, draftID))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domainErr.New(domainErr.ErrNotFound, "question draft not found", nil)
	}
	if err != nil {
		return nil, domainErr.New(domainErr.ErrInternal, "failed to get question draft", err)
	}
	return draft, nil
}

func (r *Repository) ApproveDraft(ctx context.Context, organizationID, draftID, reviewerID uuid.UUID, now time.Time, audit interviewModel.AuditEvent) (result *interviewModel.QuestionDraft, question *interviewModel.Question, err error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, nil, err
	}
	defer rollback(ctx, tx, &err)
	draft, err := scanQuestionDraft(tx.QueryRow(ctx, `SELECT `+questionDraftColumns+` FROM interview_question_drafts WHERE organization_id=$1 AND id=$2 FOR UPDATE`, organizationID, draftID))
	var reviewedBy *uuid.UUID
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil, domainErr.New(domainErr.ErrNotFound, "question draft not found", nil)
	}
	if err != nil {
		return nil, nil, err
	}
	if draft.Status != interviewModel.DraftPending {
		return nil, nil, domainErr.New(domainErr.ErrConflict, "question draft has already been decided", nil)
	}
	var interviewStatus interviewModel.Status
	var interviewVersion int
	err = tx.QueryRow(ctx, `SELECT status,version FROM interviews WHERE organization_id=$1 AND id=$2 FOR UPDATE`, organizationID, draft.InterviewID).Scan(&interviewStatus, &interviewVersion)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil, domainErr.New(domainErr.ErrNotFound, "interview not found", nil)
	}
	if err != nil {
		return nil, nil, err
	}
	if interviewStatus != interviewModel.StatusDraft && interviewStatus != interviewModel.StatusReady {
		return nil, nil, domainErr.New(domainErr.ErrConflict, "interview cannot accept approved questions in its current state", nil)
	}
	var sequence int
	if err = tx.QueryRow(ctx, `SELECT COALESCE(MAX(sequence),0)+1 FROM interview_questions WHERE interview_id=$1`, draft.InterviewID).Scan(&sequence); err != nil {
		return nil, nil, err
	}
	question = &interviewModel.Question{ID: uuid.New(), OrganizationID: organizationID, InterviewID: draft.InterviewID, Sequence: sequence, Type: draft.Type, Prompt: draft.Prompt, CompetencyIDs: draft.CompetencyIDs, Difficulty: draft.Difficulty, TimeLimitSeconds: draft.TimeLimitSeconds, CreatedAt: now}
	_, err = tx.Exec(ctx, `INSERT INTO interview_questions (id,organization_id,interview_id,sequence,type,prompt,competency_ids,difficulty,time_limit_seconds,created_at) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`, question.ID, question.OrganizationID, question.InterviewID, question.Sequence, question.Type, question.Prompt, question.CompetencyIDs, question.Difficulty, question.TimeLimitSeconds, question.CreatedAt)
	if err != nil {
		return nil, nil, mapWriteError("approve question draft", err)
	}
	_, err = tx.Exec(ctx, `UPDATE interviews SET status=$3,version=version+1,updated_at=$4 WHERE organization_id=$1 AND id=$2 AND version=$5`, organizationID, draft.InterviewID, interviewModel.StatusReady, now, interviewVersion)
	if err != nil {
		return nil, nil, err
	}
	if err = tx.QueryRow(ctx, `UPDATE interview_question_drafts SET status=$3,reviewed_by=$4,reviewed_at=$5 WHERE organization_id=$1 AND id=$2 RETURNING `+questionDraftColumns, organizationID, draftID, interviewModel.DraftApproved, reviewerID, now).Scan(&draft.ID, &draft.OrganizationID, &draft.InterviewID, &draft.RequestedBy, &reviewedBy, &draft.Type, &draft.Prompt, &draft.CompetencyIDs, &draft.Difficulty, &draft.TimeLimitSeconds, &draft.Language, &draft.Status, &draft.ModelID, &draft.ModelVersion, &draft.ReviewNotes, &draft.CreatedAt, &draft.ReviewedAt); err != nil {
		return nil, nil, err
	}
	draft.ReviewedBy = reviewedBy
	if err = InsertAudit(ctx, tx, audit); err != nil {
		return nil, nil, err
	}
	if err = tx.Commit(ctx); err != nil {
		return nil, nil, err
	}
	return draft, question, nil
}

func (r *Repository) RejectDraft(ctx context.Context, organizationID, draftID, reviewerID uuid.UUID, notes string, now time.Time, audit interviewModel.AuditEvent) (result *interviewModel.QuestionDraft, err error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer rollback(ctx, tx, &err)
	result = &interviewModel.QuestionDraft{}
	var reviewedBy *uuid.UUID
	if err = tx.QueryRow(ctx, `UPDATE interview_question_drafts SET status=$3,reviewed_by=$4,review_notes=$5,reviewed_at=$6 WHERE organization_id=$1 AND id=$2 AND status=$7 RETURNING `+questionDraftColumns, organizationID, draftID, interviewModel.DraftRejected, reviewerID, notes, now, interviewModel.DraftPending).Scan(&result.ID, &result.OrganizationID, &result.InterviewID, &result.RequestedBy, &reviewedBy, &result.Type, &result.Prompt, &result.CompetencyIDs, &result.Difficulty, &result.TimeLimitSeconds, &result.Language, &result.Status, &result.ModelID, &result.ModelVersion, &result.ReviewNotes, &result.CreatedAt, &result.ReviewedAt); errors.Is(err, pgx.ErrNoRows) {
		return nil, domainErr.New(domainErr.ErrConflict, "question draft is missing or already decided", nil)
	} else if err != nil {
		return nil, err
	}
	result.ReviewedBy = reviewedBy
	if err = InsertAudit(ctx, tx, audit); err != nil {
		return nil, err
	}
	if err = tx.Commit(ctx); err != nil {
		return nil, err
	}
	return result, nil
}

func nullableUUIDPtr(value *uuid.UUID) any {
	if value == nil {
		return nil
	}
	return *value
}
