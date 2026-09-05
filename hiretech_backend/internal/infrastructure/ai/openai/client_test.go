package openai

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	aiModel "github.com/masterfabric-go/masterfabric/internal/domain/ai/model"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) { return f(request) }

func TestClientUsesOpenAICompatibleEndpointAndStrictSchema(t *testing.T) {
	var received struct {
		Authorization string
		Path          string
		Payload       map[string]any
	}
	httpClient := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		received.Authorization = request.Header.Get("Authorization")
		received.Path = request.URL.Path
		_ = json.NewDecoder(request.Body).Decode(&received.Payload)
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(strings.NewReader(`{"model":"Qwen/Qwen3-4B","choices":[{"message":{"role":"assistant","content":"{\"score\":2}"}}],"usage":{"prompt_tokens":12,"completion_tokens":7}}`)),
		}, nil
	})}

	client, err := NewClient(httpClient, "https://provider.example/v1/", "secret", "Qwen/Qwen3-4B", "base-1")
	if err != nil {
		t.Fatal(err)
	}
	response, err := client.Complete(context.Background(), aiModel.CompletionRequest{
		Role: aiModel.RoleEvaluator, Language: aiModel.LanguageTurkish,
		Messages:   []aiModel.Message{{Role: "system", Content: "Return JSON only."}},
		JSONSchema: json.RawMessage(`{"type":"object"}`), MaxTokens: 40000,
	})
	if err != nil {
		t.Fatal(err)
	}
	if response.Content != `{"score":2}` || response.ModelVersion != "base-1" || response.PromptTokens != 12 || response.OutputTokens != 7 {
		t.Fatalf("unexpected response: %+v", response)
	}
	if received.Authorization != "Bearer secret" || received.Path != "/v1/chat/completions" {
		t.Fatalf("unexpected request metadata: %+v", received)
	}
	if received.Payload["model"] != "Qwen/Qwen3-4B" || received.Payload["max_tokens"] != float64(2048) {
		t.Fatalf("unexpected request payload: %+v", received.Payload)
	}
	format, ok := received.Payload["response_format"].(map[string]any)
	if !ok || format["type"] != "json_schema" {
		t.Fatalf("strict response format missing: %+v", received.Payload)
	}
}

func TestClientRejectsInvalidRoleAndLanguage(t *testing.T) {
	client, err := NewClient(&http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusNotFound, Body: io.NopCloser(strings.NewReader("not found"))}, nil
	})}, "https://provider.example", "", "model", "version")
	if err != nil {
		t.Fatal(err)
	}
	for _, request := range []aiModel.CompletionRequest{
		{Role: "unknown", Language: aiModel.LanguageEnglish, Messages: []aiModel.Message{{Role: "user", Content: "x"}}},
		{Role: aiModel.RoleEvaluator, Language: "fr", Messages: []aiModel.Message{{Role: "user", Content: "x"}}},
	} {
		if _, err := client.Complete(context.Background(), request); err != ErrInvalidRequest {
			t.Fatalf("expected invalid request, got %v", err)
		}
	}
}
