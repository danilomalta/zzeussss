package models

import (
	"time"

	"gorm.io/gorm"
)

// Product representa um item vendável no sistema de PDV.
type Product struct {
	gorm.Model
	Nome      string  `json:"nome"`
	Descricao string  `json:"descricao"`
	Preco     float64 `json:"preco"`
	SKU       string  `json:"sku" gorm:"uniqueIndex"`
	Estoque   int     `json:"estoque"`
	Ativo     bool    `json:"ativo" gorm:"default:true"`

	// Campos para controle inteligente de estoque (base para previsões e reabastecimento).
	EstoqueMinimo           int        `json:"estoque_minimo"`
	PontoReposicao          int        `json:"ponto_reposicao"`
	DemandaMediaDiaria      float64    `json:"demanda_media_diaria"`
	VariabilidadeDemanda    float64    `json:"variabilidade_demanda"`
	TempoReposicaoDias      int        `json:"tempo_reposicao_dias"`
	UltimaPrevisaoDemandaEm *time.Time `json:"ultima_previsao_demanda_em"`
	AlertaReposicaoAtivo    bool       `json:"alerta_reposicao_ativo" gorm:"default:false"`
}

// Order representa uma compra/pedido do cliente.
type Order struct {
	gorm.Model
	NumeroPedido string      `json:"numero_pedido" gorm:"uniqueIndex"`
	Total        float64     `json:"total"`
	Status       string      `json:"status"` // Ex.: "pendente", "concluido", "cancelado"
	Itens        []OrderItem `json:"itens" gorm:"foreignKey:OrderID"`
}

// OrderItem representa um item (linha) dentro de um pedido.
type OrderItem struct {
	gorm.Model
	OrderID    uint    `json:"order_id"`
	ProductID  uint    `json:"product_id"`
	Quantidade int     `json:"quantidade"`
	PrecoUnit  float64 `json:"preco_unit"`
}

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

