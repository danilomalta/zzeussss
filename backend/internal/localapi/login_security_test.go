package localapi

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"testing"
)

func TestHTTPLoginUniformFailureAndBruteForceLimit(t *testing.T) {
	_, app, owner := fixture(t)
	var generic []byte
	for i, id := range []string{owner.OwnerID, "unknown-owner", owner.OwnerID, "unknown-owner", owner.OwnerID} {
		raw, _ := json.Marshal(map[string]string{"identity_id": id, "password": "incorrect-password"})
		code, body := request(t, app, "POST", "/local/v1/login", string(raw), "")
		if code != 401 {
			t.Fatalf("attempt %d: %d", i, code)
		}
		if i == 0 {
			generic = body
		} else if !bytes.Equal(body, generic) {
			t.Fatal("account enumeration through response")
		}
		if bytes.Contains(body, []byte(id)) || bytes.Contains(body, []byte("incorrect-password")) {
			t.Fatal("login data leak")
		}
	}
	raw, _ := json.Marshal(map[string]string{"identity_id": owner.OwnerID, "password": "senha-forte-de-teste-1"})
	code, _ := request(t, app, "POST", "/local/v1/login", string(raw), "")
	if code != 429 {
		t.Fatalf("brute force limit: %d", code)
	}
}
func TestHTTPRecoveryUniformExistingAndUnknownAccount(t *testing.T) {
	_, app, owner := fixture(t)
	key := base64.RawURLEncoding.EncodeToString(bytes.Repeat([]byte{9}, 32))
	var generic []byte
	for i, id := range []string{owner.OwnerID, "unknown-owner"} {
		raw, _ := json.Marshal(map[string]string{"identity_id": id, "recovery_key": key, "new_password": "new-owner-password"})
		code, body := request(t, app, "POST", "/local/v1/account/recover", string(raw), "")
		if code != 401 {
			t.Fatalf("recovery: %d", code)
		}
		if i == 0 {
			generic = body
		} else if !bytes.Equal(body, generic) {
			t.Fatal("recovery account enumeration")
		}
		if bytes.Contains(body, []byte(key)) || bytes.Contains(body, []byte(id)) {
			t.Fatal("recovery data leak")
		}
	}
	// A rejected recovery must not replace the owner's password or create an account.
	code, _ := request(t, app, "POST", "/local/v1/login", `{"identity_id":"`+owner.OwnerID+`","password":"senha-forte-de-teste-1"}`, "")
	if code != 200 {
		t.Fatal("failed recovery changed owner login")
	}
}
