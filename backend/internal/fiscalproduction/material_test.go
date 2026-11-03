package fiscalproduction

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"software.sslmate.com/src/go-pkcs12"
	"titansystem-backend/internal/fiscalcore"
	"titansystem-backend/internal/testsupport/fiscalcert"
)

const disposablePassword = "fixture-only-PFX-ç-2026"

func privatePFXConfiguration(t *testing.T, c *fiscalcert.Fixture, pfx []byte) (FileConfiguration, string) {
	t.Helper()
	dir := t.TempDir()
	write := func(name string, raw []byte) string {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, raw, 0600); err != nil {
			t.Fatal(err)
		}
		return path
	}
	now := time.Now()
	crl, err := c.CRL(false, now.Add(-time.Minute), now.Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	cfg := FileConfiguration{Version: 1, Binding: bindingForTest(c), PKCS12Path: write("identity.pfx", pfx), PasswordPath: write("private-password", []byte(disposablePassword)), IssuerRootsPath: write("trusted-root.pem", pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: c.Root.Raw})), CRLPaths: []string{write("current.crl", crl)}}
	return cfg, write("installation.json", nil)
}

func writePFXConfiguration(t *testing.T, cfg FileConfiguration, path string) {
	t.Helper()
	raw, err := json.Marshal(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
}

func TestProductionA1PFXModernAndLegacyActualSignature(t *testing.T) {
	c, err := fiscalcert.New("12ABC34501DE35", "synthetic.gov.br")
	if err != nil {
		t.Fatal(err)
	}
	for name, encoder := range map[string]*pkcs12.Encoder{"modern2026": pkcs12.Modern2026, "modern2023": pkcs12.Modern2023, "legacyDES": pkcs12.LegacyDES} {
		t.Run(name, func(t *testing.T) {
			pfx, err := encoder.Encode(c.LeafKey, c.Leaf, []*x509.Certificate{c.Root}, disposablePassword)
			if err != nil {
				t.Fatal(err)
			}
			defer clear(pfx)
			cfg, path := privatePFXConfiguration(t, c, pfx)
			writePFXConfiguration(t, cfg, path)
			runtime, err := Load(path)
			if err != nil || runtime.Binding().SHA256() != cfg.Binding.SHA256() || runtime.credential.Fingerprint() != c.Credential.Fingerprint() {
				t.Fatal("PFX installation did not preserve issuer identity", err)
			}
			unsigned, _, err := fiscalcore.BuildInvalidation(fiscalcore.Invalidation{UF: "SP", CNPJ: cfg.Binding.Issuer, Model: "55", Series: 1, Year: 2026, First: 30, Last: 31, Environment: 2, Reason: "Disposable cryptographic signature check only"})
			if err != nil {
				t.Fatal(err)
			}
			signed, err := runtime.credential.SignXML("nfe_invalidation", unsigned, time.Now())
			if err != nil || runtime.credential.VerifyXML("nfe_invalidation", signed, time.Now()) != nil || !bytes.Contains(signed, []byte("<Signature")) {
				t.Fatal("PFX signer did not produce a verifiable XML signature", err)
			}
			inspection, err := InspectConfiguration(path)
			report, _ := json.Marshal(inspection)
			if err != nil || !inspection.ConfigurationValid || !inspection.CertificateAndRevocationValid || inspection.OfficialHomologationVerified || inspection.TransmissionPerformed {
				t.Fatal("offline inspection invented authority", err)
			}
			for _, private := range []string{disposablePassword, cfg.PasswordPath, cfg.PKCS12Path, cfg.Binding.Issuer, cfg.Binding.AuthorizationURL, "PRIVATE KEY"} {
				if bytes.Contains(report, []byte(private)) {
					t.Fatal("private installation data leaked in report")
				}
			}
			// The decrypted signer remains in memory; deleting input material does
			// not export a key or turn off the independent live CRL guard.
			revoked, err := c.CRL(true, time.Now().Add(-time.Minute), time.Now().Add(time.Hour))
			if err != nil || os.WriteFile(cfg.CRLPaths[0], revoked, 0600) != nil {
				t.Fatal("cannot update disposable CRL", err)
			}
			if runtime.Validate() == nil {
				t.Fatal("PFX signer ignored revocation after load")
			}
		})
	}
}

func TestProductionA1PFXRejectsUnsafeOrAmbiguousMaterial(t *testing.T) {
	c, err := fiscalcert.New("12ABC34501DE35", "synthetic.gov.br")
	if err != nil {
		t.Fatal(err)
	}
	pfx, err := pkcs12.Modern2026.Encode(c.LeafKey, c.Leaf, []*x509.Certificate{c.Root}, disposablePassword)
	if err != nil {
		t.Fatal(err)
	}
	defer clear(pfx)
	tests := map[string]func(*testing.T, *FileConfiguration, string){
		"missing_password_file": func(t *testing.T, c *FileConfiguration, p string) { c.PasswordPath = "" },
		"missing_pfx_file":      func(t *testing.T, c *FileConfiguration, p string) { c.PKCS12Path = "" },
		"mixed_pem_certificate": func(t *testing.T, c *FileConfiguration, p string) { c.CertificatePath = c.PKCS12Path },
		"mixed_pem_key":         func(t *testing.T, c *FileConfiguration, p string) { c.PrivateKeyPath = c.PKCS12Path },
		"relative_pfx":          func(t *testing.T, c *FileConfiguration, p string) { c.PKCS12Path = "identity.pfx" },
		"wrong_password": func(t *testing.T, c *FileConfiguration, p string) {
			mustPrivateWrite(t, c.PasswordPath, []byte("wrong-fixture-password"))
		},
		"empty_password": func(t *testing.T, c *FileConfiguration, p string) { mustPrivateWrite(t, c.PasswordPath, nil) },
		"nul_password": func(t *testing.T, c *FileConfiguration, p string) {
			mustPrivateWrite(t, c.PasswordPath, []byte("fixture\x00suffix"))
		},
		"non_utf8_password": func(t *testing.T, c *FileConfiguration, p string) { mustPrivateWrite(t, c.PasswordPath, []byte{255}) },
		"oversize_password": func(t *testing.T, c *FileConfiguration, p string) {
			mustPrivateWrite(t, c.PasswordPath, bytes.Repeat([]byte("x"), 1025))
		},
		"public_password_file": func(t *testing.T, c *FileConfiguration, p string) { mustPermission(t, c.PasswordPath, 0644) },
		"public_pfx_file":      func(t *testing.T, c *FileConfiguration, p string) { mustPermission(t, c.PKCS12Path, 0644) },
		"public_config_file":   func(t *testing.T, c *FileConfiguration, p string) { mustPermission(t, p, 0644) },
		"corrupt_pfx": func(t *testing.T, c *FileConfiguration, p string) {
			mustPrivateWrite(t, c.PKCS12Path, []byte("not a PFX"))
		},
		"trailing_der": func(t *testing.T, c *FileConfiguration, p string) {
			mustPrivateWrite(t, c.PKCS12Path, append(append([]byte{}, pfx...), 0))
		},
		"fingerprint_mismatch": func(t *testing.T, c *FileConfiguration, p string) {
			c.Binding.CertificateSHA256 = strings.Repeat("0", 64)
		},
		"other_issuer": func(t *testing.T, c *FileConfiguration, p string) { c.Binding.Issuer = "11222333000181" },
		"expired_installation": func(t *testing.T, c *FileConfiguration, p string) {
			c.Binding.ValidUntil = time.Now().Add(-time.Minute).Format(time.RFC3339)
		},
		"missing_crl": func(t *testing.T, c *FileConfiguration, p string) {
			c.CRLPaths[0] = filepath.Join(t.TempDir(), "missing.crl")
		},
		"symlink_pfx": func(t *testing.T, c *FileConfiguration, p string) { c.PKCS12Path = privateTestLink(t, c.PKCS12Path) },
		"symlink_password": func(t *testing.T, c *FileConfiguration, p string) {
			c.PasswordPath = privateTestLink(t, c.PasswordPath)
		},
		"embedded_roots_are_not_trusted": func(t *testing.T, cfg *FileConfiguration, p string) {
			other, err := fiscalcert.New("11222333000181", "other.gov.br")
			if err != nil {
				t.Fatal(err)
			}
			mustPrivateWrite(t, cfg.IssuerRootsPath, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: other.Root.Raw}))
		},
		"non_rsa_key": func(t *testing.T, cfg *FileConfiguration, p string) {
			key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
			if err != nil {
				t.Fatal(err)
			}
			raw, err := pkcs12.Modern2026.Encode(key, c.Leaf, []*x509.Certificate{c.Root}, disposablePassword)
			if err != nil {
				t.Fatal(err)
			}
			defer clear(raw)
			mustPrivateWrite(t, cfg.PKCS12Path, raw)
		},
	}
	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			cfg, path := privatePFXConfiguration(t, c, pfx)
			writePFXConfiguration(t, cfg, path)
			mutate(t, &cfg, path)
			writePFXConfiguration(t, cfg, path)
			_, err := Load(path)
			if !errors.Is(err, ErrUnavailable) || err.Error() != ErrUnavailable.Error() {
				t.Fatal("unsafe credential accepted or private details leaked")
			}
		})
	}
}

func mustPrivateWrite(t *testing.T, path string, raw []byte) {
	t.Helper()
	if err := os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
}
func mustPermission(t *testing.T, path string, mode os.FileMode) {
	t.Helper()
	if err := os.Chmod(path, mode); err != nil {
		t.Fatal(err)
	}
}
func privateTestLink(t *testing.T, path string) string {
	t.Helper()
	link := filepath.Join(t.TempDir(), "linked")
	if err := os.Symlink(path, link); err != nil {
		t.Fatal(err)
	}
	return link
}
