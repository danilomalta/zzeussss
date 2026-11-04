package fiscalproduction

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"time"
	"titansystem-backend/internal/fiscalcore"

	"titansystem-backend/internal/core/modules"
	"titansystem-backend/internal/localdb"
	"titansystem-backend/internal/localdb/fiscal"
	"titansystem-backend/internal/localdb/identity"
	"titansystem-backend/internal/sqlitewrite"
)

// QueryCorrection only consults the existing NF-e reference. It cannot submit
// another correction, choose a new sequence or reconstruct a request on a client.
func (p *FocusProvider) QueryCorrection(ctx context.Context, a RequestContext, in DocumentRequest) (CorrectionResult, error) {
	if in.Model != "55" || in.Environment != 2 {
		return CorrectionResult{}, ErrCheckoutProduction
	}
	if err := p.validateModel(ctx, a, 2, "55"); err != nil {
		return CorrectionResult{}, err
	}
	ref := focusReference(a.Actor.TenantID, in.SaleID)
	hash, err := correctionClaimHash(ctx, a, in.SaleID, ref)
	if errors.Is(err, sql.ErrNoRows) {
		return CorrectionResult{State: "correction_not_requested"}, nil
	}
	if err != nil {
		return CorrectionResult{}, err
	}
	previous, err := readCorrectionResult(ctx, a, in.SaleID, ref)
	if err != nil {
		return CorrectionResult{}, err
	}
	if previous.State == "correction_failed" {
		return previous, nil
	}
	current, err := p.Query(ctx, a, in) // Existing GET-only recovery, never Emit/Correct.
	if err != nil {
		return CorrectionResult{State: "correction_reconciliation_required"}, err
	}
	return p.reconcileCorrection(ctx, a, in.SaleID, ref, hash, previous, current)
}

func correctionClaimHash(ctx context.Context, a RequestContext, sale, ref string) (string, error) {
	var raw string
	err := a.DB.QueryRowContext(ctx, `SELECT payload_json FROM outbox WHERE tenant_id=? AND store_id=? AND device_id=? AND aggregate_id=? AND operation_id=? AND event_type='checkout.focus_correction'`, a.Actor.TenantID, a.Actor.StoreID, a.Device.DeviceID, sale, ref+"-cce").Scan(&raw)
	if err != nil {
		return "", err
	}
	var claim struct {
		Reference string `json:"ref"`
		Hash      string `json:"request_sha256"`
	}
	if json.Unmarshal([]byte(raw), &claim) != nil || claim.Reference != ref+"-cce" {
		return "", fiscal.ErrIntegrity
	}
	decoded, err := hex.DecodeString(claim.Hash)
	if err != nil || len(decoded) != sha256.Size {
		return "", fiscal.ErrIntegrity
	}
	return claim.Hash, nil
}

// Reconciliation records are immutable and use the existing result event type.
// Archived evidence takes precedence over the earlier uncertain POST response.
func readCorrectionResult(ctx context.Context, a RequestContext, sale, ref string) (CorrectionResult, error) {
	var raw string
	err := a.DB.QueryRowContext(ctx, `SELECT payload_json FROM outbox WHERE tenant_id=? AND store_id=? AND device_id=? AND aggregate_id=? AND event_type='checkout.focus_correction_result' AND operation_id IN(?,?,?) ORDER BY CASE operation_id WHEN ? THEN 0 WHEN ? THEN 1 ELSE 2 END LIMIT 1`, a.Actor.TenantID, a.Actor.StoreID, a.Device.DeviceID, sale, ref+"-cce-result", ref+"-cce-query", ref+"-cce-query-archived", ref+"-cce-query-archived", ref+"-cce-query").Scan(&raw)
	if errors.Is(err, sql.ErrNoRows) {
		return CorrectionResult{State: "correction_reconciliation_required"}, nil
	}
	var out CorrectionResult
	if err != nil || json.Unmarshal([]byte(raw), &out) != nil {
		return CorrectionResult{}, ErrUnavailable
	}
	if out.State != "correction_authorized_homologation" && out.State != "correction_failed" && out.State != "correction_reconciliation_required" {
		return CorrectionResult{}, fiscal.ErrIntegrity
	}
	return out, nil
}

