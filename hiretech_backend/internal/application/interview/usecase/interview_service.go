package usecase

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"strings"
	"time"

	"github.com/google/uuid"
	iamService "github.com/masterfabric-go/masterfabric/internal/domain/iam/service"
	interviewEvent "github.com/masterfabric-go/masterfabric/internal/domain/interview/event"
	interviewModel "github.com/masterfabric-go/masterfabric/internal/domain/interview/model"
	interviewRepo "github.com/masterfabric-go/masterfabric/internal/domain/interview/repository"
	"github.com/masterfabric-go/masterfabric/internal/shared/authcontext"
	domainErr "github.com/masterfabric-go/masterfabric/internal/shared/errors"
	sharedEvents "github.com/masterfabric-go/masterfabric/internal/shared/events"
	"github.com/masterfabric-go/masterfabric/internal/shared/pagination"
)

type CreateInterviewInput struct {
	Title                string
	CandidateEmail       string
	CandidateDisplayName string
	PositionTitle        string
	Seniority            string
	TechnologyTags       []string
	Mode                 interviewModel.Mode
	RubricVersion        string
	Language             interviewModel.InterviewLanguage
	QuestionSource       interviewModel.QuestionSource
	StartsAt             *time.Time
	ExpiresAt            time.Time
}

type CreateQuestionInput struct {
	InterviewID      uuid.UUID
	Type             interviewModel.QuestionType
	Prompt           string
	CompetencyIDs    []string
	Difficulty       int
	TimeLimitSeconds *int
}

type RedeemInvitationInput struct {
	Token         string
	PolicyVersion string
	Locale        string
	Purpose       string
}

type SubmitAnswerInput struct {
	InterviewID    uuid.UUID
	QuestionID     uuid.UUID
	Text           *string
	CodeLanguage   *string
	CodeContent    *string
	IdempotencyKey uuid.UUID
	SupersedesID   *uuid.UUID
}

type InvitationSecret struct {
	Invitation *interviewModel.Invitation
	Token      string
}

type CandidateAccess struct {
	AccessToken string
	Interview   *interviewModel.Interview
}

type InterviewService struct {
	repo     interviewRepo.InterviewRepository
	auth     iamService.AuthService
	secret   string
	eventBus sharedEvents.EventBus
	now      func() time.Time
}

func NewInterviewService(repo interviewRepo.InterviewRepository, auth iamService.AuthService, secret string, buses ...sharedEvents.EventBus) *InterviewService {
	var eventBus sharedEvents.EventBus
	if len(buses) > 0 {
		eventBus = buses[0]
	}
	return &InterviewService{repo: repo, auth: auth, secret: secret, eventBus: eventBus, now: func() time.Time { return time.Now().UTC() }}
}

