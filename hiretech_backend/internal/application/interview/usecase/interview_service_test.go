package usecase

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	iamService "github.com/masterfabric-go/masterfabric/internal/domain/iam/service"
	interviewModel "github.com/masterfabric-go/masterfabric/internal/domain/interview/model"
	"github.com/masterfabric-go/masterfabric/internal/shared/authcontext"
	domainErr "github.com/masterfabric-go/masterfabric/internal/shared/errors"
	"github.com/masterfabric-go/masterfabric/internal/shared/pagination"
)

var errUnexpectedRepositoryCall = errors.New("unexpected repository call")

type interviewRepositoryStub struct {
	createdInterview *interviewModel.Interview
	createdAudit     interviewModel.AuditEvent
	get              func(uuid.UUID, uuid.UUID) (*interviewModel.Interview, error)
	getQuestion      func(uuid.UUID, uuid.UUID) (*interviewModel.Question, error)
	listPage         func(*pagination.Cursor, int) ([]*interviewModel.Interview, error)
	redeem           func(string, uuid.UUID, uuid.UUID, string, *interviewModel.Consent, interviewModel.AuditEvent) (*interviewModel.Interview, error)
	start            func(*interviewModel.Session, int, interviewModel.AuditEvent) (*interviewModel.Interview, error)
	submit           func(*interviewModel.Answer, interviewModel.AuditEvent) (*interviewModel.Answer, error)
}

func (r *interviewRepositoryStub) Create(_ context.Context, interview *interviewModel.Interview, audit interviewModel.AuditEvent) error {
	r.createdInterview, r.createdAudit = interview, audit
	return nil
}
func (r *interviewRepositoryStub) Get(_ context.Context, organizationID, interviewID uuid.UUID) (*interviewModel.Interview, error) {
	if r.get == nil {
		return nil, errUnexpectedRepositoryCall
	}
	return r.get(organizationID, interviewID)
}
func (r *interviewRepositoryStub) List(context.Context, uuid.UUID, []interviewModel.Status, int) ([]*interviewModel.Interview, error) {
	return nil, errUnexpectedRepositoryCall
}
func (r *interviewRepositoryStub) ListPage(_ context.Context, _ uuid.UUID, _ []interviewModel.Status, after *pagination.Cursor, limit int) ([]*interviewModel.Interview, error) {
	if r.listPage == nil {
		return nil, errUnexpectedRepositoryCall
	}
	return r.listPage(after, limit)
}
func (r *interviewRepositoryStub) AddQuestion(context.Context, *interviewModel.Question, int, interviewModel.AuditEvent) error {
	return errUnexpectedRepositoryCall
}
func (r *interviewRepositoryStub) ListQuestions(context.Context, uuid.UUID, uuid.UUID) ([]*interviewModel.Question, error) {
	return nil, errUnexpectedRepositoryCall
}
func (r *interviewRepositoryStub) GetQuestion(_ context.Context, organizationID, questionID uuid.UUID) (*interviewModel.Question, error) {
	if r.getQuestion == nil {
		return nil, errUnexpectedRepositoryCall
	}
	return r.getQuestion(organizationID, questionID)
}
func (r *interviewRepositoryStub) Publish(context.Context, uuid.UUID, uuid.UUID, int, time.Time, interviewModel.AuditEvent) (*interviewModel.Interview, error) {
	return nil, errUnexpectedRepositoryCall
}
func (r *interviewRepositoryStub) CreateInvitation(context.Context, *interviewModel.Invitation, int, interviewModel.AuditEvent) error {
	return errUnexpectedRepositoryCall
}
func (r *interviewRepositoryStub) RedeemInvitation(_ context.Context, tokenHash string, userID, deviceID uuid.UUID, email string, consent *interviewModel.Consent, _ time.Time, audit interviewModel.AuditEvent) (*interviewModel.Interview, error) {
	if r.redeem == nil {
		return nil, errUnexpectedRepositoryCall
	}
	return r.redeem(tokenHash, userID, deviceID, email, consent, audit)
}
func (r *interviewRepositoryStub) Start(_ context.Context, session *interviewModel.Session, version int, _ time.Time, audit interviewModel.AuditEvent) (*interviewModel.Interview, error) {
	if r.start == nil {
		return nil, errUnexpectedRepositoryCall
	}
	return r.start(session, version, audit)
}
func (r *interviewRepositoryStub) SubmitAnswer(_ context.Context, answer *interviewModel.Answer, audit interviewModel.AuditEvent) (*interviewModel.Answer, error) {
	if r.submit == nil {
		return nil, errUnexpectedRepositoryCall
	}
	return r.submit(answer, audit)
}
func (r *interviewRepositoryStub) GetAnswer(context.Context, uuid.UUID, uuid.UUID) (*interviewModel.Answer, error) {
	return nil, errUnexpectedRepositoryCall
}
func (r *interviewRepositoryStub) ListAnswers(context.Context, uuid.UUID, uuid.UUID) ([]*interviewModel.Answer, error) {
	return nil, errUnexpectedRepositoryCall
}
func (r *interviewRepositoryStub) Complete(context.Context, uuid.UUID, uuid.UUID, uuid.UUID, int, time.Time, interviewModel.AuditEvent) (*interviewModel.Interview, error) {
	return nil, errUnexpectedRepositoryCall
}
func (r *interviewRepositoryStub) Cancel(context.Context, uuid.UUID, uuid.UUID, int, time.Time, interviewModel.AuditEvent) (*interviewModel.Interview, error) {
	return nil, errUnexpectedRepositoryCall
}

