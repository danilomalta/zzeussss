package fiscalproduction

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"

	_ "modernc.org/sqlite"
	"software.sslmate.com/src/go-pkcs12"
	"titansystem-backend/internal/fiscalcore"
)

// FiscalSecretVault is trusted process wiring, never an HTTP response model.
// FISCAL_MASTER_KEY must contain exactly 32 random bytes encoded as base64.
// Tenant and purpose are authenticated with the ciphertext, preventing row swaps.
type FiscalSecretVault struct {
	db   *sql.DB
	aead cipher.AEAD
}

func (*FiscalSecretVault) String() string   { return "[fiscal secret vault]" }
func (*FiscalSecretVault) GoString() string { return "[fiscal secret vault]" }
func OpenFiscalSecretVault(path string) (*FiscalSecretVault, error) {
	key, err := base64.StdEncoding.Strict().DecodeString(os.Getenv("FISCAL_MASTER_KEY"))
	defer clear(key)
	if err != nil || len(key) != 32 || !filepath.IsAbs(path) || strings.ContainsAny(path, "?#") {
		return nil, ErrUnavailable
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, ErrUnavailable
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, ErrUnavailable
	}
	// Create privately without changing permissions of an existing database.
	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_RDWR, 0600)
	if err == nil {
		f.Close()
	} else if !os.IsExist(err) {
		return nil, ErrUnavailable
	}
	if !privateReference(path) {
		return nil, ErrUnavailable
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, ErrUnavailable
	}
	if _, err = db.Exec(`CREATE TABLE IF NOT EXISTS fiscal_secret_vault (tenant TEXT NOT NULL, purpose TEXT NOT NULL, ciphertext BLOB NOT NULL, PRIMARY KEY (tenant,purpose))`); err != nil {
		db.Close()
		return nil, ErrUnavailable
	}
	return &FiscalSecretVault{db: db, aead: aead}, nil
}
func (v *FiscalSecretVault) Close() error { return v.db.Close() }
func vaultAAD(tenant, purpose string) ([]byte, error) {
	if tenant == "" || len(tenant) > 256 || strings.ContainsRune(tenant, 0) || (purpose != "certificate" && purpose != "csc" && purpose != "token") {
		return nil, ErrUnavailable
	}
	return []byte("fiscal-vault-v1\x00" + tenant + "\x00" + purpose), nil
}

// Put receives secrets only from trusted process configuration. No plaintext is
// passed to SQL, including when replacing an existing credential.
func (v *FiscalSecretVault) Put(ctx context.Context, tenant, purpose string, secret []byte) error {
	aad, err := vaultAAD(tenant, purpose)
	if err != nil || len(secret) == 0 || len(secret) > 4*fiscalcore.MaxXML {
		return ErrUnavailable
	}
	nonce := make([]byte, v.aead.NonceSize())
	if _, err = rand.Read(nonce); err != nil {
		return ErrUnavailable
	}
	encrypted := v.aead.Seal(nonce, nonce, secret, aad)
	_, err = v.db.ExecContext(ctx, `INSERT INTO fiscal_secret_vault (tenant,purpose,ciphertext) VALUES (?,?,?) ON CONFLICT(tenant,purpose) DO UPDATE SET ciphertext=excluded.ciphertext`, tenant, purpose, encrypted)
	if err != nil {
		return ErrUnavailable
	}
	return nil
}

// read is deliberately package-private; request input cannot select a tenant.
func (v *FiscalSecretVault) read(ctx context.Context, tenant, purpose string) ([]byte, error) {
	aad, err := vaultAAD(tenant, purpose)
	if err != nil {
		return nil, ErrUnavailable
	}
	var encrypted []byte
	if v.db.QueryRowContext(ctx, `SELECT ciphertext FROM fiscal_secret_vault WHERE tenant=? AND purpose=?`, tenant, purpose).Scan(&encrypted) != nil || len(encrypted) < v.aead.NonceSize()+v.aead.Overhead() || len(encrypted) > 4*fiscalcore.MaxXML+64 {
		return nil, ErrUnavailable
	}
	n := v.aead.NonceSize()
	raw, err := v.aead.Open(nil, encrypted[:n], encrypted[n:], aad)
	if err != nil {
		return nil, ErrUnavailable
	}
	return raw, nil
}

