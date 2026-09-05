package usecase

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	interviewModel "github.com/masterfabric-go/masterfabric/internal/domain/interview/model"
)

var ErrInvalidModelOutput = errors.New("invalid model output")

type QuestionDraftOutput struct {
	SchemaVersion string            `json:"schema_version"`
	InterviewID   string            `json:"interview_id"`
	Language      string            `json:"language"`
	Question      QuestionDraftData `json:"question"`
}

type QuestionDraftData struct {
	Type             string   `json:"type"`
	Prompt           string   `json:"prompt"`
	CompetencyIDs    []string `json:"competency_ids"`
	Difficulty       int      `json:"difficulty"`
	TimeLimitSeconds *int     `json:"time_limit_seconds"`
}

func ParseQuestionDraftOutput(raw []byte, expectedInterviewID uuid.UUID, expectedLanguage interviewModel.InterviewLanguage) (*QuestionDraftOutput, error) {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	var output QuestionDraftOutput
	if err := decoder.Decode(&output); err != nil {
		return nil, fmt.Errorf("%w: JSON decode failed", ErrInvalidModelOutput)
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return nil, fmt.Errorf("%w: multiple JSON documents", ErrInvalidModelOutput)
	}
	if output.SchemaVersion != "1.0.0" {
		return nil, fmt.Errorf("%w: unsupported schema version", ErrInvalidModelOutput)
	}
	if output.InterviewID != expectedInterviewID.String() {
		return nil, fmt.Errorf("%w: interview scope mismatch", ErrInvalidModelOutput)
	}
	if output.Language != string(expectedLanguage) {
		return nil, fmt.Errorf("%w: language mismatch", ErrInvalidModelOutput)
	}
	if strings.TrimSpace(output.Question.Prompt) == "" || len(output.Question.Prompt) > 8000 {
		return nil, fmt.Errorf("%w: invalid prompt", ErrInvalidModelOutput)
	}
	if output.Question.Difficulty < 1 || output.Question.Difficulty > 5 {
		return nil, fmt.Errorf("%w: invalid difficulty", ErrInvalidModelOutput)
	}
	if output.Question.TimeLimitSeconds != nil && (*output.Question.TimeLimitSeconds < 30 || *output.Question.TimeLimitSeconds > 7200) {
		return nil, fmt.Errorf("%w: invalid time limit", ErrInvalidModelOutput)
	}
	if len(output.Question.CompetencyIDs) == 0 || len(output.Question.CompetencyIDs) > 8 {
		return nil, fmt.Errorf("%w: invalid competencies", ErrInvalidModelOutput)
	}
	competencyPattern := regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{1,63}$`)
	seen := make(map[string]bool, len(output.Question.CompetencyIDs))
	for index, competency := range output.Question.CompetencyIDs {
		normalized := strings.TrimSpace(competency)
		if normalized != competency || !competencyPattern.MatchString(competency) || seen[competency] {
			return nil, fmt.Errorf("%w: invalid or duplicate competency at index %d", ErrInvalidModelOutput, index)
		}
		seen[competency] = true
	}
	switch output.Question.Type {
	case "TECHNICAL_DISCUSSION", "CODING", "SYSTEM_DESIGN", "DEBUGGING":
	default:
		return nil, fmt.Errorf("%w: invalid question type", ErrInvalidModelOutput)
	}
	return &output, nil
}

func (o *QuestionDraftOutput) QuestionModel(id, organizationID, interviewID uuid.UUID, createdAt time.Time) *interviewModel.Question {
	return &interviewModel.Question{
		ID: id, OrganizationID: organizationID, InterviewID: interviewID,
		Type: interviewModel.QuestionType(strings.ToLower(o.Question.Type)), Prompt: strings.TrimSpace(o.Question.Prompt),
		CompetencyIDs: append([]string(nil), o.Question.CompetencyIDs...), Difficulty: o.Question.Difficulty,
		TimeLimitSeconds: o.Question.TimeLimitSeconds, CreatedAt: createdAt,
	}
}
