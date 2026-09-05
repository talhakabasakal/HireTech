package authcontext

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// TokenClass identifies the authority granted by an access token.
type TokenClass string

const (
	TokenClassBootstrap          TokenClass = "bootstrap"
	TokenClassTenant             TokenClass = "tenant"
	TokenClassCandidateInterview TokenClass = "candidate_interview"
)

// ActorContext is the trusted, immutable identity and tenant context for a request.
// It is created only after the token signature and registered claims are verified.
type ActorContext struct {
	RequestID             string
	TraceID               string
	UserID                uuid.UUID
	Email                 string
	SessionID             string
	TokenClass            TokenClass
	OrganizationID        uuid.UUID
	MembershipID          uuid.UUID
	DeviceID              uuid.UUID
	InterviewID           uuid.UUID
	AuthenticationMethods []string
	AuthenticationTime    time.Time
	Permissions           []string
}

func (a ActorContext) IsAuthenticated() bool { return a.UserID != uuid.Nil }

func (a ActorContext) HasOrganization() bool { return a.OrganizationID != uuid.Nil }

func (a ActorContext) IsBootstrap() bool {
	return a.TokenClass == "" || a.TokenClass == TokenClassBootstrap
}

func (a ActorContext) HasAuthenticationMethod(method string) bool {
	for _, value := range a.AuthenticationMethods {
		if value == method {
			return true
		}
	}
	return false
}

type contextKey struct{}

func WithActor(ctx context.Context, actor ActorContext) context.Context {
	return context.WithValue(ctx, contextKey{}, actor)
}

func FromContext(ctx context.Context) (ActorContext, bool) {
	actor, ok := ctx.Value(contextKey{}).(ActorContext)
	return actor, ok && actor.IsAuthenticated()
}
