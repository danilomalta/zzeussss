// Package modules define as dependencias funcionais dos modulos Titan.
// Nao emite licencas, nao autentica usuarios e nao consulta dados de empresas.
package modules

import (
	"errors"
	"fmt"
	"sort"
)

type ID string

const (
	Core       ID = "core"
	Inventory  ID = "inventory"
	POS        ID = "pos"
	Orders     ID = "orders"
	Logistics  ID = "logistics"
	Finance    ID = "finance"
	Fiscal     ID = "fiscal"
	Accounting ID = "accounting"
	Staff      ID = "staff"
	Production ID = "production"
)

var (
	ErrUnknown     = errors.New("modulo desconhecido")
	ErrUnavailable = errors.New("modulo ou dependencia indisponivel")
)

// All retorna uma lista independente dos IDs reconhecidos.
func All() []ID {
	return []ID{Core, Inventory, POS, Orders, Logistics, Finance, Fiscal, Accounting, Staff, Production}
}

func dependencies(id ID) ([]ID, error) {
	switch id {
	case Core:
		return nil, nil
	case Inventory, Finance, Fiscal, Accounting, Staff:
		return []ID{Core}, nil
	case POS, Orders, Production:
		return []ID{Core, Inventory}, nil
	case Logistics:
		return []ID{Core, Orders}, nil
	default:
		return nil, fmt.Errorf("%w: %q", ErrUnknown, id)
	}
}

// Required calcula a selecao e suas dependencias, sem ativar ou contratar nada.
// O resultado e ordenado, sem duplicatas; a entrada nunca e alterada.
func Required(selection []ID) ([]ID, error) {
	seen := make(map[ID]bool)
	var visit func(ID) error
	visit = func(id ID) error {
		deps, err := dependencies(id)
		if err != nil {
			return err
		}
		if seen[id] {
			return nil
		}
		seen[id] = true
		for _, dep := range deps {
			if err := visit(dep); err != nil {
				return err
			}
		}
		return nil
	}
	for _, id := range selection {
		if err := visit(id); err != nil {
			return nil, err
		}
	}
	result := make([]ID, 0, len(seen))
	for id := range seen {
		result = append(result, id)
	}
	sort.Slice(result, func(i, j int) bool { return result[i] < result[j] })
	return result, nil
}

func activeSet(active []ID) (map[ID]bool, error) {
	set := make(map[ID]bool, len(active))
	for _, id := range active {
		if _, err := dependencies(id); err != nil {
			return nil, err
		}
		set[id] = true
	}
	return set, nil
}

// Validate rejeita uma configuracao cujas dependencias estejam ausentes.
// Nao verifica assinatura, vigencia, tenant ou permissao do usuario.
func Validate(active []ID) error {
	set, err := activeSet(active)
	if err != nil {
		return err
	}
	required, err := Required(active)
	if err != nil {
		return err
	}
	for _, id := range required {
		if !set[id] {
			return fmt.Errorf("%w: %s", ErrUnavailable, id)
		}
	}
	return nil
}

// Require verifica um modulo e suas dependencias contra uma configuracao
// confiavel, obtida pelo backend para a empresa da sessao. Nunca passar uma
// lista de modulos fornecida pelo navegador como prova de contratacao.
// Sucesso aqui nao substitui autorizacao de usuario, loja e aparelho.
func Require(module ID, active []ID) error {
	required, err := Required([]ID{module})
	if err != nil {
		return err
	}
	set, err := activeSet(active)
	if err != nil {
		return err
	}
	for _, id := range required {
		if !set[id] {
			return fmt.Errorf("%w: %s", ErrUnavailable, id)
		}
	}
	return nil
}
