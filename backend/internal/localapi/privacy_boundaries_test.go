package localapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"strings"
	"testing"
	"time"

	"titansystem-backend/internal/localdb/retail"
)

func TestLocalParserDiagnosticsExcludePrivateRequestData(t *testing.T) {
	_, app, _ := fixture(t)
	var captured bytes.Buffer
	// Capture parser diagnostics when startup uses the default logger. The
	// transport-gated mode may already install a non-request sink.
	if app.Server().Logger == nil {
		app.Server().Logger = log.New(&captured, "", 0)
	}
	server, client := net.Pipe()
	done := make(chan error, 1)
	go func() { done <- app.Server().ServeConn(server) }()
	defer client.Close()
	if err := client.SetDeadline(time.Now().Add(3 * time.Second)); err != nil {
		t.Fatal(err)
	}
	secrets := []string{"p09-private-token", "00000000000", "p09-private-name", "p09-private-xml"}
	raw := "POST /local/v1/login?name=" + secrets[2] + " HTTP/1.1\r\nHost: localhost\r\nAuthorization: Bearer " + secrets[0] + "\r\nX-CPF: " + secrets[1] + "\r\nContent-Length: invalid\r\nConnection: close\r\n\r\n<xml>" + secrets[3] + "</xml>"
	if _, err := io.WriteString(client, raw); err != nil {
		t.Fatal("synthetic request write failed")
	}
	response, err := io.ReadAll(client)
	if err != nil {
		t.Fatal("parser response unavailable")
	}
	<-done
	if !bytes.Contains(response, []byte("400")) {
		t.Fatal("malformed request not rejected")
	}
	for i, secret := range secrets {
		if bytes.Contains(response, []byte(secret)) {
			t.Errorf("HTTP diagnostic leaked synthetic field %d", i)
		}
		if strings.Contains(captured.String(), secret) {
			t.Errorf("parser log leaked synthetic field %d", i)
		}
	}
}

func TestRetailAuditAndCustomerMutationCommitOrRollbackTogether(t *testing.T) {
	for _, fault := range []string{"ABORT", "IGNORE"} {
		t.Run(fault, func(t *testing.T) {
			f := arSetup(t)
			customer := retail.Customer{ID: "p09-private-customer", Name: "p09-private-name", CPF: "00000000000", Phone: "p09-private-phone", ConsentReference: "p09-private-consent"}
			payload, _ := json.Marshal(map[string]any{"operation_id": "p09-customer-save", "customer": map[string]any{"id": customer.ID, "revision": customer.Revision, "name": customer.Name, "cpf": customer.CPF, "phone": customer.Phone, "consent_reference": customer.ConsentReference, "credit_limit_cents": customer.CreditLimitCents, "block_overdue": customer.BlockOverdue, "credit_enabled": customer.CreditEnabled, "loyalty_consent": customer.LoyaltyConsent}})
			trigger := fmt.Sprintf(`CREATE TRIGGER p09_reject_audit BEFORE INSERT ON retail_operations WHEN NEW.operation_id='p09-customer-save' BEGIN SELECT RAISE(%s,'synthetic private diagnostic'); END`, fault)
			if fault == "IGNORE" {
				trigger = `CREATE TRIGGER p09_reject_audit BEFORE INSERT ON retail_operations WHEN NEW.operation_id='p09-customer-save' BEGIN SELECT RAISE(IGNORE); END`
			}
			if _, err := f.db.Exec(trigger); err != nil {
				t.Fatal("audit fault setup failed")
			}
			code, body := request(t, f.app, "POST", "/local/v1/retail/customers", string(payload), f.token)
			if code != 500 {
				t.Errorf("audit failure accepted: %d", code)
			}
			for _, secret := range []string{customer.Name, customer.CPF, customer.Phone, customer.ConsentReference, f.token, "synthetic private diagnostic"} {
				if bytes.Contains(body, []byte(secret)) {
					t.Error("HTTP error leaked private input")
				}
			}
			for _, table := range []string{"retail_customers", "retail_operations"} {
				var n int
				if err := f.db.QueryRow(`SELECT count(*) FROM ` + table).Scan(&n); err != nil || n != 0 {
					t.Errorf("audit/mutation partial commit in %s", table)
				}
			}
			if _, err := f.db.Exec(`DROP TRIGGER p09_reject_audit`); err != nil {
				t.Fatal("fault release failed")
			}
			for i := 0; i < 2; i++ {
				code, _ = request(t, f.app, "POST", "/local/v1/retail/customers", string(payload), f.token)
				if code != 204 {
					t.Fatal("same operation recovery failed", code)
				}
			}
			var tenant, store, device, actor, op, hash string
			if err := f.db.QueryRow(`SELECT tenant_id,store_id,device_id,actor_id,operation_id,request_hash FROM retail_operations`).Scan(&tenant, &store, &device, &actor, &op, &hash); err != nil {
				t.Fatal("audit receipt unavailable")
			}
			if tenant != f.owner.TenantID || store != f.owner.StoreID || device != f.device.DeviceID || actor != f.owner.OwnerID || op != "p09-customer-save" || len(hash) != 64 {
				t.Fatal("audit scope or idempotency changed")
			}
			if _, err := retailServiceFor(f).Privacy(context.Background(), "p09-anonymize", customer.ID, 1, true); err != nil {
				t.Fatal("privacy request failed")
			}
			out, err := retailServiceFor(f).Privacy(context.Background(), "p09-export", customer.ID, 2, false)
			if err != nil || !out.Anonymized || out.Name != "" || out.CPF != "" || out.Phone != "" || out.ConsentReference != "" {
				t.Fatal("export resurrected personal data")
			}
			if err := retailServiceFor(f).SaveCustomer(context.Background(), "p09-resurrect", customer); err == nil {
				t.Fatal("anonymized registration resurrected")
			}
		})
	}
}
