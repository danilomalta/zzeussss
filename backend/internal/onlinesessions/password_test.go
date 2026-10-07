package onlinesessions

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"github.com/DATA-DOG/go-sqlmock"
	"golang.org/x/crypto/bcrypt"
	"strings"
	"testing"
	"time"
)

const currentPassword = "old-test-password"
const nextPassword = "new-test-password"

type passwordHashArgument struct{}

func (passwordHashArgument) Match(v driver.Value) bool {
	s, ok := v.(string)
	return ok && bcrypt.CompareHashAndPassword([]byte(s), []byte(nextPassword)) == nil
}
func accountLock(m sqlmock.Sqlmock) {
	m.ExpectExec(`SELECT pg_advisory_xact_lock\(hashtextextended`).WithArgs(testTenant + ":" + testUser).WillReturnResult(sqlmock.NewResult(0, 1))
}

func TestPasswordChangesHashAllSessionsAndAuditAtomically(t *testing.T) {
	oldHash, e := bcrypt.GenerateFromPassword([]byte(currentPassword), bcrypt.MinCost)
	if e != nil {
		t.Fatal(e)
	}
	other := "44444444-4444-4444-8444-444444444444"
	for _, fault := range []string{"none", "wrong-password", "foreign-identity", "absent-session", "revoked-session", "expired-session", "wrong-role", "write", "zero-write", "revoke", "audit", "zero-audit", "commit"} {
		t.Run(fault, func(t *testing.T) {
			s, m := testStore(t)
			now := time.Now()
			m.ExpectBegin()
			accountLock(m)
			identityRows := sqlmock.NewRows([]string{"password_hash"})
			if fault != "foreign-identity" {
				identityRows.AddRow(string(oldHash))
			}
			m.ExpectQuery(`SELECT u.password_hash.*FOR UPDATE OF u FOR SHARE OF t`).WithArgs(testUser, testTenant, "owner").WillReturnRows(identityRows)
			deny := fault == "foreign-identity" || fault == "absent-session" || fault == "revoked-session" || fault == "expired-session" || fault == "wrong-role" || fault == "wrong-password"
			if fault != "foreign-identity" {
				rows := sqlmock.NewRows([]string{"id", "role", "expires", "revoked"})
				end := now.Add(time.Hour)
				var revoked any
				role := "owner"
				if fault == "expired-session" {
					end = now.Add(-time.Minute)
				}
				if fault == "revoked-session" {
					revoked = now
				}
				if fault == "wrong-role" {
					role = "employee"
				}
				if fault != "absent-session" {
					rows.AddRow(testID, role, end, revoked)
				}
				rows.AddRow(other, "owner", now.Add(time.Hour), nil)
				m.ExpectQuery(`SELECT id, role, expires_at, revoked_at.*ORDER BY id FOR UPDATE`).WithArgs(testTenant, testUser).WillReturnRows(rows)
				m.ExpectQuery(`SELECT clock_timestamp`).WillReturnRows(sqlmock.NewRows([]string{"now"}).AddRow(now))
				if !deny {
					m.ExpectQuery(`SELECT clock_timestamp`).WillReturnRows(sqlmock.NewRows([]string{"now"}).AddRow(now))
					update := m.ExpectExec(`UPDATE users SET password_hash`).WithArgs(passwordHashArgument{}, testUser, testTenant, string(oldHash))
					if fault == "write" {
						update.WillReturnError(errors.New("private"))
					} else if fault == "zero-write" {
						update.WillReturnResult(sqlmock.NewResult(0, 0))
					} else {
						update.WillReturnResult(sqlmock.NewResult(0, 1))
						for _, id := range []string{testID, other} {
							r := m.ExpectExec(`UPDATE online_sessions SET revoked_at`).WithArgs(id, testTenant, testUser)
							if fault == "revoke" {
								r.WillReturnError(errors.New("private"))
								break
							}
							r.WillReturnResult(sqlmock.NewResult(0, 1))
							expectAudit(m, "revoked").WillReturnResult(sqlmock.NewResult(0, 1))
						}
						if fault != "revoke" {
							a := m.ExpectExec(`INSERT INTO online_password_changes`).WithArgs(sqlmock.AnyArg(), testTenant, testUser, testID, 2)
							if fault == "audit" {
								a.WillReturnError(errors.New("private"))
							} else if fault == "zero-audit" {
								a.WillReturnResult(sqlmock.NewResult(0, 0))
							} else {
								a.WillReturnResult(sqlmock.NewResult(0, 1))
							}
						}
					}
				}
			}
			if fault == "none" {
				m.ExpectCommit()
			} else if fault == "commit" {
				m.ExpectCommit().WillReturnError(errors.New("uncertain"))
			} else {
				m.ExpectRollback()
			}
			password := currentPassword
			if fault == "wrong-password" {
				password = "wrong-test-password"
			}
			err := s.ChangePassword(context.Background(), Scope{User: testUser, Tenant: testTenant, Role: "owner", Session: testID}, password, nextPassword)
			if fault == "none" {
				if err != nil {
					t.Fatal(err)
				}
			} else if deny {
				if err != ErrDenied {
					t.Fatalf("expected denied: %v", err)
				}
			} else if err != ErrUnavailable {
				t.Fatalf("expected unavailable: %v", err)
			}
			if e := m.ExpectationsWereMet(); e != nil {
				t.Fatal(e)
			}
		})
	}
}

func TestPasswordPolicyStopsBeforeDatabase(t *testing.T) {
	s, m := testStore(t)
	for _, next := range []string{"short", strings.Repeat("x", 73), " leading-space", "trailing-space ", currentPassword, string([]byte{0xff})} {
		if err := s.ChangePassword(context.Background(), Scope{Session: testID}, currentPassword, next); err != ErrPasswordPolicy {
			t.Fatalf("policy: %v", err)
		}
	}
	if err := s.ChangePassword(context.Background(), Scope{Session: testID}, strings.Repeat("x", 73), nextPassword); err != ErrDenied {
		t.Fatal(err)
	}
	if err := s.ChangePassword(context.Background(), Scope{}, currentPassword, nextPassword); err != ErrDenied {
		t.Fatal(err)
	}
	if e := m.ExpectationsWereMet(); e != nil {
		t.Fatal(e)
	}
}

func TestPasswordAccountLockFailureDoesNotTouchIdentity(t *testing.T) {
	s, m := testStore(t)
	m.ExpectBegin()
	m.ExpectExec(`SELECT pg_advisory_xact_lock`).WillReturnError(sql.ErrConnDone)
	m.ExpectRollback()
	if err := s.ChangePassword(context.Background(), Scope{Session: testID}, currentPassword, nextPassword); err != ErrUnavailable {
		t.Fatal(err)
	}
	if e := m.ExpectationsWereMet(); e != nil {
		t.Fatal(e)
	}
}
