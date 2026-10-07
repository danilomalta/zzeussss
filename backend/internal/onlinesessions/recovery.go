package onlinesessions

import (
	"context"
	"crypto/subtle"
	"database/sql"
	"errors"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

var ErrRecoveryRate = errors.New("aguarde antes de gerar outra chave")

type RecoveryKey struct {
	Key       string    `json:"recovery_key"`
	ExpiresAt time.Time `json:"expires_at"`
}

func ParseRecovery(raw string) (string, error) {
	if !strings.HasPrefix(raw, "rk1.") {
		return "", ErrDenied
	}
	return ParseRefresh("v1." + strings.TrimPrefix(raw, "rk1."))
}

func oneWrite(ctx context.Context, tx *sql.Tx, query string, args ...any) error {
	r, e := tx.ExecContext(ctx, query, args...)
	if e != nil {
		return ErrUnavailable
	}
	if n, e := r.RowsAffected(); e != nil || n != 1 {
		return ErrUnavailable
	}
	return nil
}
func recoveryAudit(ctx context.Context, tx *sql.Tx, p Scope, id, event string, count int) error {
	return oneWrite(ctx, tx, `INSERT INTO online_recovery_audit(id,tenant_id,user_id,key_id,event,revoked_count,created_at) VALUES ($1,$2,$3,$4,$5,$6,clock_timestamp())`, uuid.NewString(), p.Tenant, p.User, id, event, count)
}

// IssueRecovery requires the authenticated account's password and active session.
// One row per account bounds active-key storage. A persistent cooldown prevents
// multiple API instances from bypassing the account issuance rate.
func (s *Store) IssueRecovery(ctx context.Context, p Scope, password string) (RecoveryKey, error) {
	if !ValidID(p.Tenant) || !ValidID(p.User) || !ValidID(p.Session) {
		return RecoveryKey{}, ErrDenied
	}
	if password == "" || len(password) > 72 || !utf8.ValidString(password) {
		return RecoveryKey{}, ErrDenied
	}
	tx, e := s.begin(ctx)
	if e != nil {
		return RecoveryKey{}, e
	}
	defer tx.Rollback()
	if e = lockAccount(ctx, tx, p); e != nil {
		return RecoveryKey{}, e
	}
	var hash string
	e = tx.QueryRowContext(ctx, `SELECT u.password_hash FROM users u JOIN tenants t ON t.id=u.tenant_id WHERE u.id=$1 AND u.tenant_id=$2 AND u.role=$3 AND t.status='active' FOR UPDATE OF u FOR SHARE OF t`, p.User, p.Tenant, p.Role).Scan(&hash)
	if errors.Is(e, sql.ErrNoRows) {
		return RecoveryKey{}, ErrDenied
	}
	if e != nil {
		return RecoveryKey{}, ErrUnavailable
	}
	var end time.Time
	e = tx.QueryRowContext(ctx, `SELECT expires_at FROM online_sessions WHERE id=$1 AND tenant_id=$2 AND user_id=$3 AND role=$4 AND revoked_at IS NULL FOR UPDATE`, p.Session, p.Tenant, p.User, p.Role).Scan(&end)
	if errors.Is(e, sql.ErrNoRows) {
		return RecoveryKey{}, ErrDenied
	}
	if e != nil {
		return RecoveryKey{}, ErrUnavailable
	}
	if bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) != nil {
		return RecoveryKey{}, ErrDenied
	}
	var now time.Time
	if tx.QueryRowContext(ctx, `SELECT clock_timestamp()`).Scan(&now) != nil {
		return RecoveryKey{}, ErrUnavailable
	}
	if !end.After(now) {
		return RecoveryKey{}, ErrDenied
	}
	var recent bool
	if tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM online_recovery_keys WHERE tenant_id=$1 AND user_id=$2 AND issued_at > clock_timestamp()-interval '1 minute')`, p.Tenant, p.User).Scan(&recent) != nil {
		return RecoveryKey{}, ErrUnavailable
	}
	if recent {
		return RecoveryKey{}, ErrRecoveryRate
	}
	id := uuid.NewString()
	raw, e := refreshSecret(id)
	if e != nil {
		return RecoveryKey{}, e
	}
	raw = "rk1." + strings.TrimPrefix(raw, "v1.")
	var expires time.Time
	e = tx.QueryRowContext(ctx, `INSERT INTO online_recovery_keys(tenant_id,user_id,id,digest,credential_digest,role,issued_at,expires_at,consumed_at) SELECT $1::uuid,$2::uuid,$3::uuid,$4::text,$5::text,$6::text,clock_timestamp(),clock_timestamp()+interval '30 days',NULL WHERE EXISTS(SELECT 1 FROM online_sessions WHERE id=$7 AND tenant_id=$1 AND user_id=$2 AND role=$6 AND revoked_at IS NULL AND expires_at>clock_timestamp()) ON CONFLICT(tenant_id,user_id) DO UPDATE SET id=excluded.id,digest=excluded.digest,credential_digest=excluded.credential_digest,role=excluded.role,issued_at=excluded.issued_at,expires_at=excluded.expires_at,consumed_at=NULL RETURNING expires_at`, p.Tenant, p.User, id, tokenDigest(raw), tokenDigest(hash), p.Role, p.Session).Scan(&expires)
	if e != nil {
		return RecoveryKey{}, ErrUnavailable
	}
	if e = recoveryAudit(ctx, tx, p, id, "issued", 0); e != nil {
		return RecoveryKey{}, e
	}
	if e = safeCommit(tx); e != nil {
		return RecoveryKey{}, e
	}
	return RecoveryKey{Key: raw, ExpiresAt: expires}, nil
}

// Recover consumes a personal key and changes the password atomically. No email,
// account selection, replacement access token or automatic retry is involved.
func (s *Store) Recover(ctx context.Context, raw, next string) error {
	id, e := ParseRecovery(raw)
	if e != nil {
		return e
	}
	if len(next) < 12 || len(next) > 72 || !utf8.ValidString(next) || strings.TrimSpace(next) != next {
		return ErrPasswordPolicy
	}
	tx, e := s.begin(ctx)
	if e != nil {
		return e
	}
	defer tx.Rollback()
	var p Scope
	e = tx.QueryRowContext(ctx, `SELECT tenant_id,user_id FROM online_recovery_keys WHERE id=$1 AND digest=$2 AND consumed_at IS NULL AND expires_at>clock_timestamp()`, id, tokenDigest(raw)).Scan(&p.Tenant, &p.User)
	if errors.Is(e, sql.ErrNoRows) {
		return ErrDenied
	}
	if e != nil {
		return ErrUnavailable
	}
	if e = lockAccount(ctx, tx, p); e != nil {
		return e
	}
	var hash, role string
	e = tx.QueryRowContext(ctx, `SELECT u.password_hash,u.role FROM users u JOIN tenants t ON t.id=u.tenant_id WHERE u.id=$1 AND u.tenant_id=$2 AND t.status='active' FOR UPDATE OF u FOR SHARE OF t`, p.User, p.Tenant).Scan(&hash, &role)
	if errors.Is(e, sql.ErrNoRows) {
		return ErrDenied
	}
	if e != nil {
		return ErrUnavailable
	}
	var credential, keyRole string
	e = tx.QueryRowContext(ctx, `SELECT credential_digest,role FROM online_recovery_keys WHERE id=$1 AND tenant_id=$2 AND user_id=$3 AND digest=$4 AND consumed_at IS NULL AND expires_at>clock_timestamp() FOR UPDATE`, id, p.Tenant, p.User, tokenDigest(raw)).Scan(&credential, &keyRole)
	if errors.Is(e, sql.ErrNoRows) {
		return ErrDenied
	}
	if e != nil {
		return ErrUnavailable
	}
	if subtle.ConstantTimeCompare([]byte(credential), []byte(tokenDigest(hash))) != 1 || keyRole != role {
		return ErrDenied
	}
	if bcrypt.CompareHashAndPassword([]byte(hash), []byte(next)) == nil {
		return ErrPasswordPolicy
	}
	newHash, e := bcrypt.GenerateFromPassword([]byte(next), bcrypt.DefaultCost)
	if e != nil {
		return ErrUnavailable
	}
	if e = oneWrite(ctx, tx, `UPDATE online_recovery_keys SET consumed_at=clock_timestamp() WHERE id=$1 AND tenant_id=$2 AND user_id=$3 AND digest=$4 AND consumed_at IS NULL AND expires_at>clock_timestamp()`, id, p.Tenant, p.User, tokenDigest(raw)); e != nil {
		return e
	}
	if e = oneWrite(ctx, tx, `UPDATE users SET password_hash=$1 WHERE id=$2 AND tenant_id=$3 AND password_hash=$4`, string(newHash), p.User, p.Tenant, hash); e != nil {
		return e
	}
	rows, e := tx.QueryContext(ctx, `SELECT id FROM online_sessions WHERE tenant_id=$1 AND user_id=$2 AND revoked_at IS NULL ORDER BY id FOR UPDATE`, p.Tenant, p.User)
	if e != nil {
		return ErrUnavailable
	}
	ids := []string{}
	for rows.Next() {
		var sid string
		if rows.Scan(&sid) != nil {
			rows.Close()
			return ErrUnavailable
		}
		ids = append(ids, sid)
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return ErrUnavailable
	}
	for _, sid := range ids {
		q := p
		q.Session = sid
		if e = revokeRow(ctx, tx, q, "revoked"); e != nil {
			return e
		}
	}
	if e = recoveryAudit(ctx, tx, p, id, "recovered", len(ids)); e != nil {
		return e
	}
	return safeCommit(tx)
}
