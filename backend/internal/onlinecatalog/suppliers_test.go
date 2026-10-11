package onlinecatalog

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestSupplierStrictInputContactsAndRevision(t *testing.T) {
	v := SupplierInput{OperationID: "11111111-1111-4111-8111-111111111111", Name: "Contato", Email: "Sales@example.com", Phone: "+55 (11) 1234-5678", Active: true, Reason: "Cadastro interno"}
	b, _ := json.Marshal(v)
	got, e := DecodeSupplier("application/json", b, false)
	if e != nil || got != v {
		t.Fatal(got, e)
	}
	for _, body := range []string{`{}`, string(b) + `{}`, strings.Replace(string(b), `"active":true`, `"active":null`, 1), strings.Replace(string(b), `"name":"Contato"`, `"name":"Contato","name":"Outro"`, 1), strings.Replace(string(b), `"email":"Sales@example.com"`, `"email":"Nome <Sales@example.com>"`, 1), strings.Replace(string(b), `"phone":"+55 (11) 1234-5678"`, `"phone":"ramal"`, 1), strings.Replace(string(b), `"name":"Contato"`, `"unknown":1,"name":"Contato"`, 1)} {
		if _, e = DecodeSupplier("application/json", []byte(body), false); e != ErrInput {
			t.Fatal(body, e)
		}
	}
	if _, e = DecodeSupplier("text/plain", b, false); e != ErrInput {
		t.Fatal(e)
	}
	v.ExpectedVersion = 1
	b, _ = json.Marshal(v)
	if _, e = DecodeSupplier("application/json", b, true); e != nil {
		t.Fatal(e)
	}
	if _, e = DecodeSupplier("application/json", b, false); e != ErrInput {
		t.Fatal(e)
	}
	now := time.Now().UTC()
	p := Supplier{ID: v.OperationID, Name: v.Name, Email: v.Email, Phone: v.Phone, Active: true, Version: 1, CreatorID: v.OperationID, LastActorID: v.OperationID, Reason: v.Reason, CreatedAt: now, UpdatedAt: now}
	receipt := SupplierReceipt{OperationID: v.OperationID, SupplierID: p.ID, ActorID: p.CreatorID, Kind: "create", After: p}
	if !validSupplierReceipt(receipt) {
		t.Fatal("create")
	}
	receipt.Kind = "update"
	receipt.Before = &p
	receipt.After.Version = 2
	receipt.After.Active = false
	if !validSupplierReceipt(receipt) {
		t.Fatal("update")
	}
	receipt.After.CreatorID = "22222222-2222-4222-8222-222222222222"
	if validSupplierReceipt(receipt) {
		t.Fatal("creator changed")
	}
}
func TestSupplierQueryBoundariesAndStoreGuards(t *testing.T) {
	for _, q := range []SupplierQuery{{Active: "", Limit: 50}, {Active: "TRUE", Limit: 50}, {Active: "all", Limit: 101}, {Active: "all", Limit: 1, Offset: -1}, {Active: "all", Limit: 1, Query: "x\n"}} {
		if ValidateSupplierQuery(q) != ErrInput {
			t.Fatal(q)
		}
	}
	if ValidateSupplierQuery(SupplierQuery{Query: "%_\\", Active: "false", Limit: 100, Offset: 1000000000}) != nil {
		t.Fatal("literal")
	}
	a := Actor{}
	s := Store{}
	if _, e := s.CreateSupplier(context.Background(), a, SupplierInput{}); e != ErrInput {
		t.Fatal(e)
	}
	if _, e := s.Supplier(context.Background(), a, "bad"); e != ErrInput {
		t.Fatal(e)
	}
	if MigrateSuppliers(context.Background(), nil) != ErrUnavailable || CheckSuppliers(context.Background(), nil) != ErrUnavailable {
		t.Fatal("nil db")
	}
}
