package audit

import (
	"context"
	"encoding/json"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/masterfabric-go/masterfabric/internal/domain/audit/model"
	domainErr "github.com/masterfabric-go/masterfabric/internal/shared/errors"
	"github.com/masterfabric-go/masterfabric/internal/shared/pagination"
)

// AuditRepo implements repository.AuditRepository with PostgreSQL.
type AuditRepo struct {
	db *pgxpool.Pool
}

// NewAuditRepo creates a new AuditRepo.
func NewAuditRepo(db *pgxpool.Pool) *AuditRepo {
	return &AuditRepo{db: db}
}

func (r *AuditRepo) Create(ctx context.Context, log *model.AuditLog) error {
	if log.ID == uuid.Nil {
		log.ID = uuid.New()
	}
	log.CreatedAt = time.Now().UTC()

	_, err := r.db.Exec(ctx,
		`INSERT INTO audit_logs (id, organization_id, app_id, endpoint_id, user_id, request_id, action, resource_type, resource_id, metadata, ip_address, user_agent, created_at)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)`,
		log.ID, log.OrganizationID, log.AppID, log.EndpointID, log.UserID,
		log.RequestID, log.Action, log.ResourceType, log.ResourceID,
		log.Metadata, log.IPAddress, log.UserAgent, log.CreatedAt,
	)
	if err != nil {
		return domainErr.New(domainErr.ErrInternal, "failed to create audit log", err)
	}
	return nil
}

// RelayPending atomically projects a bounded batch of audit outbox rows into
// audit_logs. SELECT ... FOR UPDATE SKIP LOCKED allows multiple relay workers
// to run safely, while the outbox ID makes retries idempotent.
func (r *AuditRepo) RelayPending(ctx context.Context, limit int) (count int, err error) {
	if limit <= 0 {
		return 0, nil
	}

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return 0, domainErr.New(domainErr.ErrInternal, "failed to begin audit outbox relay", err)
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback(ctx)
		}
	}()

	rows, err := tx.Query(ctx, `
		SELECT id, organization_id, action, resource_type, resource_id, payload, occurred_at
		FROM audit_outbox
		WHERE published_at IS NULL
		ORDER BY occurred_at, id
		FOR UPDATE SKIP LOCKED
		LIMIT $1`, limit)
	if err != nil {
		return 0, domainErr.New(domainErr.ErrInternal, "failed to read pending audit events", err)
	}
	type pendingAuditEvent struct {
		id, organizationID, resourceID uuid.UUID
		action, resourceType           string
		payload                        []byte
		occurredAt                     time.Time
	}
	pending := make([]pendingAuditEvent, 0, limit)
	for rows.Next() {
		var event pendingAuditEvent
		if err = rows.Scan(&event.id, &event.organizationID, &event.action, &event.resourceType, &event.resourceID, &event.payload, &event.occurredAt); err != nil {
			rows.Close()
			return 0, domainErr.New(domainErr.ErrInternal, "failed to scan pending audit event", err)
		}
		pending = append(pending, event)
	}
	if err = rows.Err(); err != nil {
		rows.Close()
		return 0, domainErr.New(domainErr.ErrInternal, "failed to iterate pending audit events", err)
	}
	// pgx does not allow another command on a connection while query rows are
	// open. Materialize the bounded batch and close it before projecting; the
	// row locks remain held by this transaction until commit.
	rows.Close()

	for _, event := range pending {
		userID, requestID := auditActorAndRequest(event.payload)
		_, err = tx.Exec(ctx, `
			INSERT INTO audit_logs (id, organization_id, user_id, request_id, action, resource_type, resource_id, metadata, created_at)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
			ON CONFLICT (id) DO NOTHING`,
			event.id, event.organizationID, userID, requestID, event.action, event.resourceType, event.resourceID.String(), event.payload, event.occurredAt,
		)
		if err != nil {
			return 0, domainErr.New(domainErr.ErrInternal, "failed to project audit event", err)
		}

		_, err = tx.Exec(ctx, `UPDATE audit_outbox SET published_at=$2 WHERE id=$1 AND published_at IS NULL`, event.id, time.Now().UTC())
		if err != nil {
			return 0, domainErr.New(domainErr.ErrInternal, "failed to mark audit event published", err)
		}
		count++
	}
	if err = tx.Commit(ctx); err != nil {
		return 0, domainErr.New(domainErr.ErrInternal, "failed to commit audit outbox relay", err)
	}
	return count, nil
}

func auditActorAndRequest(payload []byte) (*uuid.UUID, string) {
	var envelope struct {
		ActorID   *string `json:"actor_id"`
		RequestID string  `json:"request_id"`
	}
	if err := json.Unmarshal(payload, &envelope); err != nil || envelope.ActorID == nil {
		return nil, envelope.RequestID
	}
	actorID, err := uuid.Parse(*envelope.ActorID)
	if err != nil || actorID == uuid.Nil {
		return nil, envelope.RequestID
	}
	return &actorID, envelope.RequestID
}

func (r *AuditRepo) ListByOrg(ctx context.Context, orgID uuid.UUID, offset, limit int) ([]*model.AuditLog, int, error) {
	var total int
	if err := r.db.QueryRow(ctx, `SELECT COUNT(*) FROM audit_logs WHERE organization_id=$1`, orgID).Scan(&total); err != nil {
		return nil, 0, domainErr.New(domainErr.ErrInternal, "failed to count audit logs", err)
	}

	rows, err := r.db.Query(ctx,
		`SELECT id, organization_id, app_id, endpoint_id, user_id, request_id, action, resource_type, resource_id, metadata, ip_address, user_agent, created_at
		 FROM audit_logs WHERE organization_id=$1 ORDER BY created_at DESC LIMIT $2 OFFSET $3`, orgID, limit, offset,
	)
	if err != nil {
		return nil, 0, domainErr.New(domainErr.ErrInternal, "failed to list audit logs", err)
	}
	defer rows.Close()

	return r.scanLogs(rows, total)
}

