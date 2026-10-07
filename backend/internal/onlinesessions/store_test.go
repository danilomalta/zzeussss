package onlinesessions

import (
	"context"
	"database/sql/driver"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
)

const testID = "11111111-1111-4111-8111-111111111112"
const testUser = "22222222-2222-4222-8222-222222222222"
const testTenant = "33333333-3333-4333-8333-333333333333"

type digestArgument struct{}

func (digestArgument) Match(v driver.Value) bool {
	s, ok := v.(string)
	return ok && len(s) == 64 && !strings.Contains(s, ".")
}
func testStore(t *testing.T) (*Store, sqlmock.Sqlmock) {
	t.Helper()
	db, m, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	return New(db, "test-only-key"), m
}
func identity(m sqlmock.Sqlmock) {
	m.ExpectQuery(`SELECT u.name, u.password_hash.*FOR SHARE OF u, t`).WithArgs(testUser, testTenant, "owner").WillReturnRows(sqlmock.NewRows([]string{"name", "password_hash"}).AddRow("Owner", "hash"))
}
func expectAudit(m sqlmock.Sqlmock, event string) *sqlmock.ExpectedExec {
	return m.ExpectExec(`INSERT INTO online_session_audit`).WithArgs(sqlmock.AnyArg(), testTenant, testUser, sqlmock.AnyArg(), event)
}

