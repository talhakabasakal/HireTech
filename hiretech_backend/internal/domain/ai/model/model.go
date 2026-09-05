package model

import "encoding/json"

type Role string

const (
	RoleInterviewer Role = "interviewer"
	RoleEvaluator   Role = "evaluator"
)

type Language string

const (
	LanguageTurkish Language = "tr"
	LanguageEnglish Language = "en"
)

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type CompletionRequest struct {
	Role         Role
	Language     Language
	ModelID      string
	ModelVersion string
	Messages     []Message
	JSONSchema   json.RawMessage
	MaxTokens    int
	Temperature  *float32
}

type CompletionResponse struct {
	Content      string
	ModelID      string
	ModelVersion string
	PromptTokens int
	OutputTokens int
}
