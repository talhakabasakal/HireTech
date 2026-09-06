package privacy

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	privacyUC "github.com/masterfabric-go/masterfabric/internal/application/privacy/usecase"
	privacyModel "github.com/masterfabric-go/masterfabric/internal/domain/privacy/model"
	"github.com/masterfabric-go/masterfabric/internal/shared/middleware"
	"github.com/masterfabric-go/masterfabric/internal/shared/response"
	"github.com/masterfabric-go/masterfabric/internal/shared/validator"
)

type Handler struct{ service *privacyUC.Service }

func NewHandler(service *privacyUC.Service) *Handler { return &Handler{service: service} }

func (h *Handler) RequestExport(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.UserIDFromContext(r.Context())
	if !ok {
		response.JSON(w, http.StatusUnauthorized, map[string]string{"error": "not authenticated"})
		return
	}
	orgID, _ := middleware.TenantIDFromContext(r.Context())
	request, err := h.service.RequestExport(r.Context(), userID, orgID)
	if err != nil {
		response.Error(w, err)
		return
	}
	response.JSON(w, http.StatusAccepted, request)
}

func (h *Handler) RequestDeletion(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.UserIDFromContext(r.Context())
	if !ok {
		response.JSON(w, http.StatusUnauthorized, map[string]string{"error": "not authenticated"})
		return
	}
	orgID, _ := middleware.TenantIDFromContext(r.Context())
	actor, _ := middleware.ActorFromContext(r.Context())
	request, err := h.service.RequestDeletion(r.Context(), userID, orgID, actor.AuthenticationTime)
	if err != nil {
		response.Error(w, err)
		return
	}
	response.JSON(w, http.StatusAccepted, request)
}

func (h *Handler) GetRequest(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.UserIDFromContext(r.Context())
	if !ok {
		response.JSON(w, http.StatusUnauthorized, map[string]string{"error": "not authenticated"})
		return
	}
	requestID, err := uuid.Parse(chi.URLParam(r, "requestId"))
	if err != nil {
		response.JSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request id"})
		return
	}
	request, err := h.service.GetRequest(r.Context(), userID, requestID)
	if err != nil {
		response.Error(w, err)
		return
	}
	response.JSON(w, http.StatusOK, request)
}

type legalHoldInput struct {
	UserID       *uuid.UUID `json:"user_id"`
	ResourceType string     `json:"resource_type" validate:"required,max=100"`
	ResourceID   string     `json:"resource_id" validate:"required,max=255"`
	Reason       string     `json:"reason" validate:"required,max=2000"`
}

func (h *Handler) CreateLegalHold(w http.ResponseWriter, r *http.Request) {
	orgID, ok := middleware.TenantIDFromContext(r.Context())
	if !ok {
		response.JSON(w, http.StatusForbidden, map[string]string{"error": "organization scope required"})
		return
	}
	actor, ok := middleware.ActorFromContext(r.Context())
	if !ok {
		response.JSON(w, http.StatusUnauthorized, map[string]string{"error": "not authenticated"})
		return
	}
	var input legalHoldInput
	if err := validator.DecodeAndValidate(r, &input); err != nil {
		response.JSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	hold := &privacyModel.LegalHold{OrganizationID: orgID, UserID: input.UserID, ResourceType: input.ResourceType, ResourceID: input.ResourceID, Reason: input.Reason, CreatedBy: actor.UserID, CreatedAt: time.Now().UTC()}
	if err := h.service.CreateLegalHold(r.Context(), hold); err != nil {
		response.Error(w, err)
		return
	}
	response.JSON(w, http.StatusCreated, hold)
}

func (h *Handler) ReleaseLegalHold(w http.ResponseWriter, r *http.Request) {
	orgID, ok := middleware.TenantIDFromContext(r.Context())
	if !ok {
		response.JSON(w, http.StatusForbidden, map[string]string{"error": "organization scope required"})
		return
	}
	holdID, err := uuid.Parse(chi.URLParam(r, "holdId"))
	if err != nil {
		response.JSON(w, http.StatusBadRequest, map[string]string{"error": "invalid hold id"})
		return
	}
	if err := h.service.ReleaseLegalHold(r.Context(), orgID, holdID); err != nil {
		response.Error(w, err)
		return
	}
	response.NoContent(w)
}
