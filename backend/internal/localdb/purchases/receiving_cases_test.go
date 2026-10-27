package purchases

import (
	"testing"
	"time"
)

func TestReceivingCaseDueCanonicalAndOverdueDoesNotChangeStatus(t *testing.T) {
	due, e := caseDue("2026-10-22T09:00:00-03:00")
	if e != nil || due != "2026-10-22T12:00:00Z" {
		t.Fatal(due, e)
	}
	for _, s := range []string{"bad", "2026-10-22T12:00:00.1Z", "2201-01-01T00:00:00Z"} {
		if _, e := caseDue(s); e == nil {
			t.Fatal(s)
		}
	}
	now, _ := time.Parse(time.RFC3339, "2026-10-22T13:00:00Z")
	row := ReceivingCase{Status: "open", DueAt: due}
	if out := caseDetail(row, now); !out.Overdue || out.Case.Status != "open" {
		t.Fatal(out)
	}
	row.Status = "resolved"
	if caseDetail(row, now).Overdue {
		t.Fatal("closed case overdue")
	}
}
func TestReceivingCaseTextAndPolicyEnums(t *testing.T) {
	if !caseText("Conferência\nRevisada", 1000, false) || caseText(" x ", 1000, false) || caseText("\x00", 1000, false) || caseText("", 1000, false) || caseKind("fraud_proven") || caseOutcome("refund_paid") {
		t.Fatal("invalid text or invented settlement accepted")
	}
}
