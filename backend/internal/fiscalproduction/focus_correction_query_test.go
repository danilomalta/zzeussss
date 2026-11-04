package fiscalproduction

import (
	"bytes"
	"context"
	"encoding/json"
	"encoding/xml"
	"log"
	"net/http"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"titansystem-backend/internal/localdb/backup"
	"titansystem-backend/internal/localdb/fiscal"
)

// Schema-only HTTP fault fixture, NOT a Focus response or XML signature proof.
// The existing archive validator is reused; no signing engine is implemented.
func correctionQueryXML(t *testing.T, text string) ([]byte, string) {
	t.Helper()
	_, event, key := archiveXMLFixture(t)
	s := strings.ReplaceAll(string(event), "110111", "110110")
	start, end := strings.Index(s, "<detEvento"), strings.Index(s, "</detEvento>")
	if start < 0 || end < start {
		t.Fatal("event fixture")
	}
	var escaped bytes.Buffer
	if err := xml.EscapeText(&escaped, []byte(text)); err != nil {
		t.Fatal(err)
	}
	detail := `<detEvento versao="1.00"><descEvento>Carta de Correcao</descEvento><xCorrecao>` + escaped.String() + `</xCorrecao><xCondUso>Synthetic fixture only</xCondUso></detEvento>`
	raw := []byte(s[:start] + detail + s[end+len("</detEvento>"):])
	if err := validateFocusArchiveXML(raw, key, "correction", 1); err != nil {
		t.Fatal("correction schema fixture", err)
	}
	return raw, key
}

