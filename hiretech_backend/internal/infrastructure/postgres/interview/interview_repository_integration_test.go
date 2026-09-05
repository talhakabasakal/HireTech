package interview_test

import (
	"context"
	"errors"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	evaluationUsecase "github.com/masterfabric-go/masterfabric/internal/application/evaluation/usecase"
	interviewUsecase "github.com/masterfabric-go/masterfabric/internal/application/interview/usecase"
	evaluationModel "github.com/masterfabric-go/masterfabric/internal/domain/evaluation/model"
	interviewModel "github.com/masterfabric-go/masterfabric/internal/domain/interview/model"
	infraAuth "github.com/masterfabric-go/masterfabric/internal/infrastructure/auth"
	pgAudit "github.com/masterfabric-go/masterfabric/internal/infrastructure/postgres/audit"
	pgEvaluation "github.com/masterfabric-go/masterfabric/internal/infrastructure/postgres/evaluation"
	pgInterview "github.com/masterfabric-go/masterfabric/internal/infrastructure/postgres/interview"
	"github.com/masterfabric-go/masterfabric/internal/shared/authcontext"
	"github.com/masterfabric-go/masterfabric/internal/shared/config"
	domainErr "github.com/masterfabric-go/masterfabric/internal/shared/errors"
)

func TestInterviewLifecyclePostgres(t *testing.T) {
	dsn := os.Getenv("INTERVIEW_TEST_DSN")
	if dsn == "" {
		t.Skip("set INTERVIEW_TEST_DSN to run PostgreSQL integration tests")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, dsn)
	require.NoError(t, err)
	t.Cleanup(pool.Close)
	require.NoError(t, pool.Ping(ctx))

	organizationID, interviewerID, candidateID, deviceID := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	suffix := uuid.NewString()
	candidateEmail := "candidate-" + suffix + "@example.test"
	_, err = pool.Exec(ctx, `INSERT INTO organizations (id,name,slug) VALUES ($1,$2,$3)`, organizationID, "Phase 4 Test", "phase-4-"+suffix)
	require.NoError(t, err, "apply migrations before running integration tests")
	_, err = pool.Exec(ctx, `INSERT INTO users (id,email,status) VALUES ($1,$2,'active'),($3,$4,'active')`, interviewerID, "interviewer-"+suffix+"@example.test", candidateID, candidateEmail)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `INSERT INTO security_devices (id,user_id,name,state) VALUES ($1,$2,'integration device','trusted')`, deviceID, candidateID)
	require.NoError(t, err)
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cleanupCancel()
		_, _ = pool.Exec(cleanupCtx, `DELETE FROM audit_logs WHERE organization_id=$1`, organizationID)
		_, _ = pool.Exec(cleanupCtx, `DELETE FROM audit_outbox WHERE organization_id=$1`, organizationID)
		_, _ = pool.Exec(cleanupCtx, `DELETE FROM organizations WHERE id=$1`, organizationID)
		_, _ = pool.Exec(cleanupCtx, `DELETE FROM users WHERE id=$1 OR id=$2`, interviewerID, candidateID)
	})

	jwtService := infraAuth.NewJWTService(config.JWTConfig{Secret: "phase-4-integration-secret", AccessTokenMinutes: 15, Issuer: "phase-4-test"})
	repository := pgInterview.NewRepository(pool)
	service := interviewUsecase.NewInterviewService(repository, jwtService, "phase-4-invitation-secret")
	evaluationRepository := pgEvaluation.NewRepository(pool)
	evaluationService := evaluationUsecase.NewEvaluationService(evaluationRepository, repository)
	tenantActor := authcontext.ActorContext{
		RequestID: "tenant-request", UserID: interviewerID, OrganizationID: organizationID,
		TokenClass: authcontext.TokenClassTenant, Permissions: []string{"*"},
	}

	created, err := service.Create(ctx, tenantActor, interviewUsecase.CreateInterviewInput{
		Title: "Backend Security Interview", CandidateEmail: candidateEmail, CandidateDisplayName: "Synthetic Candidate",
		PositionTitle: "Backend Engineer", Seniority: "Senior", TechnologyTags: []string{"Go", "PostgreSQL"},
		Mode: interviewModel.ModeAIDisabled, RubricVersion: "1.0.0", Language: interviewModel.LanguageEnglish, QuestionSource: interviewModel.QuestionSourceHuman, ExpiresAt: time.Now().UTC().Add(24 * time.Hour),
	})
	require.NoError(t, err)
	assert.Equal(t, organizationID, created.OrganizationID)
	assert.Equal(t, interviewModel.StatusDraft, created.Status)
	assert.Equal(t, interviewModel.LanguageEnglish, created.Language)
	assert.Equal(t, interviewModel.QuestionSourceHuman, created.QuestionSource)

	question, err := service.AddQuestion(ctx, tenantActor, interviewUsecase.CreateQuestionInput{
		InterviewID: created.ID, Type: interviewModel.QuestionSystemDesign,
		Prompt: "Design a tenant-safe refresh-token service.", CompetencyIDs: []string{"security", "databases"}, Difficulty: 4,
	})
	require.NoError(t, err)
	assert.Equal(t, 1, question.Sequence)

	staleQuestion := &interviewModel.Question{
		ID: uuid.New(), OrganizationID: organizationID, InterviewID: created.ID,
		Type: interviewModel.QuestionCoding, Prompt: "stale write", Difficulty: 3, CreatedAt: time.Now().UTC(),
	}
	err = repository.AddQuestion(ctx, staleQuestion, created.Version, interviewModel.AuditEvent{
		ID: uuid.New(), OrganizationID: organizationID, ActorID: interviewerID,
		Action: "stale.question", ResourceType: "question", ResourceID: staleQuestion.ID, OccurredAt: time.Now().UTC(),
	})
	assert.ErrorIs(t, err, domainErr.ErrConflict)

	published, err := service.Publish(ctx, tenantActor, created.ID)
	require.NoError(t, err)
	assert.Equal(t, interviewModel.StatusInvited, published.Status)
	invitation, err := service.CreateInvitation(ctx, tenantActor, created.ID, time.Now().UTC().Add(time.Hour))
	require.NoError(t, err)
	require.NotEmpty(t, invitation.Token)

	bootstrapActor := authcontext.ActorContext{
		RequestID: "candidate-bootstrap", UserID: candidateID, Email: candidateEmail,
		SessionID: uuid.NewString(), DeviceID: deviceID, TokenClass: authcontext.TokenClassBootstrap,
		AuthenticationTime: time.Now().UTC(),
	}
	access, err := service.RedeemInvitation(ctx, bootstrapActor, interviewUsecase.RedeemInvitationInput{
		Token: invitation.Token, PolicyVersion: "privacy-v1", Locale: "tr-TR", Purpose: "technical hiring assessment",
	})
	require.NoError(t, err)
	claims, err := jwtService.ValidateToken(ctx, access.AccessToken)
	require.NoError(t, err)
	assert.Equal(t, created.ID, claims.InterviewID)
	assert.Equal(t, organizationID, claims.OrganizationID)
	assert.Equal(t, string(authcontext.TokenClassCandidateInterview), claims.TokenClass)

	_, err = service.RedeemInvitation(ctx, bootstrapActor, interviewUsecase.RedeemInvitationInput{
		Token: invitation.Token, PolicyVersion: "privacy-v1", Locale: "tr-TR", Purpose: "technical hiring assessment",
	})
	assert.ErrorIs(t, err, domainErr.ErrUnauthorized, "invitation tokens are one-time use")

	candidateActor := authcontext.ActorContext{
		RequestID: "candidate-request", UserID: candidateID, Email: candidateEmail,
		OrganizationID: organizationID, InterviewID: created.ID, SessionID: bootstrapActor.SessionID,
		DeviceID: deviceID, TokenClass: authcontext.TokenClassCandidateInterview,
		Permissions: claims.Permissions, AuthenticationTime: claims.AuthenticationTime,
	}
	started, err := service.Start(ctx, candidateActor, created.ID)
	require.NoError(t, err)
	assert.Equal(t, interviewModel.StatusInProgress, started.Status)

	answerText := "Use tenant-bound sessions, one-time refresh records, and row locks."
	idempotencyKey := uuid.New()
	answerInput := interviewUsecase.SubmitAnswerInput{
		InterviewID: created.ID, QuestionID: question.ID, Text: &answerText, IdempotencyKey: idempotencyKey,
	}
	const parallelRetries = 8
	answers := make([]*interviewModel.Answer, parallelRetries)
	errorsFound := make([]error, parallelRetries)
	var waitGroup sync.WaitGroup
	for index := range answers {
		waitGroup.Add(1)
		go func(index int) {
			defer waitGroup.Done()
			answers[index], errorsFound[index] = service.SubmitAnswer(context.Background(), candidateActor, answerInput)
		}(index)
	}
	waitGroup.Wait()
	for index, submitErr := range errorsFound {
		require.NoError(t, submitErr)
		assert.Equal(t, answers[0].ID, answers[index].ID)
	}

	changedText := "This is a different payload."
	_, err = service.SubmitAnswer(ctx, candidateActor, interviewUsecase.SubmitAnswerInput{
		InterviewID: created.ID, QuestionID: question.ID, Text: &changedText, IdempotencyKey: idempotencyKey,
	})
	assert.ErrorIs(t, err, domainErr.ErrConflict)

	correction := "Use tenant-bound sessions, rotating tokens, row locks, and replay-family revocation."
	replacement, err := service.SubmitAnswer(ctx, candidateActor, interviewUsecase.SubmitAnswerInput{
		InterviewID: created.ID, QuestionID: question.ID, Text: &correction,
		IdempotencyKey: uuid.New(), SupersedesID: &answers[0].ID,
	})
	require.NoError(t, err)
	assert.Equal(t, answers[0].ID, *replacement.SupersedesID)
	original, err := repository.GetAnswer(ctx, organizationID, answers[0].ID)
	require.NoError(t, err)
	assert.Equal(t, interviewModel.AnswerSuperseded, original.Status)

	_, err = repository.Get(ctx, uuid.New(), created.ID)
	assert.True(t, errors.Is(err, domainErr.ErrNotFound), "cross-tenant IDs must not resolve")
	completed, err := service.Complete(ctx, candidateActor, created.ID)
	require.NoError(t, err)
	assert.Equal(t, interviewModel.StatusCompleted, completed.Status)

	report, err := evaluationService.RequestEvaluation(ctx, tenantActor, created.ID)
	require.NoError(t, err)
	assert.Equal(t, evaluationModel.ReportReadyForHumanDecision, report.Status)
	assert.True(t, report.HumanReview.Required)
	assert.Len(t, report.CriterionScores, 8)
	_, err = evaluationService.RecordHumanReview(ctx, tenantActor, report.ID, true, "Requester cannot self-approve.")
	assert.ErrorIs(t, err, domainErr.ErrForbidden)
	reviewerActor := tenantActor
	reviewerActor.UserID = candidateID
	reviewerActor.Permissions = []string{"evaluation:review", "evaluation:publish"}
	publishedReport, err := evaluationService.RecordHumanReview(ctx, reviewerActor, report.ID, true, "Reviewed the evidence and approved publication.")
	require.NoError(t, err)
	assert.Equal(t, evaluationModel.ReportPublished, publishedReport.Status)
	assert.Equal(t, evaluationModel.ReviewApproved, publishedReport.HumanReview.Status)

	var outboxCount int
	var payloads string
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*),COALESCE(string_agg(payload::text,''),'') FROM audit_outbox WHERE organization_id=$1`, organizationID).Scan(&outboxCount, &payloads))
	assert.Equal(t, 11, outboxCount, "idempotent retries and rejected writes must not create extra business outbox events")
	assert.NotContains(t, payloads, question.Prompt)
	assert.NotContains(t, payloads, answerText)
	assert.NotContains(t, payloads, correction)
	assert.NotContains(t, payloads, invitation.Token)

	auditRepository := pgAudit.NewAuditRepo(pool)
	relayed, err := auditRepository.RelayPending(ctx, 100)
	require.NoError(t, err)
	assert.Equal(t, outboxCount, relayed, "every committed outbox event must be projected")
	relayed, err = auditRepository.RelayPending(ctx, 100)
	require.NoError(t, err)
	assert.Zero(t, relayed, "published events must be safe to retry without duplicates")
	var projectedCount int
	var projectedPayloads string
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*),COALESCE(string_agg(metadata::text,''),'') FROM audit_logs WHERE organization_id=$1`, organizationID).Scan(&projectedCount, &projectedPayloads))
	assert.Equal(t, outboxCount, projectedCount)
	assert.NotContains(t, projectedPayloads, question.Prompt)
	assert.NotContains(t, projectedPayloads, answerText)
	assert.NotContains(t, projectedPayloads, correction)
	assert.NotContains(t, projectedPayloads, invitation.Token)

	var storedHash string
	require.NoError(t, pool.QueryRow(ctx, `SELECT token_hash FROM interview_invitations WHERE id=$1`, invitation.Invitation.ID).Scan(&storedHash))
	assert.NotEqual(t, invitation.Token, storedHash)
	assert.False(t, strings.Contains(storedHash, invitation.Token))
	var consentCount int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM interview_consents WHERE interview_id=$1 AND user_id=$2`, created.ID, candidateID).Scan(&consentCount))
	assert.Equal(t, 1, consentCount)
}
