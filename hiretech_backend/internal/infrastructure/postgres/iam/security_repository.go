package iam

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	iamModel "github.com/masterfabric-go/masterfabric/internal/domain/iam/model"
	domainErr "github.com/masterfabric-go/masterfabric/internal/shared/errors"
)

type OTPChallengeRepo struct{ db *pgxpool.Pool }

func NewOTPChallengeRepo(db *pgxpool.Pool) *OTPChallengeRepo { return &OTPChallengeRepo{db: db} }
func (r *OTPChallengeRepo) InvalidateActive(ctx context.Context, email string) error {
	_, err := r.db.Exec(ctx, `UPDATE security_otp_challenges SET used_at=NOW() WHERE email=$1 AND used_at IS NULL AND expires_at > NOW()`, email)
	return err
}
func (r *OTPChallengeRepo) Create(ctx context.Context, c *iamModel.OTPChallenge) error {
	if c.ID == uuid.Nil {
		c.ID = uuid.New()
	}
	if c.CreatedAt.IsZero() {
		c.CreatedAt = time.Now().UTC()
	}
	_, err := r.db.Exec(ctx, `INSERT INTO security_otp_challenges (id,user_id,email,code_hash,expires_at,attempts,max_attempts,used_at,created_at) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)`, c.ID, nullableUUID(c.UserID), c.Email, c.CodeHash, c.ExpiresAt, c.Attempts, c.MaxAttempts, c.UsedAt, c.CreatedAt)
	return err
}
func (r *OTPChallengeRepo) Replace(ctx context.Context, email string, c *iamModel.OTPChallenge) error {
	if c.CreatedAt.IsZero() {
		c.CreatedAt = time.Now().UTC()
	}
	if c.ID == uuid.Nil {
		c.ID = uuid.New()
	}
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback(ctx)
		}
	}()
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext($1))`, email); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `UPDATE security_otp_challenges SET used_at=$2 WHERE email=$1 AND used_at IS NULL`, email, c.CreatedAt); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO security_otp_challenges (id,user_id,email,code_hash,expires_at,attempts,max_attempts,used_at,created_at) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)`, c.ID, nullableUUID(c.UserID), c.Email, c.CodeHash, c.ExpiresAt, c.Attempts, c.MaxAttempts, c.UsedAt, c.CreatedAt); err != nil {
		return err
	}
	err = tx.Commit(ctx)
	return err
}

func (r *OTPChallengeRepo) GetActiveByEmail(ctx context.Context, email string, now time.Time) (*iamModel.OTPChallenge, error) {
	var c iamModel.OTPChallenge
	var userID *uuid.UUID
	err := r.db.QueryRow(ctx, `SELECT id,user_id,email,code_hash,expires_at,attempts,max_attempts,used_at,created_at FROM security_otp_challenges WHERE email=$1 AND used_at IS NULL ORDER BY created_at DESC LIMIT 1`, email).Scan(&c.ID, &userID, &c.Email, &c.CodeHash, &c.ExpiresAt, &c.Attempts, &c.MaxAttempts, &c.UsedAt, &c.CreatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domainErr.New(domainErr.ErrNotFound, "challenge not found", nil)
		}
		return nil, err
	}
	if userID != nil {
		c.UserID = *userID
	}
	if !now.Before(c.ExpiresAt) {
		return &c, nil
	}
	return &c, nil
}
func (r *OTPChallengeRepo) IncrementAttempts(ctx context.Context, id uuid.UUID, max int) (int, error) {
	var attempts int
	err := r.db.QueryRow(ctx, `UPDATE security_otp_challenges SET attempts=attempts+1 WHERE id=$1 AND used_at IS NULL AND attempts < $2 RETURNING attempts`, id, max).Scan(&attempts)
	if errors.Is(err, pgx.ErrNoRows) {
		return max, nil
	}
	return attempts, err
}
func (r *OTPChallengeRepo) Consume(ctx context.Context, id uuid.UUID, hash string, now time.Time) (bool, error) {
	var n int64
	err := r.db.QueryRow(ctx, `UPDATE security_otp_challenges SET used_at=$3 WHERE id=$1 AND code_hash=$2 AND used_at IS NULL AND expires_at > $3 AND attempts < max_attempts RETURNING 1`, id, hash, now).Scan(&n)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	return err == nil, err
}

