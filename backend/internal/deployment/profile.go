// Package deployment describes storage responsibility, not a data conversion.
package deployment

import (
	"bytes"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/netip"
	"os"
	"path/filepath"
	"strconv"
	"time"
)

var ErrProfile = errors.New("configuração de operação inválida")
var ErrUnavailable = errors.New("modo ainda indisponível neste núcleo: exige serviços online e reconciliação comercial")

type Profile struct {
	Version         int      `json:"version"`
	Mode            string   `json:"mode"`
	Listen          string   `json:"listen"`
	TLSCert         string   `json:"tls_cert"`
	TLSKey          string   `json:"tls_key"`
	AllowedNetworks []string `json:"allowed_networks"`
}

type Description struct {
	Mode                     string `json:"mode"`
	Available                bool   `json:"available"`
	Storage                  string `json:"storage"`
	Authority                string `json:"authority"`
	InternetRequired         bool   `json:"internet_required"`
	TerminalOffline          bool   `json:"terminal_offline"`
	CommercialReconciliation bool   `json:"commercial_reconciliation"`
}

func (p Profile) Describe() Description {
	d := Description{Mode: p.Mode}
	switch p.Mode {
	case "local":
		d.Available = true
		d.Storage = "sqlite"
		d.Authority = "station"
		d.TerminalOffline = true
	case "server":
		d.Available = true
		d.Storage = "sqlite"
		d.Authority = "store_server"
	case "cloud":
		d.Storage = "postgresql"
		d.Authority = "customer_cloud"
		d.InternetRequired = true
	case "hybrid":
		d.Storage = "sqlite_and_remote_service"
		d.Authority = "per_operation_pending"
	}
	return d
}

func Local(port int) Profile {
	return Profile{Version: 1, Mode: "local", Listen: net.JoinHostPort("127.0.0.1", strconv.Itoa(port)), AllowedNetworks: []string{}}
}

// PrivateRead rejects links in any path component and checks the opened inode.
// Returned errors never contain the file contents or secret-bearing paths.
func PrivateRead(path string, limit int64) ([]byte, error) {
	if !filepath.IsAbs(path) {
		return nil, ErrProfile
	}
	for part := filepath.Clean(path); ; part = filepath.Dir(part) {
		info, err := os.Lstat(part)
		if err != nil || info.Mode()&os.ModeSymlink != 0 {
			return nil, ErrProfile
		}
		if part == filepath.Dir(part) {
			break
		}
	}
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || info.Mode().Perm() != 0600 || info.Size() > limit {
		return nil, ErrProfile
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, ErrProfile
	}
	defer f.Close()
	opened, err := f.Stat()
	if err != nil || !os.SameFile(info, opened) {
		return nil, ErrProfile
	}
	body, err := io.ReadAll(io.LimitReader(f, limit+1))
	if err != nil || int64(len(body)) > limit {
		return nil, ErrProfile
	}
	return body, nil
}

func Load(path string) (Profile, error) {
	body, err := PrivateRead(path, 16*1024)
	if err != nil {
		return Profile{}, err
	}
	// All six fields are required. Duplicate, unknown, null and trailing values
	// are rejected before unmarshalling can silently choose a later field.
	decoder := json.NewDecoder(bytes.NewReader(body))
	opening, err := decoder.Token()
	if err != nil || opening != json.Delim('{') {
		return Profile{}, ErrProfile
	}
	expected := map[string]bool{"version": true, "mode": true, "listen": true, "tls_cert": true, "tls_key": true, "allowed_networks": true}
	seen := map[string]bool{}
	for decoder.More() {
		key, err := decoder.Token()
		name, ok := key.(string)
		if err != nil || !ok || !expected[name] || seen[name] {
			return Profile{}, ErrProfile
		}
		seen[name] = true
		var raw json.RawMessage
		if decoder.Decode(&raw) != nil || bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
			return Profile{}, ErrProfile
		}
	}
	closing, err := decoder.Token()
	if err != nil || closing != json.Delim('}') || len(seen) != 6 {
		return Profile{}, ErrProfile
	}
	if _, err = decoder.Token(); err != io.EOF {
		return Profile{}, ErrProfile
	}
	var p Profile
	if json.Unmarshal(body, &p) != nil || p.Validate() != nil {
		return Profile{}, ErrProfile
	}
	return p, nil
}

