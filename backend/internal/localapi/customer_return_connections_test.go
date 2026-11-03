package localapi

import (
	"context"
	"crypto/ed25519"
	"sync"
	"testing"

	"titansystem-backend/internal/core/entitlements"
	"titansystem-backend/internal/localdb"
)

func TestHTTPCustomerReturnIndependentConnectionsCannotRefundTwice(t *testing.T) {
	f := cancellationSetup(t)
	in := customerReturnInput(t, f, "return-one", 2000, 3198, "restock")
	body := customerReturnJSON(t, in)
	var seq int
	var name, path string
	if e := f.db.QueryRow(`PRAGMA database_list`).Scan(&seq, &name, &path); e != nil {
		t.Fatal(e)
	}
	second, e := localdb.Open(context.Background(), path)
	if e != nil {
		t.Fatal(e)
	}
	defer second.Close()
	v, e := entitlements.NewVerifier(map[string]ed25519.PublicKey{"test-issuer": f.private.Public().(ed25519.PublicKey)})
	if e != nil {
		t.Fatal(e)
	}
	app, e := NewWithVerifier(second, f.device, v)
	if e != nil {
		t.Fatal(e)
	}
	var wg sync.WaitGroup
	statuses := make(chan int, 2)
	wg.Add(2)
	go func() {
		defer wg.Done()
		status, _ := request(t, f.app, "POST", "/local/v1/sales/sale-one/returns", body, f.token)
		statuses <- status
	}()
	go func() {
		defer wg.Done()
		status, _ := request(t, app, "POST", "/local/v1/sales/sale-one/returns", body, f.token)
		statuses <- status
	}()
	wg.Wait()
	close(statuses)
	created := 0
	for status := range statuses {
		if status == 201 {
			created++
		} else if status != 200 && status != 409 {
			t.Fatal(status)
		}
	}
	if created != 1 {
		t.Fatal("expected exactly one new refund", created)
	}
	customerReturnRequest(t, f, in, 200)
	assertCustomerReturnCount(t, f, 1)
	assertStockBalance(t, f, f.shelf, 5000)
}
