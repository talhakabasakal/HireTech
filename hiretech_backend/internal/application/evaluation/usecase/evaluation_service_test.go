package usecase

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	aiModel "github.com/masterfabric-go/masterfabric/internal/domain/ai/model"
	evaluationModel "github.com/masterfabric-go/masterfabric/internal/domain/evaluation/model"
	evaluationRepo "github.com/masterfabric-go/masterfabric/internal/domain/evaluation/repository"
	interviewModel "github.com/masterfabric-go/masterfabric/internal/domain/interview/model"
	interviewRepo "github.com/masterfabric-go/masterfabric/internal/domain/interview/repository"
	"github.com/masterfabric-go/masterfabric/internal/shared/authcontext"
	domainErr "github.com/masterfabric-go/masterfabric/internal/shared/errors"
)

type evaluationInterviewStub struct {
	interview *interviewModel.Interview
	questions []*interviewModel.Question
	answers   []*interviewModel.Answer
}

func (s *evaluationInterviewStub) Create(context.Context, *interviewModel.Interview, interviewModel.AuditEvent) error {
	return nil
}
func (s *evaluationInterviewStub) Get(context.Context, uuid.UUID, uuid.UUID) (*interviewModel.Interview, error) {
	return s.interview, nil
}
func (s *evaluationInterviewStub) List(context.Context, uuid.UUID, []interviewModel.Status, int) ([]*interviewModel.Interview, error) {
	return nil, nil
}
func (s *evaluationInterviewStub) AddQuestion(context.Context, *interviewModel.Question, int, interviewModel.AuditEvent) error {
	return nil
}
func (s *evaluationInterviewStub) ListQuestions(context.Context, uuid.UUID, uuid.UUID) ([]*interviewModel.Question, error) {
	return s.questions, nil
}
func (s *evaluationInterviewStub) GetQuestion(context.Context, uuid.UUID, uuid.UUID) (*interviewModel.Question, error) {
	return nil, nil
}
func (s *evaluationInterviewStub) Publish(context.Context, uuid.UUID, uuid.UUID, int, time.Time, interviewModel.AuditEvent) (*interviewModel.Interview, error) {
	return nil, nil
}
func (s *evaluationInterviewStub) CreateInvitation(context.Context, *interviewModel.Invitation, int, interviewModel.AuditEvent) error {
	return nil
}
func (s *evaluationInterviewStub) RedeemInvitation(context.Context, string, uuid.UUID, uuid.UUID, string, *interviewModel.Consent, time.Time, interviewModel.AuditEvent) (*interviewModel.Interview, error) {
	return nil, nil
}
func (s *evaluationInterviewStub) Start(context.Context, *interviewModel.Session, int, time.Time, interviewModel.AuditEvent) (*interviewModel.Interview, error) {
	return nil, nil
}
func (s *evaluationInterviewStub) SubmitAnswer(context.Context, *interviewModel.Answer, interviewModel.AuditEvent) (*interviewModel.Answer, error) {
	return nil, nil
}
func (s *evaluationInterviewStub) GetAnswer(context.Context, uuid.UUID, uuid.UUID) (*interviewModel.Answer, error) {
	return nil, nil
}
func (s *evaluationInterviewStub) ListAnswers(context.Context, uuid.UUID, uuid.UUID) ([]*interviewModel.Answer, error) {
	return s.answers, nil
}
func (s *evaluationInterviewStub) Complete(context.Context, uuid.UUID, uuid.UUID, uuid.UUID, int, time.Time, interviewModel.AuditEvent) (*interviewModel.Interview, error) {
	return nil, nil
}
func (s *evaluationInterviewStub) Cancel(context.Context, uuid.UUID, uuid.UUID, int, time.Time, interviewModel.AuditEvent) (*interviewModel.Interview, error) {
	return nil, nil
}

var _ interviewRepo.InterviewRepository = (*evaluationInterviewStub)(nil)

type evaluationRepositoryStub struct {
	report       *evaluationModel.EvaluationReport
	createdJob   *evaluationModel.EvaluationJob
	createdAudit interviewModel.AuditEvent
	byID         *evaluationModel.EvaluationReport
	reviewed     bool
}

