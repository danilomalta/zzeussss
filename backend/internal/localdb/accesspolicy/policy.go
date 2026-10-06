// Package accesspolicy manages local, store-scoped access policies. Every
// mutation proves the session, device and current password inside its transaction.
package accesspolicy

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
	"titansystem-backend/internal/localdb/identity"
	"titansystem-backend/internal/localdb/localauth"
)

var (
	ErrDenied   = errors.New("policy administration denied")
	ErrInput    = errors.New("invalid policy input")
	ErrConflict = errors.New("policy revision or operation conflict")
	ErrWrite    = errors.New("policy write did not persist")
)

// All fields are required by the HTTP layer. Unused fields must be empty.
// Revision is the expected CURRENT revision (0 means not yet created).
type Input struct {
	OperationID      string              `json:"operation_id"`
	CurrentPassword  string              `json:"current_password"`
	Kind             string              `json:"kind"`
	DepartmentID     string              `json:"department_id"`
	TargetID         string              `json:"target_id"`
	Permission       identity.Permission `json:"permission"`
	Value            string              `json:"value"`
	Name             string              `json:"name"`
	ExpectedRevision int64               `json:"expected_revision"`
	Reason           string              `json:"reason"`
}
type Result struct {
	OperationID string `json:"operation_id"`
	Revision    int64  `json:"revision"`
	Repeated    bool   `json:"repeated"`
}

func validID(s string) bool {
	return s != "" && len(s) <= 128 && strings.TrimSpace(s) == s && !strings.ContainsAny(s, "\x00\r\n")
}
func validate(in Input) error {
	if !validID(in.OperationID) || !validID(in.DepartmentID) || in.ExpectedRevision < 0 || in.ExpectedRevision >= 1<<53 || strings.TrimSpace(in.Reason) == "" || len(in.Reason) > 255 || strings.ContainsAny(in.Reason, "\x00\r\n") || len(in.CurrentPassword) > 72 || len(in.CurrentPassword) < 12 {
		return ErrInput
	}
	switch in.Kind {
	case "department":
		if in.TargetID != "" || in.Permission != "" || strings.TrimSpace(in.Name) == "" || len(in.Name) > 255 || strings.ContainsAny(in.Name, "\x00\r\n") || (in.Value != "active" && in.Value != "inactive") {
			return ErrInput
		}
	case "member":
		if !validID(in.TargetID) || in.Permission != "" || in.Name != "" || in.Value != "assigned" {
			return ErrInput
		}
	case "rule":
		if !identity.KnownPermission(in.Permission) || in.Name != "" || (in.TargetID != "" && !validID(in.TargetID)) || (in.Value != "allow" && in.Value != "deny" && in.Value != "inherit") {
			return ErrInput
		}
	case "delegation":
		if !validID(in.TargetID) || in.Permission != "" || in.Name != "" || (in.Value != "active" && in.Value != "revoked") {
			return ErrInput
		}
	default:
		return ErrInput
	}
	return nil
}

func sessionTx(ctx context.Context, tx *sql.Tx, device identity.DeviceContext, token string) (localauth.Session, string, error) {
	session, err := localauth.ResolveTx(ctx, tx, token)
	if err != nil {
		return session, "", err
	}
	if session.Device != device {
		return session, "", localauth.ErrDenied
	}
	var role string
	err = tx.QueryRowContext(ctx, `SELECT role FROM memberships WHERE tenant_id=? AND identity_id=? AND status='active'`, device.TenantID, session.Actor.IdentityID).Scan(&role)
	return session, role, err
}

func delegated(ctx context.Context, tx *sql.Tx, a identity.Scope, dept string) (bool, error) {
	var count int
	err := tx.QueryRowContext(ctx, `SELECT count(*) FROM access_delegations x JOIN access_departments d
 ON d.tenant_id=x.tenant_id AND d.store_id=x.store_id AND d.id=x.department_id
 WHERE x.tenant_id=? AND x.store_id=? AND x.identity_id=? AND x.department_id=?
 AND x.status='active' AND d.status='active'`, a.TenantID, a.StoreID, a.IdentityID, dept).Scan(&count)
	return count == 1, err
}

