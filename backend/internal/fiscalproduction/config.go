package fiscalproduction

import (
	"crypto/x509"
	"encoding/pem"
	"io"
	"os"
	"path/filepath"
	"time"

	"titansystem-backend/internal/fiscalcore"
)

type FileConfiguration struct {
	Version         int                          `json:"version"`
	Binding         fiscalcore.ProductionBinding `json:"binding"`
	CertificatePath string                       `json:"certificate_path"`
	PrivateKeyPath  string                       `json:"private_key_path"`
	PKCS12Path      string                       `json:"pkcs12_path,omitempty"`
	PasswordPath    string                       `json:"pkcs12_password_path,omitempty"`
	IssuerRootsPath string                       `json:"issuer_roots_path"`
	CRLPaths        []string                     `json:"crl_paths"`
	QR              *QRFileConfiguration         `json:"qr,omitempty"`
}
type QRFileConfiguration struct {
	Version         int    `json:"version"`
	BaseURL         string `json:"base_url"`
	CSCID           string `json:"csc_id"`
	CSCPath         string `json:"csc_path"`
	ConsultationURL string `json:"consultation_url"`
}

func privateFile(path string) ([]byte, error) {
	if !filepath.IsAbs(path) {
		return nil, ErrUnavailable
	}
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || info.Mode().Perm() != 0600 || info.Size() > 4*fiscalcore.MaxXML {
		return nil, ErrUnavailable
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, ErrUnavailable
	}
	defer f.Close()
	actual, err := f.Stat()
	if err != nil || !os.SameFile(info, actual) {
		return nil, ErrUnavailable
	}
	raw, err := io.ReadAll(io.LimitReader(f, 4*fiscalcore.MaxXML+1))
	if err != nil || len(raw) > 4*fiscalcore.MaxXML {
		return nil, ErrUnavailable
	}
	return raw, nil
}

// Load never emits file contents or OS errors containing private paths. PEM
// or password-protected PKCS12 signing material is read from private0600 files.
// No browser upload/import, inline password or trust from the PFX is accepted.
func Load(path string) (*Runtime, error) {
	raw, err := privateFile(path)
	if err != nil {
		return nil, ErrUnavailable
	}
	defer clear(raw)
	var cfg FileConfiguration
	if fiscalcore.DecodeInput(raw, &cfg) != nil || cfg.Version != 1 || len(cfg.CRLPaths) == 0 || len(cfg.CRLPaths) > 16 {
		return nil, ErrUnavailable
	}
	chain, signer, err := loadSigningMaterial(cfg)
	if err != nil {
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
	credential, err := fiscalcore.NewCredentialWithChain(chain[0], chain[1:], signer, cfg.Binding.Issuer, roots, time.Now())
	if err != nil {
		return nil, ErrUnavailable
	}
	paths := append([]string(nil), cfg.CRLPaths...)
	loader := func() ([][]byte, error) {
		lists := make([][]byte, 0, len(paths))
		for _, path := range paths {
			raw, e := privateFile(path)
			if e != nil {
				return nil, ErrUnavailable
			}
			if b, tail := pem.Decode(raw); b != nil {
				if b.Type != "X509 CRL" || len(tail) != 0 {
					return nil, ErrUnavailable
				}
				raw = b.Bytes
			}
			lists = append(lists, raw)
		}
		return lists, nil
	}
	var qr *fiscalcore.QRConfiguration
	consultation := ""
	if cfg.QR != nil {
		qr = &fiscalcore.QRConfiguration{Version: cfg.QR.Version, BaseURL: cfg.QR.BaseURL, CSCID: cfg.QR.CSCID}
		consultation = cfg.QR.ConsultationURL
		if cfg.QR.Version == 2 {
			csc, e := privateFile(cfg.QR.CSCPath)
			if e != nil {
				return nil, ErrUnavailable
			}
			qr.CSC = string(csc)
			clear(csc)
		} else if cfg.QR.CSCPath != "" || cfg.QR.CSCID != "" {
			return nil, ErrUnavailable
		}
	}
	return Provision(cfg.Binding, credential, loader, qr, consultation, NetworkTrust{})
}
