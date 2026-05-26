package domain

import (
	"gorm.io/gorm"
)

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
