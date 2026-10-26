package localapi

import (
	"context"
	"crypto/rand"
	"net/http"
	"path/filepath"
	"reflect"
	"testing"
	"time"
	"titansystem-backend/internal/core/modules"
	"titansystem-backend/internal/localdb"
	"titansystem-backend/internal/localdb/backup"
	"titansystem-backend/internal/localdb/supplycycle"
	"titansystem-backend/internal/traderelay"
)

// Two independent databases, station keys, user sessions and OS processes.
// The driver explicitly relays signed documents over real authenticated HTTP;
// it is not an Internet relay, notification worker or supplier UI mock.
func pilotAuthenticateSupplier(t *testing.T, buyer *pilotHTTP, _ *stockHTTPFixture, _ string, _ string) {
	t.Helper()
	sellerFixture := stockHTTPSetup(t)
	station := pilotPrepareCLIStation(t, sellerFixture)
	sellerFixture.install(t, signedTestContract(t, sellerFixture.private, sellerFixture.owner.TenantID, 100, []modules.ID{modules.Core, modules.Inventory, modules.POS, modules.Orders, modules.Logistics, modules.Finance}), 200)
	var seq int
	var name, dbPath string
	if e := sellerFixture.db.QueryRow(`PRAGMA database_list`).Scan(&seq, &name, &dbPath); e != nil {
		t.Fatal(e)
	}
	if e := sellerFixture.db.Close(); e != nil {
		t.Fatal(e)
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = nil
	defer transport.CloseIdleConnections()
	seller := &pilotHTTP{t: t, service: pilotStart(t, dbPath, sellerFixture, station), client: &http.Client{Transport: transport, Timeout: 15 * time.Second}, token: sellerFixture.token}
	defer func() { seller.service.stop(t) }()
	type publicIdentity struct {
		Party       supplycycle.TradeParty `json:"party"`
		PublicKey   []byte                 `json:"public_key"`
		Fingerprint string                 `json:"fingerprint_sha256"`
	}
	var bi, si publicIdentity
	buyer.call("GET", "/local/v1/company-trade/identity", nil, 200, &bi)
	seller.call("GET", "/local/v1/company-trade/identity", nil, 200, &si)
	if bi.Party.TenantID == si.Party.TenantID || bi.Fingerprint == si.Fingerprint {
		t.Fatal("supplier is not independent")
	}
	buyer.call("POST", "/local/v1/company-trade/peers", supplycycle.TradePeerInput{OperationID: "pilot-trust-supplier", ID: "pilot-vendor", Remote: si.Party, PublicKey: si.PublicKey, FingerprintSHA256: si.Fingerprint, Role: "supplier", SupplierID: "supplier", VerificationReference: "Synthetic owners explicitly verify independent key fingerprints"}, 200, nil)
	seller.call("POST", "/local/v1/company-trade/peers", supplycycle.TradePeerInput{OperationID: "pilot-trust-buyer", ID: "pilot-customer", Remote: bi.Party, PublicKey: bi.PublicKey, FingerprintSHA256: bi.Fingerprint, Role: "customer", VerificationReference: "Synthetic owners explicitly verify independent key fingerprints"}, 200, nil)
	var offer supplycycle.TradeResult
	buyer.call("POST", cycleOrder+"/company-trade/export", supplycycle.TradeExportInput{OperationID: "pilot-export", PeerID: "pilot-vendor", Reason: "Envio explicito do pedido aprovado pelo comercio"}, 200, &offer)
	input := supplycycle.TradeImportInput{OperationID: "pilot-import", PeerID: "pilot-customer", Envelope: *offer.Envelope}
	if *pilotTradeCLI == "" {
		seller.call("POST", "/local/v1/company-trade/orders/import", input, 200, nil)
	} else {
		pilotRunRelayCLI(t, *pilotTradeCLI, buyer, seller, traderelay.Job{Version: 1, Source: traderelay.Endpoint{URL: buyer.service.url + "/local/v1", Party: bi.Party, ActorID: offer.Message.ActorID, Fingerprint: bi.Fingerprint}, Target: traderelay.Endpoint{URL: seller.service.url + "/local/v1", Party: si.Party, ActorID: sellerFixture.owner.OwnerID, Fingerprint: si.Fingerprint}, MessageID: offer.Message.ID, PeerID: input.PeerID, OperationID: input.OperationID}, t.TempDir(), "send")
	}
	seller.service.stop(t)
	seller.service = pilotStart(t, dbPath, sellerFixture, station)
	seller.call("GET", "/local/v1/company-trade/operations/pilot-import", nil, 200, nil)
	var decision supplycycle.TradeResult
	seller.call("POST", "/local/v1/company-trade/orders/"+offer.Message.ID+"/decision", supplycycle.TradeDecisionInput{OperationID: "pilot-supplier-decision", Decision: "confirmed", AcceptedMilli: 2000, PromisedAt: time.Now().UTC().Add(time.Hour).Format(time.RFC3339), Reason: "Fornecedor autenticado confirmou duas unidades"}, 200, &decision)
	before := decision
	// Prove the supplier's outbound decision survives its own encrypted backup.
	var restored string
	if *pilotBackupCLI != "" {
		restored = pilotCLIBackupRoundTrip(t, sellerFixture, dbPath, station)
	} else {
		ctx := context.Background()
		db, e := localdb.Open(ctx, dbPath)
		if e != nil {
			t.Fatal(e)
		}
		defer db.Close()
		dir := t.TempDir()
		archive := filepath.Join(dir, "supplier.tytbak")
		restored = filepath.Join(dir, "restored.sqlite")
		key := make([]byte, 32)
		if _, e = rand.Read(key); e != nil {
			t.Fatal(e)
		}
		if e = backup.Create(ctx, db, sellerFixture.device, key, archive); e != nil {
			t.Fatal(e)
		}
		if e = backup.Verify(ctx, archive, sellerFixture.device, key); e != nil {
			t.Fatal(e)
		}
		if e = backup.Restore(ctx, archive, restored, sellerFixture.device, key); e != nil {
			t.Fatal(e)
		}
	}
	seller.service.stop(t)
	seller.service = pilotStart(t, restored, sellerFixture, station)
	seller.call("GET", "/local/v1/me", nil, 401, nil)
	seller.token = seller.login(sellerFixture.owner.OwnerID)
	seller.call("GET", "/local/v1/company-trade/operations/pilot-supplier-decision", nil, 200, &decision)
	if !reflect.DeepEqual(before, decision) {
		t.Fatal("supplier original signed decision changed after restore")
	}
	applied := supplycycle.TradeImportInput{OperationID: "pilot-apply", PeerID: "pilot-vendor", Envelope: *decision.Envelope}
	var result supplycycle.TradeResult
	if *pilotTradeCLI == "" {
		buyer.call("POST", cycleOrder+"/company-trade/decision", applied, 200, &result)
	} else {
		// The restored sender must explicitly review retained counterpart trust
		// before relaying a saved decision. A read-only receipt is not that review.
		var recovery supplycycle.TradeRecovery
		seller.call("GET", "/local/v1/company-trade/recovery", nil, 200, &recovery)
		if recovery.Required {
			seller.call("POST", "/local/v1/company-trade/recovery/review", supplycycle.TradeRecoveryInput{OperationID: "pilot-relay-review", Generation: recovery.Generation, VerificationReference: "Synthetic supplier owner reconfirmed buyer station fingerprint " + bi.Fingerprint}, 200, nil)
		}
		job := traderelay.Job{Version: 1, Source: traderelay.Endpoint{URL: seller.service.url + "/local/v1", Party: si.Party, ActorID: sellerFixture.owner.OwnerID, Fingerprint: si.Fingerprint}, Target: traderelay.Endpoint{URL: buyer.service.url + "/local/v1", Party: bi.Party, ActorID: offer.Message.ActorID, Fingerprint: bi.Fingerprint}, MessageID: decision.Message.ID, PeerID: applied.PeerID, OperationID: applied.OperationID}
		directory := t.TempDir()
		pilotRunRelayCLI(t, *pilotTradeCLI, seller, buyer, job, directory, "send")
		pilotRunRelayCLI(t, *pilotTradeCLI, seller, buyer, job, directory, "recover")
		buyer.call("GET", "/local/v1/company-trade/operations/"+applied.OperationID, nil, 200, &result)
	}
	if result.Cycle.Decision.EvidenceMode != "authenticated_company" || !result.Cycle.Decision.ExternalVerified {
		t.Fatal("supplier acceptance is not authenticated")
	}
	buyer.call("POST", cycleOrder+"/company-trade/decision", applied, 200, &result)
	if !result.Repeated {
		t.Fatal("supplier decision duplicated")
	}
	// Never log envelopes, bearer tokens, private station files or login payloads.
	t.Logf("SUPPLIER PROCESS: %d real HTTP calls; 3 independent process lifetimes; inbound order and outbound signed decision restored unchanged", seller.steps)
}
