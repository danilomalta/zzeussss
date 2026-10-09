package usecase

import (
	"database/sql/driver"
	"errors"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"titansystem-backend/internal/core/database"
)

const discountProductQuery = `SELECT .* FROM "products" WHERE \(tenant_id = \$1 AND ativo = \$2\).*ORDER BY id ASC LIMIT \$3`
const pendingInsertQuery = `INSERT INTO "discount_suggestions" .*ON CONFLICT \("tenant_id","product_id"\) WHERE status = 'PENDING' AND deleted_at IS NULL DO NOTHING RETURNING "id"`

func discountMock(t *testing.T) sqlmock.Sqlmock {
	t.Helper()
	conn, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	db, err := gorm.Open(postgres.New(postgres.Config{Conn: conn}), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	previous := database.DB
	database.DB = db
	t.Cleanup(func() { database.DB = previous; conn.Close() })
	return mock
}

func discountProducts(count int, tenant string) *sqlmock.Rows {
	rows := sqlmock.NewRows([]string{"id", "tenant_id", "ativo", "estoque", "demanda_media_diaria"})
	for i := 1; i <= count; i++ {
		rows.AddRow(i, tenant, true, 100, 0.1)
	}
	return rows
}

func discountInsertArgs(productID int) []driver.Value {
	args := make([]driver.Value, 11)
	for i := range args {
		args[i] = sqlmock.AnyArg()
	}
	args[3] = "company-a"
	args[4] = productID
	args[5] = float64(15)
	args[8] = "PRODUTO_PARADO"
	args[9] = "PENDING"
	args[10] = ""
	return args
}

func TestDiscountEngineRollbackNeverReturnsPartialSuggestions(t *testing.T) {
	for _, scenario := range []string{"query", "second-insert", "commit"} {
		t.Run(scenario, func(t *testing.T) {
			mock := discountMock(t)
			mock.ExpectBegin()
			query := mock.ExpectQuery(discountProductQuery).WithArgs("company-a", true, 1001)
			if scenario == "query" {
				query.WillReturnError(errors.New("private query"))
				mock.ExpectRollback()
			} else {
				query.WillReturnRows(discountProducts(2, "company-a"))
				mock.ExpectQuery(pendingInsertQuery).WithArgs(discountInsertArgs(1)...).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(10))
				second := mock.ExpectQuery(pendingInsertQuery).WithArgs(discountInsertArgs(2)...)
				if scenario == "second-insert" {
					second.WillReturnError(errors.New("private constraint"))
					mock.ExpectRollback()
				} else {
					second.WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(11))
					mock.ExpectCommit().WillReturnError(errors.New("lost commit acknowledgement"))
				}
			}
			suggestions, err := RunDiscountEngine("company-a")
			if err == nil || suggestions != nil {
				t.Fatal("partial result exposed after failure")
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestDiscountEnginePendingConflictIsNotCountedAsCreated(t *testing.T) {
	mock := discountMock(t)
	mock.ExpectBegin()
	mock.ExpectQuery(discountProductQuery).WithArgs("company-a", true, 1001).WillReturnRows(discountProducts(2, "company-a"))
	mock.ExpectQuery(pendingInsertQuery).WithArgs(discountInsertArgs(1)...).WillReturnRows(sqlmock.NewRows([]string{"id"}))
	mock.ExpectQuery(pendingInsertQuery).WithArgs(discountInsertArgs(2)...).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(11))
	mock.ExpectCommit()
	suggestions, err := RunDiscountEngine("company-a")
	if err != nil || len(suggestions) != 1 || suggestions[0].ID != 11 || suggestions[0].ProductID != 2 || suggestions[0].TenantID != "company-a" {
		t.Fatalf("wrong created count: %v %v", suggestions, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestDiscountEngineRejectsLargeCatalogAndForeignRowsBeforeWriting(t *testing.T) {
	for _, tc := range []struct {
		count    int
		tenant   string
		tooLarge bool
	}{{1001, "company-a", true}, {1, "company-b", false}} {
		mock := discountMock(t)
		mock.ExpectBegin()
		mock.ExpectQuery(discountProductQuery).WithArgs("company-a", true, 1001).WillReturnRows(discountProducts(tc.count, tc.tenant))
		mock.ExpectRollback()
		result, err := RunDiscountEngine("company-a")
		if err == nil || result != nil || (tc.tooLarge && !errors.Is(err, ErrDiscountAnalysisTooLarge)) {
			t.Fatal("invalid catalog accepted")
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatal(err)
		}
	}
}

func TestDiscountEngineAcceptsExactLimitWithoutSilentTruncation(t *testing.T) {
	mock := discountMock(t)
	mock.ExpectBegin()
	rows := sqlmock.NewRows([]string{"id", "tenant_id", "ativo", "estoque", "demanda_media_diaria"})
	for i := 1; i <= 1000; i++ {
		rows.AddRow(i, "company-a", true, 0, 0)
	}
	mock.ExpectQuery(discountProductQuery).WithArgs("company-a", true, 1001).WillReturnRows(rows)
	mock.ExpectCommit()
	result, err := RunDiscountEngine("company-a")
	if err != nil || result == nil || len(result) != 0 {
		t.Fatal("exact limit refused or false suggestions generated")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
