package fiscal

import (
	"context"
	"database/sql"
	"errors"

	"titansystem-backend/internal/fiscalcore"
	"titansystem-backend/internal/localdb/identity"
)

var ErrDocumentUnavailable = errors.New("fiscal document is not printable from current evidence")

// ReadHomologationDANFE selects the actual archived document, never XML supplied
// by the HTTP caller. Historical printing needs current read authority, not an
// active issuance license or the original signing key. It does not clear the
// Restore barrier. A seller can only read their own actor/device's attempt.
func ReadHomologationDANFE(ctx context.Context, db *sql.DB, a identity.Scope, d identity.DeviceContext, intentID, token string, opts fiscalcore.PrintOptions) (fiscalcore.DANFEArtifact, error) {
	var out fiscalcore.DANFEArtifact
	if db == nil || !idPattern.MatchString(intentID) || fiscalcore.ValidatePrintOptions(opts) != nil {
		return out, ErrInvalid
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return out, err
	}
	defer tx.Rollback()
	if err = fiscalSessionTx(ctx, tx, a, d, token); err != nil {
		return out, err
	}
	all, err := readPermission(ctx, tx, a, d)
	if err != nil {
		return out, err
	}
	intent, err := intentTx(ctx, tx, a, intentID)
	if err != nil {
		return out, err
	}
	if !all && (intent.ActorID != a.IdentityID || intent.DeviceID != d.DeviceID) {
		return out, ErrNotFound
	}
	original, result, err := originalTransmissionTx(ctx, tx, a, intentID)
	if err != nil {
		return out, err
	}
	var count int
	// Do not print an old active-looking document once cancellation has begun,
	// even if its result is uncertain. A future cancelled-copy layout is distinct.
	if err = tx.QueryRowContext(ctx, `SELECT count(*) FROM fiscal_cancellation_requests WHERE tenant_id=? AND store_id=? AND intent_id=?`, a.TenantID, a.StoreID, intentID).Scan(&count); err != nil {
		return out, err
	}
	if count != 0 {
		return out, ErrDocumentUnavailable
	}
	var queryID string
	err = tx.QueryRowContext(ctx, `SELECT query_id FROM fiscal_query_requests WHERE tenant_id=? AND store_id=? AND intent_id=? ORDER BY rowid DESC LIMIT 1`, a.TenantID, a.StoreID, intentID).Scan(&queryID)
	var process []byte
	if errors.Is(err, sql.ErrNoRows) {
		evidence, e := fiscalcore.ReconcileHomologationIssuance(original, result)
		if e != nil || evidence.Protocol == nil || len(evidence.ProcessedXML) == 0 {
			return out, ErrDocumentUnavailable
		}
		process = evidence.ProcessedXML
	} else if err != nil {
		return out, err
	} else {
		// Latest status wins the read boundary, not response completion order.
		// A pending/newer query never falls back to an older authorization.
		evidence, _, e := cancellationAuthorizationTx(ctx, tx, a, d, intentID, queryID, token, true)
		if e != nil {
			return out, ErrDocumentUnavailable
		}
		process = evidence.ProcessedXML
	}
	out, err = fiscalcore.RenderHomologationNFCe(process, opts)
	if err != nil {
		return fiscalcore.DANFEArtifact{}, ErrDocumentUnavailable
	}
	return out, tx.Commit()
}
