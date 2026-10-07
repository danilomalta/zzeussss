package onlinesessions

import (
	"context"
	"database/sql"
	"errors"
	"github.com/DATA-DOG/go-sqlmock"
	"golang.org/x/crypto/bcrypt"
	"strings"
	"testing"
	"time"
)

func recoveryFixture(t *testing.T) (string, string) {
	t.Helper()
	raw, e := refreshSecret(testID)
	if e != nil {
		t.Fatal("fixture unavailable")
	}
	hash, e := bcrypt.GenerateFromPassword([]byte(currentPassword), bcrypt.MinCost)
	if e != nil {
		t.Fatal("fixture hash unavailable")
	}
	return "rk1." + strings.TrimPrefix(raw, "v1."), string(hash)
}

func TestRecoveryRejectsInvalidKeyAndPasswordBeforeDatabase(t *testing.T) {
	raw, _ := recoveryFixture(t)
	s, m := testStore(t)
	for _, key := range []string{"", strings.Replace(raw, "rk1.", "v1.", 1), raw + "=", strings.ToUpper(raw), raw[:len(raw)-1]} {
		if s.Recover(context.Background(), key, nextPassword) != ErrDenied {
			t.Fatal("invalid key accepted")
		}
	}
	for _, p := range []string{"short", strings.Repeat("x", 73), " leading-space", "trailing-space ", string([]byte{255})} {
		if s.Recover(context.Background(), raw, p) != ErrPasswordPolicy {
			t.Fatal("invalid password accepted")
		}
	}
	if e := m.ExpectationsWereMet(); e != nil {
		t.Fatal(e)
	}
}

func TestRecoveryConsumesKeyHashRevocationsAndAuditTogether(t *testing.T) {
	for _, fault := range []string{"none", "absent", "expired-or-consumed", "credential-changed", "role-changed", "same-password", "consume", "zero-consume", "write", "revocation", "audit", "zero-audit", "commit"} {
		t.Run(fault, func(t *testing.T) {
			raw, hash := recoveryFixture(t)
			s, m := testStore(t)
			m.ExpectBegin()
			pre := sqlmock.NewRows([]string{"tenant", "user"})
			if fault != "absent" {
				pre.AddRow(testTenant, testUser)
			}
			m.ExpectQuery(`SELECT tenant_id,user_id FROM online_recovery_keys`).WithArgs(testID, tokenDigest(raw)).WillReturnRows(pre)
			denied := fault == "absent" || fault == "expired-or-consumed" || fault == "credential-changed" || fault == "role-changed"
			if fault != "absent" {
				accountLock(m)
				m.ExpectQuery(`SELECT u.password_hash,u.role.*FOR UPDATE OF u`).WithArgs(testUser, testTenant).WillReturnRows(sqlmock.NewRows([]string{"hash", "role"}).AddRow(hash, "owner"))
				key := sqlmock.NewRows([]string{"credential", "role"})
				credential, role := tokenDigest(hash), "owner"
				if fault == "credential-changed" {
					credential = "changed"
				}
				if fault == "role-changed" {
					role = "manager"
				}
				if fault != "expired-or-consumed" {
					key.AddRow(credential, role)
				}
				m.ExpectQuery(`SELECT credential_digest,role FROM online_recovery_keys.*FOR UPDATE`).WithArgs(testID, testTenant, testUser, tokenDigest(raw)).WillReturnRows(key)
				if !denied && fault != "same-password" {
					consume := m.ExpectExec(`UPDATE online_recovery_keys SET consumed_at`).WithArgs(testID, testTenant, testUser, tokenDigest(raw))
					if fault == "consume" {
						consume.WillReturnError(sql.ErrConnDone)
					} else {
						n := int64(1)
						if fault == "zero-consume" {
							n = 0
						}
						consume.WillReturnResult(sqlmock.NewResult(0, n))
						if n != 0 {
							write := m.ExpectExec(`UPDATE users SET password_hash`).WithArgs(passwordHashArgument{}, testUser, testTenant, hash)
							if fault == "write" {
								write.WillReturnError(sql.ErrConnDone)
							} else {
								write.WillReturnResult(sqlmock.NewResult(0, 1))
								m.ExpectQuery(`SELECT id FROM online_sessions.*ORDER BY id FOR UPDATE`).WithArgs(testTenant, testUser).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(testID))
								rev := m.ExpectExec(`UPDATE online_sessions SET revoked_at`).WithArgs(testID, testTenant, testUser)
								if fault == "revocation" {
									rev.WillReturnError(sql.ErrConnDone)
								} else {
									rev.WillReturnResult(sqlmock.NewResult(0, 1))
									expectAudit(m, "revoked").WillReturnResult(sqlmock.NewResult(0, 1))
									a := m.ExpectExec(`INSERT INTO online_recovery_audit`).WithArgs(sqlmock.AnyArg(), testTenant, testUser, testID, "recovered", 1)
									if fault == "audit" {
										a.WillReturnError(errors.New("private"))
									} else {
										n := int64(1)
										if fault == "zero-audit" {
											n = 0
										}
										a.WillReturnResult(sqlmock.NewResult(0, n))
									}
								}
							}
						}
					}
				}
			}
			if fault == "none" {
				m.ExpectCommit()
			} else if fault == "commit" {
				m.ExpectCommit().WillReturnError(sql.ErrConnDone)
			} else {
				m.ExpectRollback()
			}
			next := nextPassword
			if fault == "same-password" {
				next = currentPassword
			}
			e := s.Recover(context.Background(), raw, next)
			want := ErrUnavailable
			if denied {
				want = ErrDenied
			}
			if fault == "same-password" {
				want = ErrPasswordPolicy
			}
			if fault == "none" {
				want = nil
			}
			if e != want {
				t.Fatalf("unexpected outcome: %v", e)
			}
			if e = m.ExpectationsWereMet(); e != nil {
				t.Fatal(e)
			}
		})
	}
}

