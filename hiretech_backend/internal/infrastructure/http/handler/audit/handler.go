package audit

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/masterfabric-go/masterfabric/internal/domain/audit/repository"
	"github.com/masterfabric-go/masterfabric/internal/shared/middleware"
	"github.com/masterfabric-go/masterfabric/internal/shared/pagination"
	"github.com/masterfabric-go/masterfabric/internal/shared/response"
)

// Handler provides Audit HTTP handlers.
type Handler struct {
	auditRepo repository.AuditRepository
}

// NewHandler creates a new Audit handler.
func NewHandler(auditRepo repository.AuditRepository) *Handler {
	return &Handler{auditRepo: auditRepo}
}

// ListByOrg returns audit logs for an organization.
func (h *Handler) ListByOrg(w http.ResponseWriter, r *http.Request) {
	orgID, err := uuid.Parse(chi.URLParam(r, "orgId"))
	if err != nil {
		response.JSON(w, http.StatusBadRequest, map[string]string{"error": "invalid org id"})
		return
	}

	params := pagination.FromRequest(r)
	logs, total, err := h.auditRepo.ListByOrg(r.Context(), orgID, params.Offset(), params.Limit())
	if err != nil {
		response.Error(w, err)
		return
	}

	response.JSON(w, http.StatusOK, pagination.NewResult(logs, params, total))
}

// ListByUser returns audit logs for a user.
func (h *Handler) ListByUser(w http.ResponseWriter, r *http.Request) {
	orgID, ok := middleware.TenantIDFromContext(r.Context())
	if !ok {
		response.JSON(w, http.StatusForbidden, map[string]string{"error": "organization scope required"})
		return
	}
	userID, err := uuid.Parse(chi.URLParam(r, "userId"))
	if err != nil {
		response.JSON(w, http.StatusBadRequest, map[string]string{"error": "invalid user id"})
		return
	}

	params := pagination.FromRequest(r)
	scopedRepo, ok := h.auditRepo.(repository.OrganizationScopedUserAuditRepository)
	if !ok {
		response.JSON(w, http.StatusServiceUnavailable, map[string]string{"error": "organization-scoped audit access unavailable"})
		return
	}
	logs, total, err := scopedRepo.ListByUserInOrg(r.Context(), orgID, userID, params.Offset(), params.Limit())
	if err != nil {
		response.Error(w, err)
		return
	}

	response.JSON(w, http.StatusOK, pagination.NewResult(logs, params, total))
}
