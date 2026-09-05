package interview

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	interviewModel "github.com/masterfabric-go/masterfabric/internal/domain/interview/model"
	domainErr "github.com/masterfabric-go/masterfabric/internal/shared/errors"
)

const interviewColumns = `id,organization_id,created_by,candidate_user_id,candidate_email,candidate_display_name,title,position_title,seniority,technology_tags,mode,language,question_source,rubric_version,status,starts_at,expires_at,started_at,completed_at,cancelled_at,version,created_at,updated_at`

type Repository struct{ db *pgxpool.Pool }

func NewRepository(db *pgxpool.Pool) *Repository { return &Repository{db: db} }

type scanner interface{ Scan(dest ...any) error }

func scanInterview(row scanner) (*interviewModel.Interview, error) {
	var value interviewModel.Interview
	var candidateID *uuid.UUID
	err := row.Scan(&value.ID, &value.OrganizationID, &value.CreatedBy, &candidateID, &value.CandidateEmail, &value.CandidateDisplayName, &value.Title, &value.PositionTitle, &value.Seniority, &value.TechnologyTags, &value.Mode, &value.Language, &value.QuestionSource, &value.RubricVersion, &value.Status, &value.StartsAt, &value.ExpiresAt, &value.StartedAt, &value.CompletedAt, &value.CancelledAt, &value.Version, &value.CreatedAt, &value.UpdatedAt)
	if candidateID != nil {
		value.CandidateUserID = *candidateID
	}
	return &value, err
}

func (r *Repository) Create(ctx context.Context, value *interviewModel.Interview, audit interviewModel.AuditEvent) (err error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer rollback(ctx, tx, &err)
	_, err = tx.Exec(ctx, `INSERT INTO interviews (`+interviewColumns+`) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,$21,$22,$23)`, value.ID, value.OrganizationID, value.CreatedBy, nullableUUID(value.CandidateUserID), value.CandidateEmail, value.CandidateDisplayName, value.Title, value.PositionTitle, value.Seniority, value.TechnologyTags, value.Mode, value.Language, value.QuestionSource, value.RubricVersion, value.Status, value.StartsAt, value.ExpiresAt, value.StartedAt, value.CompletedAt, value.CancelledAt, value.Version, value.CreatedAt, value.UpdatedAt)
	if err != nil {
		return mapWriteError("create interview", err)
	}
	if err = InsertAudit(ctx, tx, audit); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (r *Repository) Get(ctx context.Context, organizationID, interviewID uuid.UUID) (*interviewModel.Interview, error) {
	value, err := scanInterview(r.db.QueryRow(ctx, `SELECT `+interviewColumns+` FROM interviews WHERE organization_id=$1 AND id=$2`, organizationID, interviewID))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domainErr.New(domainErr.ErrNotFound, "interview not found", nil)
	}
	if err != nil {
		return nil, domainErr.New(domainErr.ErrInternal, "failed to get interview", err)
	}
	return value, nil
}

func (r *Repository) List(ctx context.Context, organizationID uuid.UUID, statuses []interviewModel.Status, limit int) ([]*interviewModel.Interview, error) {
	var rows pgx.Rows
	var err error
	if len(statuses) == 0 {
		rows, err = r.db.Query(ctx, `SELECT `+interviewColumns+` FROM interviews WHERE organization_id=$1 ORDER BY created_at DESC LIMIT $2`, organizationID, limit)
	} else {
		statusValues := make([]string, len(statuses))
		for index, status := range statuses {
			statusValues[index] = string(status)
		}
		rows, err = r.db.Query(ctx, `SELECT `+interviewColumns+` FROM interviews WHERE organization_id=$1 AND status=ANY($2) ORDER BY created_at DESC LIMIT $3`, organizationID, statusValues, limit)
	}
	if err != nil {
		return nil, domainErr.New(domainErr.ErrInternal, "failed to list interviews", err)
	}
	defer rows.Close()
	result := make([]*interviewModel.Interview, 0)
	for rows.Next() {
		value, scanErr := scanInterview(rows)
		if scanErr != nil {
			return nil, domainErr.New(domainErr.ErrInternal, "failed to scan interview", scanErr)
		}
		result = append(result, value)
	}
	return result, rows.Err()
}

