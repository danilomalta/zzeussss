package localapi

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"reflect"
	"strings"
	"testing"
	"titansystem-backend/internal/localdb/cashcount"
)

func vaultReport(t *testing.T, f *stockHTTPFixture, token string, want int) cashcount.Report {
	t.Helper()
	status, data := request(t, f.app, "GET", vaultsPath+"/report", "", token)
	if status != want {
		t.Fatalf("report %d want%d %s", status, want, data)
	}
	var r cashcount.Report
	if want == 200 {
		if err := json.Unmarshal(data, &r); err != nil {
			t.Fatal(err)
		}
		domainSchemaFile(t, "cash-vault-counts.openapi.json", "VaultReport", data)
	} else if bytes.Contains(data, []byte("total_balance_cents")) {
		t.Fatal("partial report", string(data))
	}
	return r
}
func TestHTTPVaultReportAndCSVUseExactScopedBalancesAndLatestPhysicalCount(t *testing.T) {
	f, receiver := countSetup(t)
	vaultResult(t, f, f.token, "POST", vaultsPath, strings.NewReplacer("create-vault-one", "create-vault-two", "vault-one", "vault-two").Replace(createVaultBody), 201)
	vaultResult(t, f, receiver, "POST", vaultsPath+"/vault-one/transfer", transferVaultBody, 200)
	countResult(t, f, f.token, "POST", countsPath, countStart, 201)
	report := vaultReport(t, f, receiver, 200)
	if report.Scope != "local_physical_vaults" || report.TotalBalanceCents != 290 || report.VaultCount != 2 || report.ActiveCount != 2 || report.CountingCount != 1 || report.AwaitingReviewCount != 0 || report.Items[0].LatestCount.ExpectedCents != nil {
		t.Fatal(report)
	}
	countResult(t, f, receiver, "POST", countsPath+"/count-one/submit", strings.Replace(countSubmit, "280", "180", 1), 200)
	report = vaultReport(t, f, receiver, 200)
	if report.TotalBalanceCents != 290 || report.CountingCount != 0 || report.AwaitingReviewCount != 1 || report.DivergentLatestCount != 1 || *report.Items[0].LatestCount.ExpectedCents != 190 || *report.Items[0].LatestCount.DifferenceCents != -10 {
		t.Fatal(report)
	}
	status, data := request(t, f.app, "GET", vaultsPath+"/report.csv", "", receiver)
	if status != 200 {
		t.Fatal(status, string(data))
	}
	reader := csv.NewReader(strings.NewReader(strings.TrimPrefix(string(data), "\ufeff")))
	reader.Comma = ';'
	rows, err := reader.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 4 || rows[1][1] != "summary" || rows[1][7] != "290" || rows[1][15] != "2" || rows[1][19] != "1" || rows[2][4] != "vault-one" || rows[2][7] != "190" || rows[2][10] != "190" || rows[2][11] != "180" || rows[2][12] != "-10" || rows[3][7] != "100" {
		t.Fatal(rows)
	}
	countResult(t, f, f.token, "POST", countsPath+"/count-one/reject", countAccept, 200)
	report = vaultReport(t, f, f.token, 200)
	if report.AwaitingReviewCount != 0 || report.DivergentLatestCount != 1 || report.TotalBalanceCents != 290 {
		t.Fatal("rejection silently corrected money", report)
	}
	// A new count supersedes the displayed count while old history remains readable.
	next := strings.NewReplacer("start-count-one", "start-count-two", "count-one", "count-two").Replace(countStart)
	countResult(t, f, f.token, "POST", countsPath, next, 201)
	if r := vaultReport(t, f, receiver, 200); r.Items[0].LatestCount.ID != "count-two" || r.DivergentLatestCount != 0 || r.CountingCount != 1 {
		t.Fatal(r)
	}
	if old := readCount(t, f, receiver, "count-one", 200); old.State != "rejected" || *old.DifferenceCents != -10 {
		t.Fatal(old)
	}
}
func TestHTTPVaultReportRefusesDamagedCountHistoryWithoutRepairOrPartialCSV(t *testing.T) {
	for _, damage := range []string{"balance", "receipt", "private_event"} {
		t.Run(damage, func(t *testing.T) {
			f, receiver := countSetup(t)
			countResult(t, f, f.token, "POST", countsPath, countStart, 201)
			countResult(t, f, receiver, "POST", countsPath+"/count-one/submit", countSubmit, 200)
			switch damage {
			case "balance":
				if _, err := f.db.Exec(`DROP TRIGGER cash_vault_frozen; UPDATE cash_vaults SET balance_cents=1`); err != nil {
					t.Fatal(err)
				}
			case "receipt":
				if _, err := f.db.Exec(`DROP TRIGGER cash_vault_count_ops_no_update; UPDATE cash_vault_count_operations SET result_json='{}' WHERE action='create'`); err != nil {
					t.Fatal(err)
				}
			case "private_event":
				if _, err := f.db.Exec(`DROP TRIGGER cash_vault_count_events_no_delete; DELETE FROM cash_vault_count_events WHERE event_type='cash.vault.count.create'`); err != nil {
					t.Fatal(err)
				}
			}
			before := countCounts(t, f)
			readCount(t, f, receiver, "count-one", 409)
			vaultReport(t, f, receiver, 409)
			s, b := request(t, f.app, "GET", vaultsPath+"/report.csv", "", receiver)
			if s != 409 || bytes.HasPrefix(b, []byte{0xef, 0xbb, 0xbf}) || bytes.Contains(b, []byte("summary")) {
				t.Fatal(s, string(b))
			}
			if !reflect.DeepEqual(before, countCounts(t, f)) {
				t.Fatal("read repaired history")
			}
		})
	}
}
func TestHTTPVaultReportRejectsFormulaTextKeepsJSONAndChecksQuery(t *testing.T) {
	f, receiver := countSetup(t)
	unsafe := strings.NewReplacer("create-vault-one", "create-unsafe", "vault-one", "vault-unsafe", "Cofre da loja", "=HYPERLINK(1)").Replace(createVaultBody)
	vaultResult(t, f, f.token, "POST", vaultsPath, unsafe, 201)
	if r := vaultReport(t, f, receiver, 200); r.TotalBalanceCents != 290 || r.VaultCount != 2 || r.Items[1].Vault.Name != "=HYPERLINK(1)" {
		t.Fatal(r)
	}
	if s, b := request(t, f.app, "GET", vaultsPath+"/report.csv", "", receiver); s != 409 || bytes.Contains(b, []byte("format_version")) {
		t.Fatal(s, string(b))
	}
	for _, path := range []string{vaultsPath + "/report?limit=1", vaultsPath + "/report.csv?offset=0"} {
		if s, _ := request(t, f.app, "GET", path, "", receiver); s != 400 {
			t.Fatal(path, s)
		}
	}
	if s, _ := request(t, f.app, "GET", vaultsPath+"/report", "{}", receiver); s != 400 {
		t.Fatal(s)
	}
}
func TestHTTPVaultReportEmptyArchivedAndIntegerRangeRemainExplicit(t *testing.T) {
	f, receiver := bagSetup(t)
	if r := vaultReport(t, f, receiver, 200); r.VaultCount != 0 || r.TotalBalanceCents != 0 || len(r.Items) != 0 {
		t.Fatal(r)
	}
	// Two individually valid balances can exceed the safe aggregate range.
	bagResult(t, f, f.token, "POST", bagsPath, createBagBody, 201)
	bagResult(t, f, f.token, "POST", bagsPath+"/bag-one/dispatch", dispatchBagBody, 200)
	bagResult(t, f, receiver, "POST", bagsPath+"/bag-one/receive", strings.Replace(receiveBagBody, "290", "9007199254740991", 1), 200)
	vaultResult(t, f, f.token, "POST", vaultsPath, createVaultBody, 201)
	vaultResult(t, f, receiver, "POST", vaultsPath+"/vault-one/deposit", depositVaultBody, 200)
	if r := vaultReport(t, f, receiver, 200); r.TotalBalanceCents != 9007199254740991 {
		t.Fatal(r)
	}
	vaultResult(t, f, f.token, "POST", vaultsPath, strings.NewReplacer("create-vault-one", "create-vault-two", "vault-one", "vault-two").Replace(createVaultBody), 201)
	vaultResult(t, f, f.token, "POST", vaultsPath+"/vault-two/archive", `{"operation_id":"archive-empty-two","reason":"Cofre vazio"}`, 200)
	if r := vaultReport(t, f, receiver, 200); r.ArchivedCount != 1 || r.VaultCount != 2 || r.TotalBalanceCents != 9007199254740991 {
		t.Fatal(r)
	}
	countResult(t, f, f.token, "POST", countsPath, strings.Replace(countStart, "vault-one", "vault-two", 1), 409)
	vaultResult(t, f, f.token, "POST", vaultsPath, strings.NewReplacer("create-vault-one", "create-vault-three", "vault-one", "vault-three").Replace(createVaultBody), 201)
	cashRequest(t, f.httpContractFixture, "movements", strings.Replace(withdrawalBody, "withdraw-one", "withdraw-two", 1), 200)
	bagResult(t, f, f.token, "POST", bagsPath, strings.NewReplacer("prepare-one", "prepare-two", "bag-one", "bag-two", "seal-one", "seal-two", "withdraw-one", "withdraw-two").Replace(createBagBody), 201)
	bagResult(t, f, f.token, "POST", bagsPath+"/bag-two/dispatch", `{"operation_id":"dispatch-two"}`, 200)
	bagResult(t, f, receiver, "POST", bagsPath+"/bag-two/receive", strings.NewReplacer("receive-one", "receive-two", "seal-one", "seal-two", "290", "1").Replace(receiveBagBody), 200)
	vaultResult(t, f, receiver, "POST", vaultsPath+"/vault-three/deposit", strings.NewReplacer("deposit-one", "deposit-two", "bag-one", "bag-two").Replace(depositVaultBody), 200)
	vaultReport(t, f, receiver, 422)
	if s, b := request(t, f.app, "GET", vaultsPath+"/report.csv", "", receiver); s != 422 || bytes.Contains(b, []byte("summary")) {
		t.Fatal(s, string(b))
	}
}
