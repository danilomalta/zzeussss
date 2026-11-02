package localapi

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"titansystem-backend/internal/core/entitlements"
	"titansystem-backend/internal/core/modules"
	"titansystem-backend/internal/fiscalcore"
	"titansystem-backend/internal/localdb"
	"titansystem-backend/internal/localdb/backup"
	"titansystem-backend/internal/localdb/fiscal"
)

func danfeSQLFixture(t *testing.T) (*stockHTTPFixture, fiscal.Intent, fiscalcore.RecordedRequest) {
	return danfeSQLFixtureModel(t, "65")
}

func danfeSQLFixtureModel(t *testing.T, model string) (*stockHTTPFixture, fiscal.Intent, fiscalcore.RecordedRequest) {
	t.Helper()
	f, intent, _, original := queryOriginalFixtureModel(t, model == "65", model)
	r := querySQLRecorder(t, f, f.db, intent, "danfe-authorized", f.token)
	if err := r.Reserve(context.Background(), querySQLRequest(t, r, "danfe-authorized")); err != nil {
		t.Fatal(err)
	}
	if err := r.Complete(context.Background(), querySQLResult(t, original, "danfe-authorized", "100")); err != nil {
		t.Fatal(err)
	}
	return f, intent, original
}

func TestHTTPFiscalDANFERestartRestoreAndExpiredContractKeepExactCopy(t *testing.T) {
	for _, model := range []string{"55", "65"} {
		t.Run(model, func(t *testing.T) { testDANFERestartRestore(t, model) })
	}
}

func testDANFERestartRestore(t *testing.T, model string) {
	f, intent, _ := danfeSQLFixtureModel(t, model)
	pathToPrint := danfePath(intent.ID)
	if model == "55" {
		pathToPrint = strings.Replace(pathToPrint, "width_mm=80", "width_mm=210", 1)
	}
	code, before := request(t, f.app, "GET", pathToPrint, "", f.token)
	if code != 200 {
		t.Fatal(code)
	}
	f.install(t, signedTestContract(t, f.private, f.owner.TenantID, 4, []modules.ID{modules.Core, modules.Inventory, modules.POS, modules.Staff}), 200)
	if code, body := request(t, f.app, "GET", pathToPrint, "", f.token); code != 200 || !bytes.Equal(body, before) {
		t.Fatal("contract loss removed historical copy", code)
	}
	var seq int
	var name, path string
	if err := f.db.QueryRow(`PRAGMA database_list`).Scan(&seq, &name, &path); err != nil {
		t.Fatal(err)
	}
	if err := f.db.Close(); err != nil {
		t.Fatal(err)
	}
	db, err := localdb.Open(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	f.db = db
	verifier, err := entitlements.NewVerifier(map[string]ed25519.PublicKey{"test-issuer": f.private.Public().(ed25519.PublicKey)})
	if err != nil {
		t.Fatal(err)
	}
	f.app, err = NewWithVerifier(db, f.device, verifier)
	if err != nil {
		t.Fatal(err)
	}
	if code, body := request(t, f.app, "GET", pathToPrint, "", f.token); code != 200 || !bytes.Equal(body, before) {
		t.Fatal("restart changed copy", code)
	}
	var destination string
	if *pilotBackupCLI != "" {
		station := pilotPrepareCLIStation(t, f)
		destination = pilotCLIBackupRoundTrip(t, f, path, station)
	} else {
		key := make([]byte, 32)
		if _, err = rand.Read(key); err != nil {
			t.Fatal(err)
		}
		defer clear(key)
		dir := t.TempDir()
		archive := filepath.Join(dir, "danfe.tytbak")
		destination = filepath.Join(dir, "restored.sqlite")
		if err = backup.Create(context.Background(), db, f.device, key, archive); err != nil {
			t.Fatal(err)
		}
		if err = backup.Restore(context.Background(), archive, destination, f.device, key); err != nil {
			t.Fatal(err)
		}
	}
	restored, err := localdb.Open(context.Background(), destination)
	if err != nil {
		t.Fatal(err)
	}
	defer restored.Close()
	app, err := NewWithVerifier(restored, f.device, verifier)
	if err != nil {
		t.Fatal(err)
	}
	if code, _ := request(t, app, "GET", pathToPrint, "", f.token); code != 401 {
		t.Fatal("restored old session printed", code)
	}
	token := loginToken(t, app, f.owner.OwnerID)
	if code, body := request(t, app, "GET", pathToPrint, "", token); code != 200 || !bytes.Equal(body, before) {
		t.Fatal("restore changed copy", code)
	}
	var barriers int
	if err = restored.QueryRow(`SELECT count(*) FROM fiscal_restore_barriers`).Scan(&barriers); err != nil || barriers != 1 {
		t.Fatal("print removed restore barrier", err, barriers)
	}
}
func danfePath(id string) string {
	return "/local/v1/fiscal/intents/" + id + "/danfe?width_mm=80&timezone=America%2FSao_Paulo"
}

func TestHTTPFiscalDANFERealHTTPExactArtifactAndReadonly(t *testing.T) {
	for _, model := range []string{"55", "65"} {
		t.Run(model, func(t *testing.T) { testFiscalDANFERealHTTPExactArtifactAndReadonly(t, model) })
	}
}

func testFiscalDANFERealHTTPExactArtifactAndReadonly(t *testing.T, model string) {
	danfePath := func(id string) string { return danfeModelPath(id, model) }
	f, intent, _ := danfeSQLFixtureModel(t, model)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- f.app.Listener(listener) }()
	t.Cleanup(func() {
		_ = f.app.Shutdown()
		_ = listener.Close()
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			t.Error("HTTP server did not stop")
		}
	})
	client := &http.Client{Transport: &http.Transport{Proxy: nil}, Timeout: 5 * time.Second}
	defer client.CloseIdleConnections()
	get := func(token string) (int, http.Header, []byte) {
		t.Helper()
		req, err := http.NewRequest("GET", "http://"+listener.Addr().String()+danfePath(intent.ID), nil)
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Authorization", "Bearer "+token)
		res, err := client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		body, err := io.ReadAll(res.Body)
		if err != nil {
			t.Fatal(err)
		}
		return res.StatusCode, res.Header, body
	}
	before := cancellationBusinessDigest(t, f.db)
	code, headers, body := get(f.token)
	if code != 200 {
		t.Fatalf("print %d %s", code, body)
	}
	if !strings.HasPrefix(headers.Get("Content-Type"), "text/html") || headers.Get("Cache-Control") != "no-store" || headers.Get("X-Titan-Fiscal-Scope") != "homologation" || headers.Get("X-Content-Type-Options") != "nosniff" || !strings.Contains(headers.Get("Content-Security-Policy"), "default-src 'none'") {
		t.Fatal("unsafe response headers", headers)
	}
	sum := sha256.Sum256(body)
	if headers.Get("X-Titan-Artifact-SHA256") != hex.EncodeToString(sum[:]) {
		t.Fatal("artifact hash")
	}
	if !bytes.Contains(body, []byte("31,98")) || !bytes.Contains(body, []byte("SEM VALOR FISCAL")) {
		t.Fatal("wrong archived amount or environment")
	}
	code, _, again := get(f.token)
	if code != 200 || !bytes.Equal(body, again) {
		t.Fatal("reprint changed archive", code)
	}
	if before != cancellationBusinessDigest(t, f.db) {
		t.Fatal("print mutated business state")
	}
	if code, _ := request(t, f.app, "POST", "/local/v1/logout", "", f.token); code != 204 {
		t.Fatal(code)
	}
	if code, _, _ := get(f.token); code != 401 {
		t.Fatal("revoked session printed", code)
	}
	newToken := loginToken(t, f.app, f.owner.OwnerID)
	code, _, again = get(newToken)
	if code != 200 || !bytes.Equal(body, again) {
		t.Fatal("new session lost archived copy", code)
	}
}