func (s *InterviewService) Create(ctx context.Context, actor authcontext.ActorContext, input CreateInterviewInput) (*interviewModel.Interview, error) {
	if err := requireTenantPermission(actor, "interview:create"); err != nil {
		return nil, err
	}
	if strings.TrimSpace(input.Title) == "" || strings.TrimSpace(input.CandidateEmail) == "" || strings.TrimSpace(input.PositionTitle) == "" {
		return nil, domainErr.New(domainErr.ErrValidation, "title, candidate email, and position are required", nil)
	}
	if !input.ExpiresAt.After(s.now()) {
		return nil, domainErr.New(domainErr.ErrValidation, "interview expiry must be in the future", nil)
	}
	if !validMode(input.Mode) {
		return nil, domainErr.New(domainErr.ErrValidation, "invalid interview mode", nil)
	}
	language := input.Language
	if language == "" {
		language = interviewModel.LanguageEnglish
	}
	if !validLanguage(language) {
		return nil, domainErr.New(domainErr.ErrValidation, "invalid interview language", nil)
	}
	questionSource := input.QuestionSource
	if questionSource == "" {
		questionSource = interviewModel.QuestionSourceHuman
	}
	if !validQuestionSource(questionSource) {
		return nil, domainErr.New(domainErr.ErrValidation, "invalid question source", nil)
	}
	now := s.now()
	interview := &interviewModel.Interview{
		ID: uuid.New(), OrganizationID: actor.OrganizationID, CreatedBy: actor.UserID,
		CandidateEmail: strings.ToLower(strings.TrimSpace(input.CandidateEmail)), CandidateDisplayName: strings.TrimSpace(input.CandidateDisplayName),
		Title: strings.TrimSpace(input.Title), PositionTitle: strings.TrimSpace(input.PositionTitle), Seniority: strings.TrimSpace(input.Seniority),
		TechnologyTags: compactStrings(input.TechnologyTags), Mode: input.Mode, Language: language, QuestionSource: questionSource, RubricVersion: strings.TrimSpace(input.RubricVersion),
		Status: interviewModel.StatusDraft, StartsAt: input.StartsAt, ExpiresAt: input.ExpiresAt.UTC(), Version: 1, CreatedAt: now, UpdatedAt: now,
	}
	if err := s.repo.Create(ctx, interview, s.audit(actor, "interview.created", "interview", interview.ID, uuid.Nil, "", string(interview.Status))); err != nil {
		return nil, err
	}
	s.publishChanged(ctx, actor, interview.ID, "interview.created", interview.Status, interview.Version)
	return interview, nil
}

func (s *InterviewService) List(ctx context.Context, actor authcontext.ActorContext, statuses []interviewModel.Status, limit int) ([]*interviewModel.Interview, error) {
	if err := requireTenantPermission(actor, "interview:read"); err != nil {
		return nil, err
	}
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	return s.repo.List(ctx, actor.OrganizationID, statuses, limit)
}

// InterviewPage is a bounded keyset-paginated interview result.
type InterviewPage struct {
	Items       []*interviewModel.Interview
	HasNextPage bool
	EndCursor   string
}

// ListPage returns tenant-scoped interviews after an opaque cursor. The
// repository fetches one extra item so the service can determine whether a
// next page exists without an unbounded count query.
func (s *InterviewService) ListPage(ctx context.Context, actor authcontext.ActorContext, statuses []interviewModel.Status, after *pagination.Cursor, first int) (InterviewPage, error) {
	if err := requireTenantPermission(actor, "interview:read"); err != nil {
		return InterviewPage{}, err
	}
	if first <= 0 || first > 100 {
		return InterviewPage{}, domainErr.New(domainErr.ErrValidation, "first must be between 1 and 100", nil)
	}
	values, err := s.repo.ListPage(ctx, actor.OrganizationID, statuses, after, first+1)
	if err != nil {
		return InterviewPage{}, err
	}
	page := InterviewPage{Items: values}
	if len(values) > first {
		page.HasNextPage = true
		page.Items = values[:first]
	}
	if len(page.Items) > 0 {
		last := page.Items[len(page.Items)-1]
		page.EndCursor, err = pagination.EncodeCursor(last.CreatedAt, last.ID)
		if err != nil {
			return InterviewPage{}, domainErr.New(domainErr.ErrInternal, "failed to encode interview cursor", err)
		}
	}
	return page, nil
}

func (s *InterviewService) Get(ctx context.Context, actor authcontext.ActorContext, id uuid.UUID) (*interviewModel.Interview, error) {
	interview, err := s.scopedInterview(ctx, actor, id)
	if err != nil {
		return nil, err
	}
	if actor.TokenClass != authcontext.TokenClassCandidateInterview {
		if err := requirePermission(actor, "interview:read"); err != nil {
			return nil, err
		}
	}
	return interview, nil
}

func (s *InterviewService) ListQuestions(ctx context.Context, actor authcontext.ActorContext, interviewID uuid.UUID) ([]*interviewModel.Question, error) {
	interview, err := s.scopedInterview(ctx, actor, interviewID)
	if err != nil {
		return nil, err
	}
	if actor.TokenClass != authcontext.TokenClassCandidateInterview {
		if err := requirePermission(actor, "question:read"); err != nil {
			return nil, err
		}
	}
	return s.repo.ListQuestions(ctx, interview.OrganizationID, interview.ID)
}

