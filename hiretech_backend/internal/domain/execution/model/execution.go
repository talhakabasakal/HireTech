package model

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

const (
	MaxSourceBytes = 256 * 1024
	MaxOutputBytes = 512 * 1024
	MaxTimeout     = 30 * time.Second
)

type Language string

const (
	LanguageGo         Language = "go"
	LanguageJavaScript Language = "javascript"
	LanguageTypeScript Language = "typescript"
	LanguagePython     Language = "python"
)

type Status string

const (
	StatusSucceeded Status = "succeeded"
	StatusFailed    Status = "failed"
	StatusTimedOut  Status = "timed_out"
	StatusRejected  Status = "rejected"
)

type Request struct {
	OrganizationID uuid.UUID     `json:"organization_id"`
	InterviewID    uuid.UUID     `json:"interview_id"`
	AnswerID       uuid.UUID     `json:"answer_id"`
	Language       Language      `json:"language"`
	Source         string        `json:"source"`
	Tests          string        `json:"tests,omitempty"`
	Timeout        time.Duration `json:"timeout"`
}

func (r Request) Validate() error {
	if r.OrganizationID == uuid.Nil || r.InterviewID == uuid.Nil || r.AnswerID == uuid.Nil {
		return errors.New("execution scope is required")
	}
	switch r.Language {
	case LanguageGo, LanguageJavaScript, LanguageTypeScript, LanguagePython:
	default:
		return fmt.Errorf("unsupported execution language: %s", r.Language)
	}
	if strings.TrimSpace(r.Source) == "" {
		return errors.New("execution source is required")
	}
	if len(r.Source) > MaxSourceBytes || len(r.Tests) > MaxSourceBytes {
		return errors.New("execution source exceeds the maximum size")
	}
	if r.Timeout <= 0 || r.Timeout > MaxTimeout {
		return errors.New("execution timeout is outside the allowed range")
	}
	return nil
}

type Result struct {
	ExecutionID  uuid.UUID     `json:"execution_id"`
	Status       Status        `json:"status"`
	ExitCode     int           `json:"exit_code"`
	Stdout       string        `json:"stdout,omitempty"`
	Stderr       string        `json:"stderr,omitempty"`
	Duration     time.Duration `json:"duration"`
	ResultDigest string        `json:"result_digest"`
	RunnerID     string        `json:"runner_id"`
	KeyID        string        `json:"key_id"`
	Signature    string        `json:"signature"`
}

func (r Result) Validate() error {
	if r.ExecutionID == uuid.Nil || strings.TrimSpace(r.RunnerID) == "" || strings.TrimSpace(r.KeyID) == "" || strings.TrimSpace(r.Signature) == "" {
		return errors.New("sandbox result provenance is incomplete")
	}
	switch r.Status {
	case StatusSucceeded, StatusFailed, StatusTimedOut, StatusRejected:
	default:
		return fmt.Errorf("unsupported sandbox result status: %s", r.Status)
	}
	if len(r.Stdout) > MaxOutputBytes || len(r.Stderr) > MaxOutputBytes {
		return errors.New("sandbox output exceeds the maximum size")
	}
	return nil
}
