package localapi

import (
	"bytes"
	"encoding/json"
	"testing"
)

func TestCatalogReadsActualLocalRows(t *testing.T) {
	_, app, owner := fixture(t)
	token := loginToken(t, app, owner.OwnerID)
	status, body := request(t, app, "POST", "/local/v1/products", `{"sku":"A1","name":"Café","unit":"unit","price_cents":899,"cost_cents":500}`, token)
	if status != 201 {
		t.Fatalf("produto: %d %s", status, body)
	}
	var created struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(body, &created); err != nil || created.ID == "" {
		t.Fatalf("ID do produto: %s %v", body, err)
	}
	status, body = request(t, app, "GET", "/local/v1/products", "", token)
	if status != 200 || !bytes.Contains(body, []byte(`"price_cents":899`)) || !bytes.Contains(body, []byte(created.ID)) {
		t.Fatalf("lista: %d %s", status, body)
	}
	status, body = request(t, app, "POST", "/local/v1/locations", `{"kind":"shelf","name":"Gôndola"}`, token)
	if status != 201 {
		t.Fatalf("local: %d %s", status, body)
	}
	status, body = request(t, app, "GET", "/local/v1/locations", "", token)
	if status != 200 || !bytes.Contains(body, []byte("Gôndola")) {
		t.Fatalf("lista de locais: %d %s", status, body)
	}
	status, _ = request(t, app, "POST", "/local/v1/products", `{"sku":"A2","name":"Sem sessão","price_cents":100}`, "")
	if status != 401 {
		t.Fatalf("cadastro sem sessão: %d", status)
	}
}