func (s *InterviewService) ListAnswers(ctx context.Context, actor authcontext.ActorContext, interviewID uuid.UUID) ([]*interviewModel.Answer, error) {
	interview, err := s.scopedInterview(ctx, actor, interviewID)
	if err != nil {
		return nil, err
	}
	if actor.TokenClass != authcontext.TokenClassCandidateInterview {
		if err := requirePermission(actor, "answer:read"); err != nil {
			return nil, err
		}
	}
	return s.repo.ListAnswers(ctx, interview.OrganizationID, interview.ID)
}

func (s *InterviewService) scopedInterview(ctx context.Context, actor authcontext.ActorContext, id uuid.UUID) (*interviewModel.Interview, error) {
	orgID, err := authorizedOrganization(actor, id)
	if err != nil {
		return nil, err
	}
	interview, err := s.repo.Get(ctx, orgID, id)
	if err != nil {
		return nil, err
	}
	if actor.TokenClass == authcontext.TokenClassCandidateInterview &&
		(actor.InterviewID != id || interview.CandidateUserID != uuid.Nil && interview.CandidateUserID != actor.UserID) {
		return nil, domainErr.New(domainErr.ErrForbidden, "interview access denied", nil)
	}
	return interview, nil
}

func (s *InterviewService) AddQuestion(ctx context.Context, actor authcontext.ActorContext, input CreateQuestionInput) (*interviewModel.Question, error) {
	if err := requireTenantPermission(actor, "question:manage"); err != nil {
		return nil, err
	}
	interview, err := s.repo.Get(ctx, actor.OrganizationID, input.InterviewID)
	if err != nil {
		return nil, err
	}
	if !interview.CanAddQuestion() {
		return nil, domainErr.New(domainErr.ErrConflict, "questions cannot be changed in the current interview state", nil)
	}
	if strings.TrimSpace(input.Prompt) == "" || input.Difficulty < 1 || input.Difficulty > 5 || !validQuestionType(input.Type) {
		return nil, domainErr.New(domainErr.ErrValidation, "invalid question", nil)
	}
	question := &interviewModel.Question{ID: uuid.New(), OrganizationID: actor.OrganizationID, InterviewID: interview.ID, Type: input.Type, Prompt: strings.TrimSpace(input.Prompt), CompetencyIDs: compactStrings(input.CompetencyIDs), Difficulty: input.Difficulty, TimeLimitSeconds: input.TimeLimitSeconds, CreatedAt: s.now()}
	if err := s.repo.AddQuestion(ctx, question, interview.Version, s.audit(actor, "interview.question.added", "question", question.ID, interview.ID, string(interview.Status), string(interviewModel.StatusReady))); err != nil {
		return nil, err
	}
	s.publishChanged(ctx, actor, interview.ID, "interview.question.added", interview.Status, interview.Version)
	return question, nil
}

func (s *InterviewService) Publish(ctx context.Context, actor authcontext.ActorContext, id uuid.UUID) (*interviewModel.Interview, error) {
	if err := requireTenantPermission(actor, "interview:manage"); err != nil {
		return nil, err
	}
	interview, err := s.repo.Get(ctx, actor.OrganizationID, id)
	if err != nil {
		return nil, err
	}
	if !interview.CanPublish() {
		return nil, domainErr.New(domainErr.ErrConflict, "interview is not ready to publish", nil)
	}
	updated, err := s.repo.Publish(ctx, actor.OrganizationID, id, interview.Version, s.now(), s.audit(actor, "interview.published", "interview", id, uuid.Nil, string(interview.Status), string(interviewModel.StatusInvited)))
	if err == nil {
		s.publishChanged(ctx, actor, id, "interview.published", updated.Status, updated.Version)
	}
	return updated, err
}

