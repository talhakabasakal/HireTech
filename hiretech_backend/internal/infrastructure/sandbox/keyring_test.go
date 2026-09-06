package sandbox

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	executionModel "github.com/masterfabric-go/masterfabric/internal/domain/execution/model"
)

func signedResult(t *testing.T) (executionModel.Result, ed25519.PublicKey) {
	t.Helper()
	public, private, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	result := validResult()
	result.ExecutionID = uuid.New()
	result.KeyID = "runner-key-2026-01"
	digest, err := result.ExpectedDigest()
	require.NoError(t, err)
	result.ResultDigest = digest
	payload, err := result.SigningPayload()
	require.NoError(t, err)
	result.Signature = base64.RawStdEncoding.EncodeToString(ed25519.Sign(private, payload))
	return result, public
}

func TestKeyRingVerifiesDigestAndSignature(t *testing.T) {
	result, public := signedResult(t)
	ring, err := NewKeyRing(map[string]ed25519.PublicKey{result.KeyID: public})
	require.NoError(t, err)
	require.NoError(t, ring.Verify(result))
}

func TestKeyRingRejectsTamperedResult(t *testing.T) {
	result, public := signedResult(t)
	ring, err := NewKeyRing(map[string]ed25519.PublicKey{result.KeyID: public})
	require.NoError(t, err)
	result.Stdout = "tampered"
	require.ErrorIs(t, ring.Verify(result), ErrDigestMismatch)
}

func TestKeyRingRejectsUnknownKey(t *testing.T) {
	result, _ := signedResult(t)
	otherPublic, _, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	ring, err := NewKeyRing(map[string]ed25519.PublicKey{"other": otherPublic})
	require.NoError(t, err)
	require.ErrorIs(t, ring.Verify(result), ErrUnknownRunnerKey)
}
