package resolver

import (
	"context"
	"strings"

	"github.com/google/uuid"
	"github.com/masterfabric-go/masterfabric/graph/model"
	interviewModel "github.com/masterfabric-go/masterfabric/internal/domain/interview/model"
	"github.com/masterfabric-go/masterfabric/internal/shared/authcontext"
	domainErr "github.com/masterfabric-go/masterfabric/internal/shared/errors"
)

func requireActor(ctx context.Context) (authcontext.ActorContext, error) {
	actor, ok := authcontext.FromContext(ctx)
	if !ok {
		return authcontext.ActorContext{}, domainErr.New(domainErr.ErrUnauthorized, "authentication required", nil)
	}
	return actor, nil
}

func interviewGraphQL(value *interviewModel.Interview) *model.Interview {
	if value == nil {
		return nil
	}
	var candidateUserID *uuid.UUID
	if value.CandidateUserID != uuid.Nil {
		id := value.CandidateUserID
		candidateUserID = &id
	}
	questions := make([]*model.Question, 0, len(value.Questions))
	for _, question := range value.Questions {
		questions = append(questions, questionGraphQL(question))
	}
	answers := make([]*model.Answer, 0, len(value.Answers))
	for _, answer := range value.Answers {
		answers = append(answers, answerGraphQL(answer))
	}
	return &model.Interview{
		ID: value.ID, OrganizationID: value.OrganizationID, CandidateUserID: candidateUserID,
		CandidateEmail: value.CandidateEmail, CandidateDisplayName: value.CandidateDisplayName,
		Title: value.Title, PositionTitle: value.PositionTitle, Seniority: value.Seniority,
		TechnologyTags: nonNilStrings(value.TechnologyTags),
		Mode:           model.InterviewMode(strings.ToUpper(string(value.Mode))),
		Language:       model.InterviewLanguage(strings.ToUpper(string(value.Language))),
		QuestionSource: model.QuestionSource(strings.ToUpper(string(value.QuestionSource))),
		RubricVersion:  value.RubricVersion, Status: model.InterviewStatus(strings.ToUpper(string(value.Status))),
		StartsAt: value.StartsAt, ExpiresAt: value.ExpiresAt, StartedAt: value.StartedAt,
		CompletedAt: value.CompletedAt, CancelledAt: value.CancelledAt, Version: value.Version,
		Questions: questions, Answers: answers, CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt,
	}
}

func questionGraphQL(value *interviewModel.Question) *model.Question {
	if value == nil {
		return nil
	}
	return &model.Question{
		ID: value.ID, InterviewID: value.InterviewID, Sequence: value.Sequence,
		Type: model.QuestionType(strings.ToUpper(string(value.Type))), Prompt: value.Prompt,
		CompetencyIds: nonNilStrings(value.CompetencyIDs), Difficulty: value.Difficulty,
		TimeLimitSeconds: value.TimeLimitSeconds, CreatedAt: value.CreatedAt,
	}
}

func answerGraphQL(value *interviewModel.Answer) *model.Answer {
	if value == nil {
		return nil
	}
	return &model.Answer{
		ID: value.ID, InterviewID: value.InterviewID, QuestionID: value.QuestionID,
		Status: model.AnswerStatus(strings.ToUpper(string(value.Status))),
		Text:   value.Text, CodeLanguage: value.CodeLanguage, CodeContent: value.CodeContent,
		ContentHash: value.ContentHash, IdempotencyKey: value.IdempotencyKey,
		SupersedesID: value.SupersedesID, SubmittedAt: value.SubmittedAt,
	}
}

func nonNilStrings(values []string) []string {
	if values == nil {
		return []string{}
	}
	return values
}
