package onlinecatalog

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/DATA-DOG/go-sqlmock"
	"reflect"
	"strings"
	"testing"
	"time"
)

func planningFixture43() (StockPlan, StockSettingsInput, StockSettingsReceipt, string) {
	p := StockPlan{Product: Product{ID: 1, Name: "Item", SKU: "item", PriceCents: 1000, Active: true, Version: 1}, StockUnits: 4, Settings: StockSettings{DailyDemand: "0.0000"}}
	v := StockSettingsInput{testOp, 1, 5, 8, 2, "1.0000", "Planejamento conferido"}
	r := StockSettingsReceipt{testOp, 1, fixtureActor.User, v.Reason, p, proposedStockPlan(p, v), time.Date(2026, 10, 11, 12, 0, 0, 0, time.UTC)}
	return p, v, r, stockSettingsHash(1, v)
}
func planRows43(p StockPlan) *sqlmock.Rows {
	var f interface{}
	if p.Settings.ForecastAt != nil {
		f = *p.Settings.ForecastAt
	}
	return sqlmock.NewRows([]string{"id", "name", "description", "sku", "price", "active", "version", "stock", "minimum", "reorder", "lead", "demand", "forecast"}).AddRow(p.Product.ID, p.Product.Name, p.Product.Description, p.Product.SKU, "10.00", p.Product.Active, p.Product.Version, p.StockUnits, p.Settings.MinimumUnits, p.Settings.ReorderPointUnits, p.Settings.LeadTimeDays, p.Settings.DailyDemand, f)
}
func receiptRows43(r StockSettingsReceipt, h string, empty bool) *sqlmock.Rows {
	rows := sqlmock.NewRows([]string{"op", "product", "actor", "reason", "before", "after", "created", "hash"})
	if empty {
		return rows
	}
	b, _ := json.Marshal(r.Before)
	a, _ := json.Marshal(r.After)
	return rows.AddRow(r.OperationID, r.ProductID, r.ActorID, r.Reason, b, a, r.CreatedAt, h)
}
func TestStockSettingsStrictInputExactDemandAndPermissions(t *testing.T) {
	_, v, _, _ := planningFixture43()
	b, _ := json.Marshal(v)
	if got, e := DecodeStockSettings("application/json; charset=utf-8", b); e != nil || got != v {
		t.Fatal(got, e)
	}
	for _, raw := range []string{"{}", string(b) + "{}", strings.Replace(string(b), `"minimum_units":5`, `"minimum_units":null`, 1), strings.Replace(string(b), `"minimum_units":5`, `"minimum_units":5,"minimum_units":5`, 1), strings.Replace(string(b), `"minimum_units":5`, `"minimum_units":-1`, 1), strings.Replace(string(b), `"minimum_units":5`, `"minimum_units":5.0`, 1), strings.Replace(string(b), `"lead_time_days":2`, `"lead_time_days":366`, 1), strings.Replace(string(b), `"daily_demand":"1.0000"`, `"daily_demand":1`, 1), strings.Replace(string(b), `"daily_demand":"1.0000"`, `"daily_demand":"1.00001"`, 1), strings.Replace(string(b), `"reason":"Planejamento conferido"`, `"reason":"Planejamento conferido","tenant_id":"x"`, 1)} {
		if _, e := DecodeStockSettings("application/json", []byte(raw)); e != ErrInput {
			t.Fatal("invalid input accepted", e)
		}
	}
	for _, d := range []string{"-1", "1e2", "NaN", "100000000", "", " 1"} {
		v.DailyDemand = d
		if _, e := normalizeStockSettings(v); e != ErrInput {
			t.Fatal(d, e)
		}
	}
	v.DailyDemand = "99999999.9999"
	v.MinimumUnits = MaxStockUnits
	v.ExpectedVersion = MaxVersion - 1
	if _, e := normalizeStockSettings(v); e != nil {
		t.Fatal(e)
	}
	v.ExpectedVersion = MaxVersion
	if _, e := normalizeStockSettings(v); e != ErrInput {
		t.Fatal(e)
	}
	_, v, _, _ = planningFixture43()
	v.DailyDemand = "1"
	v.Reason = " Planejamento conferido "
	normalized, e := normalizeStockSettings(v)
	if e != nil || normalized.DailyDemand != "1.0000" || stockSettingsHash(1, normalized) != stockSettingsHash(1, StockSettingsInput{testOp, 1, 5, 8, 2, "1.0000", "Planejamento conferido"}) {
		t.Fatal(normalized, e)
	}
	for _, role := range []string{"stock", "cashier", "accountant", "employee"} {
		a := fixtureActor
		a.Role = role
		s := Store{}
		if _, e := s.SetStockSettings(context.Background(), a, 1, normalized); e != ErrDenied {
			t.Fatal(role, e)
		}
		if _, e := s.ProductStockSettings(context.Background(), a, 1); e != ErrDenied {
			t.Fatal(role, e)
		}
		if _, e := s.PreviewStockSettings(context.Background(), a, 1, normalized); e != ErrDenied {
			t.Fatal(role, e)
		}
		if _, e := s.StockSettingsOperation(context.Background(), a, testOp); e != ErrDenied {
			t.Fatal(role, e)
		}
		if _, e := s.StockSettingsHistory(context.Background(), a, 1, 50, 0); e != ErrDenied {
			t.Fatal(role, e)
		}
	}
}
func TestStockSettingsPreviewNoWritesAndForecastInvalidation(t *testing.T) {
	p, v, _, _ := planningFixture43()
	now := time.Now().UTC()
	p.Settings.ForecastAt = &now
	s, m := qualityFixture(t)
	qualityLive(m)
	m.ExpectQuery("SELECT id,nome").WillReturnRows(planRows43(p))
	m.ExpectCommit()
	got, e := s.PreviewStockSettings(context.Background(), fixtureActor, 1, v)
	if e != nil || got.After.StockUnits != 4 || got.After.Settings.ForecastAt != nil || got.After.Product.Version != 2 || got.Before.Settings.ForecastAt == nil || got.Metrics.MissingToMinimum != 1 || got.Metrics.CoverageDays == nil || *got.Metrics.CoverageDays != "4.0000" || !got.Metrics.LowCoverage {
		t.Fatal(got, e)
	}
	if e = m.ExpectationsWereMet(); e != nil {
		t.Fatal(e)
	}
	v.DailyDemand = "0.0000"
	after := proposedStockPlan(p, v)
	if !reflect.DeepEqual(after.Settings.ForecastAt, p.Settings.ForecastAt) {
		t.Fatal("unchanged demand erased forecast")
	}
}
func TestStockSettingsSaveAtomicReadbackAndOutboxRollback(t *testing.T) {
	for _, outcome := range []string{"success", "failure", "ignored", "ignored-update", "changed-readback"} {
		s, m := qualityFixture(t)
		p, v, r, h := planningFixture43()
		qualityLive(m)
		m.ExpectExec("SELECT pg_advisory_xact_lock").WithArgs(fixtureActor.Tenant + ":stock-settings:" + testOp).WillReturnResult(sqlmock.NewResult(0, 1))
		m.ExpectQuery("SELECT operation_id::text").WillReturnRows(receiptRows43(r, h, true))
		m.ExpectQuery("SELECT id,nome.*FOR UPDATE").WillReturnRows(planRows43(p))
		n := int64(1)
		if outcome == "ignored-update" {
			n = 0
		}
		m.ExpectExec("UPDATE products SET estoque_minimo").WithArgs(fixtureActor.Tenant, int64(1), int64(5), int64(8), int64(2), "1.0000", int64(1)).WillReturnResult(sqlmock.NewResult(0, n))
		if outcome != "ignored-update" {
			after := r.After
			if outcome == "changed-readback" {
				after.StockUnits++
			}
			m.ExpectQuery("SELECT id,nome").WillReturnRows(planRows43(after))
			if outcome != "changed-readback" {
				m.ExpectExec("INSERT INTO online_stock_setting_operations").WillReturnResult(sqlmock.NewResult(0, 1))
				m.ExpectQuery("SELECT operation_id::text").WillReturnRows(receiptRows43(r, h, false))
				if outcome == "failure" {
					m.ExpectExec("INSERT INTO online_stock_setting_outbox").WillReturnError(errors.New("private"))
				} else {
					count := int64(1)
					if outcome == "ignored" {
						count = 0
					}
					m.ExpectExec("INSERT INTO online_stock_setting_outbox").WillReturnResult(sqlmock.NewResult(0, count))
				}
			}
		}
		if outcome == "success" {
			m.ExpectCommit()
		} else {
			m.ExpectRollback()
		}
		got, e := s.SetStockSettings(context.Background(), fixtureActor, 1, v)
		if outcome == "success" && (e != nil || !reflect.DeepEqual(got, r)) || outcome != "success" && e != ErrUnavailable {
			t.Fatal(outcome, got, e)
		}
		if e = m.ExpectationsWereMet(); e != nil {
			t.Fatal(e)
		}
	}
}
func TestStockSettingsReplayConflictStaleAndForgedSnapshot(t *testing.T) {
	for _, kind := range []string{"replay", "payload", "actor", "forged"} {
		s, m := qualityFixture(t)
		_, v, r, h := planningFixture43()
		qualityLive(m)
		if kind == "payload" {
			v.MinimumUnits++
		}
		if kind == "actor" {
			r.ActorID = "11111111-1111-4111-8111-111111111119"
		}
		if kind == "forged" {
			r.After.StockUnits++
		}
		m.ExpectExec("SELECT pg_advisory_xact_lock").WillReturnResult(sqlmock.NewResult(0, 1))
		m.ExpectQuery("SELECT operation_id::text").WillReturnRows(receiptRows43(r, h, false))
		if kind == "replay" {
			m.ExpectCommit()
		} else {
			m.ExpectRollback()
		}
		_, e := s.SetStockSettings(context.Background(), fixtureActor, 1, v)
		want := ErrConflict
		if kind == "replay" {
			want = nil
		}
		if kind == "forged" {
			want = ErrUnavailable
		}
		if e != want {
			t.Fatal(kind, e)
		}
		if e = m.ExpectationsWereMet(); e != nil {
			t.Fatal(e)
		}
	}
	s, m := qualityFixture(t)
	p, v, r, h := planningFixture43()
	p.Product.Version = 2
	qualityLive(m)
	m.ExpectExec("SELECT pg_advisory_xact_lock").WillReturnResult(sqlmock.NewResult(0, 1))
	m.ExpectQuery("SELECT operation_id::text").WillReturnRows(receiptRows43(r, h, true))
	m.ExpectQuery("SELECT id,nome.*FOR UPDATE").WillReturnRows(planRows43(p))
	m.ExpectRollback()
	if _, e := s.SetStockSettings(context.Background(), fixtureActor, 1, v); e != ErrConflict {
		t.Fatal(e)
	}
	if e := m.ExpectationsWereMet(); e != nil {
		t.Fatal(e)
	}
}
