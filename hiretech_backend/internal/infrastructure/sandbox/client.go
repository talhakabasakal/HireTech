package sandbox

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

	executionModel "github.com/masterfabric-go/masterfabric/internal/domain/execution/model"
)

type Client struct {
	httpClient   *http.Client
	endpoint     string
	authToken    string
	maxBodyBytes int64
	verifyResult func(executionModel.Result) error
}

// ResultVerifier must verify the runner signature against a trusted key ring.
// No default verifier is installed: accepting a non-empty signature string is
// not sufficient provenance for production evidence.
type ResultVerifier func(executionModel.Result) error

func NewClient(httpClient *http.Client, endpoint, authToken string, verifiers ...ResultVerifier) (*Client, error) {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: executionModel.MaxTimeout}
	}
	validatedEndpoint, err := executionEndpoint(endpoint)
	if err != nil {
		return nil, err
	}
	client := *httpClient
	// A sandbox response must not redirect to an unreviewed host or path.
	client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	var verifyResult ResultVerifier
	if len(verifiers) > 0 {
		verifyResult = verifiers[0]
	}
	return &Client{httpClient: &client, endpoint: validatedEndpoint, authToken: strings.TrimSpace(authToken), maxBodyBytes: executionModel.MaxOutputBytes + 64*1024, verifyResult: verifyResult}, nil
}

func (c *Client) Execute(ctx context.Context, request executionModel.Request) (executionModel.Result, error) {
	if c == nil {
		return executionModel.Result{}, errors.New("sandbox client is not configured")
	}
	if err := request.Validate(); err != nil {
		return executionModel.Result{}, err
	}
	body, err := json.Marshal(request)
	if err != nil {
		return executionModel.Result{}, fmt.Errorf("marshal sandbox request: %w", err)
	}
	requestContext, cancel := context.WithTimeout(ctx, request.Timeout)
	defer cancel()
	httpRequest, err := http.NewRequestWithContext(requestContext, http.MethodPost, c.endpoint, bytes.NewReader(body))
	if err != nil {
		return executionModel.Result{}, fmt.Errorf("create sandbox request: %w", err)
	}
	httpRequest.Header.Set("Content-Type", "application/json")
	if c.authToken != "" {
		httpRequest.Header.Set("Authorization", "Bearer "+c.authToken)
	}
	response, err := c.httpClient.Do(httpRequest)
	if err != nil {
		return executionModel.Result{}, fmt.Errorf("sandbox request failed: %w", err)
	}
	defer response.Body.Close()
	responseBody, err := io.ReadAll(io.LimitReader(response.Body, c.maxBodyBytes+1))
	if err != nil {
		return executionModel.Result{}, fmt.Errorf("read sandbox response: %w", err)
	}
	if int64(len(responseBody)) > c.maxBodyBytes {
		return executionModel.Result{}, errors.New("sandbox response is too large")
	}
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return executionModel.Result{}, fmt.Errorf("sandbox returned HTTP %d", response.StatusCode)
	}
	var result executionModel.Result
	if err := json.Unmarshal(responseBody, &result); err != nil {
		return executionModel.Result{}, errors.New("sandbox response was not valid JSON")
	}
	if len(result.Stdout) > executionModel.MaxOutputBytes || len(result.Stderr) > executionModel.MaxOutputBytes {
		return executionModel.Result{}, errors.New("sandbox output exceeds the maximum size")
	}
	if err := result.Validate(); err != nil {
		return executionModel.Result{}, err
	}
	if c.verifyResult == nil {
		return executionModel.Result{}, errors.New("sandbox result verifier is not configured")
	}
	if err := c.verifyResult(result); err != nil {
		return executionModel.Result{}, errors.New("sandbox result signature verification failed")
	}
	return result, nil
}

func executionEndpoint(raw string) (string, error) {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || parsed.Scheme == "" || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return "", errors.New("invalid sandbox endpoint")
	}
	if parsed.User != nil || parsed.Fragment != "" || parsed.RawQuery != "" {
		return "", errors.New("sandbox endpoint must not contain credentials, query, or fragment")
	}
	if parsed.Scheme != "https" && !isLoopbackHost(parsed.Hostname()) {
		return "", errors.New("remote sandbox endpoint must use HTTPS")
	}
	parsed.Path = strings.TrimRight(parsed.Path, "/")
	if !strings.HasSuffix(parsed.Path, "/v1/executions") {
		parsed.Path += "/v1/executions"
	}
	return parsed.String(), nil
}

func isLoopbackHost(host string) bool {
	switch strings.ToLower(strings.TrimSpace(host)) {
	case "localhost", "127.0.0.1", "[::1]", "::1":
		return true
	default:
		return false
	}
}

var _ interface {
	Execute(context.Context, executionModel.Request) (executionModel.Result, error)
} = (*Client)(nil)
