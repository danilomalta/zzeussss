package fiscalproduction

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"titansystem-backend/internal/localdb/fiscal"
	"titansystem-backend/internal/localdb/identity"
	"titansystem-backend/internal/localdb/localauth"
	"titansystem-backend/internal/sqlitewrite"
)

func authorizationFocus(t *testing.T, a RequestContext) *FocusProvider {
	t.Helper()
	p, err := newFocusProvider(TenantFiscalConfiguration{DocumentModel: "55", Environment: "homologation", CertificateType: "A1", Binding: ProviderBinding{TenantID: a.Actor.TenantID, StoreID: a.Actor.StoreID, DeviceID: a.Device.DeviceID}}, []byte("SyntheticAuthorizationToken123"))
	if err != nil {
		t.Fatal(err)
	}
	return p
}

// An authorization is a writer because RequireTx updates the license clock.
// Both gates must wait for an existing writer, never upgrade a stale read.
func TestFiscalAuthorizationWaitsForImmediateWriter(t *testing.T) {
	a := focusContext(t)
	p := authorizationFocus(t, a)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	held, release, writerDone := make(chan struct{}), make(chan struct{}), make(chan error, 1)
	go func() {
		writerDone <- sqlitewrite.Run(ctx, a.DB, fiscal.ErrConflict, func(tx *sql.Tx) error {
			close(held)
			select {
			case <-release:
				return nil
			case <-ctx.Done():
				return ctx.Err()
			}
		})
	}()
	<-held
	results, started := make(chan error, 2), make(chan struct{}, 2)
	for _, authorize := range []func() error{func() error { return authorizeCustomerCredentials(ctx, a) }, func() error { return p.scope(ctx, a, identity.Sell) }} {
		go func(fn func() error) { started <- struct{}{}; results <- fn() }(authorize)
	}
	<-started
	<-started
	var early error
	select {
	case early = <-results:
		close(release)
		<-writerDone
		t.Fatal("authorization returned before writer release", early)
	case <-time.After(50 * time.Millisecond):
	}
	close(release)
	if err := <-writerDone; err != nil {
		t.Fatal("writer", err)
	}
	for i := 0; i < 2; i++ {
		if err := <-results; err != nil {
			t.Fatal("authorization lost to writer", err)
		}
	}
}

func TestFiscalAuthorizationClockCommitAndDeniedRollback(t *testing.T) {
	for _, name := range []string{"credentials", "focus"} {
		t.Run(name, func(t *testing.T) {
			a := focusContext(t)
			p := authorizationFocus(t, a)
			authorize := func() error {
				if name == "credentials" {
					return authorizeCustomerCredentials(context.Background(), a)
				}
				return p.scope(context.Background(), a, identity.Sell)
			}
			before := time.Now().Unix() - 30
			if _, err := a.DB.Exec(`UPDATE module_contract_state SET last_observed_unix=? WHERE tenant_id=?`, before, a.Actor.TenantID); err != nil {
				t.Fatal(err)
			}
			if err := authorize(); err != nil {
				t.Fatal(err)
			}
			var observed int64
			if err := a.DB.QueryRow(`SELECT last_observed_unix FROM module_contract_state WHERE tenant_id=?`, a.Actor.TenantID).Scan(&observed); err != nil || observed <= before {
				t.Fatal("successful authorization clock not committed", observed, err)
			}
			if _, err := a.DB.Exec(`UPDATE memberships SET role='manager' WHERE tenant_id=? AND identity_id=?`, a.Actor.TenantID, a.Actor.IdentityID); err != nil {
				t.Fatal(err)
			}
			// A non-owner needs explicit store scope and a current session.
			// Exercise the owner gate rather than only missing-store refusal.
			if _, err := a.DB.Exec(`INSERT INTO membership_stores(tenant_id,identity_id,store_id) VALUES(?,?,?)`, a.Actor.TenantID, a.Actor.IdentityID, a.Actor.StoreID); err != nil {
				t.Fatal("manager store scope", err)
			}
			session, err := localauth.Login(context.Background(), a.DB, a.Device, a.Actor.IdentityID, "synthetic-test-password")
			if err != nil {
				t.Fatal("manager login", err)
			}
			a.Token = session.Token
			if _, err := a.DB.Exec(`UPDATE module_contract_state SET last_observed_unix=? WHERE tenant_id=?`, before, a.Actor.TenantID); err != nil {
				t.Fatal(err)
			}
			if err := authorize(); !errors.Is(err, identity.ErrDenied) {
				t.Fatal("non-owner authorized", err)
			}
			if err := a.DB.QueryRow(`SELECT last_observed_unix FROM module_contract_state WHERE tenant_id=?`, a.Actor.TenantID).Scan(&observed); err != nil || observed != before {
				t.Fatal("denied transaction not rolled back", observed, err)
			}
			if _, err := a.DB.Exec(`UPDATE memberships SET role='owner' WHERE tenant_id=? AND identity_id=?`, a.Actor.TenantID, a.Actor.IdentityID); err != nil {
				t.Fatal(err)
			}
		})
	}
}
