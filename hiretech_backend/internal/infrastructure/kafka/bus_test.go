package kafka

import (
	"testing"

	"github.com/stretchr/testify/assert"

	interviewEvent "github.com/masterfabric-go/masterfabric/internal/domain/interview/event"
	"github.com/masterfabric-go/masterfabric/internal/shared/events"
)

func TestDeriveEventTypeUsesExplicitDomainContract(t *testing.T) {
	assert.Equal(t, events.EventTypeInterviewChanged, deriveEventType(interviewEvent.Changed{}))
}