type authServiceStub struct {
	claims iamService.TokenClaims
}

func (s *authServiceStub) HashPassword(string) (string, error) { return "", nil }
func (s *authServiceStub) VerifyPassword(string, string) error { return nil }
func (s *authServiceStub) ValidateToken(context.Context, string) (*iamService.TokenClaims, error) {
	return &s.claims, nil
}
func (s *authServiceStub) GenerateToken(_ context.Context, claims iamService.TokenClaims) (string, error) {
	s.claims = claims
	return "candidate-token", nil
}

func TestCreateUsesTrustedTenantAndPermission(t *testing.T) {
	repository := &interviewRepositoryStub{}
	service := NewInterviewService(repository, &authServiceStub{}, "secret")
	organizationID := uuid.New()
	actor := authcontext.ActorContext{
		RequestID: "request-1", UserID: uuid.New(), OrganizationID: organizationID,
		TokenClass: authcontext.TokenClassTenant, Permissions: []string{"interview:create"},
	}
	input := CreateInterviewInput{
		Title: "  Backend Interview  ", CandidateEmail: " Candidate@Example.COM ",
		PositionTitle: " Backend Engineer ", Mode: interviewModel.ModeAIDisabled,
		Language: interviewModel.LanguageTurkish, QuestionSource: interviewModel.QuestionSourceAI,
		ExpiresAt: time.Now().UTC().Add(time.Hour),
	}
	created, err := service.Create(context.Background(), actor, input)
	require.NoError(t, err)
	assert.Equal(t, organizationID, created.OrganizationID)
	assert.Equal(t, "candidate@example.com", created.CandidateEmail)
	assert.Equal(t, "Backend Interview", created.Title)
	assert.Equal(t, interviewModel.StatusDraft, created.Status)
	assert.Equal(t, interviewModel.LanguageTurkish, created.Language)
	assert.Equal(t, interviewModel.QuestionSourceAI, created.QuestionSource)
	assert.Equal(t, organizationID, repository.createdAudit.OrganizationID)
	assert.Equal(t, "interview.created", repository.createdAudit.Action)

	actor.Permissions = nil
	_, err = service.Create(context.Background(), actor, input)
	assert.ErrorIs(t, err, domainErr.ErrForbidden)
	assert.Same(t, created, repository.createdInterview)
}