func targetRole(ctx context.Context, tx *sql.Tx, a identity.Scope, id string) (string, error) {
	var role string
	err := tx.QueryRowContext(ctx, `SELECT m.role FROM memberships m JOIN membership_stores ms
 ON ms.tenant_id=m.tenant_id AND ms.identity_id=m.identity_id
 WHERE m.tenant_id=? AND m.identity_id=? AND m.status='active' AND ms.store_id=?`, a.TenantID, id, a.StoreID).Scan(&role)
	if errors.Is(err, sql.ErrNoRows) {
		return "", ErrDenied
	}
	return role, err
}

func authorize(ctx context.Context, tx *sql.Tx, s localauth.Session, role string, in Input) error {
	a := s.Actor
	if in.TargetID == a.IdentityID {
		return ErrDenied
	} // no self-assignment or self-escalation
	if in.Kind != "department" {
		var status string
		err := tx.QueryRowContext(ctx, `SELECT status FROM access_departments WHERE tenant_id=? AND store_id=? AND id=?`, a.TenantID, a.StoreID, in.DepartmentID).Scan(&status)
		if errors.Is(err, sql.ErrNoRows) {
			return ErrDenied
		}
		if err != nil {
			return err
		}
		if status != "active" {
			return ErrDenied
		}
	}
	if in.TargetID != "" {
		target, err := targetRole(ctx, tx, a, in.TargetID)
		if err != nil {
			return err
		}
		if target == "owner" {
			return ErrDenied
		}
	}
	// The owner defines departments, delegates and their ceilings. Legacy manager
	// status alone does not grant the right to administer policies.
	if role == "owner" {
		return nil
	}
	if in.Kind == "department" || in.Kind == "delegation" || (in.Kind == "rule" && (in.TargetID == "" || in.Permission == identity.ManageStaff)) {
		return ErrDenied
	}
	yes, err := delegated(ctx, tx, a, in.DepartmentID)
	if err != nil {
		return err
	}
	if !yes {
		return ErrDenied
	}
	target, err := targetRole(ctx, tx, a, in.TargetID)
	if err != nil {
		return err
	}
	if target != "employee" && target != "cashier" && target != "stock" && target != "production" {
		return ErrDenied
	}
	var existing string
	err = tx.QueryRowContext(ctx, `SELECT department_id FROM access_department_members WHERE tenant_id=? AND store_id=? AND identity_id=?`, a.TenantID, a.StoreID, in.TargetID).Scan(&existing)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	if existing != "" && existing != in.DepartmentID {
		return ErrDenied
	}
	if in.Kind == "rule" {
		if existing != in.DepartmentID {
			return ErrDenied
		}
		if err := identity.CanOperateTx(ctx, tx, a, s.Device, in.Permission); err != nil {
			if errors.Is(err, identity.ErrDenied) {
				return ErrDenied
			}
			return err
		}
	}
	return nil
}

func Mutate(ctx context.Context, db *sql.DB, device identity.DeviceContext, token string, in Input) (Result, error) {
	var out Result
	if err := validate(in); err != nil {
		return out, err
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return out, err
	}
	defer tx.Rollback()
	session, role, err := sessionTx(ctx, tx, device, token)
	if err != nil {
		return out, err
	}
	if err = authorize(ctx, tx, session, role, in); err != nil {
		return out, err
	}
	var hash []byte
	if err = tx.QueryRowContext(ctx, `SELECT password_hash FROM local_passwords WHERE tenant_id=? AND identity_id=?`, device.TenantID, session.Actor.IdentityID).Scan(&hash); err != nil {
		return out, err
	}
	if bcrypt.CompareHashAndPassword(hash, []byte(in.CurrentPassword)) != nil {
		return out, localauth.ErrDenied
	}
	public := in
	public.CurrentPassword = "" // passwords never enter the persisted fingerprint
	bytes, _ := json.Marshal(struct {
		Actor string
		Input Input
	}{session.Actor.IdentityID, public})
	fingerprint := fmt.Sprintf("%x", sha256.Sum256(bytes))
	var oldHash string
	err = tx.QueryRowContext(ctx, `SELECT request_hash,revision FROM access_policy_events WHERE tenant_id=? AND device_id=? AND operation_id=?`, device.TenantID, device.DeviceID, in.OperationID).Scan(&oldHash, &out.Revision)
	if err == nil {
		if oldHash != fingerprint {
			return Result{}, ErrConflict
		}
		out.OperationID = in.OperationID
		out.Repeated = true
		return out, tx.Commit() // do not replay a stale write over newer policy
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return out, err
	}
	before, revision, err := writePolicy(ctx, tx, session.Actor, in)
	if err != nil {
		return out, err
	}
	after := in.Value
	if in.Kind == "department" {
		after = in.Name + " / " + in.Value
	}
	res, err := tx.ExecContext(ctx, `INSERT INTO access_policy_events VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`, device.TenantID, device.StoreID, device.DeviceID, in.OperationID, session.Actor.IdentityID, in.Kind, in.DepartmentID, in.TargetID, string(in.Permission), before, after, revision, in.Reason, fingerprint, time.Now().Unix())
	if err != nil {
		return out, err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return out, err
	}
	if n != 1 {
		return out, ErrWrite
	}
	out = Result{in.OperationID, revision, false}
	return out, tx.Commit()
}

