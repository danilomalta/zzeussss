package fiscalproduction

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"

	"titansystem-backend/internal/localdb/fiscal"
)

// An ignored write is not a durable reconciliation. The original attempt must
// remain recoverable without another POST, including when archival is pending.
func TestFocusCorrectionQueryIgnoredJournalRemainsRecoverable(t *testing.T) {
	for _, archive := range []string{"pending", "archived"} {
		t.Run(archive, func(t *testing.T) {
			a := focusContext(t)
			ctx := context.Background()
			p := faultFocus(t, func(http.ResponseWriter, *http.Request) { t.Error("journal contacted provider") })
			ref := focusReference(a.Actor.TenantID, "original-sale")
			if won, err := p.claim(ctx, a, "original-sale", ref+"-cce", "checkout.focus_correction", []byte(`{"correcao":"Synthetic original correction only"}`)); err != nil || !won {
				t.Fatal("original claim", err)
			}
			hash, err := correctionClaimHash(ctx, a, "original-sale", ref)
			if err != nil {
				t.Fatal(err)
			}
			if _, err = a.DB.Exec(`CREATE TRIGGER ignore_correction_query BEFORE INSERT ON outbox WHEN NEW.operation_id LIKE '%-cce-query%' BEGIN SELECT RAISE(IGNORE); END`); err != nil {
				t.Fatal(err)
			}
			result := CorrectionResult{State: "correction_authorized_homologation", Number: 1, ArchiveState: archive}
			if err = p.recordCorrectionReconciliation(ctx, a, "original-sale", ref, hash, result); !errors.Is(err, fiscal.ErrConflict) {
				t.Fatalf("unpersisted reconciliation reported success: %v", err)
			}
			out, err := readCorrectionResult(ctx, a, "original-sale", ref)
			if err != nil || out.State != "correction_reconciliation_required" {
				t.Fatal("attempt no longer recoverable", err)
			}
			if again, err := correctionClaimHash(ctx, a, "original-sale", ref); err != nil || again != hash {
				t.Fatal("original attempt changed", err)
			}
			if won, err := p.claim(ctx, a, "original-sale", ref+"-cce", "checkout.focus_correction", []byte(`{"correcao":"Synthetic original correction only"}`)); err != nil || won {
				t.Fatal("second POST permitted", err)
			}
			if _, err = a.DB.Exec(`DROP TRIGGER ignore_correction_query`); err != nil {
				t.Fatal(err)
			}
			for i := 0; i < 2; i++ {
				if err = p.recordCorrectionReconciliation(ctx, a, "original-sale", ref, hash, result); err != nil {
					t.Fatal("journal-only recovery", err)
				}
			}
			out, err = readCorrectionResult(ctx, a, "original-sale", ref)
			if err != nil || out != result {
				t.Fatal("original result not recovered", err)
			}
			var n int
			if err = a.DB.QueryRow(`SELECT count(*) FROM outbox WHERE event_type='checkout.focus_correction_result'`).Scan(&n); err != nil || n != 1 {
				t.Fatal("duplicate result", n, err)
			}
		})
	}
}

// These invalid processed-event documents exercise the existing XSD/identity
// checks. They are synthetic local HTTP fixtures, not Focus acceptance evidence.
func TestFocusCorrectionQueryRejectsMismatchedProcessedEvent(t *testing.T) {
	const text = "Synthetic original correction text only"
	data, key := correctionQueryXML(t, text)
	for _, scenario := range []string{"request-key", "response-key", "response-sequence", "missing-protocol"} {
		t.Run(scenario, func(t *testing.T) {
			xmlData := string(data)
			foreign := syntheticFocusKey(t)
			switch scenario {
			case "request-key":
				xmlData = strings.Replace(xmlData, "<chNFe>"+key+"</chNFe>", "<chNFe>"+foreign+"</chNFe>", 1)
			case "response-key":
				at := strings.Index(xmlData, "<retEvento")
				if at < 0 {
					t.Fatal("synthetic event result missing")
				}
				xmlData = xmlData[:at] + strings.Replace(xmlData[at:], "<chNFe>"+key+"</chNFe>", "<chNFe>"+foreign+"</chNFe>", 1)
			case "response-sequence":
				at := strings.Index(xmlData, "<retEvento")
				if at < 0 {
					t.Fatal("synthetic event result missing")
				}
				xmlData = xmlData[:at] + strings.Replace(xmlData[at:], "<nSeqEvento>1</nSeqEvento>", "<nSeqEvento>2</nSeqEvento>", 1)
			case "missing-protocol":
				xmlData = strings.Replace(xmlData, "<nProt>135260000000002</nProt>", "", 1)
			}
			if xmlData == string(data) {
				t.Fatal("fault fixture not changed")
			}
			a := focusContext(t)
			p := faultFocus(t, func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodGet {
					t.Error("recovery attempted POST")
					w.WriteHeader(500)
					return
				}
				_, _ = w.Write([]byte(xmlData))
			})
			ref := focusReference(a.Actor.TenantID, "sale")
			body, _ := json.Marshal(map[string]string{"correcao": text})
			if won, err := p.claim(context.Background(), a, "sale", ref+"-cce", "checkout.focus_correction", body); err != nil || !won {
				t.Fatal("claim", err)
			}
			hash, err := correctionClaimHash(context.Background(), a, "sale", ref)
			if err != nil {
				t.Fatal(err)
			}
			links, _ := json.Marshal(focusArchiveLinks{Correction: "/arquivos/" + key[6:20] + "/202611/XMLs/correction.xml", CorrectionNumber: 1})
			out, err := p.reconcileCorrection(context.Background(), a, "sale", ref, hash, CorrectionResult{State: "correction_reconciliation_required"}, DocumentResult{State: "authorized_homologation", Key: key, IntentID: "original-intent", Evidence: links})
			if err != nil || out.State != "correction_reconciliation_required" {
				t.Fatal("incompatible event accepted", out.State, err)
			}
			var count int
			if err = a.DB.QueryRow(`SELECT count(*) FROM outbox WHERE event_type IN ('checkout.focus_correction_result','checkout.focus_xml_archive')`).Scan(&count); err != nil || count != 0 {
				t.Fatal("incompatible event persisted", count, err)
			}
		})
	}
}