// The certificate record is JSON containing configuration (public trust and
// live CRL paths only), pkcs12 (base64 A1 bytes) and password. It is encrypted
// as a single record; no temporary plaintext files are created.
func (v *FiscalSecretVault) loadOwn(tenant string) (*Runtime, error) {
	raw, err := v.read(context.Background(), tenant, "certificate")
	if err != nil {
		return nil, ErrUnavailable
	}
	defer clear(raw)
	var bundle struct {
		Configuration FileConfiguration `json:"configuration"`
		PKCS12        []byte            `json:"pkcs12"`
		Password      []byte            `json:"password"`
	}
	defer func() { clear(bundle.PKCS12); clear(bundle.Password) }()
	if fiscalcore.DecodeInput(raw, &bundle) != nil {
		return nil, ErrUnavailable
	}
	cfg := bundle.Configuration
	if cfg.Version != 1 || cfg.Binding.TenantID != tenant || len(cfg.CRLPaths) == 0 || len(cfg.CRLPaths) > 16 || cfg.CertificatePath != "" || cfg.PrivateKeyPath != "" || cfg.PKCS12Path != "" || cfg.PasswordPath != "" || len(bundle.Password) == 0 || len(bundle.Password) > 1024 || !utf8.Valid(bundle.Password) || strings.ContainsRune(string(bundle.Password), 0) {
		return nil, ErrUnavailable
	}
	key, leaf, chain, err := pkcs12.DecodeChain(bundle.PKCS12, string(bundle.Password))
	if err != nil || leaf == nil || leaf.IsCA || len(chain) > 8 {
		return nil, ErrUnavailable
	}
	signer, ok := key.(*rsa.PrivateKey)
	if !ok {
		return nil, ErrUnavailable
	}
	rootsRaw, err := privateFile(cfg.IssuerRootsPath)
	if err != nil {
		return nil, ErrUnavailable
	}
	roots := x509.NewCertPool()
	if !roots.AppendCertsFromPEM(rootsRaw) {
		return nil, ErrUnavailable
	}
	intermediates := make([][]byte, 0, len(chain))
	for _, c := range chain {
		intermediates = append(intermediates, c.Raw)
	}
	credential, err := fiscalcore.NewCredentialWithChain(leaf.Raw, intermediates, signer, cfg.Binding.Issuer, roots, time.Now())
	if err != nil {
		return nil, ErrUnavailable
	}
	paths := append([]string(nil), cfg.CRLPaths...)
	loader := func() ([][]byte, error) {
		lists := make([][]byte, 0, len(paths))
		for _, path := range paths {
			data, e := privateFile(path)
			if e != nil {
				return nil, ErrUnavailable
			}
			if b, tail := pem.Decode(data); b != nil {
				if b.Type != "X509 CRL" || len(tail) != 0 {
					return nil, ErrUnavailable
				}
				data = b.Bytes
			}
			lists = append(lists, data)
		}
		return lists, nil
	}
	var qr *fiscalcore.QRConfiguration
	consultation := ""
	if cfg.QR != nil {
		if cfg.QR.CSCPath != "" {
			return nil, ErrUnavailable
		}
		qr = &fiscalcore.QRConfiguration{Version: cfg.QR.Version, BaseURL: cfg.QR.BaseURL, CSCID: cfg.QR.CSCID}
		consultation = cfg.QR.ConsultationURL
		if qr.Version == 2 {
			csc, e := v.read(context.Background(), tenant, "csc")
			if e != nil {
				return nil, ErrUnavailable
			}
			qr.CSC = string(csc)
			clear(csc)
		}
	}
	return Provision(cfg.Binding, credential, loader, qr, consultation, NetworkTrust{})
}

// MarshalJSON does not expose database handles or cryptographic state.
func (*FiscalSecretVault) MarshalJSON() ([]byte, error) { return json.Marshal(struct{}{}) }