func TestFocusCorrectionQueryXMLBindingAndPrivateArchive(t *testing.T) {
	const text = "Synthetic private correction sentinel"
	var logs bytes.Buffer
	oldLog := log.Writer()
	log.SetOutput(&logs)
	defer log.SetOutput(oldLog)
	xmlData, key := correctionQueryXML(t, text)
	for _, scenario := range []string{"match", "different-text", "missing-path", "later-sequence", "wrong-environment", "download-failure", "missing-key", "logout"} {
		t.Run(scenario, func(t *testing.T) {
			a := focusContext(t)
			ctx := context.Background()
			posts, gets := 0, 0
			p := faultFocus(t, func(w http.ResponseWriter, r *http.Request) {
				if r.Method != "GET" {
					posts++
					t.Error("recovery attempted write")
					return
				}
				gets++
				if scenario == "download-failure" {
					w.WriteHeader(503)
					return
				}
				data := xmlData
				if scenario == "wrong-environment" {
					data = bytes.ReplaceAll(data, []byte("<tpAmb>2</tpAmb>"), []byte("<tpAmb>1</tpAmb>"))
				}
				_, _ = w.Write(data)
			})
			p.configuration.DocumentModel = "55"
			p.configuration.Binding = ProviderBinding{TenantID: a.Actor.TenantID, StoreID: a.Actor.StoreID, DeviceID: a.Device.DeviceID}
			p.configuration.XMLArchiveKeyFile = filepath.Join(t.TempDir(), "customer-backup.key")
			if scenario != "missing-key" {
				if err := backup.InitKey(p.configuration.XMLArchiveKeyFile); err != nil {
					t.Fatal(err)
				}
			}
			ref := focusReference(a.Actor.TenantID, "sale-one")
			original := text
			if scenario == "different-text" {
				original = "A different private correction sentinel"
			}
			body, _ := json.Marshal(map[string]string{"correcao": original})
			if won, err := p.claim(ctx, a, "sale-one", ref+"-cce", "checkout.focus_correction", body); err != nil || !won {
				t.Fatal("claim", err)
			}
			hash, err := correctionClaimHash(ctx, a, "sale-one", ref)
			if err != nil {
				t.Fatal(err)
			}
			links := focusArchiveLinks{Correction: "/arquivos/" + key[6:20] + "/202611/XMLs/correction.xml", CorrectionNumber: 1}
			if scenario == "missing-path" {
				links.Correction = ""
			}
			if scenario == "later-sequence" {
				links.CorrectionNumber = 2
			}
			evidence, _ := json.Marshal(links)
			current := DocumentResult{State: "authorized_homologation", Key: key, IntentID: "original-intent", Evidence: evidence}
			if scenario == "logout" {
				if _, err := a.DB.Exec(`UPDATE local_sessions SET revoked_unix=1`); err != nil {
					t.Fatal(err)
				}
			}
			out, err := p.reconcileCorrection(ctx, a, "sale-one", ref, hash, CorrectionResult{State: "correction_reconciliation_required"}, current)
			if scenario == "logout" {
				if err == nil {
					t.Fatal("revoked session published reconciliation")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			want := "correction_reconciliation_required"
			if scenario == "match" || scenario == "missing-key" {
				want = "correction_authorized_homologation"
			}
			if out.State != want || posts != 0 {
				t.Fatal("reconciliation", out.State, posts)
			}
			if scenario == "match" && out.ArchiveState != "archived" {
				t.Fatal("archive missing")
			}
			if scenario == "missing-key" && out.ArchiveState != "pending" {
				t.Fatal("archive failure hidden")
			}
			if (scenario == "missing-path" || scenario == "later-sequence") && gets != 0 {
				t.Fatal("unreconciled sequence fetched")
			}
			rows, err := a.DB.Query(`SELECT payload_json FROM outbox`)
			if err != nil {
				t.Fatal(err)
			}
			for rows.Next() {
				var raw string
				if err := rows.Scan(&raw); err != nil {
					t.Fatal(err)
				}
				if strings.Contains(raw, text) || strings.Contains(raw, original) || strings.Contains(raw, "SyntheticFocusToken123") || strings.Contains(raw, "<procEventoNFe") {
					t.Fatal("plaintext persisted")
				}
			}
			if err := rows.Close(); err != nil {
				t.Fatal(err)
			}
			if want == "correction_authorized_homologation" {
				recorded, err := readCorrectionResult(ctx, a, "sale-one", ref)
				if err != nil || recorded.State != want || recorded.Number != 1 {
					t.Fatal("durable result", err)
				}
				if scenario == "missing-key" {
					if err := backup.InitKey(p.configuration.XMLArchiveKeyFile); err != nil {
						t.Fatal(err)
					}
					out, err = p.reconcileCorrection(ctx, a, "sale-one", ref, hash, recorded, current)
					if err != nil || out.ArchiveState != "archived" {
						t.Fatal("archive-only retry", err)
					}
					recorded, err = readCorrectionResult(ctx, a, "sale-one", ref)
					if err != nil || recorded.ArchiveState != "archived" {
						t.Fatal("stale pending record won", err)
					}
				}
			}
		})
	}

	if strings.Contains(logs.String(), text) || strings.Contains(logs.String(), "<procEventoNFe") || strings.Contains(logs.String(), "SyntheticFocusToken123") {
		t.Fatal("private data logged")
	}
}

func TestFocusCorrectionQueryConcurrentJournalAndScope(t *testing.T) {
	a := focusContext(t)
	ctx := context.Background()
	p := faultFocus(t, func(http.ResponseWriter, *http.Request) { t.Error("journal contacted HTTP") })
	ref := focusReference(a.Actor.TenantID, "sale")
	if _, err := p.claim(ctx, a, "sale", ref+"-cce", "checkout.focus_correction", []byte(`{"correcao":"Synthetic journal fixture only"}`)); err != nil {
		t.Fatal(err)
	}
	hash, err := correctionClaimHash(ctx, a, "sale", ref)
	if err != nil {
		t.Fatal(err)
	}
	out := CorrectionResult{State: "correction_authorized_homologation", Number: 1, ArchiveState: "archived"}
	var wg sync.WaitGroup
	results := make(chan error, 6)
	for i := 0; i < 6; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); results <- p.recordCorrectionReconciliation(ctx, a, "sale", ref, hash, out) }()
	}
	wg.Wait()
	close(results)
	for err := range results {
		if err != nil {
			t.Fatal("non-idempotent result", err)
		}
	}
	var count int
	if err := a.DB.QueryRow(`SELECT count(*) FROM outbox WHERE event_type='checkout.focus_correction_result'`).Scan(&count); err != nil || count != 1 {
		t.Fatal("duplicate results", count, err)
	}
	for _, field := range []string{"tenant", "store", "device"} {
		other := a
		switch field {
		case "tenant":
			other.Actor.TenantID = "foreign"
		case "store":
			other.Actor.StoreID = "foreign"
		case "device":
			other.Device.DeviceID = "foreign"
		}
		if _, err := correctionClaimHash(ctx, other, "sale", ref); err == nil {
			t.Fatal("foreign claim visible", field)
		}
		if err := p.recordCorrectionReconciliation(ctx, other, "sale", ref, hash, out); err == nil {
			t.Fatal("foreign reconciliation recorded", field)
		}
	}
}