func (s *InterviewService) CreateInvitation(ctx context.Context, actor authcontext.ActorContext, interviewID uuid.UUID, expiresAt time.Time) (*InvitationSecret, error) {
	if err := requireTenantPermission(actor, "interview:manage"); err != nil {
		return nil, err
	}
	interview, err := s.repo.Get(ctx, actor.OrganizationID, interviewID)
	if err != nil {
		return nil, err
	}
	if !interview.CanPublish() {
		return nil, domainErr.New(domainErr.ErrConflict, "interview is not ready for invitation", nil)
	}
	if expiresAt.IsZero() {
		expiresAt = s.now().Add(7 * 24 * time.Hour)
	}
	if !expiresAt.After(s.now()) || expiresAt.After(interview.ExpiresAt) {
		return nil, domainErr.New(domainErr.ErrValidation, "invalid invitation expiry", nil)
	}
	token, err := randomToken(32)
	if err != nil {
		return nil, domainErr.New(domainErr.ErrInternal, "failed to create invitation", err)
	}
	invitation := &interviewModel.Invitation{ID: uuid.New(), OrganizationID: actor.OrganizationID, InterviewID: interviewID, TokenHash: hashToken(s.secret, token), ExpiresAt: expiresAt.UTC(), CreatedAt: s.now()}
	if err := s.repo.CreateInvitation(ctx, invitation, interview.Version, s.audit(actor, "interview.invitation.created", "invitation", invitation.ID, interviewID, string(interview.Status), string(interviewModel.StatusInvited))); err != nil {
		return nil, err
	}
	s.publishChanged(ctx, actor, interviewID, "interview.invitation.created", interview.Status, interview.Version)
	return &InvitationSecret{Invitation: invitation, Token: token}, nil
}

func (s *InterviewService) RedeemInvitation(ctx context.Context, actor authcontext.ActorContext, input RedeemInvitationInput) (*CandidateAccess, error) {
	if !actor.IsBootstrap() || actor.SessionID == "" || actor.DeviceID == uuid.Nil {
		return nil, domainErr.New(domainErr.ErrForbidden, "verified session required", nil)
	}
	if strings.TrimSpace(input.Token) == "" || strings.TrimSpace(input.PolicyVersion) == "" || strings.TrimSpace(input.Purpose) == "" {
		return nil, domainErr.New(domainErr.ErrValidation, "invitation token and notice acceptance are required", nil)
	}
	consent := &interviewModel.Consent{ID: uuid.New(), UserID: actor.UserID, PolicyVersion: strings.TrimSpace(input.PolicyVersion), Locale: strings.TrimSpace(input.Locale), Purpose: strings.TrimSpace(input.Purpose), AcceptedAt: s.now()}
	interview, err := s.repo.RedeemInvitation(ctx, hashToken(s.secret, input.Token), actor.UserID, actor.DeviceID, strings.ToLower(strings.TrimSpace(actor.Email)), consent, s.now(), s.audit(actor, "interview.invitation.redeemed", "invitation", uuid.Nil, uuid.Nil, "active", "used"))
	if err != nil {
		return nil, err
	}
	token, err := s.auth.GenerateToken(ctx, iamService.TokenClaims{UserID: actor.UserID, Email: actor.Email, OrganizationID: interview.OrganizationID, SessionID: actor.SessionID, DeviceID: actor.DeviceID, InterviewID: interview.ID, TokenClass: string(authcontext.TokenClassCandidateInterview), AuthenticationMethods: actor.AuthenticationMethods, AuthenticationTime: actor.AuthenticationTime, Permissions: []string{"interview:participate", "question:read", "answer:submit"}})
	if err != nil {
		return nil, domainErr.New(domainErr.ErrInternal, "failed to issue candidate access token", err)
	}
	s.publishChanged(ctx, actor, interview.ID, "interview.invitation.redeemed", interview.Status, interview.Version)
	return &CandidateAccess{AccessToken: token, Interview: interview}, nil
}

