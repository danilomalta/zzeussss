package onlinesessions

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

var ErrDenied = errors.New("sessão inválida ou expirada")
var ErrUnavailable = errors.New("sessão indisponível")

type Scope struct{ User, Tenant, Role, Session string }
type Tokens struct {
	Access, Refresh string
	ExpiresIn       int64
	RefreshExpires  time.Time
}
type Session struct {
	ID        string     `json:"id"`
	CreatedAt time.Time  `json:"created_at"`
	ExpiresAt time.Time  `json:"expires_at"`
	RevokedAt *time.Time `json:"revoked_at"`
	Current   bool       `json:"current"`
}
type Store struct {
	db     *sql.DB
	secret string
}

func New(db *sql.DB, secret string) *Store { return &Store{db: db, secret: secret} }

func (s *Store) begin(ctx context.Context) (*sql.Tx, error) {
	if s.db == nil || s.secret == "" {
		return nil, ErrUnavailable
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, ErrUnavailable
	}
	return tx, nil
}
func safeCommit(tx *sql.Tx) error {
	if tx.Commit() != nil {
		return ErrUnavailable
	}
	return nil
}

func tokenDigest(raw string) string { h := sha256.Sum256([]byte(raw)); return hex.EncodeToString(h[:]) }
func refreshSecret(id string) (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", ErrUnavailable
	}
	return "v1." + id + "." + base64.RawURLEncoding.EncodeToString(b), nil
}
func ParseRefresh(raw string) (string, error) {
	parts := strings.Split(raw, ".")
	if len(parts) != 3 || parts[0] != "v1" || len(raw) != 83 || !ValidID(parts[1]) {
		return "", ErrDenied
	}
	b, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil || len(b) != 32 || base64.RawURLEncoding.EncodeToString(b) != parts[2] {
		return "", ErrDenied
	}
	return parts[1], nil
}
func ValidID(id string) bool {
	v, err := uuid.Parse(id)
	return err == nil && v != uuid.Nil && v.String() == id
}

func lockIdentity(ctx context.Context, tx *sql.Tx, p Scope) (name, hash string, err error) {
	err = tx.QueryRowContext(ctx, `SELECT u.name, u.password_hash FROM users u JOIN tenants t ON t.id=u.tenant_id WHERE u.id=$1 AND u.tenant_id=$2 AND u.role=$3 AND t.status='active' FOR SHARE OF u, t`, p.User, p.Tenant, p.Role).Scan(&name, &hash)
	if errors.Is(err, sql.ErrNoRows) {
		return "", "", ErrDenied
	}
	if err != nil {
		return "", "", ErrUnavailable
	}
	return
}

func audit(ctx context.Context, tx *sql.Tx, p Scope, event string) error {
	r, err := tx.ExecContext(ctx, `INSERT INTO online_session_audit(id, tenant_id, user_id, session_id, event, created_at) VALUES ($1,$2,$3,$4,$5,clock_timestamp())`, uuid.NewString(), p.Tenant, p.User, p.Session, event)
	if err != nil {
		return ErrUnavailable
	}
	n, err := r.RowsAffected()
	if err != nil || n != 1 {
		return ErrUnavailable
	}
	return nil
}

func (s *Store) pair(p Scope, name string, expires, now time.Time) (Tokens, error) {
	refresh, err := refreshSecret(p.Session)
	if err != nil {
		return Tokens{}, err
	}
	end := now.Add(15 * time.Minute)
	if expires.Before(end) {
		end = expires
	}
	seconds := end.Unix() - now.Unix()
	if seconds <= 0 {
		return Tokens{}, ErrDenied
	}
	access, err := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{"sub": p.User, "tenant_id": p.Tenant, "role": p.Role, "name": name, "sid": p.Session, "type": "access", "iat": now.Unix(), "exp": end.Unix()}).SignedString([]byte(s.secret))
	if err != nil {
		return Tokens{}, ErrUnavailable
	}
	return Tokens{Access: access, Refresh: refresh, ExpiresIn: seconds, RefreshExpires: expires}, nil
}
func insertRefresh(ctx context.Context, tx *sql.Tx, p Scope, raw string) error {
	r, err := tx.ExecContext(ctx, `INSERT INTO online_refresh_tokens(digest, session_id, created_at) VALUES ($1,$2,clock_timestamp())`, tokenDigest(raw), p.Session)
	if err != nil {
		return ErrUnavailable
	}
	n, err := r.RowsAffected()
	if err != nil || n != 1 {
		return ErrUnavailable
	}
	return nil
}

