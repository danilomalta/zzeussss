package fiscalproduction

import (
	"context"
	"encoding/json"
	"net/http"
	"path/filepath"
	"testing"
	"time"

	"titansystem-backend/internal/localdb/backup"
)

// Local synthetic XML and disposable HTTP server; not Focus/SEFAZ acceptance.
func TestFocusArchiveReplayUnknownVersionStaysPending(t *testing.T) {
	data, _, key := checkoutPrintXMLFixture(t)
	a := focusContext(t)
	ctx := context.Background()
	p := faultFocus(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Error("archive recovery attempted a fiscal mutation")
		}
		w.Write(data)
	})
	p.configuration.XMLArchiveKeyFile = filepath.Join(t.TempDir(), "disposable-archive.key")
	if err := backup.InitKey(p.configuration.XMLArchiveKeyFile); err != nil {
		t.Fatal(err)
	}
	links, _ := json.Marshal(focusArchiveLinks{Invoice: "/arquivos/" + key[6:20] + "/XMLs/" + key + ".xml"})
	in := DocumentResult{State: "authorized_homologation", Key: key, Evidence: links}
	first, err := p.save(ctx, a, "sale-one", "intent-one", in, nil)
	if err != nil || first.ArchiveState != "archived" {
		t.Fatal("valid initial archive", err)
	}
	var eventID, original string
	if err := a.DB.QueryRow(`SELECT event_id,payload_json FROM outbox WHERE event_type='checkout.focus_xml_archive'`).Scan(&eventID, &original); err != nil {
		t.Fatal(err)
	}
	var record focusArchiveRecord
	if err := json.Unmarshal([]byte(original), &record); err != nil {
		t.Fatal(err)
	}
	registry := &ProviderRegistry{entries: map[string]ProviderRegistration{a.Actor.TenantID: {Configuration: p.configuration}}}
	from, until := time.Now().Add(-time.Hour).Format(time.RFC3339), time.Now().Add(time.Hour).Format(time.RFC3339)
	if zip, err := registry.AccountingExport(ctx, a, from, until, "pt-BR"); err != nil || len(zip) == 0 {
		t.Fatal("valid archive not exportable", err)
	}
	// Simulate unsupported metadata in THIS disposable database, without
	// modifying ciphertext, XML, signing, keys or any original test.
	record.Version = 2
	unsupported, _ := json.Marshal(record)
	if _, err := a.DB.Exec(`UPDATE outbox SET payload_json=? WHERE event_id=?`, string(unsupported), eventID); err != nil {
		t.Fatal(err)
	}
	if zip, err := registry.AccountingExport(ctx, a, from, until, "pt-BR"); err == nil || len(zip) != 0 {
		t.Fatal("unsupported version exported")
	}
	replay, err := p.save(ctx, a, "sale-one", "intent-one", in, nil)
	if err != nil || replay.ArchiveState != "pending" {
		t.Errorf("unreadable existing archive reported as %s, err=%v", replay.ArchiveState, err)
	}
	if replay.State != first.State || replay.Key != first.Key || replay.IntentID != first.IntentID {
		t.Fatal("archive failure lost original authorized note")
	}
	var current string
	var count int
	if err := a.DB.QueryRow(`SELECT payload_json FROM outbox WHERE event_id=?`, eventID).Scan(&current); err != nil || current != string(unsupported) {
		t.Fatal("incompatible evidence silently replaced", err)
	}
	if err := a.DB.QueryRow(`SELECT count(*) FROM outbox WHERE event_type='checkout.focus_xml_archive'`).Scan(&count); err != nil || count != 1 {
		t.Fatal("archive duplicated", err)
	}
	if _, err := a.DB.Exec(`UPDATE outbox SET payload_json=? WHERE event_id=?`, original, eventID); err != nil {
		t.Fatal(err)
	}
	recovered, err := p.save(ctx, a, "sale-one", "intent-one", in, nil)
	if err != nil || recovered.ArchiveState != "archived" || recovered.Key != first.Key || recovered.IntentID != first.IntentID {
		t.Fatal("original evidence could not be recovered", err)
	}
}
