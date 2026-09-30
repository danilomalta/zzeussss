package modules_test

import (
	"errors"
	"reflect"
	"testing"

	"titansystem-backend/internal/core/modules"
)

func TestSupplierCanSelectOrdersAndProductionWithoutPOS(t *testing.T) {
	active, err := modules.Required([]modules.ID{modules.Orders, modules.Production})
	if err != nil {
		t.Fatal(err)
	}
	expected := []modules.ID{modules.Core, modules.Inventory, modules.Orders, modules.Production}
	if !reflect.DeepEqual(active, expected) {
		t.Fatalf("selecao fornecedor: %v", active)
	}
	if err := modules.Validate(active); err != nil {
		t.Fatal(err)
	}
	if err := modules.Require(modules.POS, active); !errors.Is(err, modules.ErrUnavailable) {
		t.Fatalf("fornecedor recebeu PDV sem selecao: %v", err)
	}
}

func TestPOSCannotOperateWithoutSelectedModuleAndInventory(t *testing.T) {
	for _, active := range [][]modules.ID{
		nil,
		{modules.Core, modules.Inventory},
		{modules.Core, modules.POS},
	} {
		if err := modules.Require(modules.POS, active); !errors.Is(err, modules.ErrUnavailable) {
			t.Fatalf("PDV indevidamente disponivel com %v: %v", active, err)
		}
	}
	active := []modules.ID{modules.Core, modules.Inventory, modules.POS}
	if err := modules.Require(modules.POS, active); err != nil {
		t.Fatalf("PDV completo foi rejeitado: %v", err)
	}
}

func TestLogisticsNeedsOrdersAndInventoryTransitively(t *testing.T) {
	active, err := modules.Required([]modules.ID{modules.Logistics})
	if err != nil {
		t.Fatal(err)
	}
	expected := []modules.ID{modules.Core, modules.Inventory, modules.Logistics, modules.Orders}
	if !reflect.DeepEqual(active, expected) {
		t.Fatalf("dependencias de logistica: %v", active)
	}
	if err := modules.Validate([]modules.ID{modules.Core, modules.Logistics, modules.Orders}); !errors.Is(err, modules.ErrUnavailable) {
		t.Fatalf("configuracao incompleta aceita: %v", err)
	}
}

func TestStaffAndAccountingDoNotForcePOSSubscription(t *testing.T) {
	active, err := modules.Required([]modules.ID{modules.Staff, modules.Accounting})
	if err != nil {
		t.Fatal(err)
	}
	expected := []modules.ID{modules.Accounting, modules.Core, modules.Staff}
	if !reflect.DeepEqual(active, expected) {
		t.Fatalf("dependencias inesperadas: %v", active)
	}
	if err := modules.Require(modules.Staff, active); err != nil {
		t.Fatal(err)
	}
	if err := modules.Require(modules.Accounting, active); err != nil {
		t.Fatal(err)
	}
}

func TestUnknownModulesAreRejected(t *testing.T) {
	if _, err := modules.Required([]modules.ID{"unknown"}); !errors.Is(err, modules.ErrUnknown) {
		t.Fatalf("selecao desconhecida: %v", err)
	}
	if err := modules.Require("", []modules.ID{modules.Core}); !errors.Is(err, modules.ErrUnknown) {
		t.Fatalf("modulo vazio: %v", err)
	}
	if err := modules.Require(modules.Core, []modules.ID{modules.Core, "unknown"}); !errors.Is(err, modules.ErrUnknown) {
		t.Fatalf("configuracao desconhecida: %v", err)
	}
}

func TestSelectionIsDeterministicAndDoesNotChangeInput(t *testing.T) {
	selection := []modules.ID{modules.POS, modules.Inventory, modules.POS}
	original := append([]modules.ID(nil), selection...)
	active, err := modules.Required(selection)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(selection, original) {
		t.Fatal("selecao de entrada alterada")
	}
	expected := []modules.ID{modules.Core, modules.Inventory, modules.POS}
	if !reflect.DeepEqual(active, expected) {
		t.Fatalf("duplicatas ou ordem inesperada: %v", active)
	}
}