func TestFocusCorrectionQueryRequiresLocalAttemptAndScope(t *testing.T) {
	ctx := context.Background()
	a := focusContext(t)
	profile, err := fiscal.Configure(ctx, a.DB, a.License, a.Actor, a.Device, fiscal.ProfileInput{OperationID: "query-profile", ProfileID: "query-profile", CNPJ: "11222333000181", LegalName: "Synthetic issuer", State: "SP", Municipality: "3550308", StateRegistration: "123456789", CRT: "1", Model: "55", Environment: "homologation", Series: 1}, a.Token)
	if err != nil {
		t.Fatal(err)
	}
	p := faultFocus(t, func(http.ResponseWriter, *http.Request) { t.Error("query without a local attempt contacted provider") })
	p.configuration.DocumentModel = "55"
	p.configuration.CRT = "1"
	p.configuration.Binding = ProviderBinding{TenantID: a.Actor.TenantID, StoreID: a.Actor.StoreID, DeviceID: a.Device.DeviceID, ProfileID: profile.Input.ProfileID, ProfileRevision: 1}
	in := DocumentRequest{SaleID: "not-attempted", Model: "55", Environment: 2}
	out, err := p.QueryCorrection(ctx, a, in)
	if err != nil || out.State != "correction_not_requested" {
		t.Fatal("unrequested", out.State, err)
	}
	foreign := a
	foreign.Actor.TenantID = "other-tenant"
	if _, err = p.QueryCorrection(ctx, foreign, in); err == nil {
		t.Fatal("foreign scope accepted")
	}
	p.configuration.Environment = "production"
	if _, err = p.QueryCorrection(ctx, a, in); err == nil {
		t.Fatal("production accepted")
	}
	p.configuration.Environment = "homologation"
	p.configuration.CertificateType = "A3"
	if _, err = p.QueryCorrection(ctx, a, in); err == nil {
		t.Fatal("A3 accepted")
	}
}

func TestFocusCorrectionQueryJournalRollbackPreservesOriginal(t *testing.T) {
	a := focusContext(t)
	ctx := context.Background()
	p := faultFocus(t, func(http.ResponseWriter, *http.Request) { t.Error("journal contacted provider") })
	ref := focusReference(a.Actor.TenantID, "sale")
	if _, err := p.claim(ctx, a, "sale", ref+"-cce", "checkout.focus_correction", []byte(`{"correcao":"Synthetic rollback fixture only"}`)); err != nil {
		t.Fatal(err)
	}
	hash, err := correctionClaimHash(ctx, a, "sale", ref)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = a.DB.Exec(`CREATE TRIGGER reject_cce_query BEFORE INSERT ON outbox WHEN NEW.operation_id LIKE '%-cce-query%' BEGIN SELECT RAISE(ABORT,'synthetic journal failure'); END`); err != nil {
		t.Fatal(err)
	}
	err = p.recordCorrectionReconciliation(ctx, a, "sale", ref, hash, CorrectionResult{State: "correction_authorized_homologation", Number: 1, ArchiveState: "pending"})
	if err == nil {
		t.Fatal("journal failure reported success")
	}
	var original, results int
	if err = a.DB.QueryRow(`SELECT count(*) FROM outbox WHERE event_type='checkout.focus_correction'`).Scan(&original); err != nil {
		t.Fatal(err)
	}
	if err = a.DB.QueryRow(`SELECT count(*) FROM outbox WHERE event_type='checkout.focus_correction_result'`).Scan(&results); err != nil {
		t.Fatal(err)
	}
	if original != 1 || results != 0 {
		t.Fatal("rollback lost original or persisted result", original, results)
	}
	out, err := readCorrectionResult(ctx, a, "sale", ref)
	if err != nil || out.State != "correction_reconciliation_required" {
		t.Fatal("recoverable attempt lost", out.State, err)
	}
}
