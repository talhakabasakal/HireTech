package iam

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/masterfabric-go/masterfabric/internal/application/iam/dto"
	"github.com/masterfabric-go/masterfabric/internal/application/iam/usecase"
	domainErr "github.com/masterfabric-go/masterfabric/internal/shared/errors"
	"github.com/masterfabric-go/masterfabric/internal/shared/middleware"
	"github.com/masterfabric-go/masterfabric/internal/shared/pagination"
	"github.com/masterfabric-go/masterfabric/internal/shared/response"
	"github.com/masterfabric-go/masterfabric/internal/shared/validator"

	iamRepo "github.com/masterfabric-go/masterfabric/internal/domain/iam/repository"
)

// Handler provides IAM HTTP handlers.
type Handler struct {
	registerUC   *usecase.RegisterUseCase
	loginUC      *usecase.LoginUseCase
	assignRoleUC *usecase.AssignRoleUseCase
	userRepo     iamRepo.UserRepository
	securityUC   *usecase.SecurityService
}

// NewHandler creates a new IAM handler.
func NewHandler(
	registerUC *usecase.RegisterUseCase,
	loginUC *usecase.LoginUseCase,
	assignRoleUC *usecase.AssignRoleUseCase,
	userRepo iamRepo.UserRepository,
	security ...*usecase.SecurityService,
) *Handler {
	return &Handler{
		registerUC:   registerUC,
		loginUC:      loginUC,
		assignRoleUC: assignRoleUC,
		userRepo:     userRepo,
		securityUC: func() *usecase.SecurityService {
			if len(security) > 0 {
				return security[0]
			}
			return nil
		}(),
	}
}