func TestCreateRequiresDurableSessionHashAndAudit(t *testing.T) {
	for _, fault := range []string{"none", "session", "hash", "audit", "commit"} {
		t.Run(fault, func(t *testing.T) {
			s, m := testStore(t)
			now := time.Now()
			m.ExpectBegin()
			accountLock(m)
			identity(m)
			m.ExpectQuery(`SELECT clock_timestamp`).WillReturnRows(sqlmock.NewRows([]string{"now"}).AddRow(now))
			e := m.ExpectExec(`INSERT INTO online_sessions`).WithArgs(sqlmock.AnyArg(), testTenant, testUser, "owner", now, now.Add(7*24*time.Hour))
			if fault == "session" {
				e.WillReturnError(errors.New("private"))
			} else {
				e.WillReturnResult(sqlmock.NewResult(0, 1))
				h := m.ExpectExec(`INSERT INTO online_refresh_tokens`).WithArgs(digestArgument{}, sqlmock.AnyArg())
				if fault == "hash" {
					h.WillReturnError(errors.New("private"))
				} else {
					h.WillReturnResult(sqlmock.NewResult(0, 1))
					a := expectAudit(m, "created")
					if fault == "audit" {
						a.WillReturnError(errors.New("private"))
					} else {
						a.WillReturnResult(sqlmock.NewResult(0, 1))
					}
				}
			}
			if fault == "none" {
				m.ExpectCommit()
			} else if fault == "commit" {
				m.ExpectCommit().WillReturnError(errors.New("commit uncertain"))
			} else {
				m.ExpectRollback()
			}
			pair, err := s.Create(context.Background(), Scope{Tenant: testTenant, User: testUser, Role: "owner"}, "hash")
			if fault == "none" {
				id, e := ParseRefresh(pair.Refresh)
				if err != nil || e != nil || !ValidID(id) || pair.Access == "" || pair.ExpiresIn != 900 {
					t.Fatal("sessão não criada")
				}
			} else if err != ErrUnavailable || pair.Access != "" || pair.Refresh != "" {
				t.Fatal("falha emitiu tokens")
			}
			if err := m.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}
func rotationLock(m sqlmock.Sqlmock, now time.Time, revoked interface{}) {
	m.ExpectQuery(`SELECT tenant_id, user_id, role, expires_at, revoked_at, clock_timestamp.*FOR UPDATE`).WithArgs(testID).WillReturnRows(sqlmock.NewRows([]string{"tenant", "user", "role", "expires", "revoked", "now"}).AddRow(testTenant, testUser, "owner", now.Add(time.Hour), revoked, now))
}

func TestRotateConsumesExactlyOnceAndCommitsReplayRevocation(t *testing.T) {
	for _, state := range []string{"unused", "consumed", "unknown", "revoked", "audit-fails", "zero-consume"} {
		t.Run(state, func(t *testing.T) {
			s, m := testStore(t)
			raw := "v1." + testID + "." + strings.Repeat("A", 43)
			now := time.Now()
			m.ExpectBegin()
			m.ExpectQuery(`SELECT tenant_id, user_id FROM online_sessions WHERE id=\$1`).WithArgs(testID).WillReturnRows(sqlmock.NewRows([]string{"tenant", "user"}).AddRow(testTenant, testUser))
			accountLock(m)
			var revoked interface{}
			if state == "revoked" {
				revoked = now
			}
			rotationLock(m, now, revoked)
			if state != "revoked" {
				identity(m)
				m.ExpectQuery(`SELECT clock_timestamp`).WillReturnRows(sqlmock.NewRows([]string{"now"}).AddRow(now))
				rows := sqlmock.NewRows([]string{"consumed_at"})
				if state == "consumed" {
					rows.AddRow(now)
				} else if state != "unknown" {
					rows.AddRow(nil)
				}
				m.ExpectQuery(`SELECT consumed_at FROM online_refresh_tokens`).WithArgs(tokenDigest(raw), testID).WillReturnRows(rows)
				if state == "consumed" {
					m.ExpectExec(`UPDATE online_sessions SET revoked_at`).WithArgs(testID, testTenant, testUser).WillReturnResult(sqlmock.NewResult(0, 1))
					expectAudit(m, "replay_revoked").WillReturnResult(sqlmock.NewResult(0, 1))
					m.ExpectCommit()
				} else if state != "unknown" {
					changed := int64(1)
					if state == "zero-consume" {
						changed = 0
					}
					m.ExpectExec(`UPDATE online_refresh_tokens SET consumed_at=.*consumed_at IS NULL`).WithArgs(tokenDigest(raw), testID).WillReturnResult(sqlmock.NewResult(0, changed))
					if changed == 1 {
						m.ExpectExec(`INSERT INTO online_refresh_tokens`).WithArgs(digestArgument{}, testID).WillReturnResult(sqlmock.NewResult(0, 1))
						a := expectAudit(m, "rotated")
						if state == "audit-fails" {
							a.WillReturnError(errors.New("private"))
						} else {
							a.WillReturnResult(sqlmock.NewResult(0, 1))
							m.ExpectCommit()
						}
					}
				}
			}
			if state != "unused" && state != "consumed" {
				m.ExpectRollback()
			}
			pair, err := s.Rotate(context.Background(), raw)
			if state == "unused" {
				if err != nil || pair.Refresh == raw || pair.RefreshExpires != now.Add(time.Hour) {
					t.Fatal("renovação incorreta")
				}
			} else if err == nil || pair.Access != "" || pair.Refresh != "" {
				t.Fatal("falha emitiu token")
			}
			if err := m.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestRefreshMalformedAndLegacySecretsDoNotReachDatabase(t *testing.T) {
	s, m := testStore(t)
	for _, raw := range []string{"", "legacy.jwt.token", "v1." + testID + ".guess", "v1.00000000-0000-0000-0000-000000000000." + strings.Repeat("A", 43)} {
		if _, err := s.Rotate(context.Background(), raw); err != ErrDenied {
			t.Fatal("segredo inválido aceito")
		}
	}
	if err := m.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestExpiryIsCheckedAfterLocksRatherThanBeforeWaiting(t *testing.T) {
	s, m := testStore(t)
	before := time.Now()
	m.ExpectBegin()
	m.ExpectQuery(`SELECT tenant_id, user_id FROM online_sessions WHERE id=\$1`).WithArgs(testID).WillReturnRows(sqlmock.NewRows([]string{"tenant", "user"}).AddRow(testTenant, testUser))
	accountLock(m)
	rotationLock(m, before, nil)
	identity(m)
	m.ExpectQuery(`SELECT clock_timestamp`).WillReturnRows(sqlmock.NewRows([]string{"now"}).AddRow(before.Add(2 * time.Hour)))
	m.ExpectRollback()
	if pair, err := s.Rotate(context.Background(), "v1."+testID+"."+strings.Repeat("A", 43)); err != ErrDenied || pair.Access != "" {
		t.Fatal("espera prolongou sessão expirada")
	}
	if err := m.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestRevocationIsScopedAndRollsBackWithAudit(t *testing.T) {
	for _, state := range []string{"logout", "foreign", "expired-actor", "audit-fails", "others"} {
		t.Run(state, func(t *testing.T) {
			s, m := testStore(t)
			now := time.Now()
			target := testID
			other := "44444444-4444-4444-8444-444444444444"
			others := state == "others"
			if state == "foreign" {
				target = other
			}
			if others {
				target = "00000000-0000-0000-0000-000000000000"
			}
			m.ExpectBegin()
			accountLock(m)
			rows := sqlmock.NewRows([]string{"id", "expires", "revoked"})
			expires := now.Add(time.Hour)
			if state == "expired-actor" {
				expires = now.Add(-time.Hour)
			}
			rows.AddRow(testID, expires, nil)
			if others {
				rows.AddRow(other, expires, nil)
			}
			m.ExpectQuery(`SELECT id, expires_at, revoked_at.*ORDER BY id FOR UPDATE`).WithArgs(testTenant, testUser, testID, others, target).WillReturnRows(rows)
			m.ExpectQuery(`SELECT clock_timestamp`).WillReturnRows(sqlmock.NewRows([]string{"now"}).AddRow(now))
			if state != "expired-actor" {
				identity(m)
			}
			if state == "foreign" || state == "expired-actor" {
				m.ExpectRollback()
			} else {
				id := testID
				event := "logout"
				if others {
					id = other
					event = "others_revoked"
				}
				m.ExpectExec(`UPDATE online_sessions SET revoked_at`).WithArgs(id, testTenant, testUser).WillReturnResult(sqlmock.NewResult(0, 1))
				a := expectAudit(m, event)
				if state == "audit-fails" {
					a.WillReturnError(errors.New("private"))
					m.ExpectRollback()
				} else {
					a.WillReturnResult(sqlmock.NewResult(0, 1))
					m.ExpectCommit()
				}
			}
			err := s.Revoke(context.Background(), Scope{Tenant: testTenant, User: testUser, Role: "owner", Session: testID}, target, others)
			if (err == nil) != (state == "logout" || state == "others") {
				t.Fatal("revogação incorreta")
			}
			if err := m.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}