func writePolicy(ctx context.Context, tx *sql.Tx, a identity.Scope, in Input) (string, int64, error) {
	var before string
	var rev int64
	var query, update, insert string
	var key, values []any
	switch in.Kind {
	case "department":
		key = []any{a.TenantID, a.StoreID, in.DepartmentID}
		query = `SELECT name || ' / ' || status,revision FROM access_departments WHERE tenant_id=? AND store_id=? AND id=?`
		update = `UPDATE access_departments SET name=?,status=?,revision=? WHERE tenant_id=? AND store_id=? AND id=? AND revision=?`
		insert = `INSERT INTO access_departments VALUES (?,?,?,?,?,?)`
		values = []any{in.Name, in.Value}
	case "member":
		key = []any{a.TenantID, a.StoreID, in.TargetID}
		query = `SELECT department_id,revision FROM access_department_members WHERE tenant_id=? AND store_id=? AND identity_id=?`
		update = `UPDATE access_department_members SET department_id=?,revision=? WHERE tenant_id=? AND store_id=? AND identity_id=? AND revision=?`
		insert = `INSERT INTO access_department_members VALUES (?,?,?,?,?)`
		values = []any{in.DepartmentID}
	case "delegation":
		key = []any{a.TenantID, a.StoreID, in.DepartmentID, in.TargetID}
		query = `SELECT status,revision FROM access_delegations WHERE tenant_id=? AND store_id=? AND department_id=? AND identity_id=?`
		update = `UPDATE access_delegations SET status=?,revision=? WHERE tenant_id=? AND store_id=? AND department_id=? AND identity_id=? AND revision=?`
		insert = `INSERT INTO access_delegations VALUES (?,?,?,?,?,?)`
		values = []any{in.Value}
	case "rule":
		kind, target := "member", in.TargetID
		if target == "" {
			kind, target = "department", in.DepartmentID
		}
		if in.TargetID != "" {
			var dept string
			err := tx.QueryRowContext(ctx, `SELECT department_id FROM access_department_members WHERE tenant_id=? AND store_id=? AND identity_id=?`, a.TenantID, a.StoreID, in.TargetID).Scan(&dept)
			if errors.Is(err, sql.ErrNoRows) || (err == nil && dept != in.DepartmentID) {
				return "", 0, ErrDenied
			}
			if err != nil {
				return "", 0, err
			}
		}
		key = []any{a.TenantID, a.StoreID, kind, target, string(in.Permission)}
		query = `SELECT effect,revision FROM access_rules WHERE tenant_id=? AND store_id=? AND target_kind=? AND target_id=? AND permission=?`
		update = `UPDATE access_rules SET effect=?,revision=? WHERE tenant_id=? AND store_id=? AND target_kind=? AND target_id=? AND permission=? AND revision=?`
		insert = `INSERT INTO access_rules VALUES (?,?,?,?,?,?,?)`
		values = []any{in.Value}
	}
	err := tx.QueryRowContext(ctx, query, key...).Scan(&before, &rev)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return "", 0, err
	}
	if rev != in.ExpectedRevision {
		return "", 0, ErrConflict
	}
	next := rev + 1
	var args []any
	if rev == 0 {
		args = append(args, key...)
		args = append(args, values...)
		args = append(args, next)
		query = insert
	} else {
		args = append(args, values...)
		args = append(args, next)
		args = append(args, key...)
		args = append(args, rev)
		query = update
	}
	res, err := tx.ExecContext(ctx, query, args...)
	if err != nil {
		return "", 0, err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return "", 0, err
	}
	if n != 1 {
		return "", 0, ErrWrite
	}
	return before, next, nil
}
