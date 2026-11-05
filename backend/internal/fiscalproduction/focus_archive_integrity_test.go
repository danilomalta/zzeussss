package fiscalproduction

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
	"titansystem-backend/internal/fiscalcore"

	"titansystem-backend/internal/localdb/backup"
	"titansystem-backend/internal/localdb/fiscal"
)

func TestFocusArchiveIgnoredWriteKeepsAuthorizedNoteRecoverable(t *testing.T) {
	data, _, key := archiveXMLFixture(t)
	a := focusContext(t)
	ctx := context.Background()
	p := faultFocus(t, func(w http.ResponseWriter, r *http.Request) { w.Write(data) })
	p.configuration.XMLArchiveKeyFile = filepath.Join(t.TempDir(), "archive.key")
	if e := backup.InitKey(p.configuration.XMLArchiveKeyFile); e != nil {
		t.Fatal(e)
	}
	if _, e := a.DB.Exec(`CREATE TRIGGER synthetic_ignore_archive BEFORE INSERT ON outbox WHEN NEW.event_type='checkout.focus_xml_archive' BEGIN SELECT RAISE(IGNORE); END`); e != nil {
		t.Fatal(e)
	}
	evidence, _ := json.Marshal(focusArchiveLinks{Invoice: "/arquivos/" + key[6:20] + "/XMLs/" + key + ".xml"})
	input := DocumentResult{State: "authorized_homologation", Key: key, Evidence: evidence}
	out, e := p.save(ctx, a, "sale-one", "intent-one", input, nil)
	if e != nil || out.State != input.State || out.Key != key || out.ArchiveState != "pending" {
		t.Errorf("archive claimed without ciphertext or note lost: state=%s archive=%s err=%v", out.State, out.ArchiveState, e)
	}
	var count int
	if e = a.DB.QueryRow(`SELECT count(*) FROM outbox WHERE event_type='checkout.fiscal_status'`).Scan(&count); e != nil || count != 1 {
		t.Fatal("authorized status lost", count, e)
	}
	if e = a.DB.QueryRow(`SELECT count(*) FROM outbox WHERE event_type='checkout.focus_xml_archive'`).Scan(&count); e != nil || count != 0 {
		t.Fatal("partial archive", count, e)
	}
	if _, e = a.DB.Exec(`DROP TRIGGER synthetic_ignore_archive`); e != nil {
		t.Fatal(e)
	}
	for i := 0; i < 2; i++ {
		out, e = p.save(ctx, a, "sale-one", "intent-one", input, nil)
		if e != nil || out.State != input.State || out.IntentID != "intent-one" || out.Key != key || out.ArchiveState != "archived" {
			t.Fatal("recovery changed original note", e)
		}
	}
	if e = a.DB.QueryRow(`SELECT count(*) FROM outbox WHERE event_type='checkout.focus_xml_archive'`).Scan(&count); e != nil || count != 1 {
		t.Fatal("archive duplicated", count, e)
	}
}

func TestFocusArchiveIgnoredDocumentOrEventRollsBack(t *testing.T) {
	invoice, cancellation, key := archiveXMLFixture(t)
	correction, correctionKey := correctionQueryXML(t, "Synthetic correction for archive rollback only")
	for _, item := range []struct {
		kind, key string
		number    int
		data      []byte
	}{{"invoice", key, 0, invoice}, {"cancellation", key, 1, cancellation}, {"correction", correctionKey, 1, correction}} {
		t.Run(item.kind, func(t *testing.T) {
			a := focusContext(t)
			ctx := context.Background()
			p := faultFocus(t, func(http.ResponseWriter, *http.Request) { t.Error("store attempted network") })
			p.configuration.XMLArchiveKeyFile = filepath.Join(t.TempDir(), "archive.key")
			if e := backup.InitKey(p.configuration.XMLArchiveKeyFile); e != nil {
				t.Fatal(e)
			}
			if _, e := a.DB.Exec(`CREATE TRIGGER synthetic_ignore_archive BEFORE INSERT ON outbox WHEN NEW.event_type='checkout.focus_xml_archive' BEGIN SELECT RAISE(IGNORE); END`); e != nil {
				t.Fatal(e)
			}
			if e := p.storeArchive(ctx, a, "sale", "intent", item.key, item.kind, item.number, item.data); !errors.Is(e, fiscal.ErrConflict) {
				t.Fatalf("required ciphertext write ignored: %v", e)
			}
			if _, e := a.DB.Exec(`DROP TRIGGER synthetic_ignore_archive`); e != nil {
				t.Fatal(e)
			}
			for i := 0; i < 2; i++ {
				if e := p.storeArchive(ctx, a, "sale", "intent", item.key, item.kind, item.number, item.data); e != nil {
					t.Fatal(e)
				}
			}
			var count int
			if e := a.DB.QueryRow(`SELECT count(*) FROM outbox WHERE event_type='checkout.focus_xml_archive'`).Scan(&count); e != nil || count != 1 {
				t.Fatal("duplicate event archive", count, e)
			}
		})
	}
}

