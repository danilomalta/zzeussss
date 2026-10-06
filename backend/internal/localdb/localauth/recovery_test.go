package localauth

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"sync"
	"testing"
)

const ownerPassword = "uma-senha-bem-longa-1"
const recoveredPassword = "outra-senha-bem-longa-2"

func TestRecoveryIsSingleUseAuditedAndRevokesEveryOwnerSession(t *testing.T) {
	db, device := fixture(t)
	hash, _ := HashPassword(ownerPassword)
	if _, err := db.Exec(`INSERT INTO local_passwords VALUES ('a','owner',?,'now')`, hash); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	session, err := Login(ctx, db, device, "owner", ownerPassword)
	if err != nil {
		t.Fatal(err)
	}
	secret := bytes.Repeat([]byte{2}, 32)
	saved := false
	if err := IssueOwnerRecovery(ctx, db, device, "owner", ownerPassword, secret, func() error { saved = true; return nil }); err != nil || !saved {
		t.Fatal("key not prepared")
	}
	key := base64.RawURLEncoding.EncodeToString(secret)
	if err := RecoverOwner(ctx, db, device, "owner", key, recoveredPassword); err != nil {
		t.Fatal(err)
	}
	if _, err := Resolve(ctx, db, session.Token); err == nil {
		t.Fatal("session survived")
	}
	if _, err := Login(ctx, db, device, "owner", ownerPassword); err == nil {
		t.Fatal("old password survived")
	}
	if _, err := Login(ctx, db, device, "owner", recoveredPassword); err != nil {
		t.Fatal(err)
	}
	if err := RecoverOwner(ctx, db, device, "owner", key, "another-later-password"); !errors.Is(err, ErrDenied) {
		t.Fatal("key reused")
	}
	var count int
	if err := db.QueryRow(`SELECT count(*) FROM account_access_operations WHERE kind='owner_recovered'`).Scan(&count); err != nil || count != 1 {
		t.Fatal("audit duplicated or absent")
	}
	var stored []byte
	if err := db.QueryRow(`SELECT token_sha256 FROM owner_recovery_keys`).Scan(&stored); err != nil || bytes.Equal(stored, secret) {
		t.Fatal("secret stored directly")
	}
}

func TestRecoveryRejectsWrongKeyScopeExpiryRevocationAndManager(t *testing.T) {
	for _, scenario := range []string{"key", "device", "tenant", "expired", "future-issued", "owner-revoked", "device-revoked", "manager"} {
		t.Run(scenario, func(t *testing.T) {
			db, device := fixture(t)
			hash, _ := HashPassword(ownerPassword)
			if _, err := db.Exec(`INSERT INTO local_passwords VALUES ('a','owner',?,'now')`, hash); err != nil {
				t.Fatal(err)
			}
			secret := bytes.Repeat([]byte{2}, 32)
			if err := IssueOwnerRecovery(context.Background(), db, device, "owner", ownerPassword, secret, func() error { return nil }); err != nil {
				t.Fatal(err)
			}
			key := base64.RawURLEncoding.EncodeToString(secret)
			switch scenario {
			case "key":
				key = base64.RawURLEncoding.EncodeToString(bytes.Repeat([]byte{3}, 32))
			case "device":
				device.DeviceID = "other"
			case "tenant":
				device.TenantID = "other"
			case "expired":
				db.Exec(`UPDATE owner_recovery_keys SET expires_unix=1`)
			case "future-issued":
				db.Exec(`UPDATE owner_recovery_keys SET issued_unix=9999999999`)
			case "owner-revoked":
				db.Exec(`UPDATE memberships SET status='revoked' WHERE identity_id='owner'`)
			case "device-revoked":
				db.Exec(`UPDATE device_pairings SET status='revoked'`)
			case "manager":
				db.Exec(`UPDATE memberships SET role='manager' WHERE identity_id='owner'`)
			}
			if err := RecoverOwner(context.Background(), db, device, "owner", key, recoveredPassword); err == nil {
				t.Fatal("accepted invalid recovery")
			}
			var used int
			if err := db.QueryRow(`SELECT count(*) FROM owner_recovery_keys WHERE consumed_unix IS NOT NULL`).Scan(&used); err != nil || used != 0 {
				t.Fatal("consumed rejected key")
			}
		})
	}
}

func TestRecoveryAuditFailureRollsBackPasswordSessionsAndConsumption(t *testing.T) {
	for _, failure := range []string{"ABORT,'test'", "IGNORE"} {
		db, device := fixture(t)
		hash, _ := HashPassword(ownerPassword)
		if _, err := db.Exec(`INSERT INTO local_passwords VALUES ('a','owner',?,'now')`, hash); err != nil {
			t.Fatal(err)
		}
		secret := bytes.Repeat([]byte{2}, 32)
		if err := IssueOwnerRecovery(context.Background(), db, device, "owner", ownerPassword, secret, func() error { return nil }); err != nil {
			t.Fatal(err)
		}
		session, err := Login(context.Background(), db, device, "owner", ownerPassword)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := db.Exec(`CREATE TRIGGER fail_recovery BEFORE INSERT ON account_access_operations WHEN NEW.kind='owner_recovered' BEGIN SELECT RAISE(` + failure + `); END`); err != nil {
			t.Fatal(err)
		}
		if err := RecoverOwner(context.Background(), db, device, "owner", base64.RawURLEncoding.EncodeToString(secret), recoveredPassword); err == nil {
			t.Fatal("audit failure accepted")
		}
		if _, err := Resolve(context.Background(), db, session.Token); err != nil {
			t.Fatal("revocation escaped rollback")
		}
		if _, err := Login(context.Background(), db, device, "owner", ownerPassword); err != nil {
			t.Fatal("password escaped rollback")
		}
		var used int
		if err := db.QueryRow(`SELECT count(*) FROM owner_recovery_keys WHERE consumed_unix IS NOT NULL`).Scan(&used); err != nil || used != 0 {
			t.Fatal("key consumption escaped rollback")
		}
	}
}

