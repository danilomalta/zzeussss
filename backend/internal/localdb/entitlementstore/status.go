package entitlementstore

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	"titansystem-backend/internal/core/entitlements"
	"titansystem-backend/internal/core/modules"
)

type Status struct {
	State         string       `json:"state"`
	Modules       []modules.ID `json:"modules"`
	ExpiresUnix   int64        `json:"expires_unix,omitempty"`
	RemainingDays int64        `json:"remaining_days"`
}

// StatusTx only displays authenticated contract metadata. It never grants writes;
// mutations must continue to use RequireTx. Caller scopes tenant to its session.
func (s *Store) StatusTx(ctx context.Context, tx *sql.Tx, tenantID string) (Status, error) {
	result := Status{State: "unavailable", Modules: make([]modules.ID, 0)}
	if s == nil || s.verifier == nil || tx == nil {
		return result, ErrNotInstalled
	}
	record, err := load(ctx, tx, tenantID)
	if err != nil {
		return result, err
	}
	// This untrusted timestamp selects a verification instant only. The verifier
	// authenticates ALL claims before any metadata is returned to the caller.
	var candidate entitlements.Claims
	if err = json.Unmarshal(record.envelope.Payload, &candidate); err != nil {
		return result, ErrCorrupt
	}
	claims, err := s.verifier.Verify(record.envelope, tenantID, time.Unix(candidate.NotBefore, 0))
	if err != nil {
		return result, err
	}
	if claims.Revision != record.revision {
		return result, ErrCorrupt
	}
	now := s.clock().UTC().Unix()
	if now < record.observed {
		return result, ErrClockRollback
	}
	result.Modules = claims.Modules
	result.ExpiresUnix = claims.ExpiresAt
	switch {
	case now < claims.NotBefore:
		result.State = "not_yet_valid"
	case now >= claims.ExpiresAt:
		result.State = "expired"
	default:
		result.State = "active"
		remaining := claims.ExpiresAt - now
		result.RemainingDays = remaining / 86400
		if remaining%86400 != 0 {
			result.RemainingDays++
		}
	}
	if _, err = tx.ExecContext(ctx, `UPDATE module_contract_state SET last_observed_unix=? WHERE tenant_id=?`, now, tenantID); err != nil {
		return Status{}, err
	}
	return result, nil
}