func (s *evaluationRepositoryStub) Create(_ context.Context, job *evaluationModel.EvaluationJob, report *evaluationModel.EvaluationReport, audit interviewModel.AuditEvent) error {
	s.createdJob, s.report, s.createdAudit = job, report, audit
	return nil
}
func (s *evaluationRepositoryStub) GetReport(context.Context, uuid.UUID, uuid.UUID) (*evaluationModel.EvaluationReport, error) {
	return s.report, nil
}
func (s *evaluationRepositoryStub) GetReportByID(context.Context, uuid.UUID, uuid.UUID) (*evaluationModel.EvaluationReport, error) {
	return s.byID, nil
}
func (s *evaluationRepositoryStub) RecordHumanReview(_ context.Context, _ uuid.UUID, _ uuid.UUID, reviewerID uuid.UUID, approved bool, notes string, now time.Time, _ interviewModel.AuditEvent) (*evaluationModel.EvaluationReport, error) {
	s.reviewed = true
	s.byID.HumanReview.Status = evaluationModel.ReviewApproved
	s.byID.HumanReview.ReviewerUserID = &reviewerID
	s.byID.HumanReview.Notes = notes
	s.byID.HumanReview.CompletedAt = &now
	if approved {
		s.byID.Status = evaluationModel.ReportPublished
	}
	return s.byID, nil
}

var _ evaluationRepo.EvaluationRepository = (*evaluationRepositoryStub)(nil)

type failingEvaluationGateway struct{}

func (failingEvaluationGateway) Complete(context.Context, aiModel.CompletionRequest) (aiModel.CompletionResponse, error) {
	return aiModel.CompletionResponse{}, errors.New("test evaluator unavailable")
}

func evaluationActor(orgID, userID uuid.UUID, permissions ...string) authcontext.ActorContext {
	return authcontext.ActorContext{UserID: userID, OrganizationID: orgID, TokenClass: authcontext.TokenClassTenant, Permissions: permissions}
}

func TestRequestEvaluationCreatesExplainableDeterministicReport(t *testing.T) {
	orgID, interviewID, userID := uuid.New(), uuid.New(), uuid.New()
	questionOne, questionTwo := uuid.New(), uuid.New()
	interview := &interviewModel.Interview{ID: interviewID, OrganizationID: orgID, Status: interviewModel.StatusCompleted, Mode: interviewModel.ModeAIDisabled, RubricVersion: "1.0.0"}
	questions := []*interviewModel.Question{
		{ID: questionOne, InterviewID: interviewID, Type: interviewModel.QuestionSystemDesign},
		{ID: questionTwo, InterviewID: interviewID, Type: interviewModel.QuestionCoding},
	}
	textOne, textTwo := "A detailed design answer with trade-offs and reliability considerations.", "A tested implementation explanation with edge cases."
	answers := []*interviewModel.Answer{
		{ID: uuid.New(), InterviewID: interviewID, QuestionID: questionOne, Status: interviewModel.AnswerSubmitted, Text: &textOne},
		{ID: uuid.New(), InterviewID: interviewID, QuestionID: questionTwo, Status: interviewModel.AnswerSubmitted, Text: &textTwo},
	}
	repository := &evaluationRepositoryStub{}
	service := NewEvaluationService(repository, &evaluationInterviewStub{interview: interview, questions: questions, answers: answers})
	report, err := service.RequestEvaluation(context.Background(), evaluationActor(orgID, userID, "evaluation:request"), interviewID)
	require.NoError(t, err)
	assert.Equal(t, evaluationModel.ReportReadyForHumanDecision, report.Status)
	assert.Equal(t, RubricID, report.RubricID)
	assert.Equal(t, DeterministicVersion, report.EvaluatorConfigurationVersion)
	assert.NotNil(t, report.OverallScore)
	assert.Len(t, report.CriterionScores, 8)
	assert.True(t, report.HumanReview.Required)
	assert.Contains(t, report.HumanReview.ReasonCodes, "POLICY_REQUIRED")
	assert.Equal(t, evaluationModel.JobCompleted, repository.createdJob.Status)
	assert.NotContains(t, repository.createdAudit.Action, textOne)
	assert.NotContains(t, repository.createdAudit.Action, textTwo)
	assert.ElementsMatch(t, []string{"answer:" + answers[0].ID.String(), "answer:" + answers[1].ID.String()}, report.EvidenceReferences)
}

