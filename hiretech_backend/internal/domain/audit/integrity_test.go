package audit

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/masterfabric-go/masterfabric/internal/domain/audit/model"
)

func TestEntryHashChangesWhenAuditEntryChanges(t *testing.T) {
	entry := model.AuditLog{ID: uuid.New(), OrganizationID: uuid.New(), Action: "login", ResourceType: "security", ResourceID: "user", CreatedAt: time.Unix(10, 0).UTC(), Metadata: []byte(`{"result":"success"}`)}
	one, err := EntryHash(entry)
	require.NoError(t, err)
	entry.Metadata = []byte(`{"result":"failure"}`)
	two, err := EntryHash(entry)
	require.NoError(t, err)
	require.NotEqual(t, one, two)
}
