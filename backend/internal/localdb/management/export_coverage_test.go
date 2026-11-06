package management

import (
	"bytes"
	"context"
	"encoding/csv"
	"testing"
)

func TestSalesCSVAlwaysDeclaresPageCoverageIncludingEmptyPage(t *testing.T) {
	f := setupSales(t)
	f.add(t, "first", "t", "s", "d", "cash", "a", "2026-10-02T10:00:00Z", "committed", 1000, 100)
	f.add(t, "second", "t", "s", "d", "cash", "b", "2026-10-02T11:00:00Z", "committed", 1000, 200)
	for _, offset := range []int{0, 20} {
		in := input()
		in.Limit = 1
		in.Offset = offset
		r, err := Sales(context.Background(), f.db, f.a, f.d, in)
		if err != nil {
			t.Fatal(err)
		}
		var buf bytes.Buffer
		if err = ExportSales(&buf, r); err != nil {
			t.Fatal(err)
		}
		rows, err := csv.NewReader(&buf).ReadAll()
		if err != nil || len(rows) < 2 {
			t.Fatal("CSV hides empty-page coverage", err)
		}
		meta := map[string]string{}
		for i, k := range rows[0] {
			meta[k] = rows[1][i]
		}
		if meta["row_type"] != "coverage" || meta["total_products"] != "2" || meta["period_revenue_cents"] != "300" || meta["page_complete"] != "false" || meta["basis"] != r.Basis {
			t.Fatal("CSV omits or conflates page and period coverage")
		}
	}
}