func (s *InterviewService) Start(ctx context.Context, actor authcontext.ActorContext, interviewID uuid.UUID) (*interviewModel.Interview, error) {
	interview, err := s.requireCandidate(ctx, actor, interviewID)
	if err != nil {
		return nil, err
	}
	if interview.Status != interviewModel.StatusInvited {
		return nil, domainErr.New(domainErr.ErrConflict, "interview cannot be started", nil)
	}
	if !s.now().Before(interview.ExpiresAt) {
		return nil, domainErr.New(domainErr.ErrConflict, "interview has expired", nil)
	}
	session := &interviewModel.Session{ID: uuid.New(), OrganizationID: interview.OrganizationID, InterviewID: interview.ID, UserID: actor.UserID, DeviceID: actor.DeviceID, Status: interviewModel.SessionActive, LastSeenAt: s.now(), Version: 1, CreatedAt: s.now()}
	updated, err := s.repo.Start(ctx, session, interview.Version, s.now(), s.audit(actor, "interview.started", "interview", interview.ID, uuid.Nil, string(interview.Status), string(interviewModel.StatusInProgress)))
	if err == nil {
		s.publishChanged(ctx, actor, interview.ID, "interview.started", updated.Status, updated.Version)
	}
	return updated, err
}

func (s *InterviewService) SubmitAnswer(ctx context.Context, actor authcontext.ActorContext, input SubmitAnswerInput) (*interviewModel.Answer, error) {
	interview, err := s.requireCandidate(ctx, actor, input.InterviewID)
	if err != nil {
		return nil, err
	}
	if interview.Status != interviewModel.StatusInProgress {
		return nil, domainErr.New(domainErr.ErrConflict, "answers are not accepted in the current interview state", nil)
	}
	question, err := s.repo.GetQuestion(ctx, interview.OrganizationID, input.QuestionID)
	if err != nil || question.InterviewID != interview.ID {
		return nil, domainErr.New(domainErr.ErrNotFound, "question not found", nil)
	}
	if input.IdempotencyKey == uuid.Nil {
		return nil, domainErr.New(domainErr.ErrValidation, "idempotency key is required", nil)
	}
	textValue := optionalTrim(input.Text)
	codeLanguage := optionalTrim(input.CodeLanguage)
	codeContent := optionalRaw(input.CodeContent)
	if textValue == nil && codeContent == nil {
		return nil, domainErr.New(domainErr.ErrValidation, "answer text or code is required", nil)
	}
	now := s.now()
	answer := &interviewModel.Answer{ID: uuid.New(), OrganizationID: interview.OrganizationID, InterviewID: interview.ID, QuestionID: question.ID, UserID: actor.UserID, Status: interviewModel.AnswerSubmitted, Text: textValue, CodeLanguage: codeLanguage, CodeContent: codeContent, ContentHash: answerHash(textValue, codeLanguage, codeContent), IdempotencyKey: input.IdempotencyKey, SupersedesID: input.SupersedesID, SubmittedAt: now, CreatedAt: now}
	saved, err := s.repo.SubmitAnswer(ctx, answer, s.audit(actor, "interview.answer.submitted", "answer", answer.ID, interview.ID, "", string(answer.Status)))
	if err == nil {
		s.publishChanged(ctx, actor, interview.ID, "interview.answer.submitted", interview.Status, interview.Version)
	}
	return saved, err
}

func (s *InterviewService) Complete(ctx context.Context, actor authcontext.ActorContext, interviewID uuid.UUID) (*interviewModel.Interview, error) {
	interview, err := s.requireCandidate(ctx, actor, interviewID)
	if err != nil {
		return nil, err
	}
	if interview.Status != interviewModel.StatusInProgress {
		return nil, domainErr.New(domainErr.ErrConflict, "interview cannot be completed", nil)
	}
	updated, err := s.repo.Complete(ctx, interview.OrganizationID, interview.ID, actor.UserID, interview.Version, s.now(), s.audit(actor, "interview.completed", "interview", interview.ID, uuid.Nil, string(interview.Status), string(interviewModel.StatusCompleted)))
	if err == nil {
		s.publishChanged(ctx, actor, interview.ID, "interview.completed", updated.Status, updated.Version)
	}
	return updated, err
}