func TestHTTPFiscalDANFERejectsAmbiguousRequestsAndOtherSellers(t *testing.T) {
	for _, model := range []string{"55", "65"} {
		t.Run(model, func(t *testing.T) { testFiscalDANFERejectsAmbiguousRequestsAndOtherSellers(t, model) })
	}
}

func testFiscalDANFERejectsAmbiguousRequestsAndOtherSellers(t *testing.T, model string) {
	danfePath := func(id string) string { return danfeModelPath(id, model) }
	f, intent, _ := danfeSQLFixtureModel(t, model)
	for _, query := range []string{"", "?width_mm=80", "?width_mm=57&timezone=America/Sao_Paulo", "?width_mm=80&width_mm=58&timezone=America/Sao_Paulo", "?width_mm=80&timezone=UTC", "?width_mm=80&timezone=America/Missing", "?width_mm=80&timezone=America/Sao_Paulo&xml=anything"} {
		t.Run(query, func(t *testing.T) {
			code, _ := request(t, f.app, "GET", "/local/v1/fiscal/intents/"+intent.ID+"/danfe"+query, "", f.token)
			if code != 400 {
				t.Fatal("ambiguous print", code)
			}
		})
	}
	if code, _ := request(t, f.app, "GET", danfePath(intent.ID), "{}", f.token); code != 400 {
		t.Fatal("GET accepted body", code)
	}
	if code, _ := request(t, f.app, "GET", danfePath("missing"), "", f.token); code != 404 {
		t.Fatal("missing record", code)
	}
	for _, role := range []string{"cashier", "accountant"} {
		code, b := request(t, f.app, "POST", "/local/v1/staff", `{"operation_id":"danfe-`+role+`","identity_id":"danfe-`+role+`","name":"Test staff","role":"`+role+`","password":"test-password-strong-1"}`, f.token)
		if code != 201 {
			t.Fatalf("staff %d %s", code, b)
		}
		code, b = request(t, f.app, "POST", "/local/v1/login", `{"identity_id":"danfe-`+role+`","password":"test-password-strong-1"}`, "")
		if code != 200 {
			t.Fatal(code)
		}
		var session struct {
			Token string `json:"token"`
		}
		if err := json.Unmarshal(b, &session); err != nil {
			t.Fatal(err)
		}
		code, _ = request(t, f.app, "GET", danfePath(intent.ID), "", session.Token)
		want := 200
		if role == "cashier" {
			want = 404
		}
		if code != want {
			t.Fatal("read scope", role, code, want)
		}
	}
}

