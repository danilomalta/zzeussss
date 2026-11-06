package entitlementstore_test

import (
	"context"
	"errors"
	"testing"

	"titansystem-backend/internal/core/modules"
	"titansystem-backend/internal/localdb/entitlementstore"
	"titansystem-backend/internal/localdb/identity"
)

// A signed withdrawal must not be acknowledged unless history and the active
// revision were persisted atomically. Fault injection affects only this test DB.
func TestIgnoredContractWithdrawalNeverReportsSuccess(t *testing.T) {
	for _, trigger := range []string{
		`CREATE TRIGGER ignore_license BEFORE INSERT ON module_contract_history WHEN NEW.revision=2 BEGIN SELECT RAISE(IGNORE); END`,
		`CREATE TRIGGER ignore_license BEFORE UPDATE ON module_contract_state WHEN NEW.revision=2 BEGIN SELECT RAISE(IGNORE); END`,
	} {
		t.Run(trigger, func(t *testing.T) {
			f := setup(t)
			install(t, f, contract(t, f, 1, []modules.ID{modules.Core, modules.Inventory, modules.POS}))
			if _, err := f.db.Exec(trigger); err != nil {
				t.Fatal(err)
			}
			withdrawal := contract(t, f, 2, []modules.ID{modules.Core})
			if _, err := f.store.Install(context.Background(), f.actor, f.device, withdrawal); !errors.Is(err, entitlementstore.ErrCorrupt) {
				t.Fatalf("unpersisted withdrawal must fail closed: %v", err)
			}
			var revision, history int
			if err := f.db.QueryRow(`SELECT revision FROM module_contract_state`).Scan(&revision); err != nil {
				t.Fatal(err)
			}
			if err := f.db.QueryRow(`SELECT count(*) FROM module_contract_history`).Scan(&history); err != nil {
				t.Fatal(err)
			}
			if revision != 1 || history != 1 {
				t.Fatal("failed withdrawal left partial state")
			}
			if _, err := f.db.Exec(`DROP TRIGGER ignore_license`); err != nil {
				t.Fatal(err)
			}
			install(t, f, withdrawal)
			if err := require(f, identity.Sell, modules.POS); !errors.Is(err, modules.ErrUnavailable) {
				t.Fatalf("withdrawn module allowed command: %v", err)
			}
		})
	}
}

func TestIgnoredContractObservationCannotAuthorizeCommandOrDiagnostic(t *testing.T) {
	f := setup(t)
	envelope := contract(t, f, 1, []modules.ID{modules.Core, modules.Inventory, modules.POS})
	install(t, f, envelope)
	if _, err := f.db.Exec(`CREATE TRIGGER ignore_observation BEFORE UPDATE ON module_contract_state BEGIN SELECT RAISE(IGNORE); END`); err != nil {
		t.Fatal(err)
	}
	if _, err := f.store.Install(context.Background(), f.actor, f.device, envelope); !errors.Is(err, entitlementstore.ErrCorrupt) {
		t.Fatalf("unpersisted replay: %v", err)
	}
	if err := require(f, identity.Sell, modules.POS); !errors.Is(err, entitlementstore.ErrCorrupt) {
		t.Fatalf("unpersisted authorization: %v", err)
	}
	tx, err := f.db.BeginTx(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	if _, err := f.store.StatusTx(context.Background(), tx, f.actor.TenantID); !errors.Is(err, entitlementstore.ErrCorrupt) {
		t.Fatalf("unpersisted diagnostic: %v", err)
	}
}
