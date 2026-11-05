package fiscal

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"titansystem-backend/internal/localdb/backup"
	"titansystem-backend/internal/localdb/identity"
)

func journalTestKey(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "disposable.key")
	if err := backup.InitKey(path); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestNativeJournalEncryptionScopeAndTamper(t *testing.T) {
	t.Setenv("TITAN_FISCAL_JOURNAL_KEY_FILE", journalTestKey(t))
	t.Setenv("TITAN_FISCAL_JOURNAL_PREVIOUS_KEY_FILES", "")
	a := identity.Scope{TenantID: "customer-a", StoreID: "store-a"}
	raw := `{"body":"synthetic personal data and XML, never official"}`
	sealed, err := sealNativeJournal(a, "note-a", "issue.request", raw)
	if err != nil || strings.Contains(sealed, raw) || strings.Contains(sealed, base64.StdEncoding.EncodeToString([]byte(raw))) {
		t.Fatal("archive not encrypted", err)
	}
	second, err := sealNativeJournal(a, "note-a", "issue.request", raw)
	if err != nil || second == sealed {
		t.Fatal("nonce was reused", err)
	}
	if opened, err := openNativeJournal(a, "note-a", "issue.request", sealed); err != nil || opened != raw {
		t.Fatal("round trip", err)
	}
	for _, c := range []struct {
		actor    identity.Scope
		id, kind string
	}{
		{identity.Scope{TenantID: "customer-b", StoreID: a.StoreID}, "note-a", "issue.request"},
		{identity.Scope{TenantID: a.TenantID, StoreID: "store-b"}, "note-a", "issue.request"},
		{a, "note-b", "issue.request"}, {a, "note-a", "issue.result"},
	} {
		if _, err := openNativeJournal(c.actor, c.id, c.kind, sealed); !errors.Is(err, ErrIntegrity) {
			t.Fatal("cross-scope ciphertext accepted", err)
		}
	}
	var envelope nativeJournalEnvelope
	if json.Unmarshal([]byte(strings.TrimPrefix(sealed, nativeJournalPrefix)), &envelope) != nil {
		t.Fatal("envelope")
	}
	envelope.Data[len(envelope.Data)-1] ^= 1
	modified, _ := json.Marshal(envelope)
	if _, err := openNativeJournal(a, "note-a", "issue.request", nativeJournalPrefix+string(modified)); !errors.Is(err, ErrIntegrity) {
		t.Fatal("tamper accepted", err)
	}
}

func TestNativeJournalKeyMissingAndHistoricalKey(t *testing.T) {
	old := journalTestKey(t)
	t.Setenv("TITAN_FISCAL_JOURNAL_KEY_FILE", old)
	t.Setenv("TITAN_FISCAL_JOURNAL_PREVIOUS_KEY_FILES", "")
	a := identity.Scope{TenantID: "customer", StoreID: "store"}
	sealed, err := sealNativeJournal(a, "note", "issue.request", `{"original":true}`)
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("TITAN_FISCAL_JOURNAL_KEY_FILE", "")
	if _, err := sealNativeJournal(a, "next", "issue.request", "data"); !errors.Is(err, ErrJournalKey) {
		t.Fatal("missing key allowed plaintext write", err)
	}
	if _, err := openNativeJournal(a, "note", "issue.request", sealed); !errors.Is(err, ErrJournalKey) {
		t.Fatal("missing key accepted", err)
	}
	t.Setenv("TITAN_FISCAL_JOURNAL_KEY_FILE", journalTestKey(t))
	if _, err := openNativeJournal(a, "note", "issue.request", sealed); !errors.Is(err, ErrJournalKey) {
		t.Fatal("wrong key accepted", err)
	}
	historical, _ := json.Marshal([]string{old})
	t.Setenv("TITAN_FISCAL_JOURNAL_PREVIOUS_KEY_FILES", string(historical))
	if raw, err := openNativeJournal(a, "note", "issue.request", sealed); err != nil || raw != `{"original":true}` {
		t.Fatal("explicit historical key could not read", err)
	}
	fresh, err := sealNativeJournal(a, "next", "issue.request", "next")
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("TITAN_FISCAL_JOURNAL_PREVIOUS_KEY_FILES", "")
	if raw, err := openNativeJournal(a, "next", "issue.request", fresh); err != nil || raw != "next" {
		t.Fatal("new writes did not use active key", err)
	}
	t.Setenv("TITAN_FISCAL_JOURNAL_PREVIOUS_KEY_FILES", "synthetic-invalid-private-reference")
	if _, err := sealNativeJournal(a, "next", "issue.request", "private payload"); !errors.Is(err, ErrJournalKey) || strings.Contains(err.Error(), "synthetic-invalid-private-reference") || strings.Contains(err.Error(), "private payload") {
		t.Fatal("invalid configuration allowed a write or exposed private input")
	}
}

func TestNativeJournalLegacyAndMalformedEnvelope(t *testing.T) {
	a := identity.Scope{TenantID: "customer", StoreID: "store"}
	legacy := `{"original":"immutable legacy row"}`
	t.Setenv("TITAN_FISCAL_JOURNAL_LEGACY_READ", "")
	if _, err := openNativeJournal(a, "note", "issue.request", legacy); !errors.Is(err, ErrJournalLegacy) {
		t.Fatal("silent downgrade to plaintext", err)
	}
	t.Setenv("TITAN_FISCAL_JOURNAL_LEGACY_READ", "true")
	if raw, err := openNativeJournal(a, "note", "issue.request", legacy); err != nil || raw != legacy {
		t.Fatal("legacy record changed", err)
	}
	for _, raw := range []string{"titan-fiscal-journal:v2:{}", nativeJournalPrefix + "{}", nativeJournalPrefix + "not json"} {
		if _, err := openNativeJournal(a, "note", "issue.request", raw); !errors.Is(err, ErrIntegrity) {
			t.Fatal("malformed envelope accepted", err)
		}
	}
}
