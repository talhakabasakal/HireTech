package sandbox

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	executionModel "github.com/masterfabric-go/masterfabric/internal/domain/execution/model"
)

func validRequest() executionModel.Request {
	return executionModel.Request{
		OrganizationID: uuid.New(), InterviewID: uuid.New(), AnswerID: uuid.New(),
		Language: executionModel.LanguageGo, Source: "package main\nfunc main() {}", Timeout: time.Second,
	}
}

func validResult() executionModel.Result {
	return executionModel.Result{ExecutionID: uuid.New(), Status: executionModel.StatusSucceeded, RunnerID: "runner-1", KeyID: "key-1", Signature: "signed-result", ResultDigest: "sha256:result"}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestClientRejectsUnsafeRemoteEndpoint(t *testing.T) {
	_, err := NewClient(nil, "http://sandbox.example.com", "")
	require.Error(t, err)
	_, err = NewClient(nil, "https://sandbox.example.com?redirect=1", "")
	require.Error(t, err)
}

func TestClientRejectsRedirectedRunnerResponse(t *testing.T) {
	redirected := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/executions" {
			http.Redirect(w, r, "/redirected", http.StatusFound)
			return
		}
		redirected = true
	}))
	defer server.Close()

	client, err := NewClient(nil, server.URL, "")
	require.NoError(t, err)
	_, err = client.Execute(context.Background(), validRequest())
	require.Error(t, err)
	require.Contains(t, err.Error(), "HTTP 302")
	require.False(t, redirected)
}

func TestClientRejectsOversizedRunnerResponse(t *testing.T) {
	transport := roundTripFunc(func(*http.Request) (*http.Response, error) {
		body := strings.Repeat("x", int(executionModel.MaxOutputBytes+64*1024+1))
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
	})
	client, err := NewClient(&http.Client{Transport: transport}, "http://127.0.0.1:9090", "")
	require.NoError(t, err)
	_, err = client.Execute(context.Background(), validRequest())
	require.EqualError(t, err, "sandbox response is too large")
}

func TestClientExecutesOnlyThroughBoundedSignedSandboxResponse(t *testing.T) {
	result := validResult()
	transport := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		require.Equal(t, http.MethodPost, r.Method)
		require.Equal(t, "Bearer sandbox-token", r.Header.Get("Authorization"))
		var request executionModel.Request
		require.NoError(t, json.NewDecoder(r.Body).Decode(&request))
		require.Equal(t, executionModel.LanguageGo, request.Language)
		body, err := json.Marshal(result)
		require.NoError(t, err)
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(string(body))), Header: make(http.Header)}, nil
	})

	client, err := NewClient(&http.Client{Transport: transport}, "http://127.0.0.1:9090", "sandbox-token", func(executionModel.Result) error { return nil })
	require.NoError(t, err)
	got, err := client.Execute(context.Background(), validRequest())
	require.NoError(t, err)
	require.Equal(t, result.ExecutionID, got.ExecutionID)
}

func TestClientFailsClosedOnMissingResultProvenance(t *testing.T) {
	transport := roundTripFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(`{"execution_id":"` + uuid.NewString() + `","status":"succeeded"}`)), Header: make(http.Header)}, nil
	})
	client, err := NewClient(&http.Client{Transport: transport}, "http://127.0.0.1:9090", "")
	require.NoError(t, err)
	_, err = client.Execute(context.Background(), validRequest())
	require.Error(t, err)
	require.True(t, strings.Contains(err.Error(), "provenance"))
}

func TestClientFailsClosedOnMissingResultDigest(t *testing.T) {
	result := validResult()
	result.ResultDigest = ""
	transport := roundTripFunc(func(*http.Request) (*http.Response, error) {
		body, err := json.Marshal(result)
		require.NoError(t, err)
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(string(body))), Header: make(http.Header)}, nil
	})
	client, err := NewClient(&http.Client{Transport: transport}, "http://127.0.0.1:9090", "")
	require.NoError(t, err)
	_, err = client.Execute(context.Background(), validRequest())
	require.EqualError(t, err, "sandbox result provenance is incomplete")
}

func TestRequestValidationRejectsOversizedOrUnboundedExecution(t *testing.T) {
	request := validRequest()
	request.Source = strings.Repeat("x", executionModel.MaxSourceBytes+1)
	require.Error(t, request.Validate())
	request = validRequest()
	request.Timeout = executionModel.MaxTimeout + time.Second
	require.Error(t, request.Validate())
}
