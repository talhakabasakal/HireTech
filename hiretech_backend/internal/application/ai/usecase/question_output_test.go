package usecase

import (
	"testing"

	"github.com/google/uuid"
	interviewModel "github.com/masterfabric-go/masterfabric/internal/domain/interview/model"
)

func TestParseQuestionDraftOutputAcceptsScopedBilingualQuestion(t *testing.T) {
	interviewID := uuid.New()
	raw := []byte(`{"schema_version":"1.0.0","interview_id":"` + interviewID.String() + `","language":"tr","question":{"type":"SYSTEM_DESIGN","prompt":"Bir URL kısaltma servisini tasarlayın.","competency_ids":["system_design","scalability"],"difficulty":4,"time_limit_seconds":900}}`)
	output, err := ParseQuestionDraftOutput(raw, interviewID, interviewModel.LanguageTurkish)
	if err != nil {
		t.Fatal(err)
	}
	if output.Question.Type != "SYSTEM_DESIGN" || output.Question.Difficulty != 4 {
		t.Fatalf("unexpected output: %+v", output)
	}
}

func TestParseQuestionDraftOutputRejectsScopeAndUnknownFields(t *testing.T) {
	interviewID := uuid.New()
	cases := [][]byte{
		[]byte(`{"schema_version":"1.0.0","interview_id":"` + uuid.New().String() + `","language":"en","question":{"type":"CODING","prompt":"x","competency_ids":["coding"],"difficulty":2,"time_limit_seconds":null}}`),
		[]byte(`{"schema_version":"1.0.0","interview_id":"` + interviewID.String() + `","language":"en","unexpected":true,"question":{"type":"CODING","prompt":"x","competency_ids":["coding"],"difficulty":2,"time_limit_seconds":null}}`),
		[]byte(`{"schema_version":"1.0.0","interview_id":"` + interviewID.String() + `","language":"en","question":{"type":"CODING","prompt":"x","competency_ids":["coding","coding"],"difficulty":2,"time_limit_seconds":null}}`),
	}
	for _, raw := range cases {
		if _, err := ParseQuestionDraftOutput(raw, interviewID, interviewModel.LanguageEnglish); err == nil {
			t.Fatalf("expected invalid output for %s", raw)
		}
	}
}
