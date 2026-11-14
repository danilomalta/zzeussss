package localapi

import (
	"fmt"
	"reflect"
	"sync"
	"testing"
)

func TestHTTPVaultReviewedBagCreditsOriginalCountAndRollsBack(t *testing.T) {
	f, receiver := vaultSetup(t)
	reviewHTTP(t, f, f.token, "POST", reviewBody("investigate", "investigate", 0, 290), 200)
	reviewHTTP(t, f, f.token, "POST", reviewBody("recount", "recount", 1, 300), 200)
	reviewHTTP(t, f, f.token, "POST", reviewBody("resolve", "resolve", 2, 300), 200)
	before := vaultCounts(t, f)
	vaultResult(t, f, f.token, "POST", vaultsPath+"/vault-one/deposit", depositVaultBody, 403)
	vaultResult(t, f, receiver, "POST", vaultsPath+"/vault-one/deposit", `{"operation_id":"inflated","bag_id":"bag-one","amount_cents":300}`, 400)
	if _, err := f.db.Exec(`CREATE TRIGGER fail_deposit_event BEFORE INSERT ON cash_vault_events WHEN NEW.event_type='cash.vault.deposit' BEGIN SELECT RAISE(ABORT,'injected'); END`); err != nil {
		t.Fatal(err)
	}
	vaultResult(t, f, receiver, "POST", vaultsPath+"/vault-one/deposit", depositVaultBody, 500)
	if !reflect.DeepEqual(before, vaultCounts(t, f)) {
		t.Fatal("deposit audit failed without full rollback")
	}
	if _, err := f.db.Exec(`DROP TRIGGER fail_deposit_event`); err != nil {
		t.Fatal(err)
	}
	got := vaultResult(t, f, receiver, "POST", vaultsPath+"/vault-one/deposit", depositVaultBody, 200)
	if got.Vaults[0].BalanceCents != 290 || got.Entries[0].DeltaCents != 290 {
		t.Fatal("recount fabricated vault money")
	}
	replay := vaultResult(t, f, receiver, "POST", vaultsPath+"/vault-one/deposit", depositVaultBody, 200)
	if !replay.Repeated || !reflect.DeepEqual(got.Entries, replay.Entries) {
		t.Fatal("deposit duplicated")
	}
	review := reviewHTTP(t, f, f.token, "GET", "", 200).Review
	if review.CountedCents != 300 || review.OriginalCountedCents != 290 || review.State != "resolved" {
		t.Fatal("deposit rewrote investigation")
	}
	if bagRead(t, f, receiver, "bag-one", 200).DifferenceCents == nil || *bagRead(t, f, receiver, "bag-one", 200).DifferenceCents != -10 {
		t.Fatal("deposit erased custody variance")
	}
}
func TestHTTPVaultReviewedBagConcurrentDepositHasOneCredit(t *testing.T) {
	f, receiver := vaultSetup(t)
	reviewHTTP(t, f, f.token, "POST", reviewBody("investigate", "investigate", 0, 290), 200)
	var wg sync.WaitGroup
	statuses := make(chan int, 5)
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			body := fmt.Sprintf(`{"operation_id":"deposit-race-%d","bag_id":"bag-one"}`, i)
			code, _ := request(t, f.app, "POST", vaultsPath+"/vault-one/deposit", body, receiver)
			statuses <- code
		}(i)
	}
	wg.Wait()
	close(statuses)
	wins := 0
	for status := range statuses {
		if status == 200 {
			wins++
		} else if status != 409 {
			t.Fatalf("unexpected deposit status %d", status)
		}
	}
	if wins != 1 || readVault(t, f, receiver, "vault-one", 200).BalanceCents != 290 {
		t.Fatal("deposit lost or duplicated money")
	}
	var n int
	if err := f.db.QueryRow(`SELECT count(*) FROM cash_vault_entries WHERE kind='deposit'`).Scan(&n); err != nil || n != 1 {
		t.Fatal("duplicate deposit entry", err)
	}
}
