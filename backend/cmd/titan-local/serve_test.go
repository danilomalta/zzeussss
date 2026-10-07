package main

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"github.com/gofiber/fiber/v2"
	"io"
	"math/big"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"titansystem-backend/internal/deployment"
	"titansystem-backend/internal/localapi"
)

func tlsFixture(t *testing.T) (deployment.Profile, *tls.Config) {
	t.Helper()
	dir := t.TempDir()
	pub, key, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	cert := &x509.Certificate{SerialNumber: big.NewInt(1), NotBefore: time.Now().Add(-time.Minute), NotAfter: time.Now().Add(time.Hour), IPAddresses: []net.IP{net.ParseIP("127.0.0.1")}, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}}
	der, err := x509.CreateCertificate(rand.Reader, cert, cert, pub, key)
	if err != nil {
		t.Fatal(err)
	}
	private, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	profile := deployment.Profile{Version: 1, Mode: "server", Listen: "127.0.0.1:8443", TLSCert: filepath.Join(dir, "cert.pem"), TLSKey: filepath.Join(dir, "key.pem"), AllowedNetworks: []string{"127.0.0.0/8"}}
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	if os.WriteFile(profile.TLSCert, certPEM, 0600) != nil || os.WriteFile(profile.TLSKey, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: private}), 0600) != nil {
		t.Fatal("files")
	}
	roots := x509.NewCertPool()
	if !roots.AppendCertsFromPEM(certPEM) {
		t.Fatal("root")
	}
	return profile, &tls.Config{RootCAs: roots, MinVersion: tls.VersionTLS12}
}

func TestServeRejectsUnavailableModesAndUnsafeProfileBeforeOpeningDB(t *testing.T) {
	for _, mode := range []string{"cloud", "hybrid"} {
		dir := t.TempDir()
		path := filepath.Join(dir, "mode.json")
		body, _ := json.Marshal(deployment.Profile{Version: 1, Mode: mode, AllowedNetworks: []string{}})
		if os.WriteFile(path, body, 0600) != nil {
			t.Fatal("write")
		}
		db := filepath.Join(dir, "absent.sqlite")
		args := []string{"--db", db, "--station", filepath.Join(dir, "missing.json"), "--deployment", path}
		if err := serveStation(args); err != deployment.ErrUnavailable {
			t.Fatalf("mode %s: %v", mode, err)
		}
		if _, err := os.Stat(db); !os.IsNotExist(err) {
			t.Fatal("created database")
		}
		var out bytes.Buffer
		if err := describeMode([]string{"--deployment", path}, &out); err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(out.String(), `"available":false`) || strings.Contains(out.String(), path) {
			t.Fatal("false availability or path leaked")
		}
		if serveStation(append(args, "--port", "8181")) == nil {
			t.Fatal("ambiguous binding")
		}
	}
	p, _ := tlsFixture(t)
	p.TLSKey = filepath.Join(t.TempDir(), "missing")
	path := filepath.Join(t.TempDir(), "profile.json")
	body, _ := json.Marshal(p)
	os.WriteFile(path, body, 0600)
	db := filepath.Join(t.TempDir(), "absent.sqlite")
	if serveStation([]string{"--db", db, "--station", "missing", "--deployment", path}) == nil {
		t.Fatal("missing TLS key")
	}
	if _, err := os.Stat(db); !os.IsNotExist(err) {
		t.Fatal("created database")
	}
}

