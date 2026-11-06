package fiscal_test

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"errors"
	"path/filepath"
	"testing"

	"titansystem-backend/internal/localdb"
	"titansystem-backend/internal/localdb/fiscal"
	"titansystem-backend/internal/localdb/identity"
	"titansystem-backend/internal/localdb/localsetup"
)

func TestCheckoutFiscalStatusRequiresDurableEvent(t *testing.T) {
	for _, mode := range []string{"IGNORE", "ABORT"} {
		t.Run(mode, func(t *testing.T) {
			ctx := context.Background()
			db, err := localdb.Open(ctx, filepath.Join(t.TempDir(), "status.sqlite"))
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { db.Close() })
			public, _, err := ed25519.GenerateKey(rand.Reader)
			if err != nil {
				t.Fatal(err)
			}
			owner, err := localsetup.Initialize(ctx, db, localsetup.Input{TenantName: "Synthetic", StoreName: "Discardable", OwnerName: "Owner", DeviceName: "Test", Password: "synthetic-discardable-password", PublicKey: public})
			if err != nil {
				t.Fatal(err)
			}
			a := identity.Scope{TenantID: owner.TenantID, StoreID: owner.StoreID, IdentityID: owner.OwnerID}
			d := identity.DeviceContext{TenantID: owner.TenantID, StoreID: owner.StoreID, DeviceID: owner.DeviceID}
			original := fiscal.CheckoutStatus{State: "recovery_required", OperationID: "original-fiscal-attempt", IntentID: "original-intent"}
			if err = fiscal.RecordCheckoutStatus(ctx, db, a, d, "original-sale", original); err != nil {
				t.Fatal(err)
			}
			var before int
			if err = db.QueryRow(`SELECT count(*) FROM outbox WHERE event_type='checkout.fiscal_status'`).Scan(&before); err != nil {
				t.Fatal(err)
			}
			raise := "RAISE(IGNORE)"
			if mode == "ABORT" {
				raise = "RAISE(ABORT,'discardable fault')"
			}
			if _, err = db.Exec(`CREATE TRIGGER reject_fiscal_status BEFORE INSERT ON outbox WHEN NEW.event_type='checkout.fiscal_status' BEGIN SELECT ` + raise + `; END`); err != nil {
				t.Fatal(err)
			}
			changed := original
			changed.State = "authorized_homologation"
			err = fiscal.RecordCheckoutStatus(ctx, db, a, d, "original-sale", changed)
			if err == nil {
				t.Fatal("fiscal status reported success without a durable event")
			}
			if mode == "IGNORE" && !errors.Is(err, fiscal.ErrConflict) {
				t.Fatalf("ignored event must return conflict, got %T", err)
			}
			var after int
			if err = db.QueryRow(`SELECT count(*) FROM outbox WHERE event_type='checkout.fiscal_status'`).Scan(&after); err != nil || after != before {
				t.Fatal("failed status write changed durable events")
			}
			var raw string
			if err = db.QueryRow(`SELECT payload_json FROM outbox WHERE event_type='checkout.fiscal_status'`).Scan(&raw); err != nil {
				t.Fatal(err)
			}
			var kept fiscal.CheckoutStatus
			if json.Unmarshal([]byte(raw), &kept) != nil || kept.State != original.State || kept.OperationID != original.OperationID || kept.IntentID != original.IntentID {
				t.Fatal("original fiscal recovery event lost or changed")
			}
		})
	}
}
