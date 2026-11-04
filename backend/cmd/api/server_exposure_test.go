package main

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"io"
	"math/big"
	"net"
	"net/http"
	"net/netip"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"titansystem-backend/internal/core/routes"
	"titansystem-backend/internal/deployment"
)

func exposureEnv(values map[string]string) func(string) string {
	return func(k string) string { return values[k] }
}

func exposureProfileFile(t *testing.T, p deployment.Profile) string {
	t.Helper()
	raw, err := json.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}
	name := filepath.Join(t.TempDir(), "profile.json")
	if err = os.WriteFile(name, raw, 0600); err != nil {
		t.Fatal(err)
	}
	return name
}

func exposureTLSFixture(t *testing.T, address, san string, from, until time.Time) (deployment.Profile, *x509.CertPool) {
	t.Helper()
	public, key, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	cert := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "Disposable online TLS"},
		NotBefore: from, NotAfter: until, IPAddresses: []net.IP{net.ParseIP(san)},
		KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}}
	der, err := x509.CreateCertificate(rand.Reader, cert, cert, public, key)
	if err != nil {
		t.Fatal(err)
	}
	private, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	defer clear(private)
	dir := t.TempDir()
	p := deployment.Profile{Version: 1, Mode: "server", Listen: address,
		TLSCert: filepath.Join(dir, "cert.pem"), TLSKey: filepath.Join(dir, "key.pem"), AllowedNetworks: []string{"127.0.0.1/32", "192.168.1.0/24"}}
	for name, block := range map[string]*pem.Block{p.TLSCert: {Type: "CERTIFICATE", Bytes: der}, p.TLSKey: {Type: "PRIVATE KEY", Bytes: private}} {
		if err = os.WriteFile(name, pem.EncodeToMemory(block), 0600); err != nil {
			t.Fatal(err)
		}
	}
	roots := x509.NewCertPool()
	parsed, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatal(err)
	}
	roots.AddCert(parsed)
	return p, roots
}

func TestOnlineExposureLoopbackDefaultAndUnsafeBindingsFailClosed(t *testing.T) {
	p, err := readOnlineExposure(exposureEnv(nil), time.Now())
	if err != nil || p.profile.Listen != "127.0.0.1:8080" || p.tls != nil || p.proxies != nil || p.allowsPeer("192.168.1.5") {
		t.Fatal("default escaped loopback")
	}
	for _, input := range []map[string]string{
		{"TITAN_API_HOST": "192.168.1.5"}, {"TITAN_API_HOST": "203.0.113.10"}, {"TITAN_API_HOST": "0.0.0.0"},
		{"TITAN_API_HOST": "::"}, {"TITAN_API_HOST": "localhost"}, {"TITAN_API_HOST": "127.0.0.1:8080"},
		{"TITAN_API_HOST": " 127.0.0.1"}, {"PORT": "+8080"}, {"TITAN_API_DEPLOYMENT": "/not-present/private-value-marker"},
		{"TITAN_API_REVERSE_PROXY_TLS": "true"}, {"TITAN_API_REVERSE_PROXY_TLS": "TRUE"},
		{"TITAN_API_TRUSTED_PROXY_IPS": "127.0.0.1"},
	} {
		_, err := readOnlineExposure(exposureEnv(input), time.Now())
		if err != errOnlineExposure || strings.Contains(err.Error(), "private-value-marker") {
			t.Fatal("unsafe input accepted or leaked")
		}
	}
	p, err = readOnlineExposure(exposureEnv(map[string]string{"TITAN_API_HOST": "::1"}), time.Now())
	if err != nil || p.profile.Listen != "[::1]:8080" || !p.allowsPeer("::1") {
		t.Fatal("IPv6 loopback refused")
	}
}

