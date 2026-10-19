package localapi

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"

	"titansystem-backend/internal/core/entitlements"
	"titansystem-backend/internal/core/modules"
	"titansystem-backend/internal/localdb"
	"titansystem-backend/internal/localdb/backup"
	"titansystem-backend/internal/localdb/cashvault"
	"titansystem-backend/internal/localdb/identity"
)

const vaultsPath = "/local/v1/cash/vaults"
const createVaultBody = `{"operation_id":"create-vault-one","vault_id":"vault-one","name":"Cofre da loja"}`
const depositVaultBody = `{"operation_id":"deposit-one","bag_id":"bag-one"}`
const supplyVaultBody = `{"operation_id":"vault-supply-one","cash_session_id":"turn-one","amount_cents":50,"reason":"Troco autorizado"}`
const transferVaultBody = `{"operation_id":"transfer-one","destination_id":"vault-two","amount_cents":100,"reason":"Reserva do cofre"}`

func vaultResult(t *testing.T, f *stockHTTPFixture, token, method, path, body string, want int) cashvault.Result {
	t.Helper()
	status, data := request(t, f.app, method, path, body, token)
	if status != want {
		t.Fatalf("%s %s: %d want %d %s", method, path, status, want, data)
	}
	var r cashvault.Result
	if want == 200 || want == 201 {
		if err := json.Unmarshal(data, &r); err != nil {
			t.Fatal(err)
		}
		domainSchemaFile(t, "cash-vaults.openapi.json", "VaultResult", data)
	}
	return r
}
func readVault(t *testing.T, f *stockHTTPFixture, token, id string, want int) cashvault.Vault {
	t.Helper()
	status, data := request(t, f.app, "GET", vaultsPath+"/"+id, "", token)
	if status != want {
		t.Fatalf("read cofre %d want %d %s", status, want, data)
	}
	var v cashvault.Vault
	if want == 200 {
		if err := json.Unmarshal(data, &v); err != nil {
			t.Fatal(err)
		}
		domainSchemaFile(t, "cash-vaults.openapi.json", "Vault", data)
	}
	return v
}
func vaultSetup(t *testing.T) (*stockHTTPFixture, string) {
	t.Helper()
	f, receiver := bagSetup(t)
	bagResult(t, f, f.token, "POST", bagsPath, createBagBody, 201)
	bagResult(t, f, f.token, "POST", bagsPath+"/bag-one/dispatch", dispatchBagBody, 200)
	bagResult(t, f, receiver, "POST", bagsPath+"/bag-one/receive", receiveBagBody, 200)
	vaultResult(t, f, f.token, "POST", vaultsPath, createVaultBody, 201)
	return f, receiver
}
func vaultCounts(t *testing.T, f *stockHTTPFixture) []int {
	t.Helper()
	counts := bagCounts(t, f)
	for _, table := range []string{"cash_vaults", "cash_vault_operations", "cash_vault_entries", "cash_vault_deposits", "cash_vault_events"} {
		var n int
		if err := f.db.QueryRow(`SELECT count(*) FROM ` + table).Scan(&n); err != nil {
			t.Fatal(err)
		}
		counts = append(counts, n)
	}
	return counts
}
func pendingVaults(t *testing.T, f *stockHTTPFixture, token string, want int) cashvault.PendingPage {
	t.Helper()
	status, data := request(t, f.app, "GET", vaultsPath+"/pending-bags", "", token)
	if status != want {
		t.Fatalf("pending %d want %d %s", status, want, data)
	}
	var p cashvault.PendingPage
	if want == 200 {
		if err := json.Unmarshal(data, &p); err != nil {
			t.Fatal(err)
		}
		domainSchemaFile(t, "cash-vaults.openapi.json", "PendingBagPage", data)
	}
	return p
}