func TestRequestEvaluationFlagsUnansweredQuestionForReview(t *testing.T) {
	orgID, interviewID := uuid.New(), uuid.New()
	questionID := uuid.New()
	interview := &interviewModel.Interview{ID: interviewID, OrganizationID: orgID, Status: interviewModel.StatusCompleted, Mode: interviewModel.ModeAIDisabled, RubricVersion: "1.0.0"}
	repository := &evaluationRepositoryStub{}
	service := NewEvaluationService(repository, &evaluationInterviewStub{
		interview: interview,
		questions: []*interviewModel.Question{{ID: questionID, InterviewID: interviewID, Type: interviewModel.QuestionSystemDesign}},
	})
	report, err := service.RequestEvaluation(context.Background(), evaluationActor(orgID, uuid.New(), "evaluation:request"), interviewID)
	require.NoError(t, err)
	assert.Equal(t, evaluationModel.ReportReviewRequired, report.Status)
	assert.Nil(t, report.OverallScore)
	assert.Contains(t, report.Gaps, "Missing answer evidence for question "+questionID.String()+".")
	assert.Contains(t, report.HumanReview.ReasonCodes, "INSUFFICIENT_EVIDENCE")
}

func TestRequestEvaluationFallsBackWhenEvaluatorIsUnavailable(t *testing.T) {
	orgID, interviewID := uuid.New(), uuid.New()
	interview := &interviewModel.Interview{ID: interviewID, OrganizationID: orgID, Status: interviewModel.StatusCompleted, Mode: interviewModel.ModeAIDisabled, RubricVersion: "1.0.0"}
	repository := &evaluationRepositoryStub{}
	service := NewEvaluationService(repository, &evaluationInterviewStub{interview: interview}, failingEvaluationGateway{})

	report, err := service.RequestEvaluation(context.Background(), evaluationActor(orgID, uuid.New(), "evaluation:request"), interviewID)
	require.NoError(t, err)
	assert.Equal(t, DeterministicVersion+"-ai-fallback", report.EvaluatorConfigurationVersion)
	assert.Contains(t, report.Limitations, "The configured Evaluator response was unavailable or failed contract validation; deterministic fallback was used and human review is required.")
	assert.True(t, report.HumanReview.Required)
}

func TestHumanReviewRequiresSeparateReviewerAndPublishesExplicitly(t *testing.T) {
	orgID, reportID, requesterID, reviewerID := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	report := &evaluationModel.EvaluationReport{
		ID: reportID, OrganizationID: orgID, InterviewID: uuid.New(), RequestedBy: requesterID,
		Status:      evaluationModel.ReportReviewRequired,
		HumanReview: &evaluationModel.HumanReview{ID: uuid.New(), ReportID: reportID, Required: true, Status: evaluationModel.ReviewPending},
	}
	repository := &evaluationRepositoryStub{byID: report}
	service := NewEvaluationService(repository, &evaluationInterviewStub{})
	requester := evaluationActor(orgID, requesterID, "evaluation:review")
	_, err := service.RecordHumanReview(context.Background(), requester, reportID, true, "I reviewed the evidence.")
	assert.ErrorIs(t, err, domainErr.ErrForbidden)

	reviewer := evaluationActor(orgID, reviewerID, "evaluation:review")
	_, err = service.RecordHumanReview(context.Background(), reviewer, reportID, true, "I reviewed the evidence and approve publication.")
	assert.ErrorIs(t, err, domainErr.ErrForbidden)

	reviewer = evaluationActor(orgID, reviewerID, "evaluation:review", "evaluation:publish")
	published, err := service.RecordHumanReview(context.Background(), reviewer, reportID, true, "I reviewed the evidence and approve publication.")
	require.NoError(t, err)
	assert.True(t, repository.reviewed)
	assert.Equal(t, evaluationModel.ReportPublished, published.Status)
	assert.Equal(t, reviewerID, *published.HumanReview.ReviewerUserID)
}