func TestOnlineExposureReusesTLSValidityAndNetworkPolicy(t *testing.T) {
	now := time.Now()
	for _, tc := range []struct {
		name, san   string
		from, until time.Time
		valid       bool
	}{
		{"valid", "192.168.1.5", now.Add(-time.Hour), now.Add(time.Hour), true},
		{"expired", "192.168.1.5", now.Add(-time.Hour), now.Add(-time.Minute), false},
		{"future", "192.168.1.5", now.Add(time.Minute), now.Add(time.Hour), false},
		{"wrong-san", "192.168.1.6", now.Add(-time.Hour), now.Add(time.Hour), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			profile, _ := exposureTLSFixture(t, "192.168.1.5:8443", tc.san, tc.from, tc.until)
			cfg, err := readOnlineExposure(exposureEnv(map[string]string{"TITAN_API_DEPLOYMENT": exposureProfileFile(t, profile), "ALLOWED_ORIGINS": "https://erp.example.invalid,titan://app"}), now)
			if (err == nil) != tc.valid {
				t.Fatal("wrong TLS admission result")
			}
			if tc.valid && (cfg.tls == nil || cfg.tls.MinVersion != 0x0303 || !cfg.allowsPeer("192.168.1.20") || cfg.allowsPeer("192.168.2.20")) {
				t.Fatal("TLS or network scope changed")
			}
		})
	}
	profile, _ := exposureTLSFixture(t, "192.168.1.5:8443", "192.168.1.5", now.Add(-time.Hour), now.Add(time.Hour))
	file := exposureProfileFile(t, profile)
	for _, extra := range []map[string]string{
		{"TITAN_API_HOST": "192.168.1.6"}, {"PORT": "8444"}, {"COOKIE_SECURE": "false"},
		{"TITAN_API_REVERSE_PROXY_TLS": "true", "TITAN_API_TRUSTED_PROXY_IPS": "127.0.0.1"},
		{"ALLOWED_ORIGINS": ""}, {"ALLOWED_ORIGINS": "*"}, {"ALLOWED_ORIGINS": "http://erp.example.invalid"},
	} {
		values := map[string]string{"TITAN_API_DEPLOYMENT": file, "ALLOWED_ORIGINS": "https://erp.example.invalid"}
		for k, v := range extra {
			values[k] = v
		}
		if _, err := readOnlineExposure(exposureEnv(values), now); err != errOnlineExposure {
			t.Fatal("contradictory or unsafe configuration accepted")
		}
	}
	if err := os.Chmod(profile.TLSKey, 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := readOnlineExposure(exposureEnv(map[string]string{"TITAN_API_DEPLOYMENT": file, "ALLOWED_ORIGINS": "https://erp.example.invalid"}), now); err != errOnlineExposure {
		t.Fatal("public private-key file accepted")
	}
}

func TestOnlineProxyIsExplicitLoopbackOnlyAndNeverTrustsForwardedIP(t *testing.T) {
	values := map[string]string{"TITAN_API_REVERSE_PROXY_TLS": "true", "TITAN_API_TRUSTED_PROXY_IPS": "127.0.0.1,::1", "ALLOWED_ORIGINS": "https://erp.example.invalid"}
	p, err := readOnlineExposure(exposureEnv(values), time.Now())
	if err != nil || !p.allowsPeer("127.0.0.1") || !p.allowsPeer("::1") || p.allowsPeer("127.0.0.2") || p.allowsPeer("192.168.1.1") {
		t.Fatal("proxy trust escaped declared peers")
	}
	for _, raw := range []string{"", "192.168.1.1", "0.0.0.0/0", "127.0.0.1/32", "localhost", "127.0.0.1,127.0.0.1", "127.0.0.1, ::1"} {
		input := map[string]string{"TITAN_API_REVERSE_PROXY_TLS": "true", "TITAN_API_TRUSTED_PROXY_IPS": raw, "ALLOWED_ORIGINS": "https://erp.example.invalid"}
		if _, err = readOnlineExposure(exposureEnv(input), time.Now()); err != errOnlineExposure {
			t.Fatal("invalid proxy trust accepted")
		}
	}
	app := fiber.New(onlineServerConfig())
	app.Use(p.gate)
	app.Get("/peer", func(c *fiber.Ctx) error { return c.SendString(c.IP()) })
	base := serveOnlineFixture(t, app)
	client := &http.Client{Timeout: 2 * time.Second, Transport: &http.Transport{DisableKeepAlives: true}}
	defer client.CloseIdleConnections()
	req, _ := http.NewRequest("GET", base+"/peer", nil)
	req.Header.Set("Forwarded", "for=203.0.113.55;proto=https")
	req.Header.Set("X-Forwarded-For", "203.0.113.55")
	req.Header.Set("X-Real-IP", "203.0.113.55")
	resp, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 || string(body) != "127.0.0.1" {
		t.Fatal("forwarded headers changed peer identity")
	}
	blocked := p
	blocked.proxies = map[netip.Addr]bool{netip.MustParseAddr("::1"): true}
	denied := fiber.New(onlineServerConfig())
	denied.Use(blocked.gate)
	denied.Get("/peer", func(c *fiber.Ctx) error { t.Error("untrusted peer reached handler"); return c.SendStatus(200) })
	forged, _ := http.NewRequest("GET", serveOnlineFixture(t, denied)+"/peer", nil)
	forged.Header.Set("X-Forwarded-For", "::1")
	forged.Header.Set("X-Real-IP", "::1")
	refused, err := client.Do(forged)
	if err != nil {
		t.Fatal(err)
	}
	refused.Body.Close()
	if refused.StatusCode != 403 || refused.Header.Get("Cache-Control") != "no-store" {
		t.Fatal("headers granted access to untrusted peer")
	}
}

