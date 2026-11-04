package secretconfig

import (
	"encoding/base64"
	"encoding/hex"
	"strings"
	"testing"
)

func TestJWTKeyPolicyIsSeparateFromDatabasePassword(t *testing.T) {
	// Test data only: neither this deterministic sample nor any example is a
	// customer key. Acceptance proves format, NOT entropy or uniqueness.
	key := base64.RawURLEncoding.EncodeToString([]byte("synthetic-nonrepeating-32-bytes!"))
	if !ValidJWTKey(key) {
		t.Fatal("canonical synthetic key refused")
	}
	hexKey := hex.EncodeToString([]byte("synthetic-nonrepeating-32-bytes!"))
	if !ValidJWTKey(hexKey) {
		t.Fatal("existing private hex provisioning refused")
	}
	invalid := []string{"", " \t", "super_secret_key_change_in_prod", "titanpass", "short", strings.Repeat("x", 64), key[:42], key + "=", " " + key, key + "\n", key[:42] + "!", strings.Repeat("A", 43), base64.RawURLEncoding.EncodeToString([]byte(strings.Repeat("ab", 16))), strings.ToUpper(hexKey), hexKey[:62], strings.Repeat("ab", 32), strings.Repeat("0", 64)}
	for i, value := range invalid {
		if ValidJWTKey(value) {
			t.Fatalf("invalid JWT key case %d accepted", i)
		}
	}
	// Low trailing bits are not canonical for a 32-byte base64url value.
	if ValidJWTKey(key[:42] + "B") {
		t.Fatal("noncanonical base64 accepted")
	}
	password := "customer-specific-database-password"
	if Unsafe(password) || ValidJWTKey(password) {
		t.Fatal("database and JWT policies conflated")
	}
}

func TestExplicitPrivateJWTGeneration(t *testing.T) {
	a, err := GenerateJWTKey()
	if err != nil || !ValidJWTKey(a) {
		t.Fatal("private generation failed")
	}
	b, err := GenerateJWTKey()
	if err != nil || !ValidJWTKey(b) || a == b {
		t.Fatal("private generation reused material")
	}
}