func TestListPageUsesBoundedKeysetCursor(t *testing.T) {
	organizationID := uuid.New()
	firstID, secondID, thirdID := uuid.New(), uuid.New(), uuid.New()
	firstCreated := time.Date(2026, 9, 5, 12, 0, 0, 0, time.UTC)
	secondCreated := firstCreated.Add(-time.Minute)
	thirdCreated := secondCreated.Add(-time.Minute)
	repository := &interviewRepositoryStub{}
	repository.listPage = func(after *pagination.Cursor, limit int) ([]*interviewModel.Interview, error) {
		assert.Nil(t, after)
		assert.Equal(t, 3, limit)
		return []*interviewModel.Interview{
			{ID: firstID, OrganizationID: organizationID, CreatedAt: firstCreated},
			{ID: secondID, OrganizationID: organizationID, CreatedAt: secondCreated},
			{ID: thirdID, OrganizationID: organizationID, CreatedAt: thirdCreated},
		}, nil
	}
	service := NewInterviewService(repository, &authServiceStub{}, "secret")
	actor := authcontext.ActorContext{UserID: uuid.New(), OrganizationID: organizationID, TokenClass: authcontext.TokenClassTenant, Permissions: []string{"interview:read"}}

	page, err := service.ListPage(context.Background(), actor, nil, nil, 2)
	require.NoError(t, err)
	assert.Len(t, page.Items, 2)
	assert.True(t, page.HasNextPage)
	assert.NotEmpty(t, page.EndCursor)

	cursor, err := pagination.DecodeCursor(page.EndCursor)
	require.NoError(t, err)
	assert.Equal(t, secondID, cursor.ID)
	assert.Equal(t, secondCreated, cursor.CreatedAt)
}

func TestListPageRejectsUnboundedSizeAndInvalidCursor(t *testing.T) {
	repository := &interviewRepositoryStub{}
	service := NewInterviewService(repository, &authServiceStub{}, "secret")
	actor := authcontext.ActorContext{UserID: uuid.New(), OrganizationID: uuid.New(), TokenClass: authcontext.TokenClassTenant, Permissions: []string{"interview:read"}}

	_, err := service.ListPage(context.Background(), actor, nil, nil, 101)
	assert.ErrorIs(t, err, domainErr.ErrValidation)
}

func TestRedeemRequiresVerifiedSessionAndIssuesInterviewScopedToken(t *testing.T) {
	organizationID, interviewID, userID, deviceID := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	repository := &interviewRepositoryStub{}
	auth := &authServiceStub{}
	service := NewInterviewService(repository, auth, "invitation-secret")
	actor := authcontext.ActorContext{
		UserID: userID, Email: "Candidate@Example.com", DeviceID: deviceID,
		TokenClass: authcontext.TokenClassBootstrap, AuthenticationTime: time.Now().UTC(),
	}
	input := RedeemInvitationInput{Token: "one-time-token", PolicyVersion: "v1", Locale: "tr-TR", Purpose: "hiring"}
	_, err := service.RedeemInvitation(context.Background(), actor, input)
	assert.ErrorIs(t, err, domainErr.ErrForbidden)

	actor.SessionID = uuid.NewString()
	repository.redeem = func(tokenHash string, redeemedUserID, redeemedDeviceID uuid.UUID, email string, consent *interviewModel.Consent, audit interviewModel.AuditEvent) (*interviewModel.Interview, error) {
		assert.NotEqual(t, input.Token, tokenHash)
		assert.Len(t, tokenHash, 64)
		assert.Equal(t, userID, redeemedUserID)
		assert.Equal(t, deviceID, redeemedDeviceID)
		assert.Equal(t, "candidate@example.com", email)
		assert.Equal(t, input.PolicyVersion, consent.PolicyVersion)
		assert.Empty(t, audit.OrganizationID, "repository assigns the organization from the invitation")
		return &interviewModel.Interview{ID: interviewID, OrganizationID: organizationID, CandidateUserID: userID}, nil
	}
	access, err := service.RedeemInvitation(context.Background(), actor, input)
	require.NoError(t, err)
	assert.Equal(t, "candidate-token", access.AccessToken)
	assert.Equal(t, interviewID, auth.claims.InterviewID)
	assert.Equal(t, organizationID, auth.claims.OrganizationID)
	assert.Equal(t, actor.SessionID, auth.claims.SessionID)
	assert.Equal(t, deviceID, auth.claims.DeviceID)
	assert.Equal(t, string(authcontext.TokenClassCandidateInterview), auth.claims.TokenClass)
	assert.ElementsMatch(t, []string{"interview:participate", "question:read", "answer:submit"}, auth.claims.Permissions)
}