func TestServerHTTPSLoginReadRevocationAndNetworkAdmission(t *testing.T) {
	dir := t.TempDir()
	path, station := filepath.Join(dir, "store.sqlite"), filepath.Join(dir, "station.json")
	password := "senha-de-teste-servidor-2026"
	var initOutput bytes.Buffer
	if err := initStation([]string{"--db", path, "--station", station, "--empresa", "Teste", "--loja", "Teste", "--dono", "Teste"}, strings.NewReader(password), &initOutput); err != nil {
		t.Fatal(err)
	}
	db, device, err := openVerified(t.Context(), path, station)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var owner string
	if err = db.QueryRow("SELECT identity_id FROM memberships WHERE role='owner' AND tenant_id=?", device.TenantID).Scan(&owner); err != nil {
		t.Fatal(err)
	}
	profile, clientTLS := tlsFixture(t)
	config, err := profile.TLS(time.Now())
	if err != nil {
		t.Fatal(err)
	}
	app, err := localapi.NewWithVerifierAndGate(db, device, nil, networkGate(profile))
	if err != nil {
		t.Fatal(err)
	}
	raw, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	listener := tls.NewListener(raw, config)
	ctx, cancel := context.WithCancel(context.Background())
	finished := make(chan error, 1)
	go func() { finished <- runLocalListener(ctx, app, listener) }()
	defer func() {
		cancel()
		select {
		case err := <-finished:
			if err != nil {
				t.Error(err)
			}
		case <-time.After(8 * time.Second):
			t.Error("shutdown blocked")
		}
	}()
	transport := &http.Transport{TLSClientConfig: clientTLS}
	defer transport.CloseIdleConnections()
	client := &http.Client{Transport: transport, Timeout: 5 * time.Second}
	base := "https://" + raw.Addr().String()
	request := func(method, url, body, token string) *http.Response {
		t.Helper()
		req, err := http.NewRequest(method, base+url, strings.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Content-Type", "application/json")
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		response, err := client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		return response
	}
	health := request("GET", "/local/v1/health", "", "")
	if health.StatusCode != 200 || health.Header.Get("Cache-Control") != "no-store" {
		t.Fatal("health")
	}
	health.Body.Close()
	anonymous := request("GET", "/local/v1/me", "", "")
	if anonymous.StatusCode != 401 {
		t.Fatal("anonymous")
	}
	anonymous.Body.Close()
	loginBody, _ := json.Marshal(map[string]string{"identity_id": owner, "password": password})
	response := request("POST", "/local/v1/login", string(loginBody), "")
	if response.StatusCode != 200 {
		t.Fatal("login", response.StatusCode)
	}
	var session struct {
		Token string `json:"token"`
	}
	if json.NewDecoder(response.Body).Decode(&session) != nil || session.Token == "" {
		t.Fatal("session")
	}
	response.Body.Close()
	me := request("GET", "/local/v1/me", "", session.Token)
	if me.StatusCode != 200 {
		t.Fatal("me")
	}
	var scope map[string]any
	json.NewDecoder(me.Body).Decode(&scope)
	me.Body.Close()
	if scope["tenant_id"] != device.TenantID || scope["device_id"] != device.DeviceID {
		t.Fatal("foreign context")
	}
	if _, err = db.Exec("UPDATE memberships SET status='revoked' WHERE identity_id=?", owner); err != nil {
		t.Fatal(err)
	}
	revoked := request("GET", "/local/v1/me", "", session.Token)
	if revoked.StatusCode != 401 {
		t.Fatal("revoked")
	}
	revoked.Body.Close()
	// HTTP client with no configured test CA must reject this certificate.
	untrusted := &http.Client{Timeout: time.Second}
	if r, err := untrusted.Get(base + "/local/v1/health"); err == nil {
		r.Body.Close()
		t.Fatal("untrusted certificate accepted")
	}
	plain := &http.Client{Timeout: time.Second}
	if r, err := plain.Get("http://" + raw.Addr().String() + "/local/v1/health"); err == nil {
		r.Body.Close()
		if r.StatusCode == 200 {
			t.Fatal("served plaintext on TLS listener")
		}
	}
	// Real peer is 127.0.0.1. A claimed trusted LAN address cannot bypass gate.
	blockedProfile := profile
	blockedProfile.AllowedNetworks = []string{"10.0.0.0/8"}
	blockedApp, err := localapi.NewWithVerifierAndGate(db, device, nil, networkGate(blockedProfile))
	if err != nil {
		t.Fatal(err)
	}
	blockedRaw, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	blockedCtx, stop := context.WithCancel(context.Background())
	blockedDone := make(chan error, 1)
	go func() { blockedDone <- runLocalListener(blockedCtx, blockedApp, tls.NewListener(blockedRaw, config)) }()
	req, _ := http.NewRequest("GET", "https://"+blockedRaw.Addr().String()+"/local/v1/health", nil)
	req.Header.Set("X-Forwarded-For", "10.1.1.1")
	req.Header.Set("Forwarded", "for=10.1.1.1")
	blocked, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	if blocked.StatusCode != 403 {
		t.Fatal("header bypassed peer IP")
	}
	blocked.Body.Close()
	stop()
	select {
	case err := <-blockedDone:
		if err != nil {
			t.Error(err)
		}
	case <-time.After(8 * time.Second):
		t.Fatal("blocked shutdown")
	}
}

func TestServeCommandStartsConfiguredTLSAndStops(t *testing.T) {
	profile, clientTLS := tlsFixture(t)
	reservation, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := reservation.Addr().(*net.TCPAddr).Port
	reservation.Close()
	profile.Listen = net.JoinHostPort("127.0.0.1", strconv.Itoa(port))
	dir := t.TempDir()
	db, station := filepath.Join(dir, "store.sqlite"), filepath.Join(dir, "station.json")
	if err = initStation([]string{"--db", db, "--station", station, "--empresa", "Teste", "--loja", "Teste", "--dono", "Teste"}, strings.NewReader("senha-descartavel-2026"), io.Discard); err != nil {
		t.Fatal(err)
	}
	profilePath := filepath.Join(dir, "profile.json")
	body, _ := json.Marshal(profile)
	if os.WriteFile(profilePath, body, 0600) != nil {
		t.Fatal("profile")
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- serveStationContext(ctx, []string{"--db", db, "--station", station, "--deployment", profilePath}, io.Discard)
	}()
	transport := &http.Transport{TLSClientConfig: clientTLS}
	defer transport.CloseIdleConnections()
	client := &http.Client{Transport: transport, Timeout: time.Second}
	defer cancel()
	deadline := time.Now().Add(5 * time.Second)
	for {
		response, err := client.Get("https://" + profile.Listen + "/local/v1/health")
		if err == nil {
			if response.StatusCode != 200 {
				t.Fatal(response.StatusCode)
			}
			response.Body.Close()
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("configured server not ready")
		}
		time.Sleep(20 * time.Millisecond)
	}
	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(8 * time.Second):
		t.Fatal("shutdown")
	}
}

func TestCancellationDrainsAlreadyActiveRequest(t *testing.T) {
	app := fiber.New(fiber.Config{DisableStartupMessage: true})
	entered, release := make(chan struct{}), make(chan struct{})
	var once sync.Once
	defer once.Do(func() { close(release) })
	app.Get("/held", func(c *fiber.Ctx) error { close(entered); <-release; return c.SendStatus(200) })
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- runLocalListener(ctx, app, listener) }()
	requestDone := make(chan struct{})
	go func() {
		defer close(requestDone)
		client := &http.Client{Timeout: 3 * time.Second}
		r, err := client.Get("http://" + listener.Addr().String() + "/held")
		if err == nil {
			r.Body.Close()
		}
	}()
	select {
	case <-entered:
	case <-time.After(3 * time.Second):
		t.Fatal("handler not started")
	}
	cancel()
	select {
	case err := <-done:
		t.Fatalf("closed without draining active request: %v", err)
	case <-time.After(100 * time.Millisecond):
	}
	once.Do(func() { close(release) })
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("shutdown blocked")
	}
	<-requestDone
}
