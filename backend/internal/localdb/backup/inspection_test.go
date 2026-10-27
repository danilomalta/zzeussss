package backup

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"titansystem-backend/internal/localdb"
	"titansystem-backend/internal/localdb/identity"
	"titansystem-backend/internal/localdb/register"
)

func TestInspectionAuthenticatesCountsChangesAndRestoredBusinessDigest(t *testing.T) {
	db, scope, key, dir := fixture(t)
	ctx := context.Background()
	first := filepath.Join(dir, "first.tytbak")
	if e := Create(ctx, db, scope, key, first); e != nil {
		t.Fatal(e)
	}
	before, e := Inspect(ctx, first, scope, key)
	if e != nil {
		t.Fatal(e)
	}
	if before.Version != 1 || before.SchemaVersion != 69 || len(before.BusinessSHA256) != 64 || before.Records["cash_sessions"] != 0 || len(before.Unavailable) != 0 {
		t.Fatal("invalid initial inspection", before)
	}
	var owner string
	if e = db.QueryRow(`SELECT identity_id FROM memberships WHERE role='owner'`).Scan(&owner); e != nil {
		t.Fatal(e)
	}
	actor := identity.Scope{TenantID: scope.TenantID, StoreID: scope.StoreID, IdentityID: owner}
	if _, e = register.Open(ctx, db, actor, scope, register.OpenInput{SessionID: "inspected-session", OpeningCents: 1000}); e != nil {
		t.Fatal(e)
	}
	second := filepath.Join(dir, "second.tytbak")
	if e = Create(ctx, db, scope, key, second); e != nil {
		t.Fatal(e)
	}
	after, e := Inspect(ctx, second, scope, key)
	if e != nil {
		t.Fatal(e)
	}
	if after.Records["cash_sessions"] != 1 || after.BusinessSHA256 == before.BusinessSHA256 || after.ArchiveSHA256 == before.ArchiveSHA256 {
		t.Fatal("changed business not detected")
	}
	path := filepath.Join(dir, "restored.sqlite")
	if e = Restore(ctx, second, path, scope, key); e != nil {
		t.Fatal(e)
	}
	restored, e := localdb.Open(ctx, path)
	if e != nil {
		t.Fatal(e)
	}
	defer restored.Close()
	third := filepath.Join(dir, "third.tytbak")
	if e = Create(ctx, restored, scope, key, third); e != nil {
		t.Fatal(e)
	}
	recovered, e := Inspect(ctx, third, scope, key)
	if e != nil {
		t.Fatal(e)
	}
	if recovered.BusinessSHA256 != after.BusinessSHA256 || !reflect.DeepEqual(recovered.Records, after.Records) || recovered.ArchiveSHA256 == after.ArchiveSHA256 {
		t.Fatal("business digest changed across encrypted restoration")
	}
	raw, _ := json.Marshal(recovered)
	for _, secret := range [][]byte{[]byte(owner), []byte(scope.TenantID), []byte("Private tenant"), key, []byte("inspected-session")} {
		if bytes.Contains(raw, secret) {
			t.Fatal("inspection leaked row data")
		}
	}
	var active int
	if e = db.QueryRow(`SELECT count(*) FROM local_sessions WHERE revoked_unix IS NULL`).Scan(&active); e != nil || active != 1 {
		t.Fatal("inspection changed source sessions", e)
	}
}

func TestInspectionRejectsWrongKeyScopeTamperingAndCancellation(t *testing.T) {
	db, scope, key, dir := fixture(t)
	ctx := context.Background()
	archive := filepath.Join(dir, "original.tytbak")
	if e := Create(ctx, db, scope, key, archive); e != nil {
		t.Fatal(e)
	}
	wrong := append([]byte{}, key...)
	wrong[0] ^= 1
	if out, e := Inspect(ctx, archive, scope, wrong); e == nil || out.Version != 0 {
		t.Fatal("wrong key inspected")
	}
	other := scope
	other.TenantID = "other"
	if _, e := Inspect(ctx, archive, other, key); e == nil {
		t.Fatal("wrong scope inspected")
	}
	cancelCtx, cancel := context.WithCancel(ctx)
	cancel()
	if _, e := Inspect(cancelCtx, archive, scope, key); e == nil {
		t.Fatal("cancelled inspection succeeded")
	}
	raw, e := os.ReadFile(archive)
	if e != nil {
		t.Fatal(e)
	}
	raw[len(raw)-1] ^= 1
	tampered := filepath.Join(dir, "tampered.tytbak")
	if e = os.WriteFile(tampered, raw, 0600); e != nil {
		t.Fatal(e)
	}
	if _, e = Inspect(ctx, tampered, scope, key); e == nil {
		t.Fatal("tampered backup inspected")
	}
}
