package middleware

import (
	"context"
	"sync"
	"time"

	"github.com/google/uuid"
)

// RequestAudit contains only allowlisted request metadata. It intentionally has
// no request body, variables, authorization header, or response data fields.
type RequestAudit struct {
	mu             sync.RWMutex
	RequestID      string
	OperationName  string
	OperationType  string
	Outcome        string
	ErrorCode      string
	Complexity     int
	StartedAt      time.Time
	Duration       time.Duration
	UserID         uuid.UUID
	OrganizationID uuid.UUID
}

type requestAuditKey struct{}

func WithRequestAudit(ctx context.Context, audit *RequestAudit) context.Context {
	return context.WithValue(ctx, requestAuditKey{}, audit)
}

func RequestAuditFromContext(ctx context.Context) *RequestAudit {
	audit, _ := ctx.Value(requestAuditKey{}).(*RequestAudit)
	return audit
}

func (a *RequestAudit) SetRequestID(requestID string) {
	if a == nil {
		return
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	a.RequestID = requestID
}

func (a *RequestAudit) SetActor(userID, organizationID uuid.UUID) {
	if a == nil {
		return
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	a.UserID = userID
	a.OrganizationID = organizationID
}

func (a *RequestAudit) SetOperation(name, operationType string, complexity int) {
	if a == nil {
		return
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	a.OperationName = name
	a.OperationType = operationType
	a.Complexity = complexity
}

func (a *RequestAudit) SetResult(outcome, errorCode string, duration time.Duration) {
	if a == nil {
		return
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	a.Outcome = outcome
	a.ErrorCode = errorCode
	a.Duration = duration
}

func (a *RequestAudit) Snapshot() RequestAudit {
	if a == nil {
		return RequestAudit{}
	}
	a.mu.RLock()
	defer a.mu.RUnlock()
	return RequestAudit{
		RequestID: a.RequestID, OperationName: a.OperationName, OperationType: a.OperationType,
		Outcome: a.Outcome, ErrorCode: a.ErrorCode, Complexity: a.Complexity,
		StartedAt: a.StartedAt, Duration: a.Duration, UserID: a.UserID,
		OrganizationID: a.OrganizationID,
	}
}
