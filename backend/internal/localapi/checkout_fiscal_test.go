package localapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"github.com/beevik/etree"
	"io"
	"net"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"time"
	"titansystem-backend/internal/fiscalcore"
	"titansystem-backend/internal/fiscalproduction"
	"titansystem-backend/internal/localdb/fiscal"
	"titansystem-backend/internal/localdb/sale"
)

func TestHTTPCheckoutNFCeHomologationIntegration(t *testing.T) {
	// Reuse the existing disposable mTLS authority/certificate and protocol builder.
	p := newProductionFixture(t, "65", "1", "America/Sao_Paulo")
	other := newProductionFixture(t, "65", "1", "America/Sao_Paulo")
	body := strings.Replace(strings.Replace(fiscalProfileBody, `"issuer-create"`, `"checkout-homol-profile"`, 1), `"expected_revision":0`, `"expected_revision":2`, 1)
	body = strings.TrimSuffix(body, "}") + `,"timezone":"America/Sao_Paulo","require_tax_pricing":true}`
	if code, raw := request(t, p.f.app, "POST", "/local/v1/fiscal/profiles", body, p.f.token); code != 201 {
		t.Fatalf("homologation profile: %d %s", code, raw)
	}
	p.binding.ProfileRevision = 3
	valid, err := p.cert.CRL(false, time.Now().Add(-time.Minute), time.Now().Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	var crlCalls atomic.Int32
	var failAfterSale atomic.Bool
	loader := func() ([][]byte, error) {
		if failAfterSale.Load() && crlCalls.Add(1) >= 3 {
			return nil, errors.New("disposable signer temporarily unavailable")
		}
		return [][]byte{valid}, nil
	}
	p.runtime, err = fiscalproduction.Provision(p.binding, p.cert.Credential, loader, &fiscalcore.QRConfiguration{Version: 3, BaseURL: "https://" + productionTestHost + "/qr"}, "https://"+productionTestHost+"/consult", fiscalproduction.NetworkTrust{ServerRoots: p.cert.Roots, DialContext: func(ctx context.Context, network, address string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, "tcp", p.authority.Listener.Addr().String())
	}})
	if err != nil {
		t.Fatal(err)
	}
	p.installApp(t, p.runtime)
	p.authority.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p.issues.Add(1)
		raw, e := io.ReadAll(io.LimitReader(r.Body, 2*fiscalcore.MaxXML))
		if e != nil {
			w.WriteHeader(400)
			return
		}
		doc := etree.NewDocument()
		if doc.ReadFromBytes(raw) != nil {
			w.WriteHeader(400)
			return
		}
		nfe := doc.Root().FindElement(".//NFe")
		if nfe == nil {
			w.WriteHeader(400)
			return
		}
		if nfe.FindElement("infNFe/ide/tpAmb").Text() != "2" || nfe.FindElement("infNFe/ide/mod").Text() != "65" {
			t.Error("checkout attempted production/wrong model")
			w.WriteHeader(400)
			return
		}
		signed := etree.NewDocument()
		signed.SetRoot(nfe.Copy())
		xml, e := signed.WriteToBytes()
		if e != nil || p.cert.Credential.VerifyXML("nfe", xml, time.Now()) != nil || fiscalcore.ValidateXML("nfe", xml) != nil {
			t.Error("unsigned/invalid invoice")
			w.WriteHeader(400)
			return
		}
		p.lock.Lock()
		p.lastSigned = xml
		p.authorizedAt = time.Now().Format(time.RFC3339)
		p.lock.Unlock()
		w.Header().Set("Content-Type", "application/xml")
		_, _ = w.Write(bytes.ReplaceAll(p.response(false, "100"), []byte("<tpAmb>1</tpAmb>"), []byte("<tpAmb>2</tpAmb>")))
	})
	policy := productionTaxPolicyInput(p)
	policy.ProfileRevision = 3
	taxRequest(t, p.f, "POST", "/local/v1/fiscal/tax-policies", policy, 201, nil)
	if err = fiscal.ConfigureHomologationSeries(context.Background(), p.f.db, journalLicense(t, p.f, p.f.db), numberingActor(p.f), p.f.device, fiscal.SeriesOriginInput{OperationID: "checkout-series", ProfileID: "issuer-one", ProfileRevision: 3, InitialNumber: 17, ReconciliationReference: "disposable existing series authority"}, p.f.token); err != nil {
		t.Fatal(err)
	}
	p.input.Environment = 2
	for _, id := range []string{"checkout-ok", "checkout-retry"} {
		q := taxQuoteFixture(policy)
		q.OperationID = id + "-quote"
		q.Context.Date = issuerToday(t, p)
		q.Item.Quantity = "1"
		q.Item.UnitPrice = "15.99"
		taxRequest(t, p.f, "POST", "/local/v1/fiscal/tax-quotes", q, 201, nil)
		plan := sale.PlanInput{OperationID: id + "-plan", CheckoutID: id, CashSessionID: "turn-one", ExpectedTotalCents: 1599, Items: []sale.Item{{ProductID: p.f.product, LocationID: p.f.shelf, QuantityMilli: 1000}}, Parts: []sale.PlanPartInput{{PartID: id + "-cash", Method: "cash", AmountCents: 1599}}, TaxQuoteOperations: []string{q.OperationID}}
		code, raw := request(t, p.f.app, "POST", planPath, planJSON(t, plan), p.f.token)
		if code != 201 {
			t.Fatalf("plan %d %s", code, raw)
		}
		invoice := p.input
		invoice.Environment = 1
		payload := map[string]any{"operation_id": id + "-finish", "received_cents": 2000, "fiscal_invoice": invoice}
		path := planPath + "/" + id + "/finalize-cash"
		code, _ = request(t, p.f.app, "POST", path, planJSON(t, payload), p.f.token)
		if code != 400 {
			t.Fatal("production accepted", code)
		}
		invoice.Environment = 2
		payload["fiscal_invoice"] = invoice
		if id == "checkout-retry" {
			crlCalls.Store(0)
			failAfterSale.Store(true)
		}
		code, raw = request(t, p.f.app, "POST", path, planJSON(t, payload), p.f.token)
		if code != 201 {
			t.Fatalf("checkout %d %s", code, raw)
		}
		var completed sale.Plan
		if json.Unmarshal(raw, &completed) != nil || completed.Completion == nil || completed.Completion.SaleID != id || completed.Completion.ChangeCents != 401 || completed.Fiscal == nil {
			t.Fatal("sale lost or bad change", string(raw))
		}
		if id == "checkout-retry" {
			if completed.Fiscal.State != "failed" {
				t.Fatalf("failure not separate from committed sale: %+v", completed.Fiscal)
			}
			code, raw = request(t, p.f.app, "GET", planPath+"/"+id, "", p.f.token)
			if code != 200 || !bytes.Contains(raw, []byte(`"state":"failed"`)) {
				t.Fatal("failure not durable", string(raw))
			}
			if !bytes.Contains(raw, []byte(`"can_reprocess":true`)) {
				t.Fatal("server original unavailable", string(raw))
			}
			before := p.issues.Load()
			// Foreign company's token cannot read or replay this company's sale.
			for _, method := range []string{"GET", "POST"} {
				target := planPath + "/" + id
				if method == "POST" {
					target += "/fiscal/reprocess"
				}
				code, _ = request(t, p.f.app, method, target, "{}", other.f.token)
				if code >= 200 && code < 300 {
					t.Fatal("foreign tenant accessed original", code)
				}
			}
			actor := numberingActor(p.f)
			actor.TenantID = other.f.owner.TenantID
			if _, e := fiscal.ReadCheckoutRequest(context.Background(), p.f.db, journalLicense(t, p.f, p.f.db), actor, p.f.device, id, p.f.token); e == nil {
				t.Fatal("foreign tenant read original")
			}
			if p.issues.Load() != before {
				t.Fatal("foreign request reached authority")
			}
			failAfterSale.Store(false)
			// Restart the HTTP application; no original body is supplied again.
			p.installApp(t, p.runtime)
			code, raw = request(t, p.f.app, "POST", planPath+"/"+id+"/fiscal/reprocess", planJSON(t, payload), p.f.token)
			if code != 400 {
				t.Fatal("reprocess accepted an invoice override", code)
			}
			payload["operation_id"] = id + "-changed-finish"
			code, raw = request(t, p.f.app, "POST", path, planJSON(t, payload), p.f.token)
			if code != 409 {
				t.Fatal("committed original request was replaceable", code, string(raw))
			}
			payload["operation_id"] = id + "-finish"
			code, raw = request(t, p.f.app, "POST", planPath+"/"+id+"/fiscal/reprocess", "{}", p.f.token)
			if code != 200 || json.Unmarshal(raw, &completed) != nil {
				t.Fatal("retry failed", code, string(raw))
			}
		}
		if id == "checkout-retry" {
			before := p.issues.Load()
			if code, _ := request(t, p.f.app, "POST", planPath+"/"+id+"/fiscal/reprocess", "{}", p.f.token); code != 200 || p.issues.Load() != before {
				t.Fatal("ID-only replay duplicated emission", code)
			}
		}
		if completed.Fiscal.State != "authorized_homologation" {
			t.Fatalf("not authorized: %+v", completed.Fiscal)
		}
		before := p.issues.Load()
		code, raw = request(t, p.f.app, "POST", path, planJSON(t, payload), p.f.token)
		if code != 200 || p.issues.Load() != before || !bytes.Contains(raw, []byte(`"authorized_homologation"`)) {
			t.Fatal("replay resent or lost authorization", code, string(raw))
		}
		var count int
		if err = p.f.db.QueryRow(`SELECT count(*) FROM sales WHERE tenant_id=? AND id=?`, p.f.owner.TenantID, id).Scan(&count); err != nil || count != 1 {
			t.Fatal("sale duplicated", count, err)
		}
	}
	if p.issues.Load() != 2 {
		t.Fatal("wrong number of authority requests", p.issues.Load())
	}
	t.Log("PASS checkout -> authorized NFC-e homologation; production blocked; failed emission keeps sale/change and durable status; retry/recovery does not duplicate sale or POST.")
}
