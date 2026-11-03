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
	"titansystem-backend/internal/localdb/cashcount"
	"titansystem-backend/internal/localdb/identity"
)

const countsPath = "/local/v1/cash/vault-counts"
const countStart = `{"operation_id":"start-count-one","count_id":"count-one","vault_id":"vault-one","counter_id":"receiver","reason":"Conferencia fisica do cofre"}`
const countSubmit = `{"operation_id":"submit-count-one","counted_cents":280}`
const countAccept = `{"operation_id":"accept-count-one","reason":"Diferenca encaminhada para apuracao"}`
const countCancel = `{"operation_id":"cancel-count-one","reason":"Contagem reagendada"}`

func countSetup(t *testing.T) (*stockHTTPFixture, string) {
	t.Helper()
	f, receiver := vaultSetup(t)
	vaultResult(t, f, receiver, "POST", vaultsPath+"/vault-one/deposit", depositVaultBody, 200)
	return f, receiver
}
func countResult(t *testing.T, f *stockHTTPFixture, token, method, path, body string, want int) cashcount.Result {
	t.Helper()
	status, data := request(t, f.app, method, path, body, token)
	if status != want {
		t.Fatalf("count %s %s: %d want%d %s", method, path, status, want, data)
	}
	var r cashcount.Result
	if want == 200 || want == 201 {
		if err := json.Unmarshal(data, &r); err != nil {
			t.Fatal(err)
		}
		domainSchemaFile(t, "cash-vault-counts.openapi.json", "CountResult", data)
	}
	return r
}
func readCount(t *testing.T, f *stockHTTPFixture, token, id string, want int) cashcount.Count {
	t.Helper()
	status, data := request(t, f.app, "GET", countsPath+"/"+id, "", token)
	if status != want {
		t.Fatalf("read count %d want%d %s", status, want, data)
	}
	var c cashcount.Count
	if want == 200 {
		if err := json.Unmarshal(data, &c); err != nil {
			t.Fatal(err)
		}
		domainSchemaFile(t, "cash-vault-counts.openapi.json", "VaultCount", data)
	}
	return c
}
func countCounts(t *testing.T, f *stockHTTPFixture) []int {
	t.Helper()
	n := vaultCounts(t, f)
	for _, table := range []string{"cash_vault_counts", "cash_vault_count_operations", "cash_vault_count_events"} {
		var v int
		if err := f.db.QueryRow(`SELECT count(*) FROM ` + table).Scan(&v); err != nil {
			t.Fatal(err)
		}
		n = append(n, v)
	}
	return n
}
func TestHTTPVaultCountFreezesMoneyRequiresIndependentReviewAndPreservesVariance(t *testing.T) {
	f, receiver := countSetup(t)
	before := vaultCounts(t, f)
	r := countResult(t, f, f.token, "POST", countsPath, countStart, 201)
	if r.Count.State != "counting" || r.Count.ExpectedCents != nil || r.Count.CountedCents != nil {
		t.Fatal(r)
	}
	if !reflect.DeepEqual(before, vaultCounts(t, f)) {
		t.Fatal("starting count moved money")
	}
	countResult(t, f, f.token, "POST", countsPath, strings.NewReplacer("start-count-one", "start-two", "count-one", "count-two").Replace(countStart), 409)
	countResult(t, f, f.token, "POST", countsPath+"/count-one/submit", countSubmit, 403)
	vaultResult(t, f, receiver, "POST", vaultsPath+"/vault-one/supply", supplyVaultBody, 409)
	vaultResult(t, f, receiver, "POST", vaultsPath+"/vault-one/archive", `{"operation_id":"archive-locked","reason":"Conferencia"}`, 409)
	// Historical retries return their original receipt and have no new financial effect.
	if r := vaultResult(t, f, receiver, "POST", vaultsPath+"/vault-one/deposit", depositVaultBody, 200); !r.Repeated {
		t.Fatal(r)
	}
	c := countResult(t, f, receiver, "POST", countsPath+"/count-one/submit", countSubmit, 200).Count
	if c.State != "submitted" || *c.ExpectedCents != 290 || *c.CountedCents != 280 || *c.DifferenceCents != -10 {
		t.Fatal(c)
	}
	countResult(t, f, receiver, "POST", countsPath+"/count-one/accept", countAccept, 403)
	countResult(t, f, f.token, "POST", countsPath+"/count-one/cancel", countCancel, 409)
	vaultResult(t, f, receiver, "POST", vaultsPath+"/vault-one/supply", supplyVaultBody, 409)
	reviewed := countResult(t, f, f.token, "POST", countsPath+"/count-one/accept", countAccept, 200)
	if reviewed.Count.State != "accepted" || reviewed.Count.ReviewerID == nil || *reviewed.Count.ReviewerID == "receiver" {
		t.Fatal(reviewed)
	}
	if !reflect.DeepEqual(before, vaultCounts(t, f)) {
		t.Fatal("review repaired money")
	}
	if r := countResult(t, f, f.token, "POST", countsPath+"/count-one/accept", countAccept, 200); !r.Repeated || !reflect.DeepEqual(r.Count, reviewed.Count) {
		t.Fatal(r)
	}
	vaultResult(t, f, receiver, "POST", vaultsPath+"/vault-one/supply", supplyVaultBody, 200)
	if c = readCount(t, f, receiver, "count-one", 200); *c.ExpectedCents != 290 || *c.DifferenceCents != -10 || len(c.History) != 3 {
		t.Fatal(c)
	}
	if v := readVault(t, f, receiver, "vault-one", 200); v.BalanceCents != 240 {
		t.Fatal(v)
	}
	if r := countResult(t, f, f.token, "GET", countsPath+"/operations/start-count-one", "", 200); r.Count.ExpectedCents != nil || r.Count.State != "counting" {
		t.Fatal("historical receipt changed", r)
	}
	countResult(t, f, receiver, "GET", countsPath+"/operations/start-count-one", "", 404)
}
func TestHTTPVaultCountBlocksBothTransferSidesAndNewDeposits(t *testing.T) {
	f, receiver := countSetup(t)
	cashRequest(t, f.httpContractFixture, "movements", strings.Replace(withdrawalBody, "withdraw-one", "withdraw-two", 1), 200)
	bagResult(t, f, f.token, "POST", bagsPath, strings.NewReplacer("prepare-one", "prepare-two", "bag-one", "bag-two", "seal-one", "seal-two", "withdraw-one", "withdraw-two").Replace(createBagBody), 201)
	bagResult(t, f, f.token, "POST", bagsPath+"/bag-two/dispatch", `{"operation_id":"dispatch-two"}`, 200)
	bagResult(t, f, receiver, "POST", bagsPath+"/bag-two/receive", strings.NewReplacer("receive-one", "receive-two", "seal-one", "seal-two").Replace(receiveBagBody), 200)
	vaultResult(t, f, f.token, "POST", vaultsPath, strings.NewReplacer("create-vault-one", "create-vault-two", "vault-one", "vault-two").Replace(createVaultBody), 201)
	vaultResult(t, f, receiver, "POST", vaultsPath+"/vault-one/transfer", transferVaultBody, 200)
	countResult(t, f, f.token, "POST", countsPath, countStart, 201)
	before := countCounts(t, f)
	vaultResult(t, f, receiver, "POST", vaultsPath+"/vault-one/transfer", strings.Replace(transferVaultBody, "transfer-one", "blocked-out", 1), 409)
	vaultResult(t, f, receiver, "POST", vaultsPath+"/vault-two/transfer", `{"operation_id":"blocked-in","destination_id":"vault-one","amount_cents":10,"reason":"Reserva"}`, 409)
	freshDeposit := strings.NewReplacer("deposit-one", "blocked-deposit", "bag-one", "bag-two").Replace(depositVaultBody)
	vaultResult(t, f, receiver, "POST", vaultsPath+"/vault-one/deposit", freshDeposit, 409)
	if p := pendingVaults(t, f, receiver, 200); p.TotalCount != 1 {
		t.Fatal("frozen credit consumed source", p)
	}
	if !reflect.DeepEqual(before, countCounts(t, f)) {
		t.Fatal("frozen vault moved")
	}
	if _, err := f.db.Exec(`UPDATE cash_vaults SET balance_cents=1 WHERE id='vault-one'`); err == nil {
		t.Fatal("SQL bypassed freeze")
	}
	countResult(t, f, f.token, "POST", countsPath+"/count-one/cancel", countCancel, 200)
	vaultResult(t, f, receiver, "POST", vaultsPath+"/vault-one/deposit", freshDeposit, 200)
	vaultResult(t, f, receiver, "POST", vaultsPath+"/vault-two/transfer", `{"operation_id":"blocked-in","destination_id":"vault-one","amount_cents":10,"reason":"Reserva"}`, 200)
	if c := readCount(t, f, receiver, "count-one", 200); c.ExpectedCents != nil || c.CountedCents != nil || c.State != "cancelled" {
		t.Fatal(c)
	}
}
func TestHTTPVaultCountFailureRollsBackHistoryAndLock(t *testing.T) {
	for _, stage := range []string{"create_event", "create_header_ignore", "submit_ignore", "submit_event", "review_event"} {
		t.Run(stage, func(t *testing.T) {
			f, receiver := countSetup(t)
			token := f.token
			path := countsPath
			body := countStart
			want := 500
			triggers := map[string]string{
				"create_event":         `CREATE TRIGGER injected BEFORE INSERT ON cash_vault_count_events BEGIN SELECT RAISE(ABORT,'event'); END`,
				"create_header_ignore": `CREATE TRIGGER injected BEFORE INSERT ON cash_vault_count_operations BEGIN SELECT RAISE(IGNORE); END`,
				"submit_ignore":        `CREATE TRIGGER injected BEFORE UPDATE ON cash_vault_counts BEGIN SELECT RAISE(IGNORE); END`,
				"submit_event":         `CREATE TRIGGER injected BEFORE INSERT ON cash_vault_count_events WHEN NEW.event_type='cash.vault.count.submit' BEGIN SELECT RAISE(ABORT,'event'); END`,
				"review_event":         `CREATE TRIGGER injected BEFORE INSERT ON cash_vault_count_events WHEN NEW.event_type='cash.vault.count.accept' BEGIN SELECT RAISE(ABORT,'event'); END`,
			}
			if strings.HasPrefix(stage, "submit") || stage == "review_event" {
				countResult(t, f, f.token, "POST", countsPath, countStart, 201)
				path = countsPath + "/count-one/submit"
				body = countSubmit
				token = receiver
			}
			if stage == "review_event" {
				countResult(t, f, receiver, "POST", path, body, 200)
				path = countsPath + "/count-one/accept"
				body = countAccept
				token = f.token
			}
			if strings.Contains(stage, "ignore") {
				want = 409
			}
			if _, err := f.db.Exec(triggers[stage]); err != nil {
				t.Fatal(err)
			}
			before := countCounts(t, f)
			countResult(t, f, token, "POST", path, body, want)
			if !reflect.DeepEqual(before, countCounts(t, f)) {
				t.Fatal("partial count")
			}
			if _, err := f.db.Exec(`DROP TRIGGER injected`); err != nil {
				t.Fatal(err)
			}
			if strings.HasPrefix(stage, "create") {
				vaultResult(t, f, receiver, "POST", vaultsPath+"/vault-one/supply", supplyVaultBody, 200)
				want = 201
			} else {
				want = 200
				vaultResult(t, f, receiver, "POST", vaultsPath+"/vault-one/supply", supplyVaultBody, 409)
			}
			countResult(t, f, token, "POST", path, body, want)
		})
	}
}
func TestHTTPVaultCountConcurrentStartSubmitAndReviewHaveOneWinner(t *testing.T) {
	f, receiver := countSetup(t)
	var wg sync.WaitGroup
	statuses := make(chan int, 2)
	for _, id := range []string{"one", "two"} {
		id := id
		wg.Add(1)
		go func() {
			defer wg.Done()
			s, _ := request(t, f.app, "POST", countsPath, strings.NewReplacer("start-count-one", "start-count-"+id, "count-one", "count-"+id).Replace(countStart), f.token)
			statuses <- s
		}()
	}
	wg.Wait()
	close(statuses)
	ok, conflict := 0, 0
	for s := range statuses {
		if s == 201 {
			ok++
		} else if s == 409 {
			conflict++
		} else {
			t.Fatal(s)
		}
	}
	if ok != 1 || conflict != 1 {
		t.Fatal(ok, conflict)
	}
	var id string
	if err := f.db.QueryRow(`SELECT id FROM cash_vault_counts`).Scan(&id); err != nil {
		t.Fatal(err)
	}
	statuses = make(chan int, 2)
	for _, amount := range []string{"280", "279"} {
		amount := amount
		wg.Add(1)
		go func() {
			defer wg.Done()
			s, _ := request(t, f.app, "POST", countsPath+"/"+id+"/submit", strings.Replace(countSubmit, "280", amount, 1), receiver)
			statuses <- s
		}()
	}
	wg.Wait()
	close(statuses)
	ok, conflict = 0, 0
	for s := range statuses {
		if s == 200 {
			ok++
		} else if s == 409 {
			conflict++
		} else {
			t.Fatal(s)
		}
	}
	if ok != 1 || conflict != 1 {
		t.Fatal(ok, conflict)
	}
	statuses = make(chan int, 2)
	for _, action := range []string{"accept", "reject"} {
		action := action
		wg.Add(1)
		go func() {
			defer wg.Done()
			s, _ := request(t, f.app, "POST", countsPath+"/"+id+"/"+action, strings.Replace(countAccept, "accept-count-one", action+"-unique", 1), f.token)
			statuses <- s
		}()
	}
	wg.Wait()
	close(statuses)
	ok, conflict = 0, 0
	for s := range statuses {
		if s == 200 {
			ok++
		} else if s == 409 {
			conflict++
		} else {
			t.Fatal(s)
		}
	}
	if ok != 1 || conflict != 1 {
		t.Fatal(ok, conflict)
	}
	if v := readVault(t, f, receiver, "vault-one", 200); v.BalanceCents != 290 {
		t.Fatal(v)
	}
}
func TestHTTPVaultCountBackupRestartPreservesFreezeAndOriginalReceipts(t *testing.T) {
	f, receiver := countSetup(t)
	want := countResult(t, f, f.token, "POST", countsPath, countStart, 201)
	dir := t.TempDir()
	key := make([]byte, 32)
	rand.Read(key)
	archive := filepath.Join(dir, "count.tytbak")
	ctx := context.Background()
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
	actor := identity.Scope{TenantID: f.owner.TenantID, StoreID: f.owner.StoreID, IdentityID: f.owner.OwnerID}
	got, err := cashcount.Get(ctx, db, actor, f.device, "count-one")
	if err != nil || !reflect.DeepEqual(got, want.Count) {
		t.Fatal(got, err)
	}
	if _, err = db.Exec(`UPDATE cash_vaults SET balance_cents=1`); err == nil {
		t.Fatal("restored freeze lost")
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
	verifier, err := entitlements.NewVerifier(map[string]ed25519.PublicKey{"test-issuer": f.private.Public().(ed25519.PublicKey)})
	if err != nil {
		t.Fatal(err)
	}
	f.app, err = NewWithVerifier(f.db, f.device, verifier)
	if err != nil {
		t.Fatal(err)
	}
	if r := countResult(t, f, f.token, "POST", countsPath, countStart, 200); !r.Repeated || !reflect.DeepEqual(r.Count, want.Count) {
		t.Fatal(r)
	}
	vaultResult(t, f, receiver, "POST", vaultsPath+"/vault-one/supply", supplyVaultBody, 409)
	countResult(t, f, receiver, "POST", countsPath+"/count-one/submit", countSubmit, 200)
	countResult(t, f, f.token, "POST", countsPath+"/count-one/accept", countAccept, 200)
}
func TestHTTPVaultCountStrictInputsRolesHistoryAndZero(t *testing.T) {
	f, receiver := countSetup(t)
	for _, bad := range []string{`{}`, strings.Replace(countStart, `"count-one"`, `"bad/id"`, 1), strings.Replace(countStart, `"reason":"Conferencia fisica do cofre"`, `"reason":"x","expected_cents":290`, 1), strings.Replace(countStart, `"counter_id":"receiver"`, `"counter_id":null`, 1)} {
		countResult(t, f, f.token, "POST", countsPath, bad, 400)
	}
	policyPerson(t, f.db, f.owner, "cashier-only", "cashier")
	cashier := loginToken(t, f.app, "cashier-only")
	countResult(t, f, cashier, "POST", countsPath, countStart, 403)
	countResult(t, f, f.token, "POST", countsPath, strings.Replace(countStart, "receiver", f.owner.OwnerID, 1), 403)
	countResult(t, f, f.token, "POST", countsPath, strings.Replace(countStart, "receiver", "cashier-only", 1), 403)
	countResult(t, f, f.token, "POST", countsPath, countStart, 201)
	for _, bad := range []string{`{"operation_id":"no-count"}`, strings.Replace(countSubmit, "280", "-1", 1), strings.Replace(countSubmit, "280", "1.1", 1), strings.Replace(countSubmit, "280", "9007199254740992", 1)} {
		countResult(t, f, receiver, "POST", countsPath+"/count-one/submit", bad, 400)
	}
	zero := strings.Replace(countSubmit, "280", "0", 1)
	c := countResult(t, f, receiver, "POST", countsPath+"/count-one/submit", zero, 200).Count
	if *c.CountedCents != 0 || *c.DifferenceCents != -290 {
		t.Fatal(c)
	}
	countResult(t, f, f.token, "POST", countsPath+"/count-one/reject", countAccept, 200)
	countResult(t, f, receiver, "POST", countsPath+"/count-one/submit", strings.Replace(zero, "submit-count-one", "rewrite", 1), 409)
	for _, q := range []string{`DELETE FROM cash_vault_counts`, `UPDATE cash_vault_counts SET expected_cents=1`, `DELETE FROM cash_vault_count_operations`, `UPDATE cash_vault_count_events SET payload_json='{}'`} {
		if _, err := f.db.Exec(q); err == nil {
			t.Fatal("editable count", q)
		}
	}
	for _, q := range []string{"?limit=0", "?limit=101", "?offset=-1", "?limit=1&limit=2", "?state=submitted", "?tenant_id=other"} {
		if s, _ := request(t, f.app, "GET", countsPath+q, "", receiver); s != 400 {
			t.Fatal(q, s)
		}
	}
	status, data := request(t, f.app, "GET", countsPath+"?limit=1", "", receiver)
	if status != 200 {
		t.Fatal(status, string(data))
	}
	domainSchemaFile(t, "cash-vault-counts.openapi.json", "CountPage", data)
	if bytes.Contains(data, []byte(`"repeated"`)) {
		t.Fatal("list returned receipts")
	}
	f.install(t, signedTestContract(t, f.private, f.owner.TenantID, 3, []modules.ID{modules.Core, modules.Inventory}), 200)
	readCount(t, f, receiver, "count-one", 200)
	countResult(t, f, f.token, "POST", countsPath, strings.NewReplacer("count-one", "count-two", "start-count-one", "start-count-two").Replace(countStart), 403)
	if _, err := f.db.Exec(`UPDATE local_sessions SET revoked_unix=1 WHERE tenant_id=? AND identity_id='receiver'`, f.owner.TenantID); err != nil {
		t.Fatal(err)
	}
	readCount(t, f, receiver, "count-one", 401)
}