// Register handles user registration.
func (h *Handler) Register(w http.ResponseWriter, r *http.Request) {
	var req dto.RegisterRequest
	if err := validator.DecodeAndValidate(r, &req); err != nil {
		response.JSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	user, err := h.registerUC.Execute(r.Context(), req)
	if err != nil {
		response.Error(w, err)
		return
	}

	response.Created(w, user)
}

// Login handles user authentication.
func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	var req dto.LoginRequest
	if err := validator.DecodeAndValidate(r, &req); err != nil {
		response.JSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	result, err := h.loginUC.Execute(r.Context(), req)
	if err != nil {
		response.Error(w, err)
		return
	}

	response.JSON(w, http.StatusOK, result)
}

// AssignRole handles role assignment.
func (h *Handler) AssignRole(w http.ResponseWriter, r *http.Request) {
	var req dto.AssignRoleRequest
	if err := validator.DecodeAndValidate(r, &req); err != nil {
		response.JSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	if err := h.assignRoleUC.Execute(r.Context(), req); err != nil {
		response.Error(w, err)
		return
	}

	response.NoContent(w)
}

// GetMe returns the current authenticated user.
func (h *Handler) GetMe(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.UserIDFromContext(r.Context())
	if !ok {
		response.JSON(w, http.StatusUnauthorized, map[string]string{"error": "not authenticated"})
		return
	}

	user, err := h.userRepo.GetByID(r.Context(), userID)
	if err != nil {
		response.Error(w, err)
		return
	}

	response.JSON(w, http.StatusOK, dto.UserInfo{
		ID:        user.ID,
		Email:     user.Email,
		FirstName: user.FirstName,
		LastName:  user.LastName,
		Status:    string(user.Status),
		CreatedAt: user.CreatedAt,
	})
}

// GetUser returns a user by ID.
func (h *Handler) GetUser(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		response.JSON(w, http.StatusBadRequest, map[string]string{"error": "invalid user id"})
		return
	}

	user, err := h.userRepo.GetByID(r.Context(), id)
	if err != nil {
		response.Error(w, err)
		return
	}

	response.JSON(w, http.StatusOK, dto.UserInfo{
		ID:        user.ID,
		Email:     user.Email,
		FirstName: user.FirstName,
		LastName:  user.LastName,
		Status:    string(user.Status),
		CreatedAt: user.CreatedAt,
	})
}

// ListUsers returns a paginated list of users.
func (h *Handler) ListUsers(w http.ResponseWriter, r *http.Request) {
	params := pagination.FromRequest(r)

	users, total, err := h.userRepo.List(r.Context(), params.Offset(), params.Limit())
	if err != nil {
		response.Error(w, err)
		return
	}

	var infos []dto.UserInfo
	for _, u := range users {
		infos = append(infos, dto.UserInfo{
			ID:        u.ID,
			Email:     u.Email,
			FirstName: u.FirstName,
			LastName:  u.LastName,
			Status:    string(u.Status),
			CreatedAt: u.CreatedAt,
		})
	}

	response.JSON(w, http.StatusOK, pagination.NewResult(infos, params, total))
}

func (h *Handler) RequestOTP(w http.ResponseWriter, r *http.Request) {
	var req dto.OTPRequest
	if err := validator.DecodeAndValidate(r, &req); err != nil {
		response.JSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request"})
		return
	}
	if h.securityUC == nil {
		response.Error(w, domainErr.New(domainErr.ErrInternal, "verification unavailable", nil))
		return
	}
	if err := h.securityUC.RequestOTP(r.Context(), req.Email); err != nil {
		response.Error(w, err)
		return
	}
	response.JSON(w, http.StatusAccepted, map[string]string{"message": "if the account is eligible, a verification code has been sent"})
}

func (h *Handler) VerifyOTP(w http.ResponseWriter, r *http.Request) {
	var req dto.OTPVerifyRequest
	if err := validator.DecodeAndValidate(r, &req); err != nil {
		response.JSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request"})
		return
	}
	if h.securityUC == nil {
		response.Error(w, domainErr.New(domainErr.ErrInternal, "verification unavailable", nil))
		return
	}
	pair, err := h.securityUC.VerifyOTP(r.Context(), req.Email, req.Code, req.DeviceName)
	if err != nil {
		response.Error(w, err)
		return
	}
	response.JSON(w, http.StatusOK, dto.SecurityTokenResponse{AccessToken: pair.AccessToken, RefreshToken: pair.RefreshToken, SessionID: pair.SessionID, DeviceID: pair.DeviceID})
}

func (h *Handler) Refresh(w http.ResponseWriter, r *http.Request) {
	var req dto.RefreshRequest
	if err := validator.DecodeAndValidate(r, &req); err != nil {
		response.JSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request"})
		return
	}
	if h.securityUC == nil {
		response.Error(w, domainErr.New(domainErr.ErrInternal, "refresh unavailable", nil))
		return
	}
	pair, err := h.securityUC.Refresh(r.Context(), req.RefreshToken)
	if err != nil {
		response.Error(w, err)
		return
	}
	response.JSON(w, http.StatusOK, dto.SecurityTokenResponse{AccessToken: pair.AccessToken, RefreshToken: pair.RefreshToken, SessionID: pair.SessionID, DeviceID: pair.DeviceID})
}

func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) {
	actor, ok := middleware.ActorFromContext(r.Context())
	if !ok {
		response.JSON(w, http.StatusUnauthorized, map[string]string{"error": "not authenticated"})
		return
	}
	if h.securityUC == nil {
		response.NoContent(w)
		return
	}
	if err := h.securityUC.Logout(r.Context(), actor); err != nil {
		response.Error(w, err)
		return
	}
	response.NoContent(w)
}
func (h *Handler) LogoutAll(w http.ResponseWriter, r *http.Request) {
	actor, ok := middleware.ActorFromContext(r.Context())
	if !ok {
		response.JSON(w, http.StatusUnauthorized, map[string]string{"error": "not authenticated"})
		return
	}
	if h.securityUC == nil {
		response.NoContent(w)
		return
	}
	if err := h.securityUC.LogoutAll(r.Context(), actor); err != nil {
		response.Error(w, err)
		return
	}
	response.NoContent(w)
}
func (h *Handler) ListDevices(w http.ResponseWriter, r *http.Request) {
	if h.securityUC == nil {
		response.Error(w, domainErr.New(domainErr.ErrInternal, "device service unavailable", nil))
		return
	}
	actor, ok := middleware.ActorFromContext(r.Context())
	if !ok {
		response.JSON(w, http.StatusUnauthorized, map[string]string{"error": "not authenticated"})
		return
	}
	devices, err := h.securityUC.ListDevices(r.Context(), actor)
	if err != nil {
		response.Error(w, err)
		return
	}
	out := make([]dto.DeviceInfo, 0, len(devices))
	for _, d := range devices {
		if d != nil {
			out = append(out, dto.DeviceInfo{ID: d.ID, Name: d.Name, State: string(d.State), LastSeenAt: d.LastSeenAt, CreatedAt: d.CreatedAt})
		}
	}
	response.JSON(w, http.StatusOK, out)
}
func (h *Handler) RevokeDevice(w http.ResponseWriter, r *http.Request) {
	if h.securityUC == nil {
		response.Error(w, domainErr.New(domainErr.ErrInternal, "device service unavailable", nil))
		return
	}
	actor, ok := middleware.ActorFromContext(r.Context())
	if !ok {
		response.JSON(w, http.StatusUnauthorized, map[string]string{"error": "not authenticated"})
		return
	}
	id, err := uuid.Parse(chi.URLParam(r, "deviceId"))
	if err != nil {
		response.JSON(w, http.StatusBadRequest, map[string]string{"error": "invalid device id"})
		return
	}
	if err = h.securityUC.RevokeDevice(r.Context(), actor, id); err != nil {
		response.Error(w, err)
		return
	}
	response.NoContent(w)
}
