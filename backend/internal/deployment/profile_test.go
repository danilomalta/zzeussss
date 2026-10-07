package deployment

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func writeProfile(t *testing.T, body []byte) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "profile.json")
	if err := os.WriteFile(path, body, 0600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestModesAndOfflineResponsibilities(t *testing.T) {
	for _, mode := range []string{"local", "server", "cloud", "hybrid"} {
		p := Profile{Version: 1, Mode: mode, AllowedNetworks: []string{}}
		if mode == "local" {
			p = Local(8181)
		}
		if mode == "server" {
			p.Listen = "192.168.1.5:8443"
			p.TLSCert = "/private/cert.pem"
			p.TLSKey = "/private/key.pem"
			p.AllowedNetworks = []string{"192.168.1.0/24"}
		}
		body, _ := json.Marshal(p)
		loaded, err := Load(writeProfile(t, body))
		if err != nil {
			t.Fatal(mode, err)
		}
		d := loaded.Describe()
		if d.Available != (mode == "local" || mode == "server") || d.TerminalOffline != (mode == "local") || d.CommercialReconciliation {
			t.Fatalf("incorrect capability: %+v", d)
		}
		if mode == "cloud" || mode == "hybrid" {
			if _, err := p.TLS(time.Now()); err != ErrUnavailable {
				t.Fatal("silently fell back to local")
			}
		}
	}
}

func TestProfilesRejectAmbiguityModesUnsafeNetworkAndFiles(t *testing.T) {
	valid := `{"version":1,"mode":"local","listen":"127.0.0.1:8181","tls_cert":"","tls_key":"","allowed_networks":[]}`
	for _, body := range []string{
		strings.Replace(valid, `"mode":"local"`, `"mode":"local","mode":"server"`, 1),
		strings.Replace(valid, `"mode":"local"`, `"mode":"anything"`, 1),
		strings.Replace(valid, `"allowed_networks":[]`, `"allowed_networks":null`, 1),
		strings.Replace(valid, `"tls_key":""`, `"unknown":""`, 1),
		valid + ` {}`, strings.Replace(valid, `127.0.0.1:8181`, `0.0.0.0:8181`, 1),
		strings.Replace(valid, `127.0.0.1:8181`, `192.168.1.1:8181`, 1),
		strings.Replace(valid, `127.0.0.1:8181`, `localhost:8181`, 1),
	} {
		if _, err := Load(writeProfile(t, []byte(body))); err == nil {
			t.Fatal("accepted ambiguous/unsafe profile")
		}
	}
	p := Profile{Version: 1, Mode: "server", Listen: "192.168.1.5:8443", TLSCert: "/cert", TLSKey: "/key"}
	for _, network := range []string{"0.0.0.0/0", "10.0.0.0/1", "8.8.8.0/24", "192.168.1.5/24", "192.168.1.0/24,10.0.0.0/8", "::/0"} {
		p.AllowedNetworks = []string{network}
		if p.Validate() == nil {
			t.Fatal("unsafe network accepted", network)
		}
	}
	p.AllowedNetworks = []string{"192.168.1.0/24", "192.168.1.0/24"}
	if p.Validate() == nil {
		t.Fatal("duplicate prefix")
	}
	path := writeProfile(t, []byte(valid))
	if os.Chmod(path, 0644) != nil {
		t.Fatal("chmod")
	}
	if _, err := Load(path); err == nil {
		t.Fatal("public config")
	}
	link := filepath.Join(t.TempDir(), "link")
	if os.Symlink(filepath.Dir(path), link) != nil {
		t.Fatal("symlink")
	}
	if _, err := PrivateRead(filepath.Join(link, "profile.json"), 16384); err == nil {
		t.Fatal("parent symlink accepted")
	}
}

func TestNetworkAdmissionIncludesIPv6AndDoesNotInferPublicTrust(t *testing.T) {
	p := Profile{Mode: "server", AllowedNetworks: []string{"192.168.1.0/24", "fd00:1234::/48"}}
	for _, ip := range []string{"192.168.1.4", "::ffff:192.168.1.4", "fd00:1234::8"} {
		if !p.Allows(ip) {
			t.Fatal("rejected", ip)
		}
	}
	for _, ip := range []string{"192.168.2.4", "8.8.8.8", "127.0.0.1", "invalid", "fd00:9999::1"} {
		if p.Allows(ip) {
			t.Fatal("allowed", ip)
		}
	}
	if !Local(8181).Allows("127.0.0.1") || Local(8181).Allows("192.168.1.1") {
		t.Fatal("local escaped loopback")
	}
}

func TestTLSRejectsExpiredWrongSANKeyAndPermissions(t *testing.T) {
	dir := t.TempDir()
	pub, key, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	keyDER, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	certPath, keyPath := filepath.Join(dir, "cert.pem"), filepath.Join(dir, "key.pem")
	if os.WriteFile(keyPath, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER}), 0600) != nil {
		t.Fatal("write")
	}
	p := Profile{Version: 1, Mode: "server", Listen: "127.0.0.1:8443", TLSCert: certPath, TLSKey: keyPath, AllowedNetworks: []string{"127.0.0.0/8"}}
	now := time.Now()
	for _, test := range []struct {
		name   string
		ip     net.IP
		expiry time.Time
		valid  bool
	}{
		{"valid", net.ParseIP("127.0.0.1"), now.Add(time.Hour), true},
		{"expired", net.ParseIP("127.0.0.1"), now.Add(-time.Minute), false},
		{"wrong-san", net.ParseIP("192.168.1.1"), now.Add(time.Hour), false},
	} {
		t.Run(test.name, func(t *testing.T) {
			template := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "Disposable test"}, NotBefore: now.Add(-time.Hour), NotAfter: test.expiry, IPAddresses: []net.IP{test.ip}, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}}
			der, err := x509.CreateCertificate(rand.Reader, template, template, pub, key)
			if err != nil {
				t.Fatal(err)
			}
			if os.WriteFile(certPath, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), 0600) != nil {
				t.Fatal("write")
			}
			config, err := p.TLS(now)
			if (err == nil) != test.valid {
				t.Fatal("TLS validity wrong")
			}
			if test.valid && config.MinVersion != 0x0303 {
				t.Fatal("TLS minimum")
			}
		})
	}
	if os.Chmod(keyPath, 0644) != nil {
		t.Fatal("chmod")
	}
	if _, err := p.TLS(now); err == nil {
		t.Fatal("public key file accepted")
	}
	if os.Chmod(keyPath, 0600) != nil {
		t.Fatal("chmod")
	}
	if os.WriteFile(keyPath, []byte("not-a-private-key"), 0600) != nil {
		t.Fatal("write")
	}
	if _, err := p.TLS(now); err == nil {
		t.Fatal("invalid key accepted")
	}
}
