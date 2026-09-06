package usecase

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"regexp"
	"strings"

	"github.com/google/uuid"
)

var ErrInvalidEvaluatorOutput = errors.New("invalid evaluator output")

type EvaluatorOutput struct {
	SchemaVersion     string               `json:"schema_version"`
	EvaluationID      string               `json:"evaluation_id"`
	InterviewID       string               `json:"interview_id"`
	RubricID          string               `json:"rubric_id"`
	RubricVersion     string               `json:"rubric_version"`
	CriterionScores   []EvaluatorCriterion `json:"criterion_scores"`
	OverallScore      *float64             `json:"overall_score"`
	OverallConfidence EvaluatorConfidence  `json:"overall_confidence"`
	Summary           EvaluatorSummary     `json:"summary"`
	DataQuality       EvaluatorDataQuality `json:"data_quality"`
	IntegrityChecks   EvaluatorIntegrity   `json:"integrity_checks"`
	HumanReview       EvaluatorHumanReview `json:"human_review"`
	ReportDisposition string               `json:"report_disposition"`
}

type EvaluatorCriterion struct {
	CriterionID        string              `json:"criterion_id"`
	Applicable         bool                `json:"applicable"`
	Score              *float64            `json:"score"`
	MaximumScore       float64             `json:"maximum_score"`
	Weight             float64             `json:"weight"`
	Confidence         EvaluatorConfidence `json:"confidence"`
	EvidenceReferences []string            `json:"evidence_references"`
	Rationale          string              `json:"rationale"`
	Limitations        []string            `json:"limitations"`
}

type EvaluatorConfidence struct {
	Score       float64  `json:"score"`
	Level       string   `json:"level"`
	ReasonCodes []string `json:"reason_codes"`
}

type EvaluatorSummary struct {
	Assessment       string              `json:"assessment"`
	Strengths        []EvaluatorEvidence `json:"strengths"`
	DevelopmentAreas []EvaluatorEvidence `json:"development_areas"`
	Limitations      []string            `json:"limitations"`
}

type EvaluatorEvidence struct {
	Statement          string   `json:"statement"`
	EvidenceReferences []string `json:"evidence_references"`
}

type EvaluatorDataQuality struct {
	SufficientForScoring    bool     `json:"sufficient_for_scoring"`
	MissingEvidenceTypes    []string `json:"missing_evidence_types"`
	ContradictionReferences []string `json:"contradiction_references"`
	Notes                   string   `json:"notes"`
}

type EvaluatorIntegrity struct {
	AllEvidenceReferencesResolved bool     `json:"all_evidence_references_resolved"`
	RubricOnlyScoring             bool     `json:"rubric_only_scoring"`
	ProtectedAttributesExcluded   bool     `json:"protected_attributes_excluded"`
	InterviewerOpinionExcluded    bool     `json:"interviewer_opinion_excluded"`
	PromptInjectionDetected       bool     `json:"prompt_injection_detected"`
	IssueCodes                    []string `json:"issue_codes"`
}

type EvaluatorHumanReview struct {
	Required        bool     `json:"required"`
	Urgency         string   `json:"urgency"`
	ReasonCodes     []string `json:"reason_codes"`
	ReviewQuestions []string `json:"review_questions"`
}

