package localapi

import (
	"bytes"
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

func TestHTTPVaultTransferAndCreditFailuresPreserveBothSides(t *testing.T) {
	for _, stage := range []string{"deposit_link", "deposit_event", "transfer_leg", "transfer_event", "creation_event"} {
		t.Run(stage, func(t *testing.T) {
			f, receiver := vaultSetup(t)
			token := receiver
			path := vaultsPath + "/vault-one/deposit"
			body := depositVaultBody
			want := 500
			if strings.HasPrefix(stage, "transfer") {
				vaultResult(t, f, receiver, "POST", path, body, 200)
				vaultResult(t, f, f.token, "POST", vaultsPath, strings.NewReplacer("create-vault-one", "create-vault-two", "vault-one", "vault-two").Replace(createVaultBody), 201)
				path = vaultsPath + "/vault-one/transfer"
				body = transferVaultBody
			}
			if stage == "creation_event" {
				path = vaultsPath
				token = f.token
				body = strings.NewReplacer("create-vault-one", "create-vault-two", "vault-one", "vault-two").Replace(createVaultBody)
			}
			triggers := map[string]string{
				"deposit_link":   `CREATE TRIGGER injected BEFORE INSERT ON cash_vault_deposits BEGIN SELECT RAISE(IGNORE); END`,
				"deposit_event":  `CREATE TRIGGER injected BEFORE INSERT ON cash_vault_events WHEN NEW.event_type='cash.vault.deposit' BEGIN SELECT RAISE(ABORT,'event'); END`,
				"transfer_leg":   `CREATE TRIGGER injected BEFORE INSERT ON cash_vault_entries WHEN NEW.kind='transfer_in' BEGIN SELECT RAISE(ABORT,'leg'); END`,
				"transfer_event": `CREATE TRIGGER injected BEFORE INSERT ON cash_vault_events WHEN NEW.event_type='cash.vault.transfer' BEGIN SELECT RAISE(ABORT,'event'); END`,
				"creation_event": `CREATE TRIGGER injected BEFORE INSERT ON cash_vault_events WHEN NEW.event_type='cash.vault.create' BEGIN SELECT RAISE(ABORT,'event'); END`,
			}
			if stage == "deposit_link" {
				want = 409
			}
			if _, err := f.db.Exec(triggers[stage]); err != nil {
				t.Fatal(err)
			}
			before := vaultCounts(t, f)
			vaultResult(t, f, token, "POST", path, body, want)
			if !reflect.DeepEqual(before, vaultCounts(t, f)) {
				t.Fatal("partial ledger", before, vaultCounts(t, f))
			}
			if strings.HasPrefix(stage, "deposit") {
				if p := pendingVaults(t, f, receiver, 200); p.TotalCount != 1 {
					t.Fatal("credit source lost", p)
				}
			}
			if strings.HasPrefix(stage, "transfer") {
				if v := readVault(t, f, receiver, "vault-one", 200); v.BalanceCents != 290 {
					t.Fatal(v)
				}
				if v := readVault(t, f, receiver, "vault-two", 200); v.BalanceCents != 0 {
					t.Fatal(v)
				}
			}
			if _, err := f.db.Exec(`DROP TRIGGER injected`); err != nil {
				t.Fatal(err)
			}
			ok := 200
			if stage == "creation_event" {
				ok = 201
			}
			vaultResult(t, f, token, "POST", path, body, ok)
		})
	}
}

func TestHTTPVaultZeroCreditAndBalanceOverflowRemainExact(t *testing.T) {
	for _, count := range []string{"0", "9007199254740991"} {
		t.Run(count, func(t *testing.T) {
			f, receiver := bagSetup(t)
			bagResult(t, f, f.token, "POST", bagsPath, createBagBody, 201)
			bagResult(t, f, f.token, "POST", bagsPath+"/bag-one/dispatch", dispatchBagBody, 200)
			bagResult(t, f, receiver, "POST", bagsPath+"/bag-one/receive", strings.Replace(receiveBagBody, "290", count, 1), 200)
			vaultResult(t, f, f.token, "POST", vaultsPath, createVaultBody, 201)
			r := vaultResult(t, f, receiver, "POST", vaultsPath+"/vault-one/deposit", depositVaultBody, 200)
			if count == "0" {
				if r.Vaults[0].BalanceCents != 0 || len(r.Entries) != 1 || r.Entries[0].DeltaCents != 0 {
					t.Fatal(r)
				}
				if p := pendingVaults(t, f, receiver, 200); p.TotalCount != 0 {
					t.Fatal(p)
				}
				vaultResult(t, f, f.token, "POST", vaultsPath+"/vault-one/archive", `{"operation_id":"archive-zero","reason":"Saldo zero conferido"}`, 200)
				return
			}
			if r.Vaults[0].BalanceCents != 9007199254740991 {
				t.Fatal(r)
			}
			cashRequest(t, f.httpContractFixture, "movements", strings.Replace(withdrawalBody, "withdraw-one", "withdraw-two", 1), 200)
			bagResult(t, f, f.token, "POST", bagsPath, strings.NewReplacer("prepare-one", "prepare-two", "bag-one", "bag-two", "seal-one", "seal-two", "withdraw-one", "withdraw-two").Replace(createBagBody), 201)
			bagResult(t, f, f.token, "POST", bagsPath+"/bag-two/dispatch", `{"operation_id":"dispatch-two"}`, 200)
			bagResult(t, f, receiver, "POST", bagsPath+"/bag-two/receive", strings.NewReplacer("receive-one", "receive-two", "seal-one", "seal-two", "290", "1").Replace(receiveBagBody), 200)
			body := strings.NewReplacer("deposit-one", "deposit-two", "bag-one", "bag-two").Replace(depositVaultBody)
			before := vaultCounts(t, f)
			vaultResult(t, f, receiver, "POST", vaultsPath+"/vault-one/deposit", body, 422)
			if !reflect.DeepEqual(before, vaultCounts(t, f)) {
				t.Fatal("overflow changed credit source")
			}
			vaultResult(t, f, f.token, "POST", vaultsPath, strings.NewReplacer("create-vault-one", "create-vault-two", "vault-one", "vault-two").Replace(createVaultBody), 201)
			vaultResult(t, f, receiver, "POST", vaultsPath+"/vault-two/deposit", body, 200)
			before = vaultCounts(t, f)
			vaultResult(t, f, receiver, "POST", vaultsPath+"/vault-two/transfer", `{"operation_id":"overflow-transfer","destination_id":"vault-one","amount_cents":1,"reason":"Limite exato"}`, 422)
			if !reflect.DeepEqual(before, vaultCounts(t, f)) {
				t.Fatal("overflow transferred part of cash")
			}
		})
	}
}

func TestHTTPVaultDetectsBalanceSourceOrReceiptDamageEvenOnEmptyPage(t *testing.T) {
	for _, damage := range []string{"balance", "cash_source", "bag_source", "receipt_pointer"} {
		t.Run(damage, func(t *testing.T) {
			f, receiver := vaultSetup(t)
			vaultResult(t, f, receiver, "POST", vaultsPath+"/vault-one/deposit", depositVaultBody, 200)
			vaultResult(t, f, receiver, "POST", vaultsPath+"/vault-one/supply", supplyVaultBody, 200)
			for _, q := range []string{`DELETE FROM cash_vault_entries`, `UPDATE cash_vault_deposits SET entry_id='changed'`, `UPDATE cash_vault_events SET payload_json='{}'`, `DELETE FROM cash_vault_operations`} {
				if _, err := f.db.Exec(q); err == nil {
					t.Fatal("immutable data changed", q)
				}
			}
			switch damage {
			case "balance":
				if _, err := f.db.Exec(`UPDATE cash_vaults SET balance_cents=1`); err != nil {
					t.Fatal(err)
				}
			case "cash_source":
				if _, err := f.db.Exec(`UPDATE cash_adjustments SET amount_cents=1 WHERE kind='supply'`); err != nil {
					t.Fatal(err)
				}
			case "bag_source":
				if _, err := f.db.Exec(`UPDATE cash_adjustments SET amount_cents=1 WHERE kind='withdrawal'`); err != nil {
					t.Fatal(err)
				}
			case "receipt_pointer":
				if _, err := f.db.Exec(`UPDATE cash_vaults SET last_operation_id=creation_operation_id`); err != nil {
					t.Fatal(err)
				}
			}
			before := vaultCounts(t, f)
			readVault(t, f, receiver, "vault-one", 409)
			status, data := request(t, f.app, "GET", vaultsPath+"/vault-one/statement?limit=1&offset=999", "", receiver)
			if status != 409 || bytes.Contains(data, []byte("balance_cents")) {
				t.Fatal("partial unchecked statement", status, string(data))
			}
			if !reflect.DeepEqual(before, vaultCounts(t, f)) {
				t.Fatal("read repaired history")
			}
		})
	}
}

func TestHTTPVaultCashOverflowRollsBackItsDebit(t *testing.T) {
	f, receiver := vaultSetup(t)
	vaultResult(t, f, receiver, "POST", vaultsPath+"/vault-one/deposit", depositVaultBody, 200)
	cashRequest(t, f.httpContractFixture, "close", cashCloseBody, 200)
	cashRequest(t, f.httpContractFixture, "open", `{"session_id":"huge-drawer","opening_cents":9223372036854775807}`, 201)
	before := vaultCounts(t, f)
	vaultResult(t, f, receiver, "POST", vaultsPath+"/vault-one/supply", strings.Replace(supplyVaultBody, "turn-one", "huge-drawer", 1), 409)
	if !reflect.DeepEqual(before, vaultCounts(t, f)) {
		t.Fatal("cash overflow partially debited vault")
	}
	if v := readVault(t, f, receiver, "vault-one", 200); v.BalanceCents != 290 {
		t.Fatal(v)
	}
}

func TestHTTPVaultReceiptConflictAndStatementPagination(t *testing.T) {
	f, receiver := vaultSetup(t)
	vaultResult(t, f, receiver, "POST", vaultsPath+"/vault-one/deposit", depositVaultBody, 200)
	vaultResult(t, f, f.token, "POST", vaultsPath, strings.NewReplacer("create-vault-one", "create-vault-two", "vault-one", "vault-two").Replace(createVaultBody), 201)
	first := vaultResult(t, f, receiver, "POST", vaultsPath+"/vault-one/transfer", transferVaultBody, 200)
	vaultResult(t, f, receiver, "POST", vaultsPath+"/vault-one/transfer", strings.Replace(transferVaultBody, "100", "101", 1), 409)
	vaultResult(t, f, receiver, "POST", vaultsPath+"/vault-one/archive", `{"operation_id":"archive-funded","reason":"Ainda tem saldo"}`, 409)
	if r := vaultResult(t, f, receiver, "POST", vaultsPath+"/vault-one/transfer", transferVaultBody, 200); !r.Repeated || !reflect.DeepEqual(r.Entries, first.Entries) {
		t.Fatal(r)
	}
	status, data := request(t, f.app, "GET", vaultsPath+"?limit=1", "", receiver)
	if status != 200 {
		t.Fatal(status)
	}
	var page struct {
		Items      []cashVaultForPage
		TotalCount int64 `json:"total_count"`
		HasMore    bool  `json:"has_more"`
	}
	if err := json.Unmarshal(data, &page); err != nil {
		t.Fatal(err)
	}
	domainSchemaFile(t, "cash-vaults.openapi.json", "VaultPage", data)
	if page.TotalCount != 2 || len(page.Items) != 1 || !page.HasMore {
		t.Fatal(page)
	}
	status, data = request(t, f.app, "GET", vaultsPath+"/vault-one/statement?limit=1&offset=100", "", receiver)
	if status != 200 {
		t.Fatal(status)
	}
	domainSchemaFile(t, "cash-vaults.openapi.json", "VaultStatement", data)
}

type cashVaultForPage struct {
	ID string `json:"id"`
}
