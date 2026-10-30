package fiscalproduction

import (
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/asn1"
	"encoding/json"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"titansystem-backend/internal/fiscalcore"
	"titansystem-backend/internal/testsupport/fiscalcert"
)

func bindingForTest(c *fiscalcert.Fixture) fiscalcore.ProductionBinding {
	now := time.Now()
	return fiscalcore.ProductionBinding{ID: "synthetic-production", TenantID: "tenant", StoreID: "store", DeviceID: "device", ProfileID: "issuer-one", ProfileRevision: 1, Issuer: "12ABC34501DE35", UF: "SP", Timezone: "America/Sao_Paulo", Model: "55", Series: 1, InitialNumber: 10, SoleAuthorityReference: "synthetic-single-authority", EndpointRegistryReference: "synthetic-endpoints", HomologationReference: "synthetic-reference-only", DeveloperReference: "synthetic-developer", AuthorizationURL: "https://synthetic.gov.br/issue", ConsultationURL: "https://synthetic.gov.br/query", CertificateSHA256: c.Credential.Fingerprint(), ValidFrom: now.Add(-time.Minute).UTC().Format(time.RFC3339), ValidUntil: now.Add(time.Hour).UTC().Format(time.RFC3339)}
}

func TestProductionRevocationEvidenceNeverDefaultsToGood(t *testing.T) {
	c, err := fiscalcert.New("12ABC34501DE35", "synthetic.gov.br")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	valid, err := c.CRL(false, now.Add(-time.Minute), now.Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if err = c.Credential.CheckRevocation([][]byte{valid}, now); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"missing", "revoked", "expired", "future", "modified", "delta", "unknown-critical"} {
		t.Run(name, func(t *testing.T) {
			var list []byte
			switch name {
			case "missing":
			case "revoked":
				list, err = c.CRL(true, now.Add(-time.Minute), now.Add(time.Hour))
			case "expired":
				list, err = c.CRL(false, now.Add(-time.Hour), now.Add(-time.Minute))
			case "future":
				list, err = c.CRL(false, now.Add(time.Minute), now.Add(time.Hour))
			case "modified":
				list = append([]byte(nil), valid...)
				list[len(list)-1] ^= 1
			default:
				oid := asn1.ObjectIdentifier{2, 5, 29, 27}
				if name == "unknown-critical" {
					oid = asn1.ObjectIdentifier{1, 2, 3, 4, 5}
				}
				template := &x509.RevocationList{Number: big.NewInt(3), ThisUpdate: now.Add(-time.Minute), NextUpdate: now.Add(time.Hour), ExtraExtensions: []pkix.Extension{{Id: oid, Critical: true, Value: []byte{2, 1, 1}}}}
				list, err = x509.CreateRevocationList(rand.Reader, template, c.Root, c.RootKey)
			}
			if err != nil {
				t.Fatal(err)
			}
			var lists [][]byte
			if list != nil {
				lists = [][]byte{list}
			}
			if c.Credential.CheckRevocation(lists, now) == nil {
				t.Fatal("invalid CRL accepted", name)
			}
		})
	}
}

func TestProductionBindingScopeExpiryAndTransportPolicy(t *testing.T) {
	c, err := fiscalcert.New("12ABC34501DE35", "synthetic.gov.br")
	if err != nil {
		t.Fatal(err)
	}
	b := bindingForTest(c)
	if !b.Valid(time.Now()) {
		t.Fatal("valid binding")
	}
	for _, name := range []string{"http", "private-host", "credentials-url", "fragment", "port", "expired", "unknown-uf", "wrong-zone", "missing-reference", "zero-number"} {
		t.Run(name, func(t *testing.T) {
			copy := b
			switch name {
			case "http":
				copy.AuthorizationURL = "http://synthetic.gov.br/issue"
			case "private-host":
				copy.AuthorizationURL = "https://127.0.0.1/issue"
			case "credentials-url":
				copy.AuthorizationURL = "https://user:secret@synthetic.gov.br/issue"
			case "fragment":
				copy.AuthorizationURL += "#value"
			case "port":
				copy.AuthorizationURL = "https://synthetic.gov.br:443/issue"
			case "expired":
				copy.ValidUntil = time.Now().Add(-time.Second).UTC().Format(time.RFC3339)
			case "unknown-uf":
				copy.UF = "XX"
			case "wrong-zone":
				copy.Timezone = "America/New_York"
			case "missing-reference":
				copy.HomologationReference = ""
			case "zero-number":
				copy.InitialNumber = 0
			}
			if copy.Valid(time.Now()) {
				t.Fatal("invalid binding accepted")
			}
		})
	}
}

