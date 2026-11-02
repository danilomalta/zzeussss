package main

import (
	"bytes"
	"context"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"software.sslmate.com/src/go-pkcs12"
	"titansystem-backend/internal/fiscalcore"
	"titansystem-backend/internal/fiscalproduction"
	"titansystem-backend/internal/testsupport/fiscalcert"
)

func disposableA1Configuration(t *testing.T) string {
	t.Helper()
	c, err := fiscalcert.New("12ABC34501DE35", "synthetic.gov.br")
	if err != nil {
		t.Fatal(err)
	}
	const password = "disposable-native-check-only"
	raw, err := pkcs12.Modern2026.Encode(c.LeafKey, c.Leaf, []*x509.Certificate{c.Root}, password)
	if err != nil {
		t.Fatal(err)
	}
	defer clear(raw)
	dir, now := t.TempDir(), time.Now()
	write := func(name string, raw []byte) string {
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, raw, 0600); err != nil {
			t.Fatal(err)
		}
		return p
	}
	crl, err := c.CRL(false, now.Add(-time.Minute), now.Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	config := fiscalproduction.FileConfiguration{Version: 1,
		Binding:    fiscalcore.ProductionBinding{ID: "cli-check", TenantID: "tenant", StoreID: "store", DeviceID: "device", ProfileID: "issuer-one", ProfileRevision: 1, Issuer: "12ABC34501DE35", UF: "SP", Timezone: "America/Sao_Paulo", Model: "55", Series: 1, InitialNumber: 10, SoleAuthorityReference: "disposable-fixture", EndpointRegistryReference: "disposable-fixture", HomologationReference: "not-official-fixture", DeveloperReference: "disposable-fixture", AuthorizationURL: "https://synthetic.gov.br/issue", ConsultationURL: "https://synthetic.gov.br/query", CertificateSHA256: c.Credential.Fingerprint(), ValidFrom: now.Add(-time.Minute).UTC().Format(time.RFC3339), ValidUntil: now.Add(time.Hour).UTC().Format(time.RFC3339)},
		PKCS12Path: write("identity.pfx", raw), PasswordPath: write("password-private", []byte(password)), IssuerRootsPath: write("root.pem", pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: c.Root.Raw})), CRLPaths: []string{write("current.crl", crl)}}
	configRaw, err := json.Marshal(config)
	if err != nil {
		t.Fatal(err)
	}
	return write("installation-private.json", configRaw)
}

func assertPrivateInspection(t *testing.T, output []byte) {
	t.Helper()
	var out fiscalproduction.ConfigurationInspection
	if err := json.Unmarshal(output, &out); err != nil || !out.ConfigurationValid || !out.CertificateAndRevocationValid || out.OfficialHomologationVerified || out.TransmissionPerformed || out.Model != "55" || out.UF != "SP" || len(out.ConfigurationSHA256) != 64 {
		t.Fatal("inspection did not reflect installation validation without pretending to transmit")
	}
	for _, forbidden := range []string{"password-private", "installation-private", "disposable-native-check-only", "12ABC34501DE35", "synthetic.gov.br", "PRIVATE KEY"} {
		if bytes.Contains(output, []byte(forbidden)) {
			t.Fatal("inspection disclosed private inputs")
		}
	}
}

func TestFiscalProductionConfigOfflineInspection(t *testing.T) {
	path := disposableA1Configuration(t)
	var out bytes.Buffer
	if err := run([]string{"check-production-config", "--in", path}, &out); err != nil {
		t.Fatal(err)
	}
	assertPrivateInspection(t, out.Bytes())
	for _, args := range [][]string{
		{"check-production-config", "--in", path, "--out", filepath.Join(t.TempDir(), "must-not-write")},
		{"check-production-config", "--in", path, "--kind", "nfe"},
		{"check-production-config", "--in", path, "--password", "never-accept-inline"},
		{"check-production-config", "--in", path, "extra"},
		{"check-production-config", "--in", filepath.Join(t.TempDir(), "private-unreadable-name")},
	} {
		out.Reset()
		err := run(args, &out)
		if err == nil || out.Len() != 0 || strings.Contains(err.Error(), "private-unreadable-name") || strings.Contains(err.Error(), "never-accept-inline") {
			t.Fatal("invalid check leaked information or reported success")
		}
	}
}

func TestFiscalProductionConfigNativeCLI(t *testing.T) {
	path := os.Getenv("TITAN_FISCAL_CLI")
	if path == "" {
		t.Skip("optional: set TITAN_FISCAL_CLI to the compiled titan-fiscal executable")
	}
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()
	config := disposableA1Configuration(t)
	cmd := exec.CommandContext(ctx, path, "check-production-config", "--in", config)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatal("compiled offline configuration check failed", err)
	}
	assertPrivateInspection(t, out)
	cmd = exec.CommandContext(ctx, path, "check-production-config", "--in", filepath.Join(t.TempDir(), "private-nonexistent"))
	out, err = cmd.CombinedOutput()
	if err == nil || bytes.Contains(out, []byte("private-nonexistent")) || bytes.Contains(out, []byte(`"configuration_valid":true`)) {
		t.Fatal("native failure invented readiness or leaked a private path")
	}
	t.Log("Compiled titan-fiscal loaded protected A1, validated identity/chain/current CRL, returned bounded non-secret JSON, made no fiscal transmission and rejected invalid installation with nonzero exit.")
}