func (s *InterviewService) Cancel(ctx context.Context, actor authcontext.ActorContext, interviewID uuid.UUID) (*interviewModel.Interview, error) {
	if err := requireTenantPermission(actor, "interview:manage"); err != nil {
		return nil, err
	}
	interview, err := s.repo.Get(ctx, actor.OrganizationID, interviewID)
	if err != nil {
		return nil, err
	}
	if interview.IsTerminal() {
		return nil, domainErr.New(domainErr.ErrConflict, "interview cannot be cancelled", nil)
	}
	updated, err := s.repo.Cancel(ctx, actor.OrganizationID, interview.ID, interview.Version, s.now(), s.audit(actor, "interview.cancelled", "interview", interview.ID, uuid.Nil, string(interview.Status), string(interviewModel.StatusCancelled)))
	if err == nil {
		s.publishChanged(ctx, actor, interview.ID, "interview.cancelled", updated.Status, updated.Version)
	}
	return updated, err
}

func (s *InterviewService) publishChanged(ctx context.Context, actor authcontext.ActorContext, interviewID uuid.UUID, eventType string, status interviewModel.Status, version int) {
	if s.eventBus == nil || actor.OrganizationID == uuid.Nil || interviewID == uuid.Nil {
		return
	}
	// Realtime delivery is advisory; the committed repository mutation remains
	// authoritative and the event contains no protected interview content.
	_ = s.eventBus.Publish(ctx, sharedEvents.TopicInterview, interviewEvent.Changed{
		OrganizationID: actor.OrganizationID, InterviewID: interviewID, ActorID: actor.UserID,
		EventType: eventType, Status: string(status), Version: version, Timestamp: s.now(),
	})
}

func (s *InterviewService) GetQuestion(ctx context.Context, actor authcontext.ActorContext, questionID uuid.UUID) (*interviewModel.Question, error) {
	question, err := s.repo.GetQuestion(ctx, actor.OrganizationID, questionID)
	if err != nil {
		return nil, err
	}
	if actor.TokenClass == authcontext.TokenClassCandidateInterview && actor.InterviewID != question.InterviewID {
		return nil, domainErr.New(domainErr.ErrForbidden, "question access denied", nil)
	}
	if actor.TokenClass != authcontext.TokenClassCandidateInterview {
		if err := requireTenantPermission(actor, "question:read"); err != nil {
			return nil, err
		}
	}
	return question, nil
}

func (s *InterviewService) GetAnswer(ctx context.Context, actor authcontext.ActorContext, answerID uuid.UUID) (*interviewModel.Answer, error) {
	answer, err := s.repo.GetAnswer(ctx, actor.OrganizationID, answerID)
	if err != nil {
		return nil, err
	}
	if actor.TokenClass == authcontext.TokenClassCandidateInterview {
		if actor.InterviewID != answer.InterviewID || actor.UserID != answer.UserID {
			return nil, domainErr.New(domainErr.ErrForbidden, "answer access denied", nil)
		}
	} else if err := requireTenantPermission(actor, "answer:read"); err != nil {
		return nil, err
	}
	return answer, nil
}

func (s *InterviewService) requireCandidate(ctx context.Context, actor authcontext.ActorContext, interviewID uuid.UUID) (*interviewModel.Interview, error) {
	if actor.TokenClass != authcontext.TokenClassCandidateInterview || actor.InterviewID != interviewID || actor.OrganizationID == uuid.Nil || actor.DeviceID == uuid.Nil {
		return nil, domainErr.New(domainErr.ErrForbidden, "candidate interview access denied", nil)
	}
	interview, err := s.repo.Get(ctx, actor.OrganizationID, interviewID)
	if err != nil {
		return nil, err
	}
	if interview.CandidateUserID != actor.UserID {
		return nil, domainErr.New(domainErr.ErrForbidden, "candidate interview access denied", nil)
	}
	return interview, nil
}

