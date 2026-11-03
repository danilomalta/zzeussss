package purchases

import (
	"slices"
	"testing"
)

func TestReceivingOverviewFlagsPreserveBusinessBoundaries(t *testing.T) {
	r := ReceivingOverview{CommercialStatus: "local_not_sent", RemainingMilli: 3000, EffectiveAcceptedMilli: 2000, PartialRejectedMilli: 500, FullRejectionCount: 1, VoidedReceiptCount: 1, TotalAppointments: 1, Agenda: AgendaCounts{Completed: 1}}
	flags := overviewFlags(r)
	for _, want := range []string{"local_receiving_authorization_missing", "accepted_quantity_pending", "partial_receiving", "rejections_recorded_review_agreement", "receipt_void_recorded_not_refund", "completed_dock_with_receiving_pending_review"} {
		if !slices.Contains(flags, want) {
			t.Fatal(flags, want)
		}
	}
}
func TestReceivingOverviewCancelledNoAutomaticReorderOrCollection(t *testing.T) {
	r := ReceivingOverview{CommercialStatus: "cancelled", RemainingMilli: 5000, TotalAppointments: 1, Agenda: AgendaCounts{Reserved: 1}}
	flags := overviewFlags(r)
	if !slices.Contains(flags, "order_cancelled_no_new_receiving") || !slices.Contains(flags, "cancelled_order_with_active_agenda_review") || slices.Contains(flags, "accepted_quantity_pending") || slices.Contains(flags, "local_receiving_authorization_missing") {
		t.Fatal(flags)
	}
}
func TestReceivingOverviewCompleteWithoutDockIsNotSupplierOrPaymentProof(t *testing.T) {
	r := ReceivingOverview{CommercialStatus: "local_not_sent", Authorization: &ReceivingAuthorization{}, PlannedMilli: 5000, EffectiveAcceptedMilli: 5000}
	flags := overviewFlags(r)
	if len(flags) != 1 || flags[0] != "no_local_dock_appointment" {
		t.Fatal(flags)
	}
}
