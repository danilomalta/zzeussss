package domain

import (
	"time"

	"gorm.io/gorm"
)

// Sale representa uma venda registrada pelo PDV.
// Ela é a entidade central do motor de vendas e deve suportar idempotência e sincronização offline.
type Sale struct {
	gorm.Model

	// Identificador estável para evitar duplicidade em reenvios (Modo Apagão).
	IDExterno string `json:"id_externo" gorm:"uniqueIndex"`

	NumeroVenda string     `json:"numero_venda" gorm:"index"`
	Status      string     `json:"status" gorm:"index"` // Ex.: "aberta", "concluida", "cancelada"
	Total       float64    `json:"total"`
	Moeda       string     `json:"moeda"`
	VendidaEm   *time.Time `json:"vendida_em" gorm:"index"`

	Origem string `json:"origem" gorm:"index"` // Ex.: "computador", "mobile"

	Itens []SaleItem `json:"itens" gorm:"foreignKey:SaleID"`
}

// SaleItem representa um item dentro de uma venda.
type SaleItem struct {
	gorm.Model
	SaleID    uint    `json:"sale_id" gorm:"index"`
	ProductID uint    `json:"product_id" gorm:"index"`
	NomeItem  string  `json:"nome_item"`
	SKU       string  `json:"sku"`

	Quantidade int     `json:"quantidade"`
	PrecoUnit  float64 `json:"preco_unit"`
	Subtotal   float64 `json:"subtotal"`
}