func TestHTTPVaultCashChainConservesCountedMoneyAndSuppliesDrawerOnce(t *testing.T) {
	f, receiver := vaultSetup(t)
	if v := readVault(t, f, f.token, "vault-one", 200); v.BalanceCents != 0 || v.State != "active" {
		t.Fatal("cofre fabricated opening money", v)
	}
	if p := pendingVaults(t, f, receiver, 200); p.TotalCount != 1 || len(p.Items) != 1 || *p.Items[0].CountedCents != 290 {
		t.Fatal(p)
	}
	if p := pendingVaults(t, f, f.token, 200); p.TotalCount != 0 {
		t.Fatal("non-receiver saw pending custody", p)
	}
	vaultResult(t, f, f.token, "POST", vaultsPath+"/vault-one/deposit", depositVaultBody, 403)
	before := vaultCounts(t, f)
	d := vaultResult(t, f, receiver, "POST", vaultsPath+"/vault-one/deposit", depositVaultBody, 200)
	if d.Vaults[0].BalanceCents != 290 || len(d.Entries) != 1 || d.Entries[0].DeltaCents != 290 || d.Entries[0].BagID == nil {
		t.Fatal(d)
	}
	after := vaultCounts(t, f)
	if !reflect.DeepEqual(before[:7], after[:7]) {
		t.Fatal("deposit moved original drawer", before, after)
	}
	if p := pendingVaults(t, f, receiver, 200); p.TotalCount != 0 {
		t.Fatal(p)
	}
	vaultResult(t, f, f.token, "POST", vaultsPath, strings.NewReplacer("create-vault-one", "create-vault-two", "vault-one", "vault-two").Replace(createVaultBody), 201)
	r := vaultResult(t, f, receiver, "POST", vaultsPath+"/vault-one/transfer", transferVaultBody, 200)
	if len(r.Entries) != 2 || r.Entries[0].DeltaCents != -100 || r.Entries[1].DeltaCents != 100 || r.Vaults[0].BalanceCents != 190 || r.Vaults[1].BalanceCents != 100 {
		t.Fatal(r)
	}
	supply := vaultResult(t, f, receiver, "POST", vaultsPath+"/vault-one/supply", supplyVaultBody, 200)
	if supply.Vaults[0].BalanceCents != 140 || supply.CashMovementID == nil || len(supply.Entries) != 1 || supply.Entries[0].DeltaCents != -50 {
		t.Fatal(supply)
	}
	for i := 0; i < 2; i++ {
		repeated := vaultResult(t, f, receiver, "POST", vaultsPath+"/vault-one/supply", supplyVaultBody, 200)
		if !repeated.Repeated || !reflect.DeepEqual(repeated.Entries, supply.Entries) {
			t.Fatal(repeated)
		}
	}
	if got := vaultResult(t, f, receiver, "POST", vaultsPath+"/vault-one/deposit", depositVaultBody, 200); !got.Repeated || got.Vaults[0].BalanceCents != 290 {
		t.Fatal("original receipt was rewritten", got)
	}
	vaultResult(t, f, receiver, "POST", vaultsPath+"/vault-two/deposit", strings.Replace(depositVaultBody, "deposit-one", "deposit-again", 1), 409)
	if v := readVault(t, f, f.token, "vault-one", 200); v.BalanceCents != 140 {
		t.Fatal(v)
	}
	if v := readVault(t, f, f.token, "vault-two", 200); v.BalanceCents != 100 {
		t.Fatal(v)
	}
	status, data := request(t, f.app, "GET", vaultsPath+"/vault-one/statement?limit=1&offset=1", "", receiver)
	if status != 200 {
		t.Fatal(status, string(data))
	}
	var st cashvault.Statement
	if err := json.Unmarshal(data, &st); err != nil {
		t.Fatal(err)
	}
	domainSchemaFile(t, "cash-vaults.openapi.json", "VaultStatement", data)
	if st.TotalCount != 3 || len(st.Items) != 1 || !st.HasMore || st.Vault.BalanceCents != 140 || st.Items[0].Kind != "transfer_out" {
		t.Fatal(st)
	}
	cashRequest(t, f.httpContractFixture, "close", `{"session_id":"turn-one","operation_id":"close-one","declared_cents":750}`, 200)
	report := closedReport(t, f.httpContractFixture, closedReportPath, 200)
	if report.Cash.ExpectedCents != 750 || report.Cash.SupplyCents != 50 || report.Cash.WithdrawalCents != 300 {
		t.Fatal(report)
	}
	if report.Cash.ExpectedCents+readVault(t, f, f.token, "vault-one", 200).BalanceCents+readVault(t, f, f.token, "vault-two", 200).BalanceCents != 990 {
		t.Fatal("counted shortfall was silently fabricated")
	}
	if c := bagCoverage(t, f, f.token, 200); c.DifferenceCents != -10 || c.ReceivedCountedCents != 290 {
		t.Fatal(c)
	}
	if got := vaultResult(t, f, receiver, "POST", vaultsPath+"/vault-one/supply", supplyVaultBody, 200); !got.Repeated {
		t.Fatal("closed drawer replay not recoverable")
	}
	if n := vaultCounts(t, f); n[0] != 2 || n[1] != 3 || n[2] != before[2] {
		t.Fatal("extra cash/journal/stock movements", n)
	}
}

