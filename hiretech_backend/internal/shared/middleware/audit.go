package middleware

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/masterfabric-go/masterfabric/internal/domain/audit/model"
	"github.com/masterfabric-go/masterfabric/internal/domain/audit/repository"
)

// AuditLog is middleware that records audit log entries for each request.
func AuditLog(auditRepo repository.AuditRepository) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if auditRepo == nil {
				next.ServeHTTP(w, r)
				return
			}

			audit := &RequestAudit{
				RequestID: r.Header.Get(RequestIDHeader),
				StartedAt: time.Now().UTC(),
			}
			ctx := WithRequestAudit(r.Context(), audit)
			wrapped := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}
			next.ServeHTTP(wrapped, r.WithContext(ctx))

			snapshot := audit.Snapshot()
			if snapshot.RequestID == "" {
				snapshot.RequestID = w.Header().Get(RequestIDHeader)
			}
			if snapshot.UserID == uuid.Nil {
				snapshot.UserID, _ = UserIDFromContext(r.Context())
			}
			if snapshot.OrganizationID == uuid.Nil {
				snapshot.OrganizationID, _ = TenantIDFromContext(r.Context())
			}
			if snapshot.Outcome == "" {
				snapshot.Outcome = "SUCCESS"
				if wrapped.statusCode >= http.StatusBadRequest {
					snapshot.Outcome = "FAILURE"
				}
			}
			if snapshot.Duration == 0 {
				snapshot.Duration = time.Since(snapshot.StartedAt)
			}

			var userIDPtr *uuid.UUID
			if snapshot.UserID != uuid.Nil {
				userIDPtr = &snapshot.UserID
			}
			action := r.Method + " " + r.URL.Path
			resourceType := "http_request"
			resourceID := r.URL.Path
			if snapshot.OperationName != "" {
				action = "graphql." + snapshot.OperationName
				resourceType = "graphql_operation"
				resourceID = snapshot.OperationName
			}
			metadata, _ := json.Marshal(map[string]any{
				"operation_name": snapshot.OperationName,
				"operation_type": snapshot.OperationType,
				"outcome":        snapshot.Outcome,
				"error_code":     snapshot.ErrorCode,
				"duration_ms":    snapshot.Duration.Milliseconds(),
				"complexity":     snapshot.Complexity,
			})

			entry := &model.AuditLog{
				OrganizationID: snapshot.OrganizationID, UserID: userIDPtr,
				RequestID: snapshot.RequestID, Action: action, ResourceType: resourceType,
				ResourceID: resourceID, Metadata: metadata, IPAddress: r.RemoteAddr, UserAgent: r.UserAgent(),
			}
			_ = auditRepo.Create(r.Context(), entry)
		})
	}
}
