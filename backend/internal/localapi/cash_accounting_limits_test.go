package localapi

import (
	"fmt"
	"testing"
	"titansystem-backend/internal/localdb/cashaccounting"
)

func periodAccountant(t *testing.T, f *stockHTTPFixture) string {
	t.Helper()
	policyPerson(t, f.db, f.owner, "period-accountant", "accountant")
	return loginToken(t, f.app, "period-accountant")
}
func TestHTTPAccountingJournalEntryLimitChecksCompletePeriod(t *testing.T) {
	for _, count := range []int{9999, 10000} {
		t.Run(fmt.Sprint(count), func(t *testing.T) {
			f := salesSetup(t)
			bulkJournalSources(t, f, count)
			body := fmt.Sprintf(`{"session_id":"turn-one","operation_id":"close-bulk","declared_cents":%d}`, 1000+count)
			cashRequest(t, f.httpContractFixture, "close", body, 200)
			setAccountingClosedAt(t, f, "turn-one", closedFirst)
			token := periodAccountant(t, f)
			q := accountingQuery(closedFirst, periodEnd)
			if count+1 > cashaccounting.MaxEntries {
				accountingReport(t, f, token, q, 422)
				accountingCSV(t, f, token, q, 422)
			} else {
				r := accountingReport(t, f, token, q, 200)
				if len(r.Entries) != 10000 || r.Summary["journal_entry_count"] != 10000 || r.Summary["sum_expected_at_closures_cents"] != 10999 {
					t.Fatal(r.Summary, len(r.Entries))
				}
				rows := accountingCSV(t, f, token, q, 200)
				n := 0
				for _, row := range rows {
					if row["row_type"] == "journal" {
						n++
					}
				}
				if n != 10000 {
					t.Fatal("partial successful export", n)
				}
			}
		})
	}
}
func TestHTTPAccountingSessionAndSelectionScanLimitsAreExplicit(t *testing.T) {
	for _, test := range []struct {
		name  string
		count int
		date  string
	}{{"period", 501, closedFirst}, {"scope", 10001, "2020-01-01T00:00:00Z"}} {
		t.Run(test.name, func(t *testing.T) {
			f := salesSetup(t)
			token := periodAccountant(t, f)
			// Guard-only fixture: these metadata rows obey SQL FKs but deliberately
			// do not claim valid full drawer histories or a throughput benchmark.
			_, err := f.db.Exec(`WITH RECURSIVE n(x) AS(SELECT 1 UNION ALL SELECT x+1 FROM n WHERE x<?)
 INSERT INTO cash_sessions(tenant_id,store_id,device_id,id,operator_id,opened_at,closed_at,opening_cents)
 SELECT tenant_id,store_id,device_id,'limit-'||n.x,operator_id,opened_at,?,0 FROM cash_sessions,n WHERE id='turn-one'`, test.count, test.date)
			if err != nil {
				t.Fatal(err)
			}
			q := accountingQuery(closedFirst, periodEnd)
			accountingReport(t, f, token, q, 422)
			accountingCSV(t, f, token, q, 422)
		})
	}
}
func TestHTTPAccountingExactMaximumAndAggregateOverflowUseClosedSources(t *testing.T) {
	f := salesSetup(t)
	cashRequest(t, f.httpContractFixture, "close", `{"session_id":"turn-one","operation_id":"close-initial","declared_cents":1000}`, 200)
	cashRequest(t, f.httpContractFixture, "open", `{"session_id":"max-turn","opening_cents":9007199254740991}`, 201)
	cashRequest(t, f.httpContractFixture, "close", `{"session_id":"max-turn","operation_id":"close-max","declared_cents":9007199254740991}`, 200)
	setAccountingClosedAt(t, f, "max-turn", closedFirst)
	cashRequest(t, f.httpContractFixture, "open", `{"session_id":"one-turn","opening_cents":1}`, 201)
	cashRequest(t, f.httpContractFixture, "close", `{"session_id":"one-turn","operation_id":"close-small","declared_cents":1}`, 200)
	setAccountingClosedAt(t, f, "one-turn", closedSecond)
	token := periodAccountant(t, f)
	if r := accountingReport(t, f, token, accountingQuery(closedFirst, closedSecond), 200); r.Summary["sum_openings_cents"] != 9007199254740991 {
		t.Fatal(r.Summary)
	}
	q := accountingQuery(closedFirst, periodEnd)
	accountingReport(t, f, token, q, 422)
	accountingCSV(t, f, token, q, 422)
}