func TestRecoveryIssuanceRequiresPasswordActiveSessionAndDurableCooldown(t *testing.T) {
	for _, fault := range []string{"none", "password", "expired", "rate", "insert", "audit", "commit"} {
		t.Run(fault, func(t *testing.T) {
			_, hash := recoveryFixture(t)
			s, m := testStore(t)
			now := time.Now()
			m.ExpectBegin()
			accountLock(m)
			m.ExpectQuery(`SELECT u.password_hash.*FOR UPDATE OF u`).WithArgs(testUser, testTenant, "owner").WillReturnRows(sqlmock.NewRows([]string{"hash"}).AddRow(hash))
			end := now.Add(time.Hour)
			if fault == "expired" {
				end = now.Add(-time.Hour)
			}
			m.ExpectQuery(`SELECT expires_at FROM online_sessions.*FOR UPDATE`).WithArgs(testID, testTenant, testUser, "owner").WillReturnRows(sqlmock.NewRows([]string{"end"}).AddRow(end))
			if fault != "password" {
				m.ExpectQuery(`SELECT clock_timestamp`).WillReturnRows(sqlmock.NewRows([]string{"now"}).AddRow(now))
				if fault != "expired" {
					m.ExpectQuery(`SELECT EXISTS.*online_recovery_keys`).WithArgs(testTenant, testUser).WillReturnRows(sqlmock.NewRows([]string{"recent"}).AddRow(fault == "rate"))
					if fault != "rate" {
						insert := m.ExpectQuery(`INSERT INTO online_recovery_keys.*WHERE EXISTS.*ON CONFLICT.*RETURNING expires_at`).WithArgs(testTenant, testUser, sqlmock.AnyArg(), sqlmock.AnyArg(), tokenDigest(hash), "owner", testID)
						if fault == "insert" {
							insert.WillReturnError(sql.ErrConnDone)
						} else {
							insert.WillReturnRows(sqlmock.NewRows([]string{"expiry"}).AddRow(now.Add(30 * 24 * time.Hour)))
							a := m.ExpectExec(`INSERT INTO online_recovery_audit`).WithArgs(sqlmock.AnyArg(), testTenant, testUser, sqlmock.AnyArg(), "issued", 0)
							if fault == "audit" {
								a.WillReturnError(sql.ErrConnDone)
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
				m.ExpectCommit().WillReturnError(sql.ErrConnDone)
			} else {
				m.ExpectRollback()
			}
			pw := currentPassword
			if fault == "password" {
				pw = "wrong-password"
			}
			key, e := s.IssueRecovery(context.Background(), Scope{Tenant: testTenant, User: testUser, Role: "owner", Session: testID}, pw)
			want := ErrUnavailable
			if fault == "none" {
				want = nil
			}
			if fault == "password" || fault == "expired" {
				want = ErrDenied
			}
			if fault == "rate" {
				want = ErrRecoveryRate
			}
			if e != want {
				t.Fatalf("unexpected issue outcome: %v", e)
			}
			if e == nil {
				if _, e = ParseRecovery(key.Key); e != nil {
					t.Fatal("invalid issued key")
				}
			} else if key.Key != "" {
				t.Fatal("key exposed after failure")
			}
			if e = m.ExpectationsWereMet(); e != nil {
				t.Fatal(e)
			}
		})
	}
}