// ListByOrgPage reads one bounded keyset page. Organization scope is part of
// the storage predicate so a cursor can never widen the tenant boundary.
func (r *AuditRepo) ListByOrgPage(ctx context.Context, orgID uuid.UUID, after *pagination.Cursor, limit int) ([]*model.AuditLog, error) {
	if limit <= 0 {
		return []*model.AuditLog{}, nil
	}
	query := `SELECT id, organization_id, app_id, endpoint_id, user_id, request_id, action, resource_type, resource_id, metadata, ip_address, user_agent, created_at
		FROM audit_logs WHERE organization_id=$1`
	args := []any{orgID}
	if after != nil {
		query += ` AND (created_at,id) < ($2,$3)`
		args = append(args, after.CreatedAt, after.ID)
	}
	args = append(args, limit)
	query += ` ORDER BY created_at DESC,id DESC LIMIT $` + strconv.Itoa(len(args))
	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, domainErr.New(domainErr.ErrInternal, "failed to list audit log page", err)
	}
	defer rows.Close()
	logs, _, err := r.scanLogs(rows, 0)
	return logs, err
}

func (r *AuditRepo) ListByUser(ctx context.Context, userID uuid.UUID, offset, limit int) ([]*model.AuditLog, int, error) {
	var total int
	if err := r.db.QueryRow(ctx, `SELECT COUNT(*) FROM audit_logs WHERE user_id=$1`, userID).Scan(&total); err != nil {
		return nil, 0, domainErr.New(domainErr.ErrInternal, "failed to count audit logs", err)
	}

	rows, err := r.db.Query(ctx,
		`SELECT id, organization_id, app_id, endpoint_id, user_id, request_id, action, resource_type, resource_id, metadata, ip_address, user_agent, created_at
		 FROM audit_logs WHERE user_id=$1 ORDER BY created_at DESC LIMIT $2 OFFSET $3`, userID, limit, offset,
	)
	if err != nil {
		return nil, 0, domainErr.New(domainErr.ErrInternal, "failed to list audit logs", err)
	}
	defer rows.Close()

	return r.scanLogs(rows, total)
}

func (r *AuditRepo) ListByUserInOrg(ctx context.Context, orgID, userID uuid.UUID, offset, limit int) ([]*model.AuditLog, int, error) {
	var total int
	if err := r.db.QueryRow(ctx, `SELECT COUNT(*) FROM audit_logs WHERE organization_id=$1 AND user_id=$2`, orgID, userID).Scan(&total); err != nil {
		return nil, 0, domainErr.New(domainErr.ErrInternal, "failed to count audit logs", err)
	}

	rows, err := r.db.Query(ctx,
		`SELECT id, organization_id, app_id, endpoint_id, user_id, request_id, action, resource_type, resource_id, metadata, ip_address, user_agent, created_at
		 FROM audit_logs WHERE organization_id=$1 AND user_id=$2 ORDER BY created_at DESC LIMIT $3 OFFSET $4`, orgID, userID, limit, offset,
	)
	if err != nil {
		return nil, 0, domainErr.New(domainErr.ErrInternal, "failed to list audit logs", err)
	}
	defer rows.Close()

	return r.scanLogs(rows, total)
}

func (r *AuditRepo) ListByResource(ctx context.Context, resourceType, resourceID string, offset, limit int) ([]*model.AuditLog, int, error) {
	var total int
	if err := r.db.QueryRow(ctx, `SELECT COUNT(*) FROM audit_logs WHERE resource_type=$1 AND resource_id=$2`, resourceType, resourceID).Scan(&total); err != nil {
		return nil, 0, domainErr.New(domainErr.ErrInternal, "failed to count audit logs", err)
	}

	rows, err := r.db.Query(ctx,
		`SELECT id, organization_id, app_id, endpoint_id, user_id, request_id, action, resource_type, resource_id, metadata, ip_address, user_agent, created_at
		 FROM audit_logs WHERE resource_type=$1 AND resource_id=$2 ORDER BY created_at DESC LIMIT $3 OFFSET $4`, resourceType, resourceID, limit, offset,
	)
	if err != nil {
		return nil, 0, domainErr.New(domainErr.ErrInternal, "failed to list audit logs", err)
	}
	defer rows.Close()

	return r.scanLogs(rows, total)
}

func (r *AuditRepo) scanLogs(rows interface {
	Next() bool
	Scan(dest ...interface{}) error
}, total int) ([]*model.AuditLog, int, error) {
	var logs []*model.AuditLog
	for rows.Next() {
		var l model.AuditLog
		if err := rows.Scan(&l.ID, &l.OrganizationID, &l.AppID, &l.EndpointID, &l.UserID,
			&l.RequestID, &l.Action, &l.ResourceType, &l.ResourceID,
			&l.Metadata, &l.IPAddress, &l.UserAgent, &l.CreatedAt); err != nil {
			return nil, 0, domainErr.New(domainErr.ErrInternal, "failed to scan audit log", err)
		}
		logs = append(logs, &l)
	}
	return logs, total, nil
}
