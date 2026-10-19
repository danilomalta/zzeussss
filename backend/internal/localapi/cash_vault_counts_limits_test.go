package localapi

import (
	"strings"
	"testing"
)

func TestHTTPVaultCountEmptyVaultFreezeMaximumCountAndArchive(t *testing.T) {
	f, receiver := countSetup(t)
	vaultResult(t, f, f.token, "POST", vaultsPath, strings.NewReplacer("create-vault-one", "create-vault-two", "vault-one", "vault-two").Replace(createVaultBody), 201)
	countResult(t, f, f.token, "POST", countsPath, strings.Replace(countStart, "vault-one", "vault-two", 1), 201)
	vaultResult(t, f, f.token, "POST", vaultsPath+"/vault-two/archive", `{"operation_id":"archive-empty-counted","reason":"Vazio"}`, 409)
	vaultResult(t, f, receiver, "POST", vaultsPath+"/vault-one/transfer", transferVaultBody, 409)
	r := countResult(t, f, receiver, "POST", countsPath+"/count-one/submit", strings.Replace(countSubmit, "280", "9007199254740991", 1), 200)
	if *r.Count.ExpectedCents != 0 || *r.Count.CountedCents != 9007199254740991 || *r.Count.DifferenceCents != 9007199254740991 {
		t.Fatal(r)
	}
	countResult(t, f, f.token, "POST", countsPath+"/count-one/accept", countAccept, 200)
	if v := readVault(t, f, receiver, "vault-two", 200); v.BalanceCents != 0 {
		t.Fatal("count fabricated cash", v)
	}
	vaultResult(t, f, f.token, "POST", vaultsPath+"/vault-two/archive", `{"operation_id":"archive-empty-counted","reason":"Vazio"}`, 200)
	if r := vaultReport(t, f, receiver, 200); r.TotalBalanceCents != 290 || r.DivergentLatestCount != 1 || r.ArchivedCount != 1 {
		t.Fatal(r)
	}
}
func TestHTTPVaultReportLimitRejectsBeforeReturningAnyVault(t *testing.T) {
	f, receiver := countSetup(t)
	// Exercise the bound without inventing 1000 valid monetary operations: all
	// fixture rows obey SQL FKs, but are deliberately not offered as valid ledgers.
	// The report must reject the size before producing any balances.
	tx, err := f.db.Begin()
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	_, err = tx.Exec(`WITH RECURSIVE n(x) AS (SELECT 1 UNION ALL SELECT x+1 FROM n WHERE x<1000)
 INSERT INTO cash_vaults(tenant_id,store_id,device_id,id,name,state,balance_cents,created_by,created_at,updated_at,creation_operation_id,last_operation_id)
 SELECT v.tenant_id,v.store_id,v.device_id,'limit-'||n.x,'Fixture','active',0,v.created_by,v.created_at,v.updated_at,v.creation_operation_id,v.last_operation_id FROM cash_vaults v,n WHERE v.id='vault-one'`)
	if err != nil {
		t.Fatal(err)
	}
	if err = tx.Commit(); err != nil {
		t.Fatal(err)
	}
	vaultReport(t, f, receiver, 422)
	if s, b := request(t, f.app, "GET", vaultsPath+"/report.csv", "", receiver); s != 422 || strings.Contains(string(b), "summary") {
		t.Fatal(s, string(b))
	}
}
