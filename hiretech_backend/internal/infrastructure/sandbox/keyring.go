package sandbox

import (
	"crypto/ed25519"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	executionModel "github.com/masterfabric-go/masterfabric/internal/domain/execution/model"
)

var (
	ErrUnknownRunnerKey = errors.New("sandbox runner key is not trusted")
	ErrInvalidSignature = errors.New("sandbox runner signature is invalid")
	ErrDigestMismatch   = errors.New("sandbox result digest is invalid")
)

// KeyRing contains only public runner keys. It is immutable after creation so
// a request cannot change trust roots while a result is being verified.
type KeyRing struct {
	keys map[string]ed25519.PublicKey
}

// NewKeyRing validates every key before returning an immutable verifier.
func NewKeyRing(keys map[string]ed25519.PublicKey) (*KeyRing, error) {
	if len(keys) == 0 {
		return nil, errors.New("sandbox key ring must contain at least one key")
	}
	copyKeys := make(map[string]ed25519.PublicKey, len(keys))
	for id, key := range keys {
		id = strings.TrimSpace(id)
		if id == "" || len(key) != ed25519.PublicKeySize {
			return nil, fmt.Errorf("invalid sandbox public key %q", id)
		}
		copyKeys[id] = append(ed25519.PublicKey(nil), key...)
	}
	return &KeyRing{keys: copyKeys}, nil
}

// NewKeyRingFromJSON loads a non-secret key-ring document. The accepted shape
// is {"key-id":"base64-ed25519-public-key"}. Secret/private keys are not
// accepted by this parser.
func NewKeyRingFromJSON(raw []byte) (*KeyRing, error) {
	var encoded map[string]string
	if err := json.Unmarshal(raw, &encoded); err != nil {
		return nil, fmt.Errorf("decode sandbox key ring: %w", err)
	}
	keys := make(map[string]ed25519.PublicKey, len(encoded))
	for id, value := range encoded {
		decoded, err := base64.RawStdEncoding.DecodeString(strings.TrimSpace(value))
		if err != nil {
			decoded, err = base64.StdEncoding.DecodeString(strings.TrimSpace(value))
		}
		if err != nil {
			return nil, fmt.Errorf("decode sandbox public key %q: %w", id, err)
		}
		keys[id] = ed25519.PublicKey(decoded)
	}
	return NewKeyRing(keys)
}

// Verify implements ResultVerifier. The digest and signature are checked
// independently; accepting either one alone would allow forged evidence.
func (k *KeyRing) Verify(result executionModel.Result) error {
	if k == nil {
		return errors.New("sandbox key ring is not configured")
	}
	key, ok := k.keys[strings.TrimSpace(result.KeyID)]
	if !ok {
		return ErrUnknownRunnerKey
	}
	expectedDigest, err := result.ExpectedDigest()
	if err != nil {
		return fmt.Errorf("build sandbox signing payload: %w", err)
	}
	if subtle.ConstantTimeCompare([]byte(expectedDigest), []byte(result.ResultDigest)) != 1 {
		return ErrDigestMismatch
	}
	signature, err := base64.RawStdEncoding.DecodeString(strings.TrimSpace(result.Signature))
	if err != nil {
		signature, err = base64.StdEncoding.DecodeString(strings.TrimSpace(result.Signature))
	}
	if err != nil || len(signature) != ed25519.SignatureSize {
		return ErrInvalidSignature
	}
	payload, err := result.SigningPayload()
	if err != nil || !ed25519.Verify(key, payload, signature) {
		return ErrInvalidSignature
	}
	return nil
}

var _ ResultVerifier = (*KeyRing)(nil).Verify