func (r *Repository) AddQuestion(ctx context.Context, question *interviewModel.Question, expectedVersion int, audit interviewModel.AuditEvent) (err error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer rollback(ctx, tx, &err)
	var status interviewModel.Status
	var currentVersion int
	err = tx.QueryRow(ctx, `SELECT status,version FROM interviews WHERE organization_id=$1 AND id=$2 FOR UPDATE`, question.OrganizationID, question.InterviewID).Scan(&status, &currentVersion)
	if errors.Is(err, pgx.ErrNoRows) {
		return domainErr.New(domainErr.ErrNotFound, "interview not found", nil)
	}
	if err != nil {
		return err
	}
	if currentVersion != expectedVersion || (status != interviewModel.StatusDraft && status != interviewModel.StatusReady) {
		return domainErr.New(domainErr.ErrConflict, "interview changed concurrently", nil)
	}
	err = tx.QueryRow(ctx, `SELECT COALESCE(MAX(sequence),0)+1 FROM interview_questions WHERE interview_id=$1`, question.InterviewID).Scan(&question.Sequence)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `INSERT INTO interview_questions (id,organization_id,interview_id,sequence,type,prompt,competency_ids,difficulty,time_limit_seconds,created_at) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`, question.ID, question.OrganizationID, question.InterviewID, question.Sequence, question.Type, question.Prompt, question.CompetencyIDs, question.Difficulty, question.TimeLimitSeconds, question.CreatedAt)
	if err != nil {
		return mapWriteError("add question", err)
	}
	_, err = tx.Exec(ctx, `UPDATE interviews SET status='ready',version=version+1,updated_at=$3 WHERE organization_id=$1 AND id=$2`, question.OrganizationID, question.InterviewID, question.CreatedAt)
	if err != nil {
		return err
	}
	if err = InsertAudit(ctx, tx, audit); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func scanQuestion(row scanner) (*interviewModel.Question, error) {
	var q interviewModel.Question
	err := row.Scan(&q.ID, &q.OrganizationID, &q.InterviewID, &q.Sequence, &q.Type, &q.Prompt, &q.CompetencyIDs, &q.Difficulty, &q.TimeLimitSeconds, &q.CreatedAt)
	return &q, err
}

const questionColumns = `id,organization_id,interview_id,sequence,type,prompt,competency_ids,difficulty,time_limit_seconds,created_at`

func (r *Repository) ListQuestions(ctx context.Context, orgID, interviewID uuid.UUID) ([]*interviewModel.Question, error) {
	rows, err := r.db.Query(ctx, `SELECT `+questionColumns+` FROM interview_questions WHERE organization_id=$1 AND interview_id=$2 ORDER BY sequence`, orgID, interviewID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]*interviewModel.Question, 0)
	for rows.Next() {
		q, e := scanQuestion(rows)
		if e != nil {
			return nil, e
		}
		out = append(out, q)
	}
	return out, rows.Err()
}
func (r *Repository) GetQuestion(ctx context.Context, orgID, questionID uuid.UUID) (*interviewModel.Question, error) {
	q, err := scanQuestion(r.db.QueryRow(ctx, `SELECT `+questionColumns+` FROM interview_questions WHERE organization_id=$1 AND id=$2`, orgID, questionID))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domainErr.New(domainErr.ErrNotFound, "question not found", nil)
	}
	return q, err
}

func (r *Repository) Publish(ctx context.Context, orgID, interviewID uuid.UUID, version int, now time.Time, audit interviewModel.AuditEvent) (*interviewModel.Interview, error) {
	return r.transition(ctx, orgID, interviewID, version, []interviewModel.Status{interviewModel.StatusReady, interviewModel.StatusInvited}, interviewModel.StatusInvited, "published", now, audit)
}

