package resolver

import (
	"strings"

	"github.com/masterfabric-go/masterfabric/graph/model"
	evaluationModel "github.com/masterfabric-go/masterfabric/internal/domain/evaluation/model"
)

func evaluationReportGraphQL(value *evaluationModel.EvaluationReport) *model.EvaluationReport {
	if value == nil {
		return nil
	}
	criteria := make([]*model.CriterionScore, 0, len(value.CriterionScores))
	for _, criterion := range value.CriterionScores {
		criteria = append(criteria, &model.CriterionScore{
			CriterionID: criterion.CriterionID, Applicable: criterion.Applicable, Score: criterion.Score,
			MaximumScore: criterion.MaximumScore, Weight: criterion.Weight, Confidence: criterion.Confidence,
			EvidenceReferences: nonNilStrings(criterion.EvidenceReferences), Rationale: criterion.Rationale,
			Limitations: nonNilStrings(criterion.Limitations),
		})
	}
	return &model.EvaluationReport{
		ID: value.ID, InterviewID: value.InterviewID,
		Status:   model.EvaluationStatus(strings.ToUpper(string(value.Status))),
		RubricID: value.RubricID, RubricVersion: value.RubricVersion,
		EvaluatorConfigurationVersion: value.EvaluatorConfigurationVersion,
		OverallScore:                  value.OverallScore, OverallConfidence: value.OverallConfidence,
		CriterionScores: criteria, Strengths: nonNilStrings(value.Strengths), Gaps: nonNilStrings(value.Gaps),
		EvidenceReferences: nonNilStrings(value.EvidenceReferences), Limitations: nonNilStrings(value.Limitations),
		HumanReview: humanReviewGraphQL(value.HumanReview), GeneratedAt: value.GeneratedAt, PublishedAt: value.PublishedAt,
		CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt,
	}
}

func humanReviewGraphQL(value *evaluationModel.HumanReview) *model.HumanReview {
	if value == nil {
		return nil
	}
	return &model.HumanReview{
		Required: value.Required, Urgency: model.HumanReviewUrgency(strings.ToUpper(string(value.Urgency))),
		ReasonCodes: nonNilStrings(value.ReasonCodes), Status: string(value.Status), ReviewerUserID: value.ReviewerUserID,
		Notes: value.Notes, CompletedAt: value.CompletedAt,
	}
}
