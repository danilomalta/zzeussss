package onlinesessions

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

var ErrPasswordPolicy = errors.New("nova senha deve ter 12 a 72 bytes, sem espaços nas extremidades e ser diferente da atual")

// All credential/session mutations take this account lock BEFORE row locks.
// It serializes password changes with login, refresh and logout, including new
// sessions not yet visible to a session-row scan. Hash collisions only serialize
// unrelated accounts; authorization still checks the full tenant/user scope.
func lockAccount(ctx context.Context, tx *sql.Tx, p Scope) error {
	if _, err := tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1, 748203081))`, p.Tenant+":"+p.User); err != nil {
		return ErrUnavailable
	}
	return nil
}

func ValidatePasswordChange(current, next string) error {
	if current == "" || len(current) > 72 || !utf8.ValidString(current) {
		return ErrDenied
	}
	if len(next) < 12 || len(next) > 72 || !utf8.ValidString(next) || strings.TrimSpace(next) != next || next == current {
		return ErrPasswordPolicy
	}
	return nil
}

// ChangePassword changes only the authenticated account. The actor is checked
// again under locks and server time; hash, every revocation and audit commit
// together. It never returns a replacement token or a password/hash.
func (s *Store) ChangePassword(ctx context.Context, p Scope, current, next string) error {
	if !ValidID(p.Session) {
		return ErrDenied
	}
	if err := ValidatePasswordChange(current, next); err != nil {
		return err
	}
	// Hash outside locks; the HTTP limiter bounds work. Credential verification
	// remains inside the transaction, against the locked current database value.
	newHash, err := bcrypt.GenerateFromPassword([]byte(next), bcrypt.DefaultCost)
	if err != nil {
		return ErrUnavailable
	}
	tx, err := s.begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err = lockAccount(ctx, tx, p); err != nil {
		return err
	}
	var hash string
	err = tx.QueryRowContext(ctx, `SELECT u.password_hash FROM users u JOIN tenants t ON t.id=u.tenant_id WHERE u.id=$1 AND u.tenant_id=$2 AND u.role=$3 AND t.status='active' FOR UPDATE OF u FOR SHARE OF t`, p.User, p.Tenant, p.Role).Scan(&hash)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrDenied
	}
	if err != nil {
		return ErrUnavailable
	}
	rows, err := tx.QueryContext(ctx, `SELECT id, role, expires_at, revoked_at FROM online_sessions WHERE tenant_id=$1 AND user_id=$2 ORDER BY id FOR UPDATE`, p.Tenant, p.User)
	if err != nil {
		return ErrUnavailable
	}
	ids := []string{}
	found := false
	var expires time.Time
	for rows.Next() {
		var id, role string
		var end time.Time
		var revoked sql.NullTime
		if rows.Scan(&id, &role, &end, &revoked) != nil {
			rows.Close()
			return ErrUnavailable
		}
		if id == p.Session && role == p.Role && !revoked.Valid {
			found = true
			expires = end
		}
		if !revoked.Valid {
			ids = append(ids, id)
		}
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return ErrUnavailable
	}
	var now time.Time
	if tx.QueryRowContext(ctx, `SELECT clock_timestamp()`).Scan(&now) != nil {
		return ErrUnavailable
	}
	if !found || !expires.After(now) || bcrypt.CompareHashAndPassword([]byte(hash), []byte(current)) != nil {
		return ErrDenied
	}
	if bcrypt.CompareHashAndPassword([]byte(hash), []byte(next)) == nil {
		return ErrPasswordPolicy
	}
	// Bcrypt takes time too; do not write using a session that expired while
	// credentials were being verified.
	if tx.QueryRowContext(ctx, `SELECT clock_timestamp()`).Scan(&now) != nil {
		return ErrUnavailable
	}
	if !expires.After(now) {
		return ErrDenied
	}
	result, err := tx.ExecContext(ctx, `UPDATE users SET password_hash=$1 WHERE id=$2 AND tenant_id=$3 AND password_hash=$4`, string(newHash), p.User, p.Tenant, hash)
	if err != nil {
		return ErrUnavailable
	}
	if n, e := result.RowsAffected(); e != nil || n != 1 {
		return ErrUnavailable
	}
	for _, id := range ids {
		q := p
		q.Session = id
		if err = revokeRow(ctx, tx, q, "revoked"); err != nil {
			return err
		}
	}
	result, err = tx.ExecContext(ctx, `INSERT INTO online_password_changes(id, tenant_id, user_id, actor_session_id, revoked_count, created_at) VALUES ($1,$2,$3,$4,$5,clock_timestamp())`, uuid.NewString(), p.Tenant, p.User, p.Session, len(ids))
	if err != nil {
		return ErrUnavailable
	}
	if n, e := result.RowsAffected(); e != nil || n != 1 {
		return ErrUnavailable
	}
	return safeCommit(tx)
}
