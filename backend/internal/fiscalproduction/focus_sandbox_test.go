package fiscalproduction

import (
	"context"
	"crypto/rand"
	"encoding/hex"
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
// This disposable Simples Nacional fixture must match the sandbox issuer's CRT.
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
	nonce := make([]byte, 16)
	if _, err = rand.Read(nonce); err != nil {
		t.Fatal("cannot allocate disposable sale identifier")
	}
	ref := focusReference("explicit-sandbox-tenant", hex.EncodeToString(nonce))
	// Names and codes are from the official NotaFiscalXML/ItemNotaFiscalXML
	// schema and its examples (CFOP 5102, NCM 18061000). No inferred tax rates.
	body, _ := json.Marshal(map[string]any{"cnpj_emitente": cnpj, "data_emissao": time.Now().UTC().Format(time.RFC3339), "natureza_operacao": "VENDA AO CONSUMIDOR", "tipo_documento": "1", "local_destino": "1", "consumidor_final": "1", "presenca_comprador": "1", "indicador_inscricao_estadual_destinatario": "9", "modalidade_frete": "9", "items": []map[string]string{{"numero_item": "1", "codigo_produto": "SANDBOX", "descricao": "TESTE EM HOMOLOGACAO SEM VALOR FISCAL", "codigo_ncm": "18061000", "cfop": "5102", "unidade_comercial": "UN", "quantidade_comercial": "1.000", "valor_unitario_comercial": "1.00", "valor_bruto": "1.00", "icms_origem": "0", "icms_situacao_tributaria": "102", "pis_situacao_tributaria": "07", "cofins_situacao_tributaria": "07"}}, "formas_pagamento": []map[string]string{{"forma_pagamento": "01", "valor_pagamento": "1.00"}}, "valor_total": "1.00"})
	ctx, cancel := context.WithTimeout(context.Background(), 80*time.Second)
	defer cancel()
	emitted, err := p.exchange(ctx, "POST", ref, cnpj, body)
	if err != nil { // Consultation is the ONLY recovery, even if POST timed out.
		emitted, err = p.exchange(ctx, "GET", ref, cnpj, nil)
	}
	if err != nil || emitted.State != "authorized_homologation" {
		t.Fatal("REAL homologation did not return NFC-e authorization; inspect issuer configuration privately")
	}
	consulted, err := p.exchange(ctx, "GET", ref, cnpj, nil)
	if err != nil || consulted.State != "authorized_homologation" || consulted.Key != emitted.Key {
		t.Fatal("REAL homologation consultation did not confirm the same NFC-e")
	}
}