func TestFocusArchiveConcurrentSameDocument(t *testing.T) {
	data, _, key := archiveXMLFixture(t)
	ctx := context.Background()
	a := focusContext(t)
	p := faultFocus(t, func(http.ResponseWriter, *http.Request) { t.Error("store attempted network") })
	p.configuration.XMLArchiveKeyFile = filepath.Join(t.TempDir(), "archive.key")
	if e := backup.InitKey(p.configuration.XMLArchiveKeyFile); e != nil {
		t.Fatal(e)
	}
	const n = 6
	errs := make(chan error, n)
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); errs <- p.storeArchive(ctx, a, "sale", "intent", key, "invoice", 0, data) }()
	}
	wg.Wait()
	close(errs)
	for e := range errs {
		if e != nil {
			t.Fatal("identical archive replay", e)
		}
	}
	var count int
	if e := a.DB.QueryRow(`SELECT count(*) FROM outbox WHERE event_type='checkout.focus_xml_archive'`).Scan(&count); e != nil || count != 1 {
		t.Fatal("duplicate archive", count, e)
	}
	var raw string
	if e := a.DB.QueryRow(`SELECT payload_json FROM outbox WHERE event_type='checkout.focus_xml_archive'`).Scan(&raw); e != nil {
		t.Fatal(e)
	}
	var record focusArchiveRecord
	if e := json.Unmarshal([]byte(raw), &record); e != nil {
		t.Fatal(e)
	}
	crypt, e := archiveCrypt(p.configuration.XMLArchiveKeyFile)
	if e != nil {
		t.Fatal(e)
	}
	size := crypt.NonceSize()
	got, e := crypt.Open(nil, record.Ciphertext[:size], record.Ciphertext[size:], archiveAAD(a, record))
	defer clear(got)
	if e != nil || string(got) != string(data) || record.Sale != "sale" || record.Intent != "intent" || record.Key != key {
		t.Fatal("mixed/corrupt archive", e)
	}
}

func TestFocusArchiveAndDANFERejectRedirectsAndHTML(t *testing.T) {
	key := syntheticFocusKey(t)
	// Downloaded model-55 paths are fixed by the existing access key validator.
	if key[20:22] != "55" {
		_, _, key = archiveXMLFixture(t)
	}
	for _, kind := range []string{"xml", "pdf"} {
		for _, mode := range []string{"redirect", "html"} {
			t.Run(kind+"/"+mode, func(t *testing.T) {
				calls := 0
				p := faultFocus(t, func(w http.ResponseWriter, r *http.Request) {
					calls++
					if mode == "redirect" {
						http.Redirect(w, r, "/unapproved-target", http.StatusFound)
						return
					}
					w.Header().Set("Content-Type", "text/html")
					w.Write([]byte("<html><script>synthetic_not_a_fiscal_document()</script></html>"))
				})
				resource := "/arquivos/" + key[6:20] + "/" + key + "."
				var data []byte
				var e error
				if kind == "xml" {
					data, e = p.downloadArchiveXML(context.Background(), resource+"xml", key, "invoice", 0)
				} else {
					data, e = p.downloadDANFE(context.Background(), resource+"pdf", key)
				}
				if e == nil || len(data) != 0 || calls != 1 {
					t.Fatal("redirect/HTML admitted", calls, e)
				}
			})
		}
	}
}

func TestFocusArchiveWrongKeyBlocksPrintAndExport(t *testing.T) {
	data, _, key := checkoutPrintXMLFixture(t)
	ctx := context.Background()
	a := focusContext(t)
	p := faultFocus(t, func(http.ResponseWriter, *http.Request) { t.Error("local print/export attempted network") })
	good, bad := filepath.Join(t.TempDir(), "good.key"), filepath.Join(t.TempDir(), "wrong.key")
	for _, file := range []string{good, bad} {
		if e := backup.InitKey(file); e != nil {
			t.Fatal(e)
		}
	}
	p.configuration.XMLArchiveKeyFile = good
	if e := p.storeArchive(ctx, a, "sale", "intent", key, "invoice", 0, data); e != nil {
		t.Fatal(e)
	}
	var raw string
	if e := a.DB.QueryRow(`SELECT payload_json FROM outbox WHERE event_type='checkout.focus_xml_archive'`).Scan(&raw); e != nil {
		t.Fatal(e)
	}
	var record focusArchiveRecord
	if e := json.Unmarshal([]byte(raw), &record); e != nil {
		t.Fatal(e)
	}
	opts := fiscalcore.PrintOptions{WidthMM: 80, Timezone: "America/Sao_Paulo"}
	if out, e := renderCheckoutArchive(a, record, bad, opts); e == nil || len(out.HTML) != 0 {
		t.Fatal("wrong key returned cupom", e)
	}
	cfg := p.configuration
	cfg.XMLArchiveKeyFile = bad
	registry := &ProviderRegistry{entries: map[string]ProviderRegistration{a.Actor.TenantID: {Configuration: cfg}}}
	from, until := time.Now().Add(-time.Hour).Format(time.RFC3339), time.Now().Add(time.Hour).Format(time.RFC3339)
	if zip, e := registry.AccountingExport(ctx, a, from, until, "pt-BR"); e == nil || len(zip) != 0 {
		t.Fatal("wrong key returned export", e)
	}
	registry.entries[a.Actor.TenantID] = ProviderRegistration{Configuration: p.configuration}
	if zip, e := registry.AccountingExport(ctx, a, from, until, "pt-BR"); e != nil || len(zip) == 0 {
		t.Fatal("correct key export failed", e)
	}
	out, e := renderCheckoutArchive(a, record, good, opts)
	if e != nil || out.Key != key || !strings.Contains(string(out.HTML), "SEM VALOR FISCAL") {
		t.Fatal("correct key print failed", e)
	}
}
