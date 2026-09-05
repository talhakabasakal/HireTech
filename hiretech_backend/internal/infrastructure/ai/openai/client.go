package openai

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	aiModel "github.com/masterfabric-go/masterfabric/internal/domain/ai/model"
)

var ErrInvalidRequest = errors.New("invalid AI completion request")

type Client struct {
	httpClient   *http.Client
	endpoint     string
	apiKey       string
	modelID      string
	modelVersion string
	maxBodyBytes int64
}

func NewClient(httpClient *http.Client, baseURL, apiKey, modelID, modelVersion string) (*Client, error) {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 30 * time.Second}
	}
	endpoint, err := completionEndpoint(baseURL)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(modelID) == "" {
		return nil, fmt.Errorf("model ID is required")
	}
	return &Client{httpClient: httpClient, endpoint: endpoint, apiKey: apiKey, modelID: modelID, modelVersion: modelVersion, maxBodyBytes: 1 << 20}, nil
}

// SetMaxResponseBytes bounds provider responses for this client.
func (c *Client) SetMaxResponseBytes(limit int64) {
	if limit > 0 {
		c.maxBodyBytes = limit
	}
}

func (c *Client) Complete(ctx context.Context, request aiModel.CompletionRequest) (aiModel.CompletionResponse, error) {
	if len(request.Messages) == 0 || request.Role == "" {
		return aiModel.CompletionResponse{}, ErrInvalidRequest
	}
	if request.Role != aiModel.RoleInterviewer && request.Role != aiModel.RoleEvaluator {
		return aiModel.CompletionResponse{}, ErrInvalidRequest
	}
	if request.Language != aiModel.LanguageTurkish && request.Language != aiModel.LanguageEnglish {
		return aiModel.CompletionResponse{}, ErrInvalidRequest
	}
	modelID := request.ModelID
	if modelID == "" {
		modelID = c.modelID
	}
	if modelID == "" {
		return aiModel.CompletionResponse{}, ErrInvalidRequest
	}
	maxTokens := request.MaxTokens
	if maxTokens <= 0 || maxTokens > 32768 {
		maxTokens = 2048
	}
	payload := chatRequest{Model: modelID, Messages: request.Messages, MaxTokens: maxTokens, Temperature: request.Temperature}
	if len(request.JSONSchema) > 0 {
		var schema any
		if err := json.Unmarshal(request.JSONSchema, &schema); err != nil {
			return aiModel.CompletionResponse{}, fmt.Errorf("invalid JSON schema: %w", ErrInvalidRequest)
		}
		payload.ResponseFormat = &responseFormat{Type: "json_schema", JSONSchema: &jsonSchema{Name: string(request.Role), Strict: true, Schema: schema}}
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return aiModel.CompletionResponse{}, fmt.Errorf("marshal AI request: %w", err)
	}
	httpRequest, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint, bytes.NewReader(body))
	if err != nil {
		return aiModel.CompletionResponse{}, fmt.Errorf("create AI request: %w", err)
	}
	httpRequest.Header.Set("Content-Type", "application/json")
	if c.apiKey != "" {
		httpRequest.Header.Set("Authorization", "Bearer "+c.apiKey)
	}
	response, err := c.httpClient.Do(httpRequest)
	if err != nil {
		return aiModel.CompletionResponse{}, fmt.Errorf("AI provider request failed: %w", err)
	}
	defer response.Body.Close()
	limited := io.LimitReader(response.Body, c.maxBodyBytes+1)
	responseBody, err := io.ReadAll(limited)
	if err != nil {
		return aiModel.CompletionResponse{}, fmt.Errorf("read AI provider response: %w", err)
	}
	if int64(len(responseBody)) > c.maxBodyBytes {
		return aiModel.CompletionResponse{}, errors.New("AI provider response too large")
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return aiModel.CompletionResponse{}, fmt.Errorf("AI provider returned HTTP %d", response.StatusCode)
	}
	var decoded chatResponse
	if err := json.Unmarshal(responseBody, &decoded); err != nil {
		return aiModel.CompletionResponse{}, errors.New("AI provider response was not valid JSON")
	}
	if len(decoded.Choices) == 0 || decoded.Choices[0].Message.Content == "" {
		return aiModel.CompletionResponse{}, errors.New("AI provider response had no completion")
	}
	return aiModel.CompletionResponse{
		Content: decoded.Choices[0].Message.Content, ModelID: modelID, ModelVersion: c.modelVersion,
		PromptTokens: decoded.Usage.PromptTokens, OutputTokens: decoded.Usage.CompletionTokens,
	}, nil
}

func completionEndpoint(baseURL string) (string, error) {
	parsed, err := url.Parse(strings.TrimSpace(baseURL))
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return "", fmt.Errorf("invalid AI endpoint")
	}
	parsed.Path = strings.TrimRight(parsed.Path, "/")
	if !strings.HasSuffix(parsed.Path, "/chat/completions") {
		if !strings.HasSuffix(parsed.Path, "/v1") {
			parsed.Path += "/v1"
		}
		parsed.Path += "/chat/completions"
	}
	return parsed.String(), nil
}

type chatRequest struct {
	Model          string            `json:"model"`
	Messages       []aiModel.Message `json:"messages"`
	MaxTokens      int               `json:"max_tokens"`
	Temperature    *float32          `json:"temperature,omitempty"`
	ResponseFormat *responseFormat   `json:"response_format,omitempty"`
}

type responseFormat struct {
	Type       string      `json:"type"`
	JSONSchema *jsonSchema `json:"json_schema,omitempty"`
}

type jsonSchema struct {
	Name   string `json:"name"`
	Strict bool   `json:"strict"`
	Schema any    `json:"schema"`
}

type chatResponse struct {
	ID      string `json:"id"`
	Model   string `json:"model"`
	Choices []struct {
		Message aiModel.Message `json:"message"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
	} `json:"usage"`
}