func TestHTTPVaultRefusesUnreceivedBagInsufficientFundsAndAdoptingOldSupply(t *testing.T) {
	f, receiver := vaultSetup(t)
	if status, data := request(t, f.app, "GET", vaultsPath+"/pending-bags", "", receiver); status != 200 || !bytes.Contains(data, []byte("bag-one")) {
		t.Fatal(status, string(data))
	}
	vaultResult(t, f, receiver, "POST", vaultsPath+"/vault-one/supply", supplyVaultBody, 409)
	vaultResult(t, f, receiver, "POST", vaultsPath+"/vault-one/deposit", depositVaultBody, 200)
	cashRequest(t, f.httpContractFixture, "movements", strings.Replace(supplyBody, "supply-one", "vault-supply-one", 1), 200)
	before := vaultCounts(t, f)
	vaultResult(t, f, receiver, "POST", vaultsPath+"/vault-one/supply", supplyVaultBody, 409)
	if !reflect.DeepEqual(before, vaultCounts(t, f)) {
		t.Fatal("adopted an unbacked legacy supply")
	}
	cashRequest(t, f.httpContractFixture, "movements", strings.Replace(withdrawalBody, "withdraw-one", "other-withdrawal", 1), 200)
	other := strings.NewReplacer("prepare-one", "prepare-two", "bag-one", "bag-two", "withdraw-one", "other-withdrawal", "seal-one", "seal-two").Replace(createBagBody)
	bagResult(t, f, f.token, "POST", bagsPath, other, 201)
	vaultResult(t, f, receiver, "POST", vaultsPath+"/vault-one/deposit", strings.NewReplacer("deposit-one", "deposit-two", "bag-one", "bag-two").Replace(depositVaultBody), 409)
	cashRequest(t, f.httpContractFixture, "close", cashCloseBody, 200)
	vaultResult(t, f, receiver, "POST", vaultsPath+"/vault-one/supply", strings.Replace(supplyVaultBody, "vault-supply-one", "supply-new", 1), 409)
	vaultResult(t, f, receiver, "GET", vaultsPath+"/operations/deposit-one", "", 200)
	vaultResult(t, f, f.token, "GET", vaultsPath+"/operations/deposit-one", "", 404)
}