func (r *Repository) CreateInvitation(ctx context.Context, inv *interviewModel.Invitation, version int, audit interviewModel.AuditEvent) (err error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer rollback(ctx, tx, &err)
	tag, err := tx.Exec(ctx, `UPDATE interviews SET status='invited',version=version+1,updated_at=$4 WHERE organization_id=$1 AND id=$2 AND version=$3 AND status IN ('ready','invited')`, inv.OrganizationID, inv.InterviewID, version, inv.CreatedAt)
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		return domainErr.New(domainErr.ErrConflict, "interview changed concurrently", nil)
	}
	_, err = tx.Exec(ctx, `UPDATE interview_invitations SET revoked_at=$3 WHERE organization_id=$1 AND interview_id=$2 AND used_at IS NULL AND revoked_at IS NULL`, inv.OrganizationID, inv.InterviewID, inv.CreatedAt)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `INSERT INTO interview_invitations (id,organization_id,interview_id,token_hash,expires_at,used_at,revoked_at,created_at) VALUES ($1,$2,$3,$4,$5,$6,$7,$8)`, inv.ID, inv.OrganizationID, inv.InterviewID, inv.TokenHash, inv.ExpiresAt, inv.UsedAt, inv.RevokedAt, inv.CreatedAt)
	if err != nil {
		return mapWriteError("create invitation", err)
	}
	if err = InsertAudit(ctx, tx, audit); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (r *Repository) RedeemInvitation(ctx context.Context, tokenHash string, userID, deviceID uuid.UUID, email string, consent *interviewModel.Consent, now time.Time, audit interviewModel.AuditEvent) (result *interviewModel.Interview, err error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer rollback(ctx, tx, &err)
	var invID, orgID, interviewID uuid.UUID
	err = tx.QueryRow(ctx, `SELECT i.id,i.organization_id,i.interview_id FROM interview_invitations i JOIN interviews v ON v.id=i.interview_id AND v.organization_id=i.organization_id WHERE i.token_hash=$1 AND i.used_at IS NULL AND i.revoked_at IS NULL AND i.expires_at>$2 AND lower(v.candidate_email)=lower($3) AND v.status='invited' FOR UPDATE`, tokenHash, now, email).Scan(&invID, &orgID, &interviewID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domainErr.New(domainErr.ErrUnauthorized, "invalid or expired invitation", nil)
	}
	if err != nil {
		return nil, err
	}
	_, err = tx.Exec(ctx, `UPDATE interview_invitations SET used_at=$2 WHERE id=$1`, invID, now)
	if err != nil {
		return nil, err
	}
	tag, err := tx.Exec(ctx, `UPDATE interviews SET candidate_user_id=$3,updated_at=$4,version=version+1 WHERE organization_id=$1 AND id=$2 AND (candidate_user_id IS NULL OR candidate_user_id=$3)`, orgID, interviewID, userID, now)
	if err != nil {
		return nil, err
	}
	if tag.RowsAffected() != 1 {
		return nil, domainErr.New(domainErr.ErrForbidden, "invitation belongs to another candidate", nil)
	}
	consent.OrganizationID = orgID
	consent.InterviewID = interviewID
	_, err = tx.Exec(ctx, `INSERT INTO interview_consents (id,organization_id,interview_id,user_id,policy_version,locale,purpose,accepted_at) VALUES ($1,$2,$3,$4,$5,$6,$7,$8) ON CONFLICT DO NOTHING`, consent.ID, orgID, interviewID, userID, consent.PolicyVersion, consent.Locale, consent.Purpose, consent.AcceptedAt)
	if err != nil {
		return nil, err
	}
	audit.OrganizationID = orgID
	audit.ResourceID = invID
	audit.ParentID = interviewID
	if err = InsertAudit(ctx, tx, audit); err != nil {
		return nil, err
	}
	result, err = scanInterview(tx.QueryRow(ctx, `SELECT `+interviewColumns+` FROM interviews WHERE organization_id=$1 AND id=$2`, orgID, interviewID))
	if err != nil {
		return nil, err
	}
	err = tx.Commit(ctx)
	return result, err
}

