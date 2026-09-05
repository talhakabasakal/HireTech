package aiadmin

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	aiadminModel "github.com/masterfabric-go/masterfabric/internal/domain/aiadmin/model"
	domainErr "github.com/masterfabric-go/masterfabric/internal/shared/errors"
)

type Repository struct{ db *pgxpool.Pool }

func NewRepository(db *pgxpool.Pool) *Repository { return &Repository{db: db} }

type stored struct {
	ID        uuid.UUID       `json:"id"`
	Key       string          `json:"key"`
	Payload   json.RawMessage `json:"payload"`
	Version   int             `json:"version"`
	Action    string          `json:"action"`
	Actor     uuid.UUID       `json:"actor"`
	CreatedAt time.Time       `json:"createdAt"`
}

func (r *Repository) GetWorkspace(ctx context.Context, org uuid.UUID) (*aiadminModel.Workspace, error) {
	return r.getWorkspace(ctx, org, "")
}

func (r *Repository) GetActiveWorkspace(ctx context.Context, org uuid.UUID) (*aiadminModel.Workspace, error) {
	return r.getWorkspace(ctx, org, " AND lifecycle_status='active'")
}

func (r *Repository) getWorkspace(ctx context.Context, org uuid.UUID, filter string) (*aiadminModel.Workspace, error) {
	rows, err := r.db.Query(ctx, `SELECT id, resource, resource_key, payload, version, action, lifecycle_status, actor_id, approved_by, created_at FROM ai_configuration_versions WHERE organization_id=$1`+filter+` ORDER BY created_at DESC`, org)
	if err != nil {
		return nil, domainErr.New(domainErr.ErrInternal, "failed to load AI configuration", err)
	}
	defer rows.Close()
	workspace := &aiadminModel.Workspace{Models: []*aiadminModel.Model{}, Prompts: []*aiadminModel.Prompt{}, Routing: []*aiadminModel.RoutingRule{}, Versions: []*aiadminModel.ConfigurationVersion{}, AuditEvents: []*aiadminModel.AuditEvent{}}
	seen := map[string]bool{}
	for rows.Next() {
		var id uuid.UUID
		var resource, key, action string
		var payload []byte
		var version int
		var lifecycleStatus string
		var actor uuid.UUID
		var approvedBy *uuid.UUID
		var created time.Time
		if err := rows.Scan(&id, &resource, &key, &payload, &version, &action, &lifecycleStatus, &actor, &approvedBy, &created); err != nil {
			return nil, domainErr.New(domainErr.ErrInternal, "failed to scan AI configuration", err)
		}
		workspace.Versions = append(workspace.Versions, &aiadminModel.ConfigurationVersion{ID: id, Resource: resource, Version: version, Action: action, Status: lifecycleStatus, Actor: actor, ApprovedBy: approvedBy, CreatedAt: created})
		if seen[resource+":"+key] {
			continue
		}
		seen[resource+":"+key] = true
		switch resource {
		case "model":
			var v aiadminModel.Model
			if json.Unmarshal(payload, &v) == nil {
				v.ID = id
				workspace.Models = append(workspace.Models, &v)
			}
		case "prompt":
			var v aiadminModel.Prompt
			if json.Unmarshal(payload, &v) == nil {
				v.ID = id
				workspace.Prompts = append(workspace.Prompts, &v)
			}
		case "routing":
			var v aiadminModel.RoutingRule
			if json.Unmarshal(payload, &v) == nil {
				v.ID = id
				workspace.Routing = append(workspace.Routing, &v)
			}
		case "rubric":
			if workspace.Rubric == nil {
				var v aiadminModel.Rubric
				if json.Unmarshal(payload, &v) == nil {
					v.ID = id
					workspace.Rubric = &v
				}
			}
		}
	}
	return workspace, rows.Err()
}

