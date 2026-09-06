package audit

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	auditDomain "github.com/masterfabric-go/masterfabric/internal/domain/audit"
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
	if log.RetentionUntil.IsZero() {
		log.RetentionUntil = log.CreatedAt.AddDate(2, 0, 0)
	}
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return domainErr.New(domainErr.ErrInternal, "failed to begin audit write", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1, 0))`, log.OrganizationID.String()); err != nil {
		return domainErr.New(domainErr.ErrInternal, "failed to lock audit chain", err)
	}
	if err = tx.QueryRow(ctx, `SELECT COALESCE((SELECT entry_hash FROM audit_logs WHERE organization_id=$1 ORDER BY created_at DESC,id DESC LIMIT 1), '')`, log.OrganizationID).Scan(&log.PreviousHash); err != nil {
		return domainErr.New(domainErr.ErrInternal, "failed to read audit chain head", err)
	}
	log.EntryHash, err = auditDomain.EntryHash(*log)
	if err != nil {
		return domainErr.New(domainErr.ErrInternal, "failed to hash audit entry", err)
	}
	_, err = tx.Exec(ctx,
		`INSERT INTO audit_logs (id, organization_id, app_id, endpoint_id, user_id, request_id, action, resource_type, resource_id, metadata, ip_address, user_agent, created_at, previous_hash, entry_hash, retention_until, legal_hold)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17)`,
		log.ID, log.OrganizationID, log.AppID, log.EndpointID, log.UserID,
		log.RequestID, log.Action, log.ResourceType, log.ResourceID,
		log.Metadata, log.IPAddress, log.UserAgent, log.CreatedAt, log.PreviousHash, log.EntryHash, log.RetentionUntil, log.LegalHold,
	)
	if err != nil {
		return domainErr.New(domainErr.ErrInternal, "failed to create audit log", err)
	}
	if err = tx.Commit(ctx); err != nil {
		return domainErr.New(domainErr.ErrInternal, "failed to commit audit write", err)
	}
	return nil
}

func (r *AuditRepo) VerifyIntegrity(ctx context.Context, organizationID uuid.UUID) error {
	rows, err := r.db.Query(ctx, `SELECT id, organization_id, request_id, action, resource_type, resource_id, metadata, created_at, previous_hash, entry_hash
		FROM audit_logs WHERE organization_id=$1 ORDER BY created_at ASC,id ASC`, organizationID)
	if err != nil {
		return domainErr.New(domainErr.ErrInternal, "failed to read audit integrity chain", err)
	}
	defer rows.Close()
	previous := ""
	for rows.Next() {
		var log model.AuditLog
		if err := rows.Scan(&log.ID, &log.OrganizationID, &log.RequestID, &log.Action, &log.ResourceType, &log.ResourceID, &log.Metadata, &log.CreatedAt, &log.PreviousHash, &log.EntryHash); err != nil {
			return domainErr.New(domainErr.ErrInternal, "failed to scan audit integrity chain", err)
		}
		if log.PreviousHash != previous || log.EntryHash == "" {
			return fmt.Errorf("audit integrity chain mismatch at %s", log.ID)
		}
		expected, err := auditDomain.EntryHash(log)
		if err != nil || expected != log.EntryHash {
			return fmt.Errorf("audit entry hash mismatch at %s", log.ID)
		}
		previous = log.EntryHash
	}
	if err := rows.Err(); err != nil {
		return domainErr.New(domainErr.ErrInternal, "failed to iterate audit integrity chain", err)
	}
	return nil
}

func (r *AuditRepo) PurgeExpired(ctx context.Context, now time.Time, limit int) (int, error) {
	if limit <= 0 {
		return 0, nil
	}
	result, err := r.db.Exec(ctx, `WITH doomed AS (
		SELECT id FROM audit_logs WHERE retention_until < $1 AND legal_hold = FALSE ORDER BY retention_until,id LIMIT $2
	) DELETE FROM audit_logs WHERE id IN (SELECT id FROM doomed)`, now.UTC(), limit)
	if err != nil {
		return 0, domainErr.New(domainErr.ErrInternal, "failed to purge expired audit logs", err)
	}
	return int(result.RowsAffected()), nil
}

func (r *AuditRepo) SetLegalHold(ctx context.Context, resourceType, resourceID string, held bool) error {
	result, err := r.db.Exec(ctx, `UPDATE audit_logs SET legal_hold=$3 WHERE resource_type=$1 AND resource_id=$2`, resourceType, resourceID, held)
	if err != nil {
		return domainErr.New(domainErr.ErrInternal, "failed to update audit legal hold", err)
	}
	if result.RowsAffected() == 0 {
		return domainErr.New(domainErr.ErrNotFound, "audit resource not found", nil)
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
		projectedAt := time.Now().UTC()
		projected := model.AuditLog{
			ID: event.id, OrganizationID: event.organizationID, UserID: userID,
			RequestID: requestID, Action: event.action, ResourceType: event.resourceType,
			ResourceID: event.resourceID.String(), Metadata: event.payload, CreatedAt: projectedAt,
			RetentionUntil: projectedAt.AddDate(2, 0, 0),
		}
		if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1, 0))`, event.organizationID.String()); err != nil {
			return 0, domainErr.New(domainErr.ErrInternal, "failed to lock projected audit chain", err)
		}
		if err = tx.QueryRow(ctx, `SELECT COALESCE((SELECT entry_hash FROM audit_logs WHERE organization_id=$1 ORDER BY created_at DESC,id DESC LIMIT 1), '')`, event.organizationID).Scan(&projected.PreviousHash); err != nil {
			return 0, domainErr.New(domainErr.ErrInternal, "failed to read projected audit chain head", err)
		}
		projected.EntryHash, err = auditDomain.EntryHash(projected)
		if err != nil {
			return 0, domainErr.New(domainErr.ErrInternal, "failed to hash projected audit entry", err)
		}
		_, err = tx.Exec(ctx, `
			INSERT INTO audit_logs (id, organization_id, user_id, request_id, action, resource_type, resource_id, metadata, created_at, previous_hash, entry_hash, retention_until, legal_hold)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)
			ON CONFLICT (id) DO NOTHING`,
			projected.ID, projected.OrganizationID, projected.UserID, projected.RequestID, projected.Action, projected.ResourceType, projected.ResourceID, projected.Metadata, projected.CreatedAt, projected.PreviousHash, projected.EntryHash, projected.RetentionUntil, projected.LegalHold,
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