func (p *FocusProvider) reconcileCorrection(ctx context.Context, a RequestContext, sale, ref, hash string, previous CorrectionResult, current DocumentResult) (CorrectionResult, error) {
	pending := CorrectionResult{State: "correction_reconciliation_required"}
	var links focusArchiveLinks
	if current.Key == "" || current.IntentID == "" || json.Unmarshal(current.Evidence, &links) != nil || links.Correction == "" || links.CorrectionNumber < 1 || links.CorrectionNumber > 20 {
		return pending, nil
	}
	// With no definitive POST result we only reconcile the first sequence. A
	// later remote history is not guessed from the latest link or the current date.
	expected := previous.Number
	if expected == 0 {
		expected = 1
	}
	if links.CorrectionNumber != expected {
		return pending, nil
	}
	data, err := p.downloadArchiveXML(ctx, links.Correction, current.Key, "correction", expected)
	if err != nil {
		return pending, nil
	}
	defer clear(data)
	// Existing download validates XSD, key, homologation and event/sequence.
	// The exact text must also match the durable original request fingerprint.
	var doc struct {
		Event struct {
			Info struct {
				ID     string `xml:"Id,attr"`
				CNPJ   string `xml:"CNPJ"`
				Detail struct {
					Text struct {
						XMLName xml.Name
						Value   string `xml:",chardata"`
					} `xml:"xCorrecao"`
				} `xml:"detEvento"`
			} `xml:"infEvento"`
		} `xml:"evento"`
		Result struct {
			Info struct {
				Protocol string `xml:"nProt"`
			} `xml:"infEvento"`
		} `xml:"retEvento"`
	}
	if xml.Unmarshal(data, &doc) != nil || doc.Event.Info.CNPJ != current.Key[6:20] || doc.Event.Info.ID != "ID110110"+current.Key+fmt.Sprintf("%02d", expected) || doc.Result.Info.Protocol == "" || doc.Event.Info.Detail.Text.XMLName.Space != fiscalcore.NFeNS || !transportText(doc.Event.Info.Detail.Text.Value, 15, 1000) {
		return pending, nil
	}
	body, _ := json.Marshal(map[string]string{"correcao": doc.Event.Info.Detail.Text.Value})
	sum := sha256.Sum256(body)
	clear(body)
	if hex.EncodeToString(sum[:]) != hash {
		return pending, nil
	}
	out := CorrectionResult{State: "correction_authorized_homologation", Number: expected, ArchiveState: "pending"}
	if err = p.storeArchive(ctx, a, sale, current.IntentID, current.Key, "correction", expected, data); err == nil {
		out.ArchiveState = "archived"
	}
	// Recheck authorization after HTTP, including when XML archival failed.
	if err = p.scope(ctx, a, identity.CancelSale); err != nil {
		return pending, err
	}
	if err = p.recordCorrectionReconciliation(ctx, a, sale, ref, hash, out); err != nil {
		return pending, err
	}
	return out, nil
}

func (p *FocusProvider) recordCorrectionReconciliation(ctx context.Context, a RequestContext, sale, ref, hash string, out CorrectionResult) error {
	operation := ref + "-cce-query"
	if out.ArchiveState == "archived" {
		operation += "-archived"
	}
	raw, _ := json.Marshal(out)
	return sqlitewrite.Run(ctx, a.DB, fiscal.ErrConflict, func(tx *sql.Tx) error {
		if err := fiscal.ValidateSaleSessionTx(ctx, tx, a.Actor, a.Device, a.Token); err != nil {
			return err
		}
		if err := a.License.RequireTx(ctx, tx, a.Actor, a.Device, identity.CancelSale, modules.Fiscal); err != nil {
			return err
		}
		var role string
		if err := tx.QueryRowContext(ctx, `SELECT role FROM memberships WHERE tenant_id=? AND identity_id=? AND status='active'`, a.Actor.TenantID, a.Actor.IdentityID).Scan(&role); err != nil {
			return err
		}
		if role != "owner" {
			return identity.ErrDenied
		}
		var barriers int
		if err := tx.QueryRowContext(ctx, `SELECT count(*) FROM fiscal_restore_barriers WHERE tenant_id=?`, a.Actor.TenantID).Scan(&barriers); err != nil {
			return err
		}
		if barriers != 0 {
			return fiscal.ErrRestored
		}
		var marker string
		if err := tx.QueryRowContext(ctx, `SELECT payload_json FROM outbox WHERE tenant_id=? AND store_id=? AND device_id=? AND aggregate_id=? AND operation_id=? AND event_type='checkout.focus_correction'`, a.Actor.TenantID, a.Actor.StoreID, a.Device.DeviceID, sale, ref+"-cce").Scan(&marker); err != nil {
			return err
		}
		var claim struct {
			Hash string `json:"request_sha256"`
		}
		if json.Unmarshal([]byte(marker), &claim) != nil || claim.Hash != hash {
			return fiscal.ErrConflict
		}
		var old string
		err := tx.QueryRowContext(ctx, `SELECT payload_json FROM outbox WHERE tenant_id=? AND store_id=? AND device_id=? AND aggregate_id=? AND operation_id=? AND event_type='checkout.focus_correction_result'`, a.Actor.TenantID, a.Actor.StoreID, a.Device.DeviceID, sale, operation).Scan(&old)
		if err == nil {
			if old != string(raw) {
				return fiscal.ErrConflict
			}
			return nil
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		id, err := localdb.NewID()
		if err != nil {
			return err
		}
		_, err = tx.ExecContext(ctx, `INSERT INTO outbox(event_id,tenant_id,store_id,device_id,operation_id,aggregate_id,event_type,schema_version,payload_json,created_at) VALUES(?,?,?,?,?,?,'checkout.focus_correction_result',1,?,?)`, id, a.Actor.TenantID, a.Actor.StoreID, a.Device.DeviceID, operation, sale, string(raw), time.Now().UTC().Format(time.RFC3339Nano))
		return err
	})
}