func TestHTTPVaultFailureRollsBackCashJournalBalancesOperationsAndEvents(t *testing.T) {
	for _, stage := range []string{"entry_ignore", "header_abort", "balance_ignore", "vault_event", "cash_event", "cash_journal"} {
		t.Run(stage, func(t *testing.T) {
			f, receiver := vaultSetup(t)
			vaultResult(t, f, receiver, "POST", vaultsPath+"/vault-one/deposit", depositVaultBody, 200)
			triggers := map[string]string{
				"entry_ignore":   `CREATE TRIGGER injected BEFORE INSERT ON cash_vault_entries WHEN NEW.kind='cash_supply' BEGIN SELECT RAISE(IGNORE); END`,
				"header_abort":   `CREATE TRIGGER injected BEFORE INSERT ON cash_vault_operations WHEN NEW.kind='supply' BEGIN SELECT RAISE(ABORT,'header failed'); END`,
				"balance_ignore": `CREATE TRIGGER injected BEFORE UPDATE ON cash_vaults BEGIN SELECT RAISE(IGNORE); END`,
				"vault_event":    `CREATE TRIGGER injected BEFORE INSERT ON cash_vault_events WHEN NEW.event_type='cash.vault.supply' BEGIN SELECT RAISE(ABORT,'event failed'); END`,
				"cash_event":     `CREATE TRIGGER injected BEFORE INSERT ON outbox WHEN NEW.event_type='cash.movement' BEGIN SELECT RAISE(ABORT,'cash event failed'); END`,
				"cash_journal":   `CREATE TRIGGER injected BEFORE INSERT ON cash_journal_entries WHEN NEW.category='supply' BEGIN SELECT RAISE(ABORT,'journal failed'); END`,
			}
			if _, err := f.db.Exec(triggers[stage]); err != nil {
				t.Fatal(err)
			}
			before := vaultCounts(t, f)
			want := 500
			if strings.Contains(stage, "ignore") {
				want = 409
			}
			vaultResult(t, f, receiver, "POST", vaultsPath+"/vault-one/supply", supplyVaultBody, want)
			if !reflect.DeepEqual(before, vaultCounts(t, f)) {
				t.Fatal("partial supply", before, vaultCounts(t, f))
			}
			if v := readVault(t, f, receiver, "vault-one", 200); v.BalanceCents != 290 {
				t.Fatal(v)
			}
			if _, err := f.db.Exec(`DROP TRIGGER injected`); err != nil {
				t.Fatal(err)
			}
			vaultResult(t, f, receiver, "POST", vaultsPath+"/vault-one/supply", supplyVaultBody, 200)
		})
	}
}

func TestHTTPVaultConcurrentSpendAndDepositHaveSingleFinancialEffect(t *testing.T) {
	f, receiver := vaultSetup(t)
	var wg sync.WaitGroup
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			vaultResult(t, f, receiver, "POST", vaultsPath+"/vault-one/deposit", depositVaultBody, 200)
		}()
	}
	wg.Wait()
	if v := readVault(t, f, receiver, "vault-one", 200); v.BalanceCents != 290 {
		t.Fatal(v)
	}
	statusChannel := make(chan int, 2)
	for _, op := range []string{"spend-one", "spend-two"} {
		op := op
		wg.Add(1)
		go func() {
			defer wg.Done()
			body := strings.NewReplacer("vault-supply-one", op, `"amount_cents":50`, `"amount_cents":200`).Replace(supplyVaultBody)
			status, _ := request(t, f.app, "POST", vaultsPath+"/vault-one/supply", body, receiver)
			statusChannel <- status
		}()
	}
	wg.Wait()
	close(statusChannel)
	ok, conflict := 0, 0
	for status := range statusChannel {
		if status == 200 {
			ok++
		} else if status == 409 {
			conflict++
		} else {
			t.Fatal(status)
		}
	}
	if ok != 1 || conflict != 1 {
		t.Fatal(ok, conflict)
	}
	if v := readVault(t, f, receiver, "vault-one", 200); v.BalanceCents != 90 {
		t.Fatal(v)
	}
	var cash int
	if err := f.db.QueryRow(`SELECT count(*) FROM cash_adjustments WHERE kind='supply'`).Scan(&cash); err != nil || cash != 1 {
		t.Fatal(cash, err)
	}
}