func (p Profile) Validate() error {
	if p.Version != 1 {
		return ErrProfile
	}
	switch p.Mode {
	case "cloud", "hybrid":
		// Descriptions are valid, but these modes must not launch a local server or
		// silently select local storage instead. Configuration is not migration.
		if p.Listen != "" || p.TLSCert != "" || p.TLSKey != "" || len(p.AllowedNetworks) != 0 {
			return ErrProfile
		}
		return nil
	case "local", "server":
	default:
		return ErrProfile
	}
	host, port, err := net.SplitHostPort(p.Listen)
	ip := net.ParseIP(host)
	number, e := strconv.Atoi(port)
	if err != nil || ip == nil || ip.IsUnspecified() || e != nil || number < 1 || number > 65535 {
		return ErrProfile
	}
	if p.Mode == "local" {
		if !ip.IsLoopback() || p.TLSCert != "" || p.TLSKey != "" || len(p.AllowedNetworks) != 0 {
			return ErrProfile
		}
	} else {
		if !ip.IsLoopback() && !ip.IsPrivate() {
			return ErrProfile
		}
		if !filepath.IsAbs(p.TLSCert) || !filepath.IsAbs(p.TLSKey) || p.TLSCert == p.TLSKey || len(p.AllowedNetworks) < 1 || len(p.AllowedNetworks) > 16 {
			return ErrProfile
		}
		seen := map[string]bool{}
		for _, value := range p.AllowedNetworks {
			prefix, err := privatePrefix(value)
			if err != nil || seen[prefix.String()] {
				return ErrProfile
			}
			seen[prefix.String()] = true
		}
	}
	return nil
}

func privatePrefix(value string) (netip.Prefix, error) {
	prefix, err := netip.ParsePrefix(value)
	if err != nil || prefix != prefix.Masked() {
		return netip.Prefix{}, ErrProfile
	}
	for _, raw := range []string{"10.0.0.0/8", "172.16.0.0/12", "192.168.0.0/16", "127.0.0.0/8", "fc00::/7", "::1/128"} {
		parent := netip.MustParsePrefix(raw)
		if prefix.Addr().BitLen() == parent.Addr().BitLen() && prefix.Bits() >= parent.Bits() && parent.Contains(prefix.Addr()) {
			return prefix, nil
		}
	}
	return netip.Prefix{}, ErrProfile
}

func (p Profile) Allows(remote string) bool {
	ip, err := netip.ParseAddr(remote)
	if err != nil {
		return false
	}
	ip = ip.Unmap()
	if p.Mode == "local" {
		return ip.IsLoopback()
	}
	if p.Mode != "server" {
		return false
	}
	for _, value := range p.AllowedNetworks {
		prefix, err := privatePrefix(value)
		if err == nil && prefix.Contains(ip) {
			return true
		}
	}
	return false
}

func (p Profile) TLS(now time.Time) (*tls.Config, error) {
	if err := p.Validate(); err != nil {
		return nil, err
	}
	if !p.Describe().Available {
		return nil, ErrUnavailable
	}
	if p.Mode == "local" {
		return nil, nil
	}
	certificatePEM, err := PrivateRead(p.TLSCert, 1<<20)
	if err != nil {
		return nil, ErrProfile
	}
	keyPEM, err := PrivateRead(p.TLSKey, 1<<20)
	if err != nil {
		return nil, ErrProfile
	}
	defer clear(keyPEM)
	certificate, err := tls.X509KeyPair(certificatePEM, keyPEM)
	if err != nil || len(certificate.Certificate) == 0 {
		return nil, ErrProfile
	}
	leaf, err := x509.ParseCertificate(certificate.Certificate[0])
	host, _, _ := net.SplitHostPort(p.Listen)
	if err != nil || now.Before(leaf.NotBefore) || !now.Before(leaf.NotAfter) || leaf.VerifyHostname(host) != nil {
		return nil, ErrProfile
	}
	if len(leaf.ExtKeyUsage) > 0 {
		allowed := false
		for _, usage := range leaf.ExtKeyUsage {
			if usage == x509.ExtKeyUsageServerAuth || usage == x509.ExtKeyUsageAny {
				allowed = true
			}
		}
		if !allowed {
			return nil, ErrProfile
		}
	}
	return &tls.Config{MinVersion: tls.VersionTLS12, Certificates: []tls.Certificate{certificate}}, nil
}
