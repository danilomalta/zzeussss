package fiscalproduction

import (
	"context"
	"encoding/json"
	"net/http"
	"sync/atomic"
	"testing"
	"titansystem-backend/internal/localdb/fiscal"
)

func TestFocusContingencyNeverCallsOfflineAuthorizationSEFAZUntilFinalized(t *testing.T) {
	a := focusContext(t)
	key := syntheticFocusKey(t)
	ref := focusReference(a.Actor.TenantID, "offline-sale")
	var posts, gets atomic.Int32
	p := faultFocus(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" {
			posts.Add(1)
			w.WriteHeader(201)
		} else {
			gets.Add(1)
		}
		json.NewEncoder(w).Encode(map[string]any{"cnpj_emitente": "12345678000123", "ref": ref, "status": "autorizado", "chave_nfe": "NFe" + key, "contingencia_offline": true, "contingencia_offline_efetivada": r.Method == "GET"})
	})
	intent := fiscal.Intent{ID: "offline-intent", Snapshot: fiscal.Snapshot{Profile: fiscal.ProfileInput{CNPJ: "12345678000123", Model: "65"}}}
	first, e := p.submit(context.Background(), a, "offline-sale", intent, ref, []byte(`{}`))
	if e != nil || first.State != "pending" || first.ErrorCode != "focus_contingency_pending" {
		t.Fatal("premature authorization", first.State, e)
	}
	restarted := *p
	out, e := restarted.submit(context.Background(), a, "offline-sale", intent, ref, []byte(`{}`))
	if e != nil || out.State != "authorized_homologation" {
		t.Fatal("final consultation", e)
	}
	if posts.Load() != 1 || gets.Load() != 1 {
		t.Fatal("duplicate invoice")
	}
}
