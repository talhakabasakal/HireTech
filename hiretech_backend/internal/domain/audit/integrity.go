package audit

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"

	"github.com/masterfabric-go/masterfabric/internal/domain/audit/model"
)

// EntryHash computes the tamper-evident hash for one audit entry. The hash is
// chained per organization by the storage adapter; metadata is included as
// bytes so redaction or mutation cannot be hidden from verification.
func EntryHash(log model.AuditLog) (string, error) {
	payload, err := json.Marshal(struct {
		PreviousHash   string `json:"previous_hash"`
		ID             string `json:"id"`
		OrganizationID string `json:"organization_id"`
		RequestID      string `json:"request_id"`
		Action         string `json:"action"`
		ResourceType   string `json:"resource_type"`
		ResourceID     string `json:"resource_id"`
		Metadata       []byte `json:"metadata"`
		CreatedAt      int64  `json:"created_at_unix_nano"`
	}{log.PreviousHash, log.ID.String(), log.OrganizationID.String(), log.RequestID, log.Action, log.ResourceType, log.ResourceID, log.Metadata, log.CreatedAt.UnixNano()})
	if err != nil {
		return "", fmt.Errorf("marshal audit integrity payload: %w", err)
	}
	hash := sha256.Sum256(payload)
	return hex.EncodeToString(hash[:]), nil
}