func (r *Repository) Start(ctx context.Context, session *interviewModel.Session, version int, now time.Time, audit interviewModel.AuditEvent) (result *interviewModel.Interview, err error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer rollback(ctx, tx, &err)
	tag, err := tx.Exec(ctx, `UPDATE interviews SET status='in_progress',started_at=$5,updated_at=$5,version=version+1 WHERE organization_id=$1 AND id=$2 AND candidate_user_id=$3 AND version=$4 AND status='invited' AND expires_at>$5`, session.OrganizationID, session.InterviewID, session.UserID, version, now)
	if err != nil {
		return nil, err
	}
	if tag.RowsAffected() != 1 {
		return nil, domainErr.New(domainErr.ErrConflict, "interview cannot be started", nil)
	}
	session.StartedAt = &now
	_, err = tx.Exec(ctx, `INSERT INTO interview_sessions (id,organization_id,interview_id,user_id,device_id,status,started_at,completed_at,last_seen_at,version,created_at) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)`, session.ID, session.OrganizationID, session.InterviewID, session.UserID, session.DeviceID, session.Status, session.StartedAt, session.CompletedAt, session.LastSeenAt, session.Version, session.CreatedAt)
	if err != nil {
		return nil, mapWriteError("start interview", err)
	}
	if err = InsertAudit(ctx, tx, audit); err != nil {
		return nil, err
	}
	result, err = scanInterview(tx.QueryRow(ctx, `SELECT `+interviewColumns+` FROM interviews WHERE organization_id=$1 AND id=$2`, session.OrganizationID, session.InterviewID))
	if err != nil {
		return nil, err
	}
	err = tx.Commit(ctx)
	return result, err
}

const answerColumns = `id,organization_id,interview_id,question_id,session_id,user_id,status,answer_text,code_language,code_content,content_hash,idempotency_key,supersedes_id,submitted_at,created_at`

