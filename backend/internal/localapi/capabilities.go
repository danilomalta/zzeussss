package localapi

import (
	"database/sql"
	"errors"
	"github.com/gofiber/fiber/v2"
	"titansystem-backend/internal/core/entitlements"
	"titansystem-backend/internal/core/modules"
	"titansystem-backend/internal/localdb/entitlementstore"
	"titansystem-backend/internal/localdb/identity"
	"titansystem-backend/internal/localdb/localauth"
)

func (s *Server) capabilities(c *fiber.Ctx) error {
	session := c.Locals("session").(localauth.Session)
	tx, err := s.DB.BeginTx(c.UserContext(), nil)
	if err != nil {
		return c.SendStatus(fiber.StatusInternalServerError)
	}
	defer tx.Rollback()
	var role string
	err = tx.QueryRowContext(c.UserContext(), `SELECT m.role FROM memberships m JOIN device_pairings p ON p.tenant_id=m.tenant_id
 WHERE m.tenant_id=? AND m.identity_id=? AND m.status='active' AND p.store_id=? AND p.device_id=? AND p.status='approved'
 AND (m.role='owner' OR EXISTS (SELECT 1 FROM membership_stores ms WHERE ms.tenant_id=m.tenant_id AND ms.identity_id=m.identity_id AND ms.store_id=p.store_id))`, session.Actor.TenantID, session.Actor.IdentityID, session.Actor.StoreID, session.Device.DeviceID).Scan(&role)
	if errors.Is(err, sql.ErrNoRows) {
		return c.SendStatus(fiber.StatusUnauthorized)
	}
	if err != nil {
		return c.SendStatus(fiber.StatusInternalServerError)
	}
	permissions := make([]identity.Permission, 0)
	for _, p := range []identity.Permission{identity.ViewCatalog, identity.Sell, identity.ManageStock, identity.ManageStaff, identity.ViewAccounting, identity.ManageProduction, identity.ViewOrders, identity.ManageReplenishment, identity.ManageCash, identity.CancelSale} {
		err = identity.CanOperateTx(c.UserContext(), tx, session.Actor, session.Device, p)
		if err == nil {
			permissions = append(permissions, p)
		} else if !errors.Is(err, identity.ErrDenied) {
			return c.SendStatus(fiber.StatusInternalServerError)
		}
	}
	license := entitlementstore.Status{State: "unavailable", Modules: []modules.ID{}}
	if s.contracts != nil {
		license, err = s.contracts.StatusTx(c.UserContext(), tx, session.Actor.TenantID)
		if err != nil {
			switch {
			case errors.Is(err, entitlementstore.ErrNotInstalled):
				license = entitlementstore.Status{State: "not_installed", Modules: []modules.ID{}}
			case errors.Is(err, entitlementstore.ErrClockRollback):
				license = entitlementstore.Status{State: "clock_blocked", Modules: []modules.ID{}}
			case errors.Is(err, entitlementstore.ErrCorrupt), errors.Is(err, entitlements.ErrSignature), errors.Is(err, entitlements.ErrTrust), errors.Is(err, entitlements.ErrClaims), errors.Is(err, entitlements.ErrTenant), errors.Is(err, entitlements.ErrValidity):
				license = entitlementstore.Status{State: "invalid", Modules: []modules.ID{}}
			default:
				return c.SendStatus(fiber.StatusInternalServerError)
			}
		}
	}
	if err = tx.Commit(); err != nil {
		return c.SendStatus(fiber.StatusInternalServerError)
	}
	return c.JSON(fiber.Map{"tenant_id": session.Actor.TenantID, "store_id": session.Actor.StoreID, "device_id": session.Device.DeviceID, "identity_id": session.Actor.IdentityID, "role": role, "permissions": permissions, "license": license})
}