func (r *Repository) GetModel(ctx context.Context, org uuid.UUID, modelID string) (*aiadminModel.Model, error) {
	var id uuid.UUID
	var payload []byte
	if err := r.db.QueryRow(ctx, `SELECT id, payload FROM ai_configuration_versions WHERE organization_id=$1 AND resource='model' AND resource_key=$2 ORDER BY version DESC LIMIT 1`, org, modelID).Scan(&id, &payload); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domainErr.New(domainErr.ErrNotFound, "AI model not found", err)
		}
		return nil, domainErr.New(domainErr.ErrInternal, "failed to load AI model", err)
	}
	var value aiadminModel.Model
	if err := json.Unmarshal(payload, &value); err != nil {
		return nil, domainErr.New(domainErr.ErrInternal, "failed to decode AI model", err)
	}
	value.ID = id
	return &value, nil
}

func (r *Repository) write(ctx context.Context, org, actor uuid.UUID, resource, key string, value any) (uuid.UUID, int, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return uuid.Nil, 0, domainErr.New(domainErr.ErrInternal, "failed to begin AI configuration transaction", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// Version allocation and insertion must be serialized per tenant/resource/key.
	// Without this transaction-scoped advisory lock, concurrent admin writes can
	// both observe the same MAX(version)+1 and fail with a unique-key violation.
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1 || ':' || $2 || ':' || $3, 0))`, org.String(), resource, key); err != nil {
		return uuid.Nil, 0, domainErr.New(domainErr.ErrInternal, "failed to lock AI configuration version", err)
	}

	var version int
	if err := tx.QueryRow(ctx, `SELECT COALESCE(MAX(version),0)+1 FROM ai_configuration_versions WHERE organization_id=$1 AND resource=$2 AND resource_key=$3`, org, resource, key).Scan(&version); err != nil {
		return uuid.Nil, 0, domainErr.New(domainErr.ErrInternal, "failed to version AI configuration", err)
	}
	payload, err := json.Marshal(value)
	if err != nil {
		return uuid.Nil, 0, domainErr.New(domainErr.ErrInternal, "failed to encode AI configuration", err)
	}
	var versioned map[string]any
	if json.Unmarshal(payload, &versioned) == nil {
		versioned["version"] = version
		payload, _ = json.Marshal(versioned)
	}
	id := uuid.New()
	now := time.Now().UTC()
	_, err = tx.Exec(ctx, `INSERT INTO ai_configuration_versions (id, organization_id, resource, resource_key, payload, version, action, lifecycle_status, actor_id, created_at) VALUES ($1,$2,$3,$4,$5,$6,'created','draft',$7,$8)`, id, org, resource, key, payload, version, actor, now)
	if err != nil {
		return uuid.Nil, 0, domainErr.New(domainErr.ErrInternal, "failed to save AI configuration", err)
	}
	if err := insertAdminAudit(ctx, tx, org, actor, "ai_configuration.created", resource, id, key, version, now); err != nil {
		return uuid.Nil, 0, domainErr.New(domainErr.ErrInternal, "failed to record AI configuration audit", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return uuid.Nil, 0, domainErr.New(domainErr.ErrInternal, "failed to commit AI configuration", err)
	}
	return id, version, nil
}

func insertAdminAudit(ctx context.Context, tx pgx.Tx, org, actor uuid.UUID, action, resource string, resourceID uuid.UUID, key string, version int, occurredAt time.Time) error {
	payload, err := json.Marshal(map[string]any{"actor_id": actor, "resource_key": key, "version": version})
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `INSERT INTO audit_outbox (id,organization_id,action,resource_type,resource_id,payload,occurred_at) VALUES ($1,$2,$3,$4,$5,$6,$7)`, uuid.New(), org, action, resource, resourceID, payload, occurredAt)
	return err
}

func validateConfigurationReference(resource, key string, version int) error {
	if resource != "model" && resource != "prompt" && resource != "routing" && resource != "rubric" {
		return domainErr.New(domainErr.ErrValidation, "invalid AI configuration resource", nil)
	}
	if key == "" || version < 1 {
		return domainErr.New(domainErr.ErrValidation, "invalid AI configuration reference", nil)
	}
	return nil
}

func (r *Repository) Approve(ctx context.Context, org, actor uuid.UUID, resource, key string, version int) (result *aiadminModel.ConfigurationVersion, err error) {
	if err := validateConfigurationReference(resource, key, version); err != nil {
		return nil, err
	}
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, domainErr.New(domainErr.ErrInternal, "failed to begin AI configuration approval", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1 || ':' || $2 || ':' || $3, 0))`, org.String(), resource, key); err != nil {
		return nil, domainErr.New(domainErr.ErrInternal, "failed to lock AI configuration", err)
	}
	var id, createdBy uuid.UUID
	var lifecycle string
	var createdAt time.Time
	if err = tx.QueryRow(ctx, `SELECT id,actor_id,lifecycle_status,created_at FROM ai_configuration_versions WHERE organization_id=$1 AND resource=$2 AND resource_key=$3 AND version=$4 FOR UPDATE`, org, resource, key, version).Scan(&id, &createdBy, &lifecycle, &createdAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domainErr.New(domainErr.ErrNotFound, "AI configuration version not found", nil)
		}
		return nil, domainErr.New(domainErr.ErrInternal, "failed to load AI configuration version", err)
	}
	if lifecycle != aiadminModel.ConfigurationDraft {
		return nil, domainErr.New(domainErr.ErrConflict, "AI configuration version is not a draft", nil)
	}
	if createdBy == actor {
		return nil, domainErr.New(domainErr.ErrForbidden, "the draft author cannot approve the same configuration", nil)
	}
	if _, err = tx.Exec(ctx, `UPDATE ai_configuration_versions SET lifecycle_status='archived' WHERE organization_id=$1 AND resource=$2 AND resource_key=$3 AND lifecycle_status='active'`, org, resource, key); err != nil {
		return nil, domainErr.New(domainErr.ErrInternal, "failed to archive active AI configuration", err)
	}
	now := time.Now().UTC()
	if _, err = tx.Exec(ctx, `UPDATE ai_configuration_versions SET lifecycle_status='active',approved_by=$5,approved_at=$6 WHERE organization_id=$1 AND resource=$2 AND resource_key=$3 AND version=$4`, org, resource, key, version, actor, now); err != nil {
		return nil, domainErr.New(domainErr.ErrInternal, "failed to approve AI configuration", err)
	}
	if err = insertAdminAudit(ctx, tx, org, actor, "ai_configuration.approved", resource, id, key, version, now); err != nil {
		return nil, domainErr.New(domainErr.ErrInternal, "failed to record AI configuration audit", err)
	}
	if err = tx.Commit(ctx); err != nil {
		return nil, domainErr.New(domainErr.ErrInternal, "failed to commit AI configuration approval", err)
	}
	approved := actor
	return &aiadminModel.ConfigurationVersion{ID: id, Resource: resource, Version: version, Action: "activated", Status: aiadminModel.ConfigurationActive, Actor: createdBy, ApprovedBy: &approved, CreatedAt: createdAt}, nil
}