func TestHTTPFiscalDANFENewPendingStatusNeverFallsBack(t *testing.T) {
	for _, model := range []string{"55", "65"} {
		t.Run(model, func(t *testing.T) { testFiscalDANFENewPendingStatusNeverFallsBack(t, model) })
	}
}

func testFiscalDANFENewPendingStatusNeverFallsBack(t *testing.T, model string) {
	danfePath := func(id string) string { return danfeModelPath(id, model) }
	f, intent, original := danfeSQLFixtureModel(t, model)
	if code, _ := request(t, f.app, "GET", danfePath(intent.ID), "", f.token); code != 200 {
		t.Fatal(code)
	}
	r := querySQLRecorder(t, f, f.db, intent, "newer-pending", f.token)
	if err := r.Reserve(context.Background(), querySQLRequest(t, r, "newer-pending")); err != nil {
		t.Fatal(err)
	}
	if code, _ := request(t, f.app, "GET", danfePath(intent.ID), "", f.token); code != 409 {
		t.Fatal("used older authorized result", code)
	}
	if err := r.Complete(context.Background(), querySQLResult(t, original, "newer-pending", "217")); err != nil {
		t.Fatal(err)
	}
	if code, _ := request(t, f.app, "GET", danfePath(intent.ID), "", f.token); code != 409 {
		t.Fatal("not-found became authorization", code)
	}
}

func TestHTTPFiscalDANFEDirectIssuanceWithoutQuery(t *testing.T) {
	for _, model := range []string{"55", "65"} {
		t.Run(model, func(t *testing.T) { testFiscalDANFEDirectIssuanceWithoutQuery(t, model) })
	}
}

func testFiscalDANFEDirectIssuanceWithoutQuery(t *testing.T, model string) {
	danfePath := func(id string) string { return danfeModelPath(id, model) }
	f, intent, _, _ := queryOriginalFixtureModel(t, model == "65", model, true)
	if code, body := request(t, f.app, "GET", danfePath(intent.ID), "", f.token); code != 200 || !bytes.Contains(body, []byte("31,98")) {
		t.Fatal("direct archive", code)
	}
	var count int
	if err := f.db.QueryRow(`SELECT count(*) FROM fiscal_query_requests`).Scan(&count); err != nil || count != 0 {
		t.Fatal("print sent a consultation", err, count)
	}
	for _, dimension := range []string{"tenant", "store", "actor", "device"} {
		a, d := numberingActor(f), f.device
		switch dimension {
		case "tenant":
			a.TenantID = "foreign"
			d.TenantID = "foreign"
		case "store":
			a.StoreID = "foreign"
			d.StoreID = "foreign"
		case "actor":
			a.IdentityID = "foreign"
		case "device":
			d.DeviceID = "foreign"
		}
		if _, err := fiscal.ReadHomologationDANFE(context.Background(), f.db, a, d, intent.ID, f.token, fiscalcore.PrintOptions{WidthMM: map[string]int{"55": 210, "65": 80}[model], Timezone: "America/Sao_Paulo"}); err == nil {
			t.Fatal("wrong context printed", dimension)
		}
	}
}

func TestHTTPFiscalDANFEPendingCancellationBlocksActiveCopy(t *testing.T) {
	for _, model := range []string{"55", "65"} {
		t.Run(model, func(t *testing.T) { testFiscalDANFEPendingCancellationBlocksActiveCopy(t, model) })
	}
}

func testFiscalDANFEPendingCancellationBlocksActiveCopy(t *testing.T, model string) {
	danfePath := func(id string) string { return danfeModelPath(id, model) }
	f, intent, original := danfeSQLFixtureModel(t, model)
	query := querySQLRecorder(t, f, f.db, intent, "query-authorized", f.token)
	if err := query.Reserve(context.Background(), querySQLRequest(t, query, "query-authorized")); err != nil {
		t.Fatal(err)
	}
	if err := query.Complete(context.Background(), querySQLResult(t, original, "query-authorized", "100")); err != nil {
		t.Fatal(err)
	}
	r := cancellationSQLRecorder(t, f, f.db, intent, "cancel-print", f.token)
	in := fiscalcore.Cancellation{Key: original.Key, CNPJ: original.Issuer, Environment: 2, Protocol: "135260000000001", Time: time.Now().Add(time.Second), Reason: "Synthetic cancellation decision only"}
	if err := r.Reserve(context.Background(), cancellationSQLRequest(t, r, f, "cancel-print", in)); err != nil {
		t.Fatal(err)
	}
	if code, _ := request(t, f.app, "GET", danfePath(intent.ID), "", f.token); code != 409 {
		t.Fatal("pending cancellation printed active copy", code)
	}
}

func danfeModelPath(id, model string) string {
	path := danfePath(id)
	if model == "55" {
		path = strings.Replace(path, "width_mm=80", "width_mm=210", 1)
	}
	return path
}