func scanAnswer(row scanner) (*interviewModel.Answer, error) {
	var a interviewModel.Answer
	err := row.Scan(&a.ID, &a.OrganizationID, &a.InterviewID, &a.QuestionID, &a.SessionID, &a.UserID, &a.Status, &a.Text, &a.CodeLanguage, &a.CodeContent, &a.ContentHash, &a.IdempotencyKey, &a.SupersedesID, &a.SubmittedAt, &a.CreatedAt)
	return &a, err
}
func (r *Repository) SubmitAnswer(ctx context.Context, a *interviewModel.Answer, audit interviewModel.AuditEvent) (result *interviewModel.Answer, err error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer rollback(ctx, tx, &err)
	lockKey := a.OrganizationID.String() + ":" + a.UserID.String() + ":" + a.IdempotencyKey.String()
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, lockKey); err != nil {
		return nil, err
	}
	existing, findErr := scanAnswer(tx.QueryRow(ctx, `SELECT `+answerColumns+` FROM interview_answers WHERE organization_id=$1 AND user_id=$2 AND idempotency_key=$3`, a.OrganizationID, a.UserID, a.IdempotencyKey))
	if findErr == nil {
		if existing.InterviewID != a.InterviewID || existing.QuestionID != a.QuestionID || existing.ContentHash != a.ContentHash || !sameOptionalUUID(existing.SupersedesID, a.SupersedesID) {
			return nil, domainErr.New(domainErr.ErrConflict, "idempotency key was already used for another answer", nil)
		}
		_ = tx.Rollback(ctx)
		return existing, nil
	}
	if !errors.Is(findErr, pgx.ErrNoRows) {
		return nil, findErr
	}
	var status interviewModel.Status
	var candidate uuid.UUID
	err = tx.QueryRow(ctx, `SELECT status,candidate_user_id FROM interviews WHERE organization_id=$1 AND id=$2 FOR UPDATE`, a.OrganizationID, a.InterviewID).Scan(&status, &candidate)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domainErr.New(domainErr.ErrNotFound, "interview not found", nil)
	}
	if err != nil {
		return nil, err
	}
	if status != interviewModel.StatusInProgress || candidate != a.UserID {
		return nil, domainErr.New(domainErr.ErrForbidden, "answer submission denied", nil)
	}
	var questionInterview uuid.UUID
	err = tx.QueryRow(ctx, `SELECT interview_id FROM interview_questions WHERE organization_id=$1 AND id=$2`, a.OrganizationID, a.QuestionID).Scan(&questionInterview)
	if err != nil || questionInterview != a.InterviewID {
		return nil, domainErr.New(domainErr.ErrNotFound, "question not found", nil)
	}
	err = tx.QueryRow(ctx, `SELECT id FROM interview_sessions WHERE organization_id=$1 AND interview_id=$2 AND user_id=$3 AND status='active' FOR UPDATE`, a.OrganizationID, a.InterviewID, a.UserID).Scan(&a.SessionID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domainErr.New(domainErr.ErrConflict, "interview session is not active", nil)
	}
	if err != nil {
		return nil, err
	}
	if a.SupersedesID != nil {
		tag, e := tx.Exec(ctx, `UPDATE interview_answers SET status='superseded' WHERE id=$1 AND organization_id=$2 AND interview_id=$3 AND question_id=$4 AND user_id=$5 AND status='submitted'`, *a.SupersedesID, a.OrganizationID, a.InterviewID, a.QuestionID, a.UserID)
		if e != nil {
			return nil, e
		}
		if tag.RowsAffected() != 1 {
			return nil, domainErr.New(domainErr.ErrConflict, "answer version cannot be superseded", nil)
		}
	}
	_, err = tx.Exec(ctx, `INSERT INTO interview_answers (`+answerColumns+`) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15)`, a.ID, a.OrganizationID, a.InterviewID, a.QuestionID, a.SessionID, a.UserID, a.Status, a.Text, a.CodeLanguage, a.CodeContent, a.ContentHash, a.IdempotencyKey, a.SupersedesID, a.SubmittedAt, a.CreatedAt)
	if err != nil {
		return nil, mapWriteError("submit answer", err)
	}
	if err = InsertAudit(ctx, tx, audit); err != nil {
		return nil, err
	}
	err = tx.Commit(ctx)
	return a, err
}
func (r *Repository) GetAnswer(ctx context.Context, orgID, answerID uuid.UUID) (*interviewModel.Answer, error) {
	a, err := scanAnswer(r.db.QueryRow(ctx, `SELECT `+answerColumns+` FROM interview_answers WHERE organization_id=$1 AND id=$2`, orgID, answerID))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domainErr.New(domainErr.ErrNotFound, "answer not found", nil)
	}
	return a, err
}
func (r *Repository) ListAnswers(ctx context.Context, orgID, interviewID uuid.UUID) ([]*interviewModel.Answer, error) {
	rows, err := r.db.Query(ctx, `SELECT `+answerColumns+` FROM interview_answers WHERE organization_id=$1 AND interview_id=$2 ORDER BY created_at`, orgID, interviewID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]*interviewModel.Answer, 0)
	for rows.Next() {
		a, e := scanAnswer(rows)
		if e != nil {
			return nil, e
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

func (r *Repository) Complete(ctx context.Context, orgID, interviewID, userID uuid.UUID, version int, now time.Time, audit interviewModel.AuditEvent) (result *interviewModel.Interview, err error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer rollback(ctx, tx, &err)
	tag, err := tx.Exec(ctx, `UPDATE interviews SET status='completed',completed_at=$5,updated_at=$5,version=version+1 WHERE organization_id=$1 AND id=$2 AND candidate_user_id=$3 AND version=$4 AND status='in_progress'`, orgID, interviewID, userID, version, now)
	if err != nil {
		return nil, err
	}
	if tag.RowsAffected() != 1 {
		return nil, domainErr.New(domainErr.ErrConflict, "interview cannot be completed", nil)
	}
	tag, err = tx.Exec(ctx, `UPDATE interview_sessions SET status='completed',completed_at=$4,last_seen_at=$4,version=version+1 WHERE organization_id=$1 AND interview_id=$2 AND user_id=$3 AND status='active'`, orgID, interviewID, userID, now)
	if err != nil {
		return nil, err
	}
	if tag.RowsAffected() != 1 {
		return nil, domainErr.New(domainErr.ErrConflict, "interview session is not active", nil)
	}
	if err = InsertAudit(ctx, tx, audit); err != nil {
		return nil, err
	}
	result, err = scanInterview(tx.QueryRow(ctx, `SELECT `+interviewColumns+` FROM interviews WHERE organization_id=$1 AND id=$2`, orgID, interviewID))
	if err != nil {
		return nil, err
	}
	err = tx.Commit(ctx)
	return result, err
}
func (r *Repository) Cancel(ctx context.Context, orgID, interviewID uuid.UUID, version int, now time.Time, audit interviewModel.AuditEvent) (*interviewModel.Interview, error) {
	return r.transition(ctx, orgID, interviewID, version, []interviewModel.Status{interviewModel.StatusDraft, interviewModel.StatusReady, interviewModel.StatusInvited, interviewModel.StatusInProgress}, interviewModel.StatusCancelled, "cancelled", now, audit)
}

func (r *Repository) transition(ctx context.Context, orgID, interviewID uuid.UUID, version int, from []interviewModel.Status, to interviewModel.Status, timeColumn string, now time.Time, audit interviewModel.AuditEvent) (result *interviewModel.Interview, err error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer rollback(ctx, tx, &err)
	allowed := make([]string, len(from))
	for i, status := range from {
		allowed[i] = string(status)
	}
	query := `UPDATE interviews SET status=$5,updated_at=$6,version=version+1`
	if timeColumn == "cancelled" {
		query += `,cancelled_at=$6`
	}
	query += ` WHERE organization_id=$1 AND id=$2 AND version=$3 AND status=ANY($4)`
	tag, err := tx.Exec(ctx, query, orgID, interviewID, version, allowed, to, now)
	if err != nil {
		return nil, err
	}
	if tag.RowsAffected() != 1 {
		return nil, domainErr.New(domainErr.ErrConflict, "interview changed concurrently", nil)
	}
	if err = InsertAudit(ctx, tx, audit); err != nil {
		return nil, err
	}
	result, err = scanInterview(tx.QueryRow(ctx, `SELECT `+interviewColumns+` FROM interviews WHERE organization_id=$1 AND id=$2`, orgID, interviewID))
	if err != nil {
		return nil, err
	}
	err = tx.Commit(ctx)
	return result, err
}

func InsertAudit(ctx context.Context, tx pgx.Tx, event interviewModel.AuditEvent) error {
	if event.ID == uuid.Nil {
		event.ID = uuid.New()
	}
	payload, err := json.Marshal(map[string]any{"actor_id": nullableUUID(event.ActorID), "session_id": event.SessionID, "device_id": nullableUUID(event.DeviceID), "request_id": event.RequestID, "parent_id": nullableUUID(event.ParentID), "from_status": event.FromStatus, "to_status": event.ToStatus})
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `INSERT INTO audit_outbox (id,organization_id,action,resource_type,resource_id,payload,occurred_at) VALUES ($1,$2,$3,$4,$5,$6,$7)`, event.ID, event.OrganizationID, event.Action, event.ResourceType, event.ResourceID, payload, event.OccurredAt)
	return err
}
func rollback(ctx context.Context, tx pgx.Tx, err *error) {
	if *err != nil {
		_ = tx.Rollback(ctx)
	}
}
func nullableUUID(id uuid.UUID) any {
	if id == uuid.Nil {
		return nil
	}
	return id
}
func sameOptionalUUID(left, right *uuid.UUID) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	return *left == *right
}
func mapWriteError(action string, err error) error {
	if strings.Contains(err.Error(), "duplicate key") {
		return domainErr.New(domainErr.ErrConflict, action+" conflicts with existing state", nil)
	}
	return domainErr.New(domainErr.ErrInternal, "failed to "+action, err)
}