func (r *Repository) Rollback(ctx context.Context, org, actor uuid.UUID, resource, key string, version int) (result *aiadminModel.ConfigurationVersion, err error) {
	if err := validateConfigurationReference(resource, key, version); err != nil {
		return nil, err
	}
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, domainErr.New(domainErr.ErrInternal, "failed to begin AI configuration rollback", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1 || ':' || $2 || ':' || $3, 0))`, org.String(), resource, key); err != nil {
		return nil, domainErr.New(domainErr.ErrInternal, "failed to lock AI configuration", err)
	}
	var id, createdBy uuid.UUID
	var lifecycle string
	var createdAt time.Time
	if err = tx.QueryRow(ctx, `SELECT id,actor_id,lifecycle_status,created_at FROM ai_configuration_versions WHERE organization_id=$1 AND resource=$2 AND resource_key=$3 AND version=$4 FOR UPDATE`, org, resource, key, version).Scan(&id, &createdBy, &lifecycle, &createdAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domainErr.New(domainErr.ErrNotFound, "AI configuration version not found", nil)
		}
		return nil, domainErr.New(domainErr.ErrInternal, "failed to load AI configuration version", err)
	}
	if lifecycle == aiadminModel.ConfigurationDraft {
		return nil, domainErr.New(domainErr.ErrConflict, "a draft must be approved before rollback", nil)
	}
	if _, err = tx.Exec(ctx, `UPDATE ai_configuration_versions SET lifecycle_status='archived' WHERE organization_id=$1 AND resource=$2 AND resource_key=$3 AND lifecycle_status='active'`, org, resource, key); err != nil {
		return nil, domainErr.New(domainErr.ErrInternal, "failed to archive active AI configuration", err)
	}
	now := time.Now().UTC()
	if _, err = tx.Exec(ctx, `UPDATE ai_configuration_versions SET lifecycle_status='active',approved_by=$5,approved_at=$6 WHERE organization_id=$1 AND resource=$2 AND resource_key=$3 AND version=$4`, org, resource, key, version, actor, now); err != nil {
		return nil, domainErr.New(domainErr.ErrInternal, "failed to rollback AI configuration", err)
	}
	if err = insertAdminAudit(ctx, tx, org, actor, "ai_configuration.rolled_back", resource, id, key, version, now); err != nil {
		return nil, domainErr.New(domainErr.ErrInternal, "failed to record AI configuration audit", err)
	}
	if err = tx.Commit(ctx); err != nil {
		return nil, domainErr.New(domainErr.ErrInternal, "failed to commit AI configuration rollback", err)
	}
	approved := actor
	return &aiadminModel.ConfigurationVersion{ID: id, Resource: resource, Version: version, Action: "activated", Status: aiadminModel.ConfigurationActive, Actor: createdBy, ApprovedBy: &approved, CreatedAt: createdAt}, nil
}

func (r *Repository) RegisterModel(ctx context.Context, org, actor uuid.UUID, input aiadminModel.RegisterModelInput) (*aiadminModel.Model, error) {
	now := time.Now().UTC()
	value := &aiadminModel.Model{ModelID: input.ModelID, DisplayName: input.DisplayName, ProviderLabel: input.ProviderLabel, Roles: input.Roles, Status: input.Status, LatencyClass: input.LatencyClass, CreatedAt: now, UpdatedAt: now}
	id, _, err := r.write(ctx, org, actor, "model", input.ModelID, value)
	if err != nil {
		return nil, err
	}
	value.ID = id
	return value, nil
}
func (r *Repository) CreatePromptVersion(ctx context.Context, org, actor uuid.UUID, input aiadminModel.CreatePromptInput) (*aiadminModel.Prompt, error) {
	now := time.Now().UTC()
	key := string(input.Role)
	value := &aiadminModel.Prompt{Role: input.Role, Name: input.Name, Prompt: input.Prompt, Status: aiadminModel.PromptActive, UpdatedAt: now, UpdatedBy: actor}
	id, version, err := r.write(ctx, org, actor, "prompt", key, value)
	if err != nil {
		return nil, err
	}
	value.ID = id
	value.Version = version
	return value, nil
}
func (r *Repository) UpdateRouting(ctx context.Context, org, actor uuid.UUID, input aiadminModel.UpdateRoutingInput) (*aiadminModel.RoutingRule, error) {
	now := time.Now().UTC()
	value := &aiadminModel.RoutingRule{Role: input.Role, PrimaryModelID: input.PrimaryModelID, FallbackModelID: input.FallbackModelID, TimeoutSeconds: input.TimeoutSeconds, Enabled: input.Enabled, UpdatedAt: now, UpdatedBy: actor}
	id, _, err := r.write(ctx, org, actor, "routing", string(input.Role), value)
	if err != nil {
		return nil, err
	}
	value.ID = id
	return value, nil
}
func (r *Repository) PublishRubric(ctx context.Context, org, actor uuid.UUID, input aiadminModel.PublishRubricInput) (*aiadminModel.Rubric, error) {
	now := time.Now().UTC()
	value := &aiadminModel.Rubric{Name: input.Name, Criteria: input.Criteria, Status: "active", UpdatedAt: now, UpdatedBy: actor}
	id, version, err := r.write(ctx, org, actor, "rubric", "default", value)
	if err != nil {
		return nil, err
	}
	value.ID = id
	value.Version = version
	return value, nil
}