func TestProductionPrivateConfigurationAndLiveCRLReload(t *testing.T) {
	c, err := fiscalcert.New("12ABC34501DE35", "synthetic.gov.br")
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	write := func(name string, raw []byte) string {
		t.Helper()
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, raw, 0600); err != nil {
			t.Fatal(err)
		}
		return p
	}
	key, err := x509.MarshalPKCS8PrivateKey(c.LeafKey)
	if err != nil {
		t.Fatal(err)
	}
	defer clear(key)
	valid, err := c.CRL(false, time.Now().Add(-time.Minute), time.Now().Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	config := FileConfiguration{Version: 1, Binding: bindingForTest(c), CertificatePath: write("leaf.pem", pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: c.Leaf.Raw})), PrivateKeyPath: write("private.pem", pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: key})), IssuerRootsPath: write("roots.pem", pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: c.Root.Raw})), CRLPaths: []string{write("current.crl", valid)}}
	raw, _ := json.Marshal(config)
	path := write("production.json", raw)
	runtime, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if runtime.Binding().SHA256() != config.Binding.SHA256() {
		t.Fatal("configuration changed")
	}
	if err = os.Chmod(path, 0644); err != nil {
		t.Fatal(err)
	}
	if _, err = Load(path); err == nil {
		t.Fatal("public credential configuration accepted")
	}
	if err = os.Chmod(path, 0600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "linked.json")
	if err = os.Symlink(path, link); err != nil {
		t.Fatal(err)
	}
	if _, err = Load(link); err == nil {
		t.Fatal("symlink configuration accepted")
	}
	revoked, err := c.CRL(true, time.Now().Add(-time.Minute), time.Now().Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	write("current.crl", revoked)
	if runtime.Validate() == nil {
		t.Fatal("credential revocation ignored without restart")
	}
	if _, err = Load(filepath.Join(dir, "credential-with-private-name.json")); err == nil || strings.Contains(err.Error(), "credential-with-private-name") {
		t.Fatal("private path leaked")
	}
}

func TestProductionNFCeSupplementBindingExcludesCSCSecret(t *testing.T) {
	c, err := fiscalcert.New("12ABC34501DE35", "synthetic.gov.br")
	if err != nil {
		t.Fatal(err)
	}
	valid, err := c.CRL(false, time.Now().Add(-time.Minute), time.Now().Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	b := bindingForTest(c)
	b.Model = "65"
	qr := &fiscalcore.QRConfiguration{Version: 2, BaseURL: "https://synthetic.gov.br/qr", CSCID: "1", CSC: "synthetic-csc-for-automated-test-only"}
	runtime, err := Provision(b, c.Credential, func() ([][]byte, error) { return [][]byte{valid}, nil }, qr, "https://synthetic.gov.br/consult", NetworkTrust{})
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(runtime.Binding())
	if strings.Contains(string(raw), qr.CSC) || runtime.Binding().SupplementSHA256 == "" {
		t.Fatal("supplement/secret binding")
	}
	bad := runtime.Binding()
	bad.SupplementSHA256 = strings.Repeat("0", 64)
	if _, err = Provision(bad, c.Credential, func() ([][]byte, error) { return [][]byte{valid}, nil }, qr, "https://synthetic.gov.br/consult", NetworkTrust{}); err == nil {
		t.Fatal("mismatched supplement accepted")
	}
	qr.CSC = ""
	if _, err = Provision(b, c.Credential, func() ([][]byte, error) { return [][]byte{valid}, nil }, qr, "https://synthetic.gov.br/consult", NetworkTrust{}); err == nil {
		t.Fatal("missing CSC accepted")
	}
}