func TestHTTPVaultBackupRestartAndOperationReplayPreserveBalances(t *testing.T) {
	f, receiver := vaultSetup(t)
	vaultResult(t, f, receiver, "POST", vaultsPath+"/vault-one/deposit", depositVaultBody, 200)
	want := vaultResult(t, f, receiver, "POST", vaultsPath+"/vault-one/supply", supplyVaultBody, 200)
	balance := readVault(t, f, receiver, "vault-one", 200)
	ctx := context.Background()
	dir := t.TempDir()
	archive := filepath.Join(dir, "vault.tytbak")
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		t.Fatal(err)
	}
	if err := backup.Create(ctx, f.db, f.device, key, archive); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "restored.sqlite")
	if err := backup.Restore(ctx, archive, path, f.device, key); err != nil {
		t.Fatal(err)
	}
	db, err := localdb.Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	actor := identity.Scope{TenantID: f.owner.TenantID, StoreID: f.owner.StoreID, IdentityID: "receiver"}
	if got, err := cashvault.Get(ctx, db, actor, f.device, "vault-one"); err != nil || !reflect.DeepEqual(got, balance) {
		t.Fatal("restored balance", got, err)
	}
	if got, err := cashvault.Receipt(ctx, db, actor, f.device, "vault-supply-one"); err != nil || !reflect.DeepEqual(got, want) {
		t.Fatal("restored receipt", got, err)
	}
	if _, err = db.Exec(`DELETE FROM cash_vault_entries`); err == nil {
		t.Fatal("restored ledger editable")
	}
	var n int
	var name, original string
	if err = f.db.QueryRow(`PRAGMA database_list`).Scan(&n, &name, &original); err != nil {
		t.Fatal(err)
	}
	if err = f.db.Close(); err != nil {
		t.Fatal(err)
	}
	f.db, err = localdb.Open(ctx, original)
	if err != nil {
		t.Fatal(err)
	}
	defer f.db.Close()
	v, err := entitlements.NewVerifier(map[string]ed25519.PublicKey{"test-issuer": f.private.Public().(ed25519.PublicKey)})
	if err != nil {
		t.Fatal(err)
	}
	f.app, err = NewWithVerifier(f.db, f.device, v)
	if err != nil {
		t.Fatal(err)
	}
	if r := vaultResult(t, f, receiver, "POST", vaultsPath+"/vault-one/supply", supplyVaultBody, 200); !r.Repeated || !reflect.DeepEqual(r.Entries, want.Entries) {
		t.Fatal("lost response resupplied cash", r)
	}
}

func TestHTTPVaultStrictInputsAccessAndImmutableArchive(t *testing.T) {
	f, receiver := vaultSetup(t)
	for _, bad := range []string{`{}`, strings.Replace(createVaultBody, `"vault-one"`, `null`, 1), strings.Replace(createVaultBody, `"name":"Cofre da loja"`, `"name":"Cofre da loja","balance_cents":1`, 1), strings.Replace(createVaultBody, "vault-one", "bad/id", 1)} {
		vaultResult(t, f, f.token, "POST", vaultsPath, bad, 400)
	}
	for _, q := range []string{"?limit=0", "?limit=101", "?offset=-1", "?offset=1000000001", "?limit=1&limit=2", "?tenant_id=other"} {
		if status, _ := request(t, f.app, "GET", vaultsPath+q, "", f.token); status != 400 {
			t.Fatal(q, status)
		}
	}
	vaultResult(t, f, receiver, "POST", vaultsPath+"/vault-one/deposit", strings.Replace(depositVaultBody, `"bag_id":"bag-one"`, `"bag_id":"bag-one","amount_cents":290`, 1), 400)
	policyPerson(t, f.db, f.owner, "cashier-only", "cashier")
	readVault(t, f, loginToken(t, f.app, "cashier-only"), "vault-one", 403)
	archived := vaultResult(t, f, f.token, "POST", vaultsPath+"/vault-one/archive", `{"operation_id":"archive-empty","reason":"Cofre substituido"}`, 200)
	if archived.Vaults[0].State != "archived" {
		t.Fatal(archived)
	}
	vaultResult(t, f, receiver, "POST", vaultsPath+"/vault-one/deposit", depositVaultBody, 409)
	for _, q := range []string{`DELETE FROM cash_vaults`, `UPDATE cash_vaults SET name='rewrite'`, `UPDATE cash_vault_operations SET reason='changed'`, `DELETE FROM cash_vault_events`} {
		if _, err := f.db.Exec(q); err == nil {
			t.Fatal("history changed", q)
		}
	}
	f.install(t, signedTestContract(t, f.private, f.owner.TenantID, 3, []modules.ID{modules.Core, modules.Inventory}), 200)
	readVault(t, f, receiver, "vault-one", 200)
	vaultResult(t, f, f.token, "POST", vaultsPath, strings.NewReplacer("create-vault-one", "create-other", "vault-one", "other-vault").Replace(createVaultBody), 403)
	if _, err := f.db.Exec(`UPDATE local_sessions SET revoked_unix=1 WHERE tenant_id=? AND identity_id='receiver'`, f.owner.TenantID); err != nil {
		t.Fatal(err)
	}
	readVault(t, f, receiver, "vault-one", 401)
}
