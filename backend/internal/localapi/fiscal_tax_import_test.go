package localapi

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"testing"
	"titansystem-backend/internal/localdb/fiscal"
)

func TestFiscalTaxAtomicImportConcurrentWinnerAndFileHash(t *testing.T) {
	f := fiscalFixture(t)
	const workers = 6
	start := make(chan struct{})
	var wg sync.WaitGroup
	type answer struct {
		code      int
		body, raw []byte
	}
	answers := make(chan answer, workers)
	for n := 0; n < workers; n++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			first := taxPolicyFixture(f)
			first.OperationID = fmt.Sprintf("table-%d-first", n)
			second := taxPolicyFixture(f)
			second.Rule.ID = "policy-two"
			second.OperationID = fmt.Sprintf("table-%d-second", n)
			second.Rule.ValidFrom = "2027-01-01"
			second.Rule.ValidUntil = "2027-12-31"
			table := fiscal.TaxTable{Version: 1, Revision: fmt.Sprintf("table-%d", n), Regime: "simples_nacional", Policies: []fiscal.TaxPolicyInput{first, second}}
			raw, _ := json.Marshal(table)
			<-start
			code, body := request(t, f.app, "POST", "/local/v1/fiscal/tax-tables/import", string(raw), f.token)
			answers <- answer{code, body, raw}
		}(n)
	}
	close(start)
	wg.Wait()
	close(answers)
	wins := 0
	var winning answer
	for a := range answers {
		if a.code == 201 {
			wins++
			winning = a
		} else if a.code != 409 {
			t.Fatalf("wanted revision conflict, got %d %s", a.code, a.body)
		}
	}
	if wins != 1 {
		t.Fatalf("winners=%d", wins)
	}
	sum := sha256.Sum256(winning.raw)
	want := hex.EncodeToString(sum[:])
	rows, e := f.db.Query(`SELECT request_json FROM fiscal_tax_policies ORDER BY policy_id`)
	if e != nil {
		t.Fatal(e)
	}
	defer rows.Close()
	count := 0
	for rows.Next() {
		var raw string
		var in fiscal.TaxPolicyInput
		if rows.Scan(&raw) != nil || json.Unmarshal([]byte(raw), &in) != nil {
			t.Fatal("unreadable row")
		}
		if in.Review == nil || in.Review.FileSHA256 != want || in.AccountantApproved() {
			t.Fatal("mixed file, hash or approval")
		}
		count++
	}
	if rows.Err() != nil || count != 2 {
		t.Fatal("partial file", count, rows.Err())
	}
	code, body := request(t, f.app, "POST", "/local/v1/fiscal/tax-tables/import", string(winning.raw), f.token)
	var saved []fiscal.TaxPolicy
	if code != 201 || json.Unmarshal(body, &saved) != nil || len(saved) != 2 || !saved[0].Repeated || !saved[1].Repeated {
		t.Fatal("recovery changed import")
	}
}
func TestFiscalTaxAtomicImportRollsBackAndRejectsInvalidFile(t *testing.T) {
	f := fiscalFixture(t)
	first := taxPolicyFixture(f)
	second := first
	second.Rule.ID = "policy-two"
	second.OperationID = "second-row"
	second.ProfileRevision = 999
	table := fiscal.TaxTable{Version: 1, Revision: "v1", Regime: "simples_nacional", Policies: []fiscal.TaxPolicyInput{first, second}}
	raw, _ := json.Marshal(table)
	code, _ := request(t, f.app, "POST", "/local/v1/fiscal/tax-tables/import", string(raw), f.token)
	if code != 409 {
		t.Fatal("expected rollback conflict", code)
	}
	var count int
	f.db.QueryRow(`SELECT count(*) FROM fiscal_tax_policies`).Scan(&count)
	if count != 0 {
		t.Fatal("partial publication", count)
	}
	for _, bad := range []string{strings.Replace(string(raw), `"active":true,`, "", 1), strings.Replace(string(raw), `,"adds_to_total":false`, "", 1), strings.Replace(string(raw), `"valid_from":"2026-01-01"`, `"valid_from":""`, 1), strings.Replace(string(raw), `"sources":["https://www.planalto.gov.br/ccivil_03/leis/lcp/lcp87.htm"]`, `"sources":[]`, 1)} {
		if _, e := fiscal.DecodeTaxTable([]byte(bad)); e == nil {
			t.Fatal("invalid schema/source/validity accepted")
		}
	}
}
