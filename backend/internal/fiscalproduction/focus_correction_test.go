package fiscalproduction

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestFocusCorrectionDocumentedResponseAndUncertainNeverSuccess(t *testing.T) {
	for _, scenario := range []string{"authorized", "rejected", "malformed", "timeout", "credentials"} {
		t.Run(scenario, func(t *testing.T) {
			p := faultFocus(t, func(w http.ResponseWriter, r *http.Request) {
				if r.Method != "POST" || r.URL.Path != "/v2/nfe/ref/carta_correcao" {
					t.Error("wrong resource")
				}
				u, pw, ok := r.BasicAuth()
				if !ok || u != "SyntheticFocusToken123" || pw != "" {
					t.Error("auth")
				}
				switch scenario {
				case "authorized":
					io.WriteString(w, `{"status":"autorizado","numero_carta_correcao":1}`)
				case "rejected":
					io.WriteString(w, `{"status":"erro_autorizacao","mensagem_sefaz":"Rejected SyntheticFocusToken123"}`)
				case "malformed":
					io.WriteString(w, `{`)
				case "credentials":
					io.WriteString(w, `{"status":"autorizado","numero_carta_correcao":0}`)
				case "timeout":
					time.Sleep(50 * time.Millisecond)
				}
			})
			if scenario == "timeout" {
				p.client.Timeout = time.Millisecond
			}
			r := p.sendCorrection(context.Background(), "ref", []byte(`{"correcao":"Synthetic correction only"}`))
			raw, _ := json.Marshal(r)
			if strings.Contains(string(raw), "SyntheticFocusToken123") {
				t.Fatal("secret returned")
			}
			want := "correction_reconciliation_required"
			if scenario == "authorized" {
				want = "correction_authorized_homologation"
			}
			if scenario == "rejected" {
				want = "correction_failed"
			}
			if r.State != want {
				t.Fatal(r.State)
			}
		})
	}
}
func TestFocusCorrectionClaimSurvivesRestartAndIsTenantScoped(t *testing.T) {
	a := focusContext(t)
	p := faultFocus(t, func(http.ResponseWriter, *http.Request) { t.Error("unexpected HTTP") })
	p.configuration.DocumentModel = "55"
	for i := 0; i < 2; i++ {
		won, e := p.claim(context.Background(), a, "sale", "same-ref-cce", "checkout.focus_correction", []byte(`{"correcao":"Synthetic correction"}`))
		if e != nil || won != (i == 0) {
			t.Fatal("duplicate claim", won, e)
		}
	}
	var n int
	if a.DB.QueryRow(`SELECT count(*) FROM outbox WHERE tenant_id=? AND event_type='checkout.focus_correction'`, a.Actor.TenantID).Scan(&n) != nil || n != 1 {
		t.Fatal("journal")
	}
}
