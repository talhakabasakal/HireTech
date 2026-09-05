package usecase

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	aiModel "github.com/masterfabric-go/masterfabric/internal/domain/ai/model"
	interviewModel "github.com/masterfabric-go/masterfabric/internal/domain/interview/model"
	"github.com/masterfabric-go/masterfabric/internal/shared/authcontext"
	domainErr "github.com/masterfabric-go/masterfabric/internal/shared/errors"
)

type questionDraftGatewayStub struct {
	response aiModel.CompletionResponse
	request  aiModel.CompletionRequest
}

func (s *questionDraftGatewayStub) Complete(_ context.Context, request aiModel.CompletionRequest) (aiModel.CompletionResponse, error) {
	s.request = request
	return s.response, nil
}

type questionDraftRepositoryStub struct {
	created      *interviewModel.QuestionDraft
	createdAudit interviewModel.AuditEvent
	draft        *interviewModel.QuestionDraft
	approved     bool
	rejected     bool
}

func (r *questionDraftRepositoryStub) CreateDraft(_ context.Context, draft *interviewModel.QuestionDraft, audit interviewModel.AuditEvent) error {
	r.created, r.createdAudit = draft, audit
	return nil
}
func (r *questionDraftRepositoryStub) GetDraft(context.Context, uuid.UUID, uuid.UUID) (*interviewModel.QuestionDraft, error) {
	if r.draft != nil {
		return r.draft, nil
	}
	return r.created, nil
}
func (r *questionDraftRepositoryStub) ApproveDraft(_ context.Context, _ uuid.UUID, _ uuid.UUID, _ uuid.UUID, _ time.Time, _ interviewModel.AuditEvent) (*interviewModel.QuestionDraft, *interviewModel.Question, error) {
	r.approved = true
	return r.draft, &interviewModel.Question{ID: uuid.New()}, nil
}
func (r *questionDraftRepositoryStub) RejectDraft(_ context.Context, _ uuid.UUID, _ uuid.UUID, _ uuid.UUID, _ string, _ time.Time, _ interviewModel.AuditEvent) (*interviewModel.QuestionDraft, error) {
	r.rejected = true
	return r.draft, nil
}

func tenantActor(org, user uuid.UUID, permissions ...string) authcontext.ActorContext {
	return authcontext.ActorContext{OrganizationID: org, UserID: user, TokenClass: authcontext.TokenClassTenant, Permissions: permissions}
}

func TestRequestQuestionDraftRequiresAIInterviewAndPersistsValidatedOutput(t *testing.T) {
	org, interviewID, userID := uuid.New(), uuid.New(), uuid.New()
	interviewRepo := &interviewRepositoryStub{get: func(receivedOrg, receivedInterview uuid.UUID) (*interviewModel.Interview, error) {
		require.Equal(t, org, receivedOrg)
		require.Equal(t, interviewID, receivedInterview)
		return &interviewModel.Interview{ID: interviewID, OrganizationID: org, Language: interviewModel.LanguageTurkish, QuestionSource: interviewModel.QuestionSourceAI, Status: interviewModel.StatusDraft}, nil
	}}
	raw, err := json.Marshal(map[string]any{"schema_version": "1.0.0", "interview_id": interviewID.String(), "language": "tr", "question": map[string]any{"type": "CODING", "prompt": "Bir cache katmanı tasarla.", "competency_ids": []string{"system-design"}, "difficulty": 3, "time_limit_seconds": 900}})
	require.NoError(t, err)
	gateway := &questionDraftGatewayStub{response: aiModel.CompletionResponse{Content: string(raw), ModelID: "fine-tuned-interviewer", ModelVersion: "2026-08-01"}}
	drafts := &questionDraftRepositoryStub{}
	service := NewQuestionDraftService(interviewRepo, drafts, gateway)
	service.now = func() time.Time { return time.Date(2026, 9, 4, 10, 0, 0, 0, time.UTC) }

	draft, err := service.RequestDraft(context.Background(), tenantActor(org, userID, "question:manage"), RequestQuestionDraftInput{InterviewID: interviewID, Type: interviewModel.QuestionCoding, CompetencyIDs: []string{"system-design"}, Difficulty: 3, TaskBrief: "Focus on cache invalidation."})
	require.NoError(t, err)
	require.NotNil(t, draft)
	assert.Equal(t, interviewModel.DraftPending, draft.Status)
	assert.Equal(t, "fine-tuned-interviewer", draft.ModelID)
	assert.Equal(t, interviewModel.LanguageTurkish, draft.Language)
	assert.Equal(t, "interviewer", string(gateway.request.Role))
	assert.Equal(t, "tr", string(gateway.request.Language))
	assert.NotEmpty(t, gateway.request.JSONSchema)
	assert.Same(t, draft, drafts.created)
	assert.Equal(t, "interview.question_draft.created", drafts.createdAudit.Action)
}

func TestRequestQuestionDraftRejectsHumanConfiguredInterview(t *testing.T) {
	org, interviewID := uuid.New(), uuid.New()
	interviewRepo := &interviewRepositoryStub{get: func(uuid.UUID, uuid.UUID) (*interviewModel.Interview, error) {
		return &interviewModel.Interview{ID: interviewID, OrganizationID: org, Language: interviewModel.LanguageEnglish, QuestionSource: interviewModel.QuestionSourceHuman, Status: interviewModel.StatusDraft}, nil
	}}
	service := NewQuestionDraftService(interviewRepo, &questionDraftRepositoryStub{}, &questionDraftGatewayStub{})
	_, err := service.RequestDraft(context.Background(), tenantActor(org, uuid.New(), "question:manage"), RequestQuestionDraftInput{InterviewID: interviewID, Type: interviewModel.QuestionCoding, CompetencyIDs: []string{"go"}, Difficulty: 2})
	assert.ErrorIs(t, err, domainErr.ErrConflict)
}

func TestQuestionDraftReviewCannotBeDoneByRequester(t *testing.T) {
	org, interviewID, userID, draftID := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	draft := &interviewModel.QuestionDraft{ID: draftID, OrganizationID: org, InterviewID: interviewID, RequestedBy: userID, Status: interviewModel.DraftPending}
	drafts := &questionDraftRepositoryStub{draft: draft}
	service := NewQuestionDraftService(&interviewRepositoryStub{}, drafts, nil)
	actor := tenantActor(org, userID, "question:manage")
	_, err := service.ApproveDraft(context.Background(), actor, draftID)
	assert.ErrorIs(t, err, domainErr.ErrForbidden)
	_, err = service.RejectDraft(context.Background(), actor, draftID, "needs revision")
	assert.ErrorIs(t, err, domainErr.ErrForbidden)
	assert.False(t, drafts.approved)
	assert.False(t, drafts.rejected)
}