// ParseEvaluatorOutput strictly accepts one JSON document and verifies the
// identity/scope fields before the evaluation use case persists anything.
func ParseEvaluatorOutput(raw []byte, expectedEvaluationID, expectedInterviewID uuid.UUID) (*EvaluatorOutput, error) {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	var output EvaluatorOutput
	if err := decoder.Decode(&output); err != nil {
		return nil, fmt.Errorf("%w: JSON decode failed", ErrInvalidEvaluatorOutput)
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return nil, fmt.Errorf("%w: multiple JSON documents", ErrInvalidEvaluatorOutput)
	}
	if output.SchemaVersion != "1.0.0" || output.EvaluationID != expectedEvaluationID.String() || output.InterviewID != expectedInterviewID.String() {
		return nil, fmt.Errorf("%w: evaluation scope or schema mismatch", ErrInvalidEvaluatorOutput)
	}
	if strings.TrimSpace(output.RubricID) == "" || !regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{1,127}$`).MatchString(output.RubricID) || !regexp.MustCompile(`^[0-9]+\.[0-9]+\.[0-9]+$`).MatchString(output.RubricVersion) {
		return nil, fmt.Errorf("%w: rubric metadata is required", ErrInvalidEvaluatorOutput)
	}
	if len(output.CriterionScores) == 0 || len(output.CriterionScores) > 20 {
		return nil, fmt.Errorf("%w: criterion scores are required", ErrInvalidEvaluatorOutput)
	}
	if err := validateConfidence(output.OverallConfidence); err != nil {
		return nil, err
	}
	assessment := strings.TrimSpace(output.Summary.Assessment)
	if assessment == "" || len(assessment) > 4000 {
		return nil, fmt.Errorf("%w: summary assessment is invalid", ErrInvalidEvaluatorOutput)
	}
	if output.IntegrityChecks.PromptInjectionDetected && !contains(output.HumanReview.ReasonCodes, "PROMPT_INJECTION") {
		return nil, fmt.Errorf("%w: prompt injection must trigger human review", ErrInvalidEvaluatorOutput)
	}
	if output.ReportDisposition != "DRAFT" && output.ReportDisposition != "REVIEW_REQUIRED" && output.ReportDisposition != "READY_FOR_HUMAN_DECISION" {
		return nil, fmt.Errorf("%w: invalid report disposition", ErrInvalidEvaluatorOutput)
	}
	if output.HumanReview.Required && output.HumanReview.Urgency == "NONE" {
		return nil, fmt.Errorf("%w: required human review cannot have NONE urgency", ErrInvalidEvaluatorOutput)
	}
	if !output.HumanReview.Required && (output.HumanReview.Urgency != "NONE" || len(output.HumanReview.ReasonCodes) != 0 || len(output.HumanReview.ReviewQuestions) != 0) {
		return nil, fmt.Errorf("%w: optional human review contains review data", ErrInvalidEvaluatorOutput)
	}
	seenCriteria := make(map[string]struct{}, len(output.CriterionScores))
	weightedScore := 0.0
	weightTotal := 0.0
	for _, criterion := range output.CriterionScores {
		if _, exists := seenCriteria[criterion.CriterionID]; exists {
			return nil, fmt.Errorf("%w: duplicate criterion %q", ErrInvalidEvaluatorOutput, criterion.CriterionID)
		}
		seenCriteria[criterion.CriterionID] = struct{}{}
		if strings.TrimSpace(criterion.CriterionID) == "" || criterion.MaximumScore != 4 || criterion.Confidence.Score < 0 || criterion.Confidence.Score > 1 || strings.TrimSpace(criterion.Rationale) == "" {
			return nil, fmt.Errorf("%w: invalid criterion %q", ErrInvalidEvaluatorOutput, criterion.CriterionID)
		}
		if err := validateConfidence(criterion.Confidence); err != nil {
			return nil, err
		}
		if criterion.Applicable {
			if criterion.Score == nil || *criterion.Score < 0 || *criterion.Score > 4 || (*criterion.Score*2) != float64(int(*criterion.Score*2)) || criterion.Weight <= 0 || criterion.Weight > 1 || len(criterion.EvidenceReferences) == 0 {
				return nil, fmt.Errorf("%w: applicable criterion %q is incomplete", ErrInvalidEvaluatorOutput, criterion.CriterionID)
			}
			weightedScore += *criterion.Score * criterion.Weight
			weightTotal += criterion.Weight
		} else if criterion.Score != nil || criterion.Weight != 0 || len(criterion.EvidenceReferences) != 0 || len(criterion.Limitations) == 0 {
			return nil, fmt.Errorf("%w: non-applicable criterion %q is inconsistent", ErrInvalidEvaluatorOutput, criterion.CriterionID)
		}
	}
	if weightTotal > 1.000001 {
		return nil, fmt.Errorf("%w: criterion weights exceed one", ErrInvalidEvaluatorOutput)
	}
	if !output.IntegrityChecks.AllEvidenceReferencesResolved || !output.IntegrityChecks.RubricOnlyScoring || !output.IntegrityChecks.ProtectedAttributesExcluded || !output.IntegrityChecks.InterviewerOpinionExcluded {
		if !output.HumanReview.Required {
			return nil, fmt.Errorf("%w: failed integrity checks must trigger human review", ErrInvalidEvaluatorOutput)
		}
	}
	if !output.DataQuality.SufficientForScoring {
		if output.OverallScore != nil || output.ReportDisposition != "REVIEW_REQUIRED" || !output.HumanReview.Required {
			return nil, fmt.Errorf("%w: insufficient data quality requires a review-only report", ErrInvalidEvaluatorOutput)
		}
	} else if weightTotal == 0 {
		return nil, fmt.Errorf("%w: sufficient report has no applicable weighted criterion", ErrInvalidEvaluatorOutput)
	} else {
		expectedScore := weightedScore / weightTotal / 4 * 100
		if output.OverallScore == nil || math.Abs(*output.OverallScore-expectedScore) > 0.01 {
			return nil, fmt.Errorf("%w: overall score does not match rubric weights", ErrInvalidEvaluatorOutput)
		}
	}
	if output.ReportDisposition == "READY_FOR_HUMAN_DECISION" && !output.HumanReview.Required {
		return nil, fmt.Errorf("%w: human decision disposition requires human review", ErrInvalidEvaluatorOutput)
	}
	return &output, nil
}

func validateConfidence(value EvaluatorConfidence) error {
	if value.Score < 0 || value.Score > 1 || value.Level != "LOW" && value.Level != "MEDIUM" && value.Level != "HIGH" || len(value.ReasonCodes) == 0 {
		return fmt.Errorf("%w: invalid confidence", ErrInvalidEvaluatorOutput)
	}
	return nil
}

func contains(values []string, wanted string) bool {
	for _, value := range values {
		if value == wanted {
			return true
		}
	}
	return false
}
