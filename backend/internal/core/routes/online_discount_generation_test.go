package routes

import (
	"encoding/json"
	"errors"
	"io"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/gofiber/fiber/v2"
)

func TestOnlineDiscountGenerationEmptyLimitAndFailureResponses(t *testing.T) {
	t.Setenv("JWT_SECRET", "isolated-test-secret")
	for _, scenario := range []string{"empty", "too-large", "query-failure", "commit-failure"} {
		t.Run(scenario, func(t *testing.T) {
			mock := onlineMock(t)
			mock.ExpectQuery(`SELECT EXISTS`).WithArgs("operator", "company-a", "owner", "11111111-1111-4111-8111-111111111112").WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))
			mock.ExpectBegin()
			query := mock.ExpectQuery(`SELECT .* FROM "products" WHERE \(tenant_id = \$1 AND ativo = \$2\).*ORDER BY id ASC LIMIT \$3`).WithArgs("company-a", true, 1001)
			want := 200
			if scenario == "query-failure" {
				query.WillReturnError(errors.New("private-query-detail"))
				mock.ExpectRollback()
				want = 500
			} else {
				rows := sqlmock.NewRows([]string{"id", "tenant_id", "ativo"})
				if scenario == "too-large" {
					for i := 1; i <= 1001; i++ {
						rows.AddRow(i, "company-a", true)
					}
				}
				query.WillReturnRows(rows)
				switch scenario {
				case "too-large":
					mock.ExpectRollback()
					want = 409
				case "commit-failure":
					mock.ExpectCommit().WillReturnError(errors.New("private-commit-detail"))
					want = 500
				default:
					mock.ExpectCommit()
				}
			}
			app := fiber.New()
			Registrar(app)
			req := httptest.NewRequest("POST", "/api/v1/discounts/suggest", nil)
			req.Header.Set("Authorization", "Bearer "+onlineToken(t, "company-a", "owner"))
			resp, err := app.Test(req)
			if err != nil {
				t.Fatal(err)
			}
			defer resp.Body.Close()
			data, err := io.ReadAll(resp.Body)
			if err != nil {
				t.Fatal(err)
			}
			if resp.StatusCode != want || strings.Contains(string(data), "private-") {
				t.Fatalf("wrong result %d", resp.StatusCode)
			}
			var body map[string]interface{}
			if json.Unmarshal(data, &body) != nil {
				t.Fatal("invalid JSON")
			}
			if want == 200 {
				list, ok := body["sugestoes"].([]interface{})
				if !ok || len(list) != 0 || body["items_gerados"] != float64(0) || body["message"] != "Análise concluída." {
					t.Fatal("invalid empty response")
				}
			} else if _, ok := body["items_gerados"]; ok {
				t.Fatal("failure claims completed analysis")
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}