func TestRecoveryConcurrentAttemptsOnlyOneCommits(t *testing.T) {
	db, device := fixture(t)
	hash, _ := HashPassword(ownerPassword)
	if _, err := db.Exec(`INSERT INTO local_passwords VALUES ('a','owner',?,'now')`, hash); err != nil {
		t.Fatal(err)
	}
	secret := bytes.Repeat([]byte{2}, 32)
	if err := IssueOwnerRecovery(context.Background(), db, device, "owner", ownerPassword, secret, func() error { return nil }); err != nil {
		t.Fatal(err)
	}
	key := base64.RawURLEncoding.EncodeToString(secret)
	var wg sync.WaitGroup
	results := make(chan error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			results <- RecoverOwner(context.Background(), db, device, "owner", key, recoveredPassword)
		}()
	}
	wg.Wait()
	close(results)
	successes := 0
	for err := range results {
		if err == nil {
			successes++
		} else if !errors.Is(err, ErrDenied) {
			t.Fatal(err)
		}
	}
	if successes != 1 {
		t.Fatal("multiple recoveries")
	}
}

func TestRecoveryPreparationRequiresPasswordAndPreservesOldKeyOnFileFailure(t *testing.T) {
	db, device := fixture(t)
	hash, _ := HashPassword(ownerPassword)
	if _, err := db.Exec(`INSERT INTO local_passwords VALUES ('a','owner',?,'now')`, hash); err != nil {
		t.Fatal(err)
	}
	secret := bytes.Repeat([]byte{2}, 32)
	called := false
	if err := IssueOwnerRecovery(context.Background(), db, device, "owner", "wrong", secret, func() error { called = true; return nil }); err == nil || called {
		t.Fatal("published before authenticating")
	}
	if err := IssueOwnerRecovery(context.Background(), db, device, "owner", ownerPassword, secret, func() error { return nil }); err != nil {
		t.Fatal(err)
	}
	if err := IssueOwnerRecovery(context.Background(), db, device, "owner", ownerPassword, bytes.Repeat([]byte{3}, 32), func() error { return errors.New("disk failure") }); err == nil {
		t.Fatal("disk failure accepted")
	}
	if err := RecoverOwner(context.Background(), db, device, "owner", base64.RawURLEncoding.EncodeToString(secret), recoveredPassword); err != nil {
		t.Fatal("old key lost")
	}
}

func TestOwnPasswordChangeInvalidatesPreparedRecoveryKey(t *testing.T) {
	db, device := fixture(t)
	hash, _ := HashPassword(ownerPassword)
	if _, err := db.Exec(`INSERT INTO local_passwords VALUES ('a','owner',?,'now')`, hash); err != nil {
		t.Fatal(err)
	}
	secret := bytes.Repeat([]byte{2}, 32)
	if err := IssueOwnerRecovery(context.Background(), db, device, "owner", ownerPassword, secret, func() error { return nil }); err != nil {
		t.Fatal(err)
	}
	session, err := Login(context.Background(), db, device, "owner", ownerPassword)
	if err != nil {
		t.Fatal(err)
	}
	if err := ChangeOwnPassword(context.Background(), db, device, session.Token, ownerPassword, recoveredPassword); err != nil {
		t.Fatal(err)
	}
	if err := RecoverOwner(context.Background(), db, device, "owner", base64.RawURLEncoding.EncodeToString(secret), "another-later-password"); !errors.Is(err, ErrDenied) {
		t.Fatal("old recovery key survived password change")
	}
}

func TestRecoveryRotationAuditFailureLeavesOldKeyActiveAndMarksFileAsUncommitted(t *testing.T) {
	db, device := fixture(t)
	hash, _ := HashPassword(ownerPassword)
	if _, err := db.Exec(`INSERT INTO local_passwords VALUES ('a','owner',?,'now')`, hash); err != nil {
		t.Fatal(err)
	}
	secret := bytes.Repeat([]byte{2}, 32)
	if err := IssueOwnerRecovery(context.Background(), db, device, "owner", ownerPassword, secret, func() error { return nil }); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`CREATE TRIGGER fail_rotation BEFORE INSERT ON account_access_operations WHEN NEW.kind='recovery_issued' BEGIN SELECT RAISE(IGNORE); END`); err != nil {
		t.Fatal(err)
	}
	persisted := false
	if err := IssueOwnerRecovery(context.Background(), db, device, "owner", ownerPassword, bytes.Repeat([]byte{3}, 32), func() error { persisted = true; return nil }); err == nil || !persisted {
		t.Fatal("incorrect file/DB failure handling")
	}
	if _, err := db.Exec(`DROP TRIGGER fail_rotation`); err != nil {
		t.Fatal(err)
	}
	if err := RecoverOwner(context.Background(), db, device, "owner", base64.RawURLEncoding.EncodeToString(secret), recoveredPassword); err != nil {
		t.Fatal("rotation failure invalidated old key")
	}
}
