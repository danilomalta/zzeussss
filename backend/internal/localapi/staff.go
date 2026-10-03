package localapi

import (
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/gofiber/fiber/v2"
	"golang.org/x/crypto/bcrypt"
	"strings"
	"time"
	"titansystem-backend/internal/core/modules"
	"titansystem-backend/internal/localdb/identity"
	"titansystem-backend/internal/localdb/localauth"
)

type staffInput struct {
	OperationID string `json:"operation_id"`
	IdentityID  string `json:"identity_id"`
	Name        string `json:"name"`
	Role        string `json:"role"`
	Password    string `json:"password"`
}

func (s *Server) createStaff(c *fiber.Ctx) error {
	if s.contracts == nil {
		return c.SendStatus(fiber.StatusServiceUnavailable)
	}
	if len(c.Body()) > 4096 {
		return c.SendStatus(fiber.StatusRequestEntityTooLarge)
	}
	var in staffInput
	if err := decodeCash(c.Body(), &in, []string{"operation_id", "identity_id", "name", "role", "password"}); err != nil {
		return c.SendStatus(fiber.StatusBadRequest)
	}
	in.Name = strings.TrimSpace(in.Name)
	if in.Name == "" || len(in.Name) > 255 || strings.TrimSpace(in.OperationID) == "" || strings.TrimSpace(in.OperationID) != in.OperationID || len(in.OperationID) > 128 || strings.TrimSpace(in.IdentityID) == "" || strings.TrimSpace(in.IdentityID) != in.IdentityID || len(in.IdentityID) > 128 || len(in.Password) < 12 || len(in.Password) > 72 || strings.TrimSpace(in.Password) != in.Password {
		return c.SendStatus(fiber.StatusBadRequest)
	}
	session := c.Locals("session").(localauth.Session)
	tx, err := s.DB.BeginTx(c.UserContext(), nil)
	if err != nil {
		return c.SendStatus(fiber.StatusInternalServerError)
	}
	defer tx.Rollback()
	if err = s.contracts.RequireTx(c.UserContext(), tx, session.Actor, session.Device, identity.ManageStaff, modules.Staff); err != nil {
		return catalogError(c, err)
	}
	var issuer string
	if err = tx.QueryRowContext(c.UserContext(), `SELECT role FROM memberships WHERE tenant_id=? AND identity_id=?`, session.Actor.TenantID, session.Actor.IdentityID).Scan(&issuer); err != nil {
		return c.SendStatus(fiber.StatusInternalServerError)
	}
	permitted := in.Role == "employee" || in.Role == "cashier" || in.Role == "stock"
	if issuer == "owner" {
		permitted = permitted || in.Role == "manager" || in.Role == "production" || in.Role == "accountant" || in.Role == "supplier"
	}
	if !permitted || session.Actor.IdentityID == in.IdentityID {
		return c.SendStatus(fiber.StatusForbidden)
	}
	// Fingerprint excludes password. Retry also checks the stored bcrypt hash.
	body, _ := json.Marshal([]string{in.IdentityID, in.Name, in.Role, session.Actor.StoreID, session.Actor.IdentityID})
	fingerprint := fmt.Sprintf("%x", sha256.Sum256(body))
	var existing, actor, hash string
	err = tx.QueryRowContext(c.UserContext(), `SELECT identity_id,created_by,request_hash FROM staff_registrations WHERE tenant_id=? AND device_id=? AND operation_id=?`, session.Actor.TenantID, session.Device.DeviceID, in.OperationID).Scan(&existing, &actor, &hash)
	if err == nil {
		if existing != in.IdentityID || actor != session.Actor.IdentityID || hash != fingerprint {
			return c.SendStatus(fiber.StatusConflict)
		}
		var stored []byte
		if err = tx.QueryRowContext(c.UserContext(), `SELECT password_hash FROM local_passwords WHERE tenant_id=? AND identity_id=?`, session.Actor.TenantID, in.IdentityID).Scan(&stored); err != nil {
			return c.SendStatus(fiber.StatusConflict)
		}
		if bcrypt.CompareHashAndPassword(stored, []byte(in.Password)) != nil {
			return c.SendStatus(fiber.StatusConflict)
		}
		if err = tx.Commit(); err != nil {
			return c.SendStatus(fiber.StatusInternalServerError)
		}
		return c.JSON(fiber.Map{"identity_id": in.IdentityID, "repeated": true})
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return c.SendStatus(fiber.StatusInternalServerError)
	}
	hashPassword, err := localauth.HashPassword(in.Password)
	if err != nil {
		return c.SendStatus(fiber.StatusBadRequest)
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	for _, stmt := range []struct {
		query string
		args  []any
	}{
		{`INSERT INTO identities VALUES (?,?,?)`, []any{in.IdentityID, in.Name, now}},
		{`INSERT INTO memberships VALUES (?,?,?,'active',?)`, []any{session.Actor.TenantID, in.IdentityID, in.Role, now}},
		{`INSERT INTO membership_stores VALUES (?,?,?)`, []any{session.Actor.TenantID, in.IdentityID, session.Actor.StoreID}},
		{`INSERT INTO local_passwords VALUES (?,?,?,?)`, []any{session.Actor.TenantID, in.IdentityID, hashPassword, now}},
		{`INSERT INTO staff_registrations VALUES (?,?,?,?,?,?,?,?)`, []any{session.Actor.TenantID, session.Actor.StoreID, session.Device.DeviceID, in.OperationID, in.IdentityID, session.Actor.IdentityID, fingerprint, now}},
	} {
		result, err := tx.ExecContext(c.UserContext(), stmt.query, stmt.args...)
		if err != nil {
			return c.SendStatus(fiber.StatusConflict)
		}
		count, err := result.RowsAffected()
		if err != nil || count != 1 {
			return c.SendStatus(fiber.StatusInternalServerError)
		}
	}
	if err = tx.Commit(); err != nil {
		return c.SendStatus(fiber.StatusInternalServerError)
	}
	return c.Status(fiber.StatusCreated).JSON(fiber.Map{"identity_id": in.IdentityID, "repeated": false})
}

func (s *Server) listStaff(c *fiber.Ctx) error {
	session := c.Locals("session").(localauth.Session)
	if s.contracts == nil {
		return c.SendStatus(fiber.StatusServiceUnavailable)
	}
	tx, err := s.DB.BeginTx(c.UserContext(), nil)
	if err != nil {
		return c.SendStatus(fiber.StatusInternalServerError)
	}
	defer tx.Rollback()
	if err = s.contracts.RequireTx(c.UserContext(), tx, session.Actor, session.Device, identity.ManageStaff, modules.Staff); err != nil {
		return catalogError(c, err)
	}
	rows, err := tx.QueryContext(c.UserContext(), `SELECT i.id,i.display_name,m.role,m.status FROM identities i JOIN memberships m ON m.identity_id=i.id JOIN membership_stores ms ON ms.tenant_id=m.tenant_id AND ms.identity_id=m.identity_id WHERE m.tenant_id=? AND ms.store_id=? ORDER BY i.display_name,i.id LIMIT 100`, session.Actor.TenantID, session.Actor.StoreID)
	if err != nil {
		return c.SendStatus(fiber.StatusInternalServerError)
	}
	items := make([]fiber.Map, 0)
	for rows.Next() {
		var id, name, role, status string
		if err = rows.Scan(&id, &name, &role, &status); err != nil {
			rows.Close()
			return c.SendStatus(fiber.StatusInternalServerError)
		}
		items = append(items, fiber.Map{"identity_id": id, "name": name, "role": role, "status": status})
	}
	err = rows.Err()
	closeErr := rows.Close()
	if err != nil || closeErr != nil {
		return c.SendStatus(fiber.StatusInternalServerError)
	}
	if err = tx.Commit(); err != nil {
		return c.SendStatus(fiber.StatusInternalServerError)
	}
	return c.JSON(fiber.Map{"items": items})
}
