package localapi

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"time"

	"github.com/gofiber/fiber/v2"
	"titansystem-backend/internal/localdb"
	"titansystem-backend/internal/localdb/identity"
	"titansystem-backend/internal/localdb/localauth"
	"titansystem-backend/internal/sqlitewrite"
)

var errExportAudit = errors.New("export audit not persisted")

// Audits the artifact prepared for a response, NOT proof of receipt by a client.
// No document content, customer name, token or private path enters this journal.
// It shares the existing local outbox, never the financial ledger or a provider.
func (s *Server) recordReportExport(c *fiber.Ctx, kind, from, to string, data []byte) error {
	switch kind {
	case "management_sales_page", "closed_cash_period", "fiscal_homologation_period":
	default:
		return errExportAudit
	}
	ss := c.Locals("session").(localauth.Session)
	id, err := localdb.NewID()
	if err != nil {
		return err
	}
	digest := sha256.Sum256(data)
	raw, err := json.Marshal(struct {
		Kind   string `json:"kind"`
		Actor  string `json:"actor_id"`
		From   string `json:"from"`
		To     string `json:"to"`
		SHA256 string `json:"sha256"`
	}{kind, ss.Actor.IdentityID, from, to, hex.EncodeToString(digest[:])})
	if err != nil {
		return err
	}
	return sqlitewrite.Run(c.UserContext(), s.DB, errExportAudit, func(tx *sql.Tx) error {
		current, err := localauth.ResolveTx(c.UserContext(), tx, ss.Token)
		if err != nil || current.Actor != ss.Actor || current.Device != ss.Device || current.Device != s.Device {
			return identity.ErrDenied
		}
		if err = identity.CanOperateTx(c.UserContext(), tx, current.Actor, current.Device, identity.ViewAccounting); err != nil {
			return err
		}
		result, err := tx.ExecContext(c.UserContext(), `INSERT INTO outbox(event_id,tenant_id,store_id,device_id,operation_id,aggregate_id,event_type,schema_version,payload_json,created_at) VALUES(?,?,?,?,?,?,'audit.report_export',1,?,?)`, id, current.Actor.TenantID, current.Actor.StoreID, current.Device.DeviceID, id, id, string(raw), time.Now().UTC().Format(time.RFC3339Nano))
		if err != nil {
			return err
		}
		n, err := result.RowsAffected()
		if err != nil {
			return err
		}
		if n != 1 {
			return errExportAudit
		}
		return nil
	})
}