// Create rechecks the credential hash under identity/tenant row locks. Nothing is
// returned until session, token hash and audit have committed together.
func (s *Store) Create(ctx context.Context, p Scope, expectedHash string) (Tokens, error) {
	tx, err := s.begin(ctx)
	if err != nil {
		return Tokens{}, err
	}
	defer tx.Rollback()
	if err = lockAccount(ctx, tx, p); err != nil {
		return Tokens{}, err
	}
	name, hash, err := lockIdentity(ctx, tx, p)
	if err != nil {
		return Tokens{}, err
	}
	if hash != expectedHash {
		return Tokens{}, ErrDenied
	}
	p.Session = uuid.NewString()
	var now time.Time
	if tx.QueryRowContext(ctx, `SELECT clock_timestamp()`).Scan(&now) != nil {
		return Tokens{}, ErrUnavailable
	}
	expires := now.Add(7 * 24 * time.Hour)
	tokens, err := s.pair(p, name, expires, now)
	if err != nil {
		return Tokens{}, err
	}
	r, err := tx.ExecContext(ctx, `INSERT INTO online_sessions(id, tenant_id, user_id, role, created_at, expires_at) VALUES ($1,$2,$3,$4,$5,$6)`, p.Session, p.Tenant, p.User, p.Role, now, expires)
	if err != nil {
		return Tokens{}, ErrUnavailable
	}
	n, err := r.RowsAffected()
	if err != nil || n != 1 {
		return Tokens{}, ErrUnavailable
	}
	if err = insertRefresh(ctx, tx, p, tokens.Refresh); err != nil {
		return Tokens{}, err
	}
	if err = audit(ctx, tx, p, "created"); err != nil {
		return Tokens{}, err
	}
	if err = safeCommit(tx); err != nil {
		return Tokens{}, err
	}
	return tokens, nil
}

// Rotate serializes all uses of a family on its session row. Only a matching,
// previously consumed hash may revoke the family; random guesses cannot do so.
func (s *Store) Rotate(ctx context.Context, raw string) (Tokens, error) {
	id, err := ParseRefresh(raw)
	if err != nil {
		return Tokens{}, err
	}
	tx, err := s.begin(ctx)
	if err != nil {
		return Tokens{}, err
	}
	defer tx.Rollback()
	p := Scope{Session: id}
	err = tx.QueryRowContext(ctx, `SELECT tenant_id, user_id FROM online_sessions WHERE id=$1`, id).Scan(&p.Tenant, &p.User)
	if errors.Is(err, sql.ErrNoRows) {
		return Tokens{}, ErrDenied
	}
	if err != nil {
		return Tokens{}, ErrUnavailable
	}
	if err = lockAccount(ctx, tx, p); err != nil {
		return Tokens{}, err
	}
	lockedTenant, lockedUser := p.Tenant, p.User
	var expires, now time.Time
	var revoked sql.NullTime
	err = tx.QueryRowContext(ctx, `SELECT tenant_id, user_id, role, expires_at, revoked_at, clock_timestamp() FROM online_sessions WHERE id=$1 FOR UPDATE`, id).Scan(&p.Tenant, &p.User, &p.Role, &expires, &revoked, &now)
	if errors.Is(err, sql.ErrNoRows) {
		return Tokens{}, ErrDenied
	}
	if err != nil {
		return Tokens{}, ErrUnavailable
	}
	if p.Tenant != lockedTenant || p.User != lockedUser {
		return Tokens{}, ErrDenied
	}
	if revoked.Valid {
		return Tokens{}, ErrDenied
	}
	name, _, err := lockIdentity(ctx, tx, p)
	if err != nil {
		return Tokens{}, err
	}
	// Read time after both locks have been acquired; lock waiting must not
	// extend an expired session using a timestamp from before the wait.
	if tx.QueryRowContext(ctx, `SELECT clock_timestamp()`).Scan(&now) != nil {
		return Tokens{}, ErrUnavailable
	}
	if !expires.After(now) {
		return Tokens{}, ErrDenied
	}
	var consumed sql.NullTime
	err = tx.QueryRowContext(ctx, `SELECT consumed_at FROM online_refresh_tokens WHERE digest=$1 AND session_id=$2`, tokenDigest(raw), id).Scan(&consumed)
	if errors.Is(err, sql.ErrNoRows) {
		return Tokens{}, ErrDenied
	}
	if err != nil {
		return Tokens{}, ErrUnavailable
	}
	if consumed.Valid {
		if err = revokeRow(ctx, tx, p, "replay_revoked"); err != nil {
			return Tokens{}, err
		}
		if err = safeCommit(tx); err != nil {
			return Tokens{}, err
		}
		return Tokens{}, ErrDenied
	}
	tokens, err := s.pair(p, name, expires, now)
	if err != nil {
		return Tokens{}, err
	}
	r, err := tx.ExecContext(ctx, `UPDATE online_refresh_tokens SET consumed_at=clock_timestamp() WHERE digest=$1 AND session_id=$2 AND consumed_at IS NULL`, tokenDigest(raw), id)
	if err != nil {
		return Tokens{}, ErrUnavailable
	}
	n, err := r.RowsAffected()
	if err != nil || n != 1 {
		return Tokens{}, ErrUnavailable
	}
	if err = insertRefresh(ctx, tx, p, tokens.Refresh); err != nil {
		return Tokens{}, err
	}
	if err = audit(ctx, tx, p, "rotated"); err != nil {
		return Tokens{}, err
	}
	if err = safeCommit(tx); err != nil {
		return Tokens{}, err
	}
	return tokens, nil
}

