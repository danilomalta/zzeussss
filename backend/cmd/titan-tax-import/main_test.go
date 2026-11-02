package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"titansystem-backend/internal/fiscalcore"
	"titansystem-backend/internal/localdb/fiscal"
)

func TestTaxImportRefusesRemoteAuthorityUnknownDataAndSecretOutput(t *testing.T) {
	path := filepath.Join(t.TempDir(), "table.json")
	os.WriteFile(path, []byte(`{"version":1,"revision":"v1","regime":"simples_nacional","policies":[]}`), 0600)
	for _, api := range []string{"http://192.0.2.2", "https://name.invalid", "https://user:pass@127.0.0.1", "http://127.0.0.1"} {
		var out bytes.Buffer
		err := run(context.Background(), []string{"--in", path, "--api", api}, strings.NewReader("SyntheticSessionSecret"), &out)
		if err == nil || out.Len() != 0 || strings.Contains(err.Error(), "SyntheticSessionSecret") {
			t.Fatal("unsafe import")
		}
	}
}

func TestTaxImportPublishesVersionedRulesOnlyToLoopbackWithUnapprovedDefault(t *testing.T) {
	table := fiscal.TaxTable{Version: 1, Revision: "v1", Regime: "simples_nacional", Policies: []fiscal.TaxPolicyInput{{OperationID: "import-one", ProfileID: "profile", ProfileRevision: 1, Active: true,
		Rule: fiscalcore.TaxRule{ID: "policy", ProductID: "product", CFOP: "5102", NCM: "19059090", OriginUF: "SP", DestinationUF: "SP", CRT: "1", Model: "65", ValidFrom: "2026-01-01", ValidUntil: "2026-12-31", LegalReference: "Synthetic calculation fixture only", ApprovalReference: "owner-fixture", BaseRounding: "tax_final", Components: []fiscalcore.TaxComponent{{Code: "ICMS", Method: "percentage", Rate: "18", ReductionPercent: "0", MVA: "0"}}},
		Case: fiscalcore.TaxCase{Operation: "sale", Purpose: "final_consumption", OriginMunicipality: "3550308", DestinationMunicipality: "3550308", RecipientStatus: "noncontributor", FinalConsumer: "yes", ProductOrigin: "0"}, Sources: []string{"https://www.planalto.gov.br/ccivil_03/leis/lcp/lcp87.htm"}}}}
	var calls int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.URL.Path != "/local/v1/fiscal/tax-policies" || r.Header.Get("Authorization") != "Bearer SyntheticSessionSecret" {
			t.Error("wrong local authority")
		}
		var input fiscal.TaxPolicyInput
		json.NewDecoder(r.Body).Decode(&input)
		if input.Review == nil || input.AccountantApproved() || input.Review.TableRevision != "v1" {
			t.Error("invented approval")
		}
		w.WriteHeader(201)
		json.NewEncoder(w).Encode(fiscal.TaxPolicy{Input: input, Revision: 1})
	}))
	defer server.Close()
	path := filepath.Join(t.TempDir(), "table.json")
	raw, _ := json.Marshal(table)
	os.WriteFile(path, raw, 0600)
	var output bytes.Buffer
	if err := run(context.Background(), []string{"--in", path, "--api", server.URL}, strings.NewReader("SyntheticSessionSecret"), &output); err != nil || calls != 1 {
		t.Fatal("local import", err, calls)
	}
	if strings.Contains(output.String(), "SyntheticSessionSecret") || strings.Contains(output.String(), "5102") {
		t.Fatal("sensitive data in output")
	}
}
