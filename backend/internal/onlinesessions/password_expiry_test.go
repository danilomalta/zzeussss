package onlinesessions

import (
	"context"
	"github.com/DATA-DOG/go-sqlmock"
	"golang.org/x/crypto/bcrypt"
	"testing"
	"time"
)

func TestPasswordExpiryDuringCredentialVerificationPreventsWrite(t *testing.T) {
	s, m := testStore(t)
	hash, e := bcrypt.GenerateFromPassword([]byte(currentPassword), bcrypt.MinCost)
	if e != nil {
		t.Fatal(e)
	}
	now := time.Now()
	m.ExpectBegin()
	accountLock(m)
	m.ExpectQuery(`SELECT u.password_hash`).WithArgs(testUser, testTenant, "owner").WillReturnRows(sqlmock.NewRows([]string{"hash"}).AddRow(string(hash)))
	m.ExpectQuery(`SELECT id, role, expires_at, revoked_at`).WithArgs(testTenant, testUser).WillReturnRows(sqlmock.NewRows([]string{"id", "role", "expires", "revoked"}).AddRow(testID, "owner", now.Add(time.Minute), nil))
	m.ExpectQuery(`SELECT clock_timestamp`).WillReturnRows(sqlmock.NewRows([]string{"now"}).AddRow(now))
	m.ExpectQuery(`SELECT clock_timestamp`).WillReturnRows(sqlmock.NewRows([]string{"now"}).AddRow(now.Add(2 * time.Minute)))
	m.ExpectRollback()
	if err := s.ChangePassword(context.Background(), Scope{User: testUser, Tenant: testTenant, Role: "owner", Session: testID}, currentPassword, nextPassword); err != ErrDenied {
		t.Fatal(err)
	}
	if e = m.ExpectationsWereMet(); e != nil {
		t.Fatal(e)
	}
}
