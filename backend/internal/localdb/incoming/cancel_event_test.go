package incoming

import (
	"context"
	"errors"
	"testing"
)

func TestCancelledSaleEventNeedsItsOwnGrantAndReceiptIsStable(t *testing.T) {
	f := setup(t)
	f.grant(t) // Only sale.committed was approved.
	f.event.EventType = "sale.cancelled"
	message := f.message(t)
	ctx := context.Background()
	if _, err := Receive(ctx, f.receiverDB, f.receiver, f.receiverKey, message); !errors.Is(err, ErrDenied) {
		t.Fatalf("aprovacao de venda autorizou cancelamento: %v", err)
	}
	if err := Grant(ctx, f.receiverDB, f.actor, f.receiver, f.sender, "sale.cancelled"); err != nil {
		t.Fatal(err)
	}
	first, err := Receive(ctx, f.receiverDB, f.receiver, f.receiverKey, message)
	if err != nil {
		t.Fatal(err)
	}
	second, err := Receive(ctx, f.receiverDB, f.receiver, f.receiverKey, message)
	if err != nil || !second.Repeated || second.Receipt.ReceiptID != first.Receipt.ReceiptID {
		t.Fatalf("repeticao: %+v %v", second, err)
	}
	receivedCount(t, f, 1)
	var count int
	if err := f.receiverDB.QueryRow(`SELECT COUNT(*) FROM sale_cancellations`).Scan(&count); err != nil || count != 0 {
		t.Fatalf("transporte aplicou estorno: %d %v", count, err)
	}
}
