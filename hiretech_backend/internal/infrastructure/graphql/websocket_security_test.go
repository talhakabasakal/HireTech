package graphql

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestWebsocketOriginAllowedRequiresConfiguredBrowserOrigin(t *testing.T) {
	allow := websocketOriginAllowed([]string{"https://app.example.com"})

	request := httptest.NewRequest("GET", "http://server/graphql", nil)
	request.Header.Set("Origin", "https://app.example.com")
	require.True(t, allow(request))

	request.Header.Set("Origin", "https://attacker.example.com")
	require.False(t, allow(request))

	request.Header.Del("Origin")
	require.True(t, allow(request))
}

func TestPersistedOperationGateAcceptsJSONAndRejectsUnallowlistedHash(t *testing.T) {
	query := "subscription InterviewUpdates { interviewUpdated(interviewId: \"00000000-0000-0000-0000-000000000001\") { interviewId } }"
	digest := sha256.Sum256([]byte(query))
	hash := hex.EncodeToString(digest[:])
	extensions := map[string]any{"persistedQuery": map[string]any{"version": float64(1), "sha256Hash": hash}}

	require.NoError(t, validatePersistedOperationParams(query, extensions, map[string]struct{}{hash: {}}))
	require.Error(t, validatePersistedOperationParams(query, extensions, map[string]struct{}{}))
}
