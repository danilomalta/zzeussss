package fiscalproduction

import (
	"context"
	"encoding/json"
	"flag"
	"os"
	"testing"
	"time"
)

// CATEGORY 2: REAL Focus homologation; NEVER part of an ordinary test run.
// Customer must configure A1, CSC and the legal issuer in Focus beforehand.
// Manual command (the agent must not execute it):
// go test -timeout 120s -count=1 ./internal/fiscalproduction -run '^TestFocusSandboxRealNFCe$' -args -focus-sandbox
// Required private environment: FOCUS_SANDBOX_TOKEN, FOCUS_SANDBOX_CNPJ.
// Customer-reviewed payload and a durable attempt file are required; no fiscal defaults.
var runFocusSandbox = flag.Bool("focus-sandbox", false, "explicitly allow REAL Focus homologation emission")

func TestFocusSandboxRealNFCe(t *testing.T) {
	if os.Getenv("FOCUS_SANDBOX_TOKEN") == "" || os.Getenv("FOCUS_SANDBOX_CNPJ") == "" {
		t.Skip("REAL Focus test not executed: FOCUS_SANDBOX_TOKEN and FOCUS_SANDBOX_CNPJ are required")
	}
	if !*runFocusSandbox {
		t.Skip("REAL Focus test not executed: explicit -focus-sandbox opt-in is required")
	}
	token := []byte(os.Getenv("FOCUS_SANDBOX_TOKEN"))
	defer clear(token)
	cnpj := os.Getenv("FOCUS_SANDBOX_CNPJ")
	vault, _ := testVault(t)
	if err := vault.Put(context.Background(), "explicit-sandbox-tenant", "token", token); err != nil {
		t.Fatal("cannot encrypt sandbox credential")
	}
	stored, err := vault.read(context.Background(), "explicit-sandbox-tenant", "token")
	if err != nil {
		t.Fatal("cannot load sandbox credential")
	}
	defer clear(stored)
	p, err := newFocusProvider(TenantFiscalConfiguration{Environment: "homologation", CertificateType: "A1"}, stored)
	if err != nil {
		t.Fatal("sandbox provider configuration rejected")
	}
	defer clear(p.token)
	body, ref, firstAttempt, err := prepareCustomerSandbox(os.Getenv("FOCUS_SANDBOX_PAYLOAD_FILE"), os.Getenv("FOCUS_SANDBOX_ATTEMPT_FILE"), cnpj, os.Getenv("FOCUS_SANDBOX_SALE_ID"))
	if err != nil {
		t.Fatal("private customer payload or durable attempt configuration rejected; no emission attempted")
	}
	defer clear(body)
	ctx, cancel := context.WithTimeout(context.Background(), 80*time.Second)
	defer cancel()
	// The attempt is durably recorded before POST. Re-running only consults.
	var emitted DocumentResult
	if firstAttempt {
		emitted, err = p.exchange(ctx, "POST", ref, cnpj, body)
		if err != nil { // Consultation is the ONLY recovery after an uncertain POST.
			emitted, err = p.exchange(ctx, "GET", ref, cnpj, nil)
		}
	} else {
		emitted, err = p.exchange(ctx, "GET", ref, cnpj, nil)
	}
	if err != nil || emitted.State != "authorized_homologation" {
		t.Fatal("REAL homologation did not return NFC-e authorization; inspect issuer configuration privately")
	}
	consulted, err := p.exchange(ctx, "GET", ref, cnpj, nil)
	if err != nil || consulted.State != "authorized_homologation" || consulted.Key != emitted.Key {
		t.Fatal("REAL homologation consultation did not confirm the same NFC-e")
	}
	var links focusArchiveLinks
	if json.Unmarshal(consulted.Evidence, &links) != nil || links.Invoice == "" {
		t.Fatal("REAL authorization lacks retrievable invoice XML; keep acceptance pending")
	}
	xml, err := p.downloadArchiveXML(ctx, links.Invoice, consulted.Key, "invoice", 0)
	if err != nil {
		t.Fatal("REAL authorized XML could not be obtained or verified; keep acceptance pending")
	}
	defer clear(xml)
	if !sandboxProtocolPresent(xml) {
		t.Fatal("REAL authorized XML lacks protocol; keep acceptance pending")
	}
	t.Log("REAL homologation authorization, same-reference consultation and XML/protocol checks completed; no automatic cancellation")
}