func revokeRow(ctx context.Context, tx *sql.Tx, p Scope, event string) error {
	r, err := tx.ExecContext(ctx, `UPDATE online_sessions SET revoked_at=clock_timestamp() WHERE id=$1 AND tenant_id=$2 AND user_id=$3 AND revoked_at IS NULL`, p.Session, p.Tenant, p.User)
	if err != nil {
		return ErrUnavailable
	}
	n, err := r.RowsAffected()
	if err != nil {
		return ErrUnavailable
	}
	if n == 0 {
		var already bool
		if tx.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM online_sessions WHERE id=$1 AND tenant_id=$2 AND user_id=$3 AND revoked_at IS NOT NULL)`, p.Session, p.Tenant, p.User).Scan(&already) != nil || !already {
			return ErrUnavailable
		}
		return nil
	}
	return audit(ctx, tx, p, event)
}

func (s *Store) List(ctx context.Context, p Scope) ([]Session, error) {
	if s.db == nil {
		return nil, ErrUnavailable
	}
	rows, err := s.db.QueryContext(ctx, `SELECT id, created_at, expires_at, revoked_at FROM online_sessions WHERE tenant_id=$1 AND user_id=$2 ORDER BY created_at DESC, id DESC LIMIT 100`, p.Tenant, p.User)
	if err != nil {
		return nil, ErrUnavailable
	}
	defer rows.Close()
	items := make([]Session, 0)
	for rows.Next() {
		var item Session
		if rows.Scan(&item.ID, &item.CreatedAt, &item.ExpiresAt, &item.RevokedAt) != nil {
			return nil, ErrUnavailable
		}
		item.Current = item.ID == p.Session
		items = append(items, item)
	}
	if rows.Err() != nil {
		return nil, ErrUnavailable
	}
	return items, nil
}

// Revoke requires the actor's session under lock, restricts targets to the same
// user/company, and locks multiple sessions in a deterministic order.
func (s *Store) Revoke(ctx context.Context, p Scope, target string, others bool) error {
	if !ValidID(p.Session) || (!others && !ValidID(target)) {
		return ErrDenied
	}
	tx, err := s.begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err = lockAccount(ctx, tx, p); err != nil {
		return err
	}
	// Lock actor and targets in one ordered query, avoiding two-admin deadlocks.
	rows, err := tx.QueryContext(ctx, `SELECT id, expires_at, revoked_at FROM online_sessions WHERE tenant_id=$1 AND user_id=$2 AND (id=$3 OR ($4 AND revoked_at IS NULL) OR id=$5) ORDER BY id FOR UPDATE`, p.Tenant, p.User, p.Session, others, target)
	if err != nil {
		return ErrUnavailable
	}
	ids := []string{}
	actorActive := false
	var now, actorExpires time.Time
	var actorRevoked bool
	for rows.Next() {
		var id string
		var expires time.Time
		var revoked sql.NullTime
		if rows.Scan(&id, &expires, &revoked) != nil {
			rows.Close()
			return ErrUnavailable
		}
		if id == p.Session {
			actorExpires = expires
			actorRevoked = revoked.Valid
			actorActive = true
		}
		if (others && id != p.Session) || (!others && id == target) {
			ids = append(ids, id)
		}
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return ErrUnavailable
	}
	if tx.QueryRowContext(ctx, `SELECT clock_timestamp()`).Scan(&now) != nil {
		return ErrUnavailable
	}
	if !actorActive || actorRevoked || !actorExpires.After(now) {
		return ErrDenied
	}
	if _, _, err = lockIdentity(ctx, tx, p); err != nil {
		return err
	}
	if !others && len(ids) == 0 {
		return ErrDenied
	}
	for _, id := range ids {
		q := p
		q.Session = id
		event := "revoked"
		if others {
			event = "others_revoked"
		} else if id == p.Session {
			event = "logout"
		}
		if err = revokeRow(ctx, tx, q, event); err != nil {
			return err
		}
	}
	return safeCommit(tx)
}
