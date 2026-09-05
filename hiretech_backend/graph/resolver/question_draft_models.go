package resolver

import (
	"strings"

	"github.com/masterfabric-go/masterfabric/graph/model"
	interviewModel "github.com/masterfabric-go/masterfabric/internal/domain/interview/model"
)

func questionDraftGraphQL(value *interviewModel.QuestionDraft) *model.QuestionDraft {
	if value == nil {
		return nil
	}
	return &model.QuestionDraft{ID: value.ID, InterviewID: value.InterviewID, RequestedBy: value.RequestedBy, ReviewedBy: value.ReviewedBy, Type: model.QuestionType(strings.ToUpper(string(value.Type))), Prompt: value.Prompt, CompetencyIds: nonNilStrings(value.CompetencyIDs), Difficulty: value.Difficulty, TimeLimitSeconds: value.TimeLimitSeconds, Language: model.InterviewLanguage(strings.ToUpper(string(value.Language))), Status: model.QuestionDraftStatus(strings.ToUpper(string(value.Status))), ModelID: value.ModelID, ModelVersion: value.ModelVersion, ReviewNotes: value.ReviewNotes, CreatedAt: value.CreatedAt, ReviewedAt: value.ReviewedAt}
}