func TestCandidateScopeAndAnswerAudit(t *testing.T) {
	organizationID, interviewID, questionID, candidateID, deviceID := uuid.New(), uuid.New(), uuid.New(), uuid.New(), uuid.New()
	repository := &interviewRepositoryStub{}
	service := NewInterviewService(repository, &authServiceStub{}, "secret")
	actor := authcontext.ActorContext{
		RequestID: "candidate-request", UserID: candidateID, OrganizationID: organizationID,
		InterviewID: interviewID, DeviceID: deviceID, TokenClass: authcontext.TokenClassCandidateInterview,
	}
	_, err := service.Start(context.Background(), actor, uuid.New())
	assert.ErrorIs(t, err, domainErr.ErrForbidden)

	repository.get = func(receivedOrganizationID, receivedInterviewID uuid.UUID) (*interviewModel.Interview, error) {
		assert.Equal(t, organizationID, receivedOrganizationID)
		assert.Equal(t, interviewID, receivedInterviewID)
		return &interviewModel.Interview{
			ID: interviewID, OrganizationID: organizationID, CandidateUserID: candidateID,
			Status: interviewModel.StatusInProgress, Version: 4, ExpiresAt: time.Now().UTC().Add(time.Hour),
		}, nil
	}
	tenantReader := authcontext.ActorContext{
		UserID: uuid.New(), OrganizationID: organizationID, TokenClass: authcontext.TokenClassTenant,
		Permissions: []string{"interview:read"},
	}
	_, err = service.ListAnswers(context.Background(), tenantReader, interviewID)
	assert.ErrorIs(t, err, domainErr.ErrForbidden, "interview:read must not expose candidate answers")

	repository.getQuestion = func(receivedOrganizationID, receivedQuestionID uuid.UUID) (*interviewModel.Question, error) {
		return &interviewModel.Question{ID: receivedQuestionID, OrganizationID: receivedOrganizationID, InterviewID: interviewID}, nil
	}
	answerText := "sensitive candidate answer"
	idempotencyKey := uuid.New()
	repository.submit = func(answer *interviewModel.Answer, audit interviewModel.AuditEvent) (*interviewModel.Answer, error) {
		assert.Equal(t, questionID, answer.QuestionID)
		assert.Equal(t, candidateID, answer.UserID)
		assert.NotEmpty(t, answer.ContentHash)
		assert.NotContains(t, audit.Action, answerText)
		assert.Equal(t, "interview.answer.submitted", audit.Action)
		assert.Equal(t, interviewID, audit.ParentID)
		return answer, nil
	}
	answer, err := service.SubmitAnswer(context.Background(), actor, SubmitAnswerInput{
		InterviewID: interviewID, QuestionID: questionID, Text: &answerText, IdempotencyKey: idempotencyKey,
	})
	require.NoError(t, err)
	assert.Equal(t, idempotencyKey, answer.IdempotencyKey)
	assert.NotEqual(t, answerText, answer.ContentHash)
}