func (s *InterviewService) audit(actor authcontext.ActorContext, action, resourceType string, resourceID, parentID uuid.UUID, from, to string) interviewModel.AuditEvent {
	return interviewModel.AuditEvent{ID: uuid.New(), OrganizationID: actor.OrganizationID, ActorID: actor.UserID, SessionID: actor.SessionID, DeviceID: actor.DeviceID, RequestID: actor.RequestID, Action: action, ResourceType: resourceType, ResourceID: resourceID, ParentID: parentID, FromStatus: from, ToStatus: to, OccurredAt: s.now()}
}

func requireTenantPermission(actor authcontext.ActorContext, permission string) error {
	if actor.TokenClass != authcontext.TokenClassTenant || actor.OrganizationID == uuid.Nil {
		return domainErr.New(domainErr.ErrForbidden, "tenant access required", nil)
	}
	return requirePermission(actor, permission)
}

func requirePermission(actor authcontext.ActorContext, permission string) error {
	for _, granted := range actor.Permissions {
		if granted == permission || granted == "*" {
			return nil
		}
	}
	return domainErr.New(domainErr.ErrForbidden, "permission denied", nil)
}

func authorizedOrganization(actor authcontext.ActorContext, interviewID uuid.UUID) (uuid.UUID, error) {
	if actor.OrganizationID == uuid.Nil {
		return uuid.Nil, domainErr.New(domainErr.ErrForbidden, "organization context required", nil)
	}
	if actor.TokenClass == authcontext.TokenClassCandidateInterview && actor.InterviewID != interviewID {
		return uuid.Nil, domainErr.New(domainErr.ErrForbidden, "interview access denied", nil)
	}
	if actor.TokenClass != authcontext.TokenClassTenant && actor.TokenClass != authcontext.TokenClassCandidateInterview {
		return uuid.Nil, domainErr.New(domainErr.ErrForbidden, "interview access denied", nil)
	}
	return actor.OrganizationID, nil
}

func validLanguage(language interviewModel.InterviewLanguage) bool {
	return language == interviewModel.LanguageTurkish || language == interviewModel.LanguageEnglish
}

func validQuestionSource(source interviewModel.QuestionSource) bool {
	return source == interviewModel.QuestionSourceHuman || source == interviewModel.QuestionSourceAI
}

func validMode(mode interviewModel.Mode) bool {
	return mode == interviewModel.ModeAIDisabled || mode == interviewModel.ModeGuidedAI || mode == interviewModel.ModeAICollaboration
}
func validQuestionType(kind interviewModel.QuestionType) bool {
	return kind == interviewModel.QuestionTechnicalDiscussion || kind == interviewModel.QuestionCoding || kind == interviewModel.QuestionSystemDesign || kind == interviewModel.QuestionDebugging
}
func compactStrings(values []string) []string {
	out := make([]string, 0, len(values))
	seen := map[string]bool{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" && !seen[value] {
			seen[value] = true
			out = append(out, value)
		}
	}
	return out
}
func optionalTrim(value *string) *string {
	if value == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*value)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}
func optionalRaw(value *string) *string {
	if value == nil || *value == "" {
		return nil
	}
	copy := *value
	return &copy
}
func randomToken(size int) (string, error) {
	raw := make([]byte, size)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(raw), nil
}
func hashToken(secret, token string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(token))
	return hex.EncodeToString(mac.Sum(nil))
}
func answerHash(text, language, code *string) string {
	hash := sha256.New()
	for _, value := range []*string{text, language, code} {
		if value != nil {
			_, _ = hash.Write([]byte(*value))
		}
		_, _ = hash.Write([]byte{0})
	}
	return hex.EncodeToString(hash.Sum(nil))
}