func TestOnlineTLSListenerUsesCustomerCertificateAndKeepsCORS(t *testing.T) {
	// Reserve only a disposable loopback port, never an installed service port.
	reserved, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	address := reserved.Addr().String()
	reserved.Close()
	now := time.Now()
	profile, roots := exposureTLSFixture(t, address, "127.0.0.1", now.Add(-time.Hour), now.Add(time.Hour))
	const origin = "https://erp.example.invalid"
	t.Setenv("ALLOWED_ORIGINS", origin)
	p, err := readOnlineExposure(exposureEnv(map[string]string{"TITAN_API_DEPLOYMENT": exposureProfileFile(t, profile), "ALLOWED_ORIGINS": origin}), now)
	if err != nil {
		t.Fatal(err)
	}
	ln, err := p.listen()
	if err != nil {
		t.Fatal(err)
	}
	app := fiber.New(onlineServerConfig())
	app.Use(p.gate)
	routes.Registrar(app)
	done := make(chan error, 1)
	ctx, cancel := context.WithCancel(context.Background())
	go func() { done <- serveOnline(ctx, app, ln, 2*time.Second, func() error { return nil }) }()
	t.Cleanup(func() {
		cancel()
		select {
		case err := <-done:
			if err != nil {
				t.Error("TLS server shutdown failed")
			}
		case <-time.After(3 * time.Second):
			t.Error("TLS server did not stop")
		}
	})
	client := &http.Client{Timeout: 2 * time.Second, Transport: &http.Transport{TLSClientConfig: &tls.Config{RootCAs: roots, MinVersion: tls.VersionTLS12}, DisableKeepAlives: true}}
	defer client.CloseIdleConnections()
	req, _ := http.NewRequest("GET", "https://"+address+"/api/v1/saude", nil)
	req.Header.Set("Origin", origin)
	resp, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 || string(body) != `{"status":"ativo"}` || resp.TLS == nil || resp.Header.Get("Access-Control-Allow-Origin") != origin {
		t.Fatal("HTTPS listener/CORS did not preserve contract")
	}
}

func TestOnlineOriginsAreExplicitAndNeverContainCredentials(t *testing.T) {
	for _, raw := range []string{"", "*", "null", "https://*.invalid", "https://user:private-value-marker@erp.invalid", "https://erp.invalid/path", "https://erp.invalid?key=private-value-marker", "https://erp.invalid#x", "https://erp.invalid,https://erp.invalid", "http://erp.invalid", "https://erp.invalid:99999"} {
		if onlineSecureOrigins(raw) {
			t.Fatal("unsafe origin accepted")
		}
	}
	if !onlineSecureOrigins("https://erp.example.invalid:8443,titan://app") {
		t.Fatal("explicit secure origins refused")
	}
}