type SessionRepo struct{ db *pgxpool.Pool }

func NewSessionRepo(db *pgxpool.Pool) *SessionRepo { return &SessionRepo{db: db} }
func scanSession(row interface{ Scan(...any) error }) (*iamModel.Session, error) {
	var s iamModel.Session
	err := row.Scan(&s.ID, &s.UserID, &s.OrganizationID, &s.DeviceID, &s.FamilyID, &s.RefreshTokenHash, &s.RefreshTokenExpiresAt, &s.CreatedAt, &s.LastUsedAt, &s.RevokedAt, &s.AuthenticationTime)
	if err != nil {
		return nil, err
	}
	return &s, nil
}

const sessionColumns = `id,user_id,organization_id,device_id,family_id,refresh_token_hash,refresh_token_expires_at,created_at,last_used_at,revoked_at,authentication_time`

func (r *SessionRepo) Create(ctx context.Context, s *iamModel.Session) error {
	_, err := r.db.Exec(ctx, `INSERT INTO security_sessions (`+sessionColumns+`) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)`, s.ID, s.UserID, nullableUUID(s.OrganizationID), s.DeviceID, s.FamilyID, s.RefreshTokenHash, s.RefreshTokenExpiresAt, s.CreatedAt, s.LastUsedAt, s.RevokedAt, s.AuthenticationTime)
	if err != nil {
		return err
	}
	_, err = r.db.Exec(ctx, `INSERT INTO security_refresh_tokens (session_id,token_hash,used_at,created_at) VALUES ($1,$2,NULL,$3)`, s.ID, s.RefreshTokenHash, s.CreatedAt)
	return err
}
func (r *SessionRepo) Get(ctx context.Context, id uuid.UUID) (*iamModel.Session, error) {
	s, err := scanSession(r.db.QueryRow(ctx, `SELECT `+sessionColumns+` FROM security_sessions WHERE id=$1`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domainErr.New(domainErr.ErrNotFound, "session not found", nil)
	}
	return s, err
}
func (r *SessionRepo) Rotate(ctx context.Context, id uuid.UUID, presented, replacement string, expiresAt, now time.Time) (*iamModel.Session, bool, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, false, err
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback(ctx)
		}
	}()
	s, err := scanSession(tx.QueryRow(ctx, `SELECT `+sessionColumns+` FROM security_sessions WHERE id=$1 FOR UPDATE`, id))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			_ = tx.Rollback(ctx)
			return nil, false, nil
		}
		return nil, false, err
	}
	var used *time.Time
	err = tx.QueryRow(ctx, `SELECT used_at FROM security_refresh_tokens WHERE session_id=$1 AND token_hash=$2 FOR UPDATE`, id, presented).Scan(&used)
	if errors.Is(err, pgx.ErrNoRows) {
		_ = tx.Rollback(ctx)
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	if used != nil {
		_ = tx.Rollback(ctx)
		return s, true, nil
	}
	if s.RevokedAt != nil {
		_ = tx.Rollback(ctx)
		return s, false, nil
	}
	if _, err = tx.Exec(ctx, `UPDATE security_refresh_tokens SET used_at=$3 WHERE session_id=$1 AND token_hash=$2 AND used_at IS NULL`, id, presented, now); err != nil {
		return nil, false, err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO security_refresh_tokens (session_id,token_hash,used_at,created_at) VALUES ($1,$2,NULL,$3)`, id, replacement, now); err != nil {
		return nil, false, err
	}
	if _, err = tx.Exec(ctx, `UPDATE security_sessions SET refresh_token_hash=$2,refresh_token_expires_at=$3,last_used_at=$4 WHERE id=$1`, id, replacement, expiresAt, now); err != nil {
		return nil, false, err
	}
	s.RefreshTokenHash = replacement
	s.RefreshTokenExpiresAt = expiresAt
	s.LastUsedAt = now
	if err = tx.Commit(ctx); err != nil {
		return nil, false, err
	}
	return s, false, nil
}
func (r *SessionRepo) Revoke(ctx context.Context, id uuid.UUID, now time.Time) error {
	_, err := r.db.Exec(ctx, `UPDATE security_sessions SET revoked_at=COALESCE(revoked_at,$2) WHERE id=$1`, id, now)
	return err
}
func (r *SessionRepo) RevokeFamily(ctx context.Context, family uuid.UUID, now time.Time) error {
	_, err := r.db.Exec(ctx, `UPDATE security_sessions SET revoked_at=COALESCE(revoked_at,$2) WHERE family_id=$1`, family, now)
	return err
}
func (r *SessionRepo) RevokeAllByUser(ctx context.Context, user uuid.UUID, now time.Time) error {
	_, err := r.db.Exec(ctx, `UPDATE security_sessions SET revoked_at=COALESCE(revoked_at,$2) WHERE user_id=$1 AND revoked_at IS NULL`, user, now)
	return err
}
func (r *SessionRepo) ListByUser(ctx context.Context, user uuid.UUID) ([]*iamModel.Session, error) {
	rows, err := r.db.Query(ctx, `SELECT `+sessionColumns+` FROM security_sessions WHERE user_id=$1 ORDER BY last_used_at DESC`, user)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*iamModel.Session
	for rows.Next() {
		s, scanErr := scanSession(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

type DeviceRepo struct{ db *pgxpool.Pool }

func NewDeviceRepo(db *pgxpool.Pool) *DeviceRepo { return &DeviceRepo{db: db} }
func (r *DeviceRepo) Create(ctx context.Context, d *iamModel.Device) error {
	if d.ID == uuid.Nil {
		d.ID = uuid.New()
	}
	if d.CreatedAt.IsZero() {
		d.CreatedAt = time.Now().UTC()
	}
	_, err := r.db.Exec(ctx, `INSERT INTO security_devices (id,user_id,name,fingerprint_hash,state,last_seen_at,created_at,revoked_at) VALUES ($1,$2,$3,$4,$5,$6,$7,$8)`, d.ID, d.UserID, d.Name, d.FingerprintHash, d.State, d.LastSeenAt, d.CreatedAt, d.RevokedAt)
	return err
}
func scanDevice(row interface{ Scan(...any) error }) (*iamModel.Device, error) {
	var d iamModel.Device
	err := row.Scan(&d.ID, &d.UserID, &d.Name, &d.FingerprintHash, &d.State, &d.LastSeenAt, &d.CreatedAt, &d.RevokedAt)
	return &d, err
}
func (r *DeviceRepo) GetByID(ctx context.Context, id uuid.UUID) (*iamModel.Device, error) {
	d, err := scanDevice(r.db.QueryRow(ctx, `SELECT id,user_id,name,fingerprint_hash,state,last_seen_at,created_at,revoked_at FROM security_devices WHERE id=$1`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domainErr.New(domainErr.ErrNotFound, "device not found", nil)
	}
	return d, err
}
func (r *DeviceRepo) ListByUser(ctx context.Context, user uuid.UUID) ([]*iamModel.Device, error) {
	rows, err := r.db.Query(ctx, `SELECT id,user_id,name,fingerprint_hash,state,last_seen_at,created_at,revoked_at FROM security_devices WHERE user_id=$1 ORDER BY last_seen_at DESC`, user)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*iamModel.Device
	for rows.Next() {
		d, e := scanDevice(rows)
		if e != nil {
			return nil, e
		}
		d.FingerprintHash = ""
		out = append(out, d)
	}
	return out, rows.Err()
}
func (r *DeviceRepo) Revoke(ctx context.Context, id, user uuid.UUID, now time.Time) error {
	result, err := r.db.Exec(ctx, `UPDATE security_devices SET state='revoked',revoked_at=COALESCE(revoked_at,$3) WHERE id=$1 AND user_id=$2`, id, user, now)
	if err == nil && result.RowsAffected() == 0 {
		return domainErr.New(domainErr.ErrNotFound, "device not found", nil)
	}
	return err
}
func nullableUUID(id uuid.UUID) any {
	if id == uuid.Nil {
		return nil
	}
	return id
}
