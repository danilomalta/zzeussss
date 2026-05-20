package b2blogistics

/*
STOCK MONITOR SERVICE
=====================
Este arquivo possui o objetivo estratégico de monitorar constantemente (ou via eventos/fila) 
os níveis de estoque dos comércios clientes do TitanSystem.

Regra de Negócio:
1. Deve se comunicar com o banco de dados (tabela de Products/Inventory).
2. Sempre que a quantidade de um produto essencial (ex: comodity, produto alvo) baixar de 
   um limiar ('ReorderPoint'), ele deve gerar um alerta interno.
3. Este alerta será capturado pelo Supplier Matching para notificar os produtores de forma proativa.
*/

// StockMonitor define os métodos obrigatórios para a monitoria de estoque.
type StockMonitor interface {
	CheckInventoryThresholds() error
	GenerateLowStockAlert(productID string, currentQty int) error
}

// ProductStockAlert representa o payload interno de alerta
type ProductStockAlert struct {
	ProductID        string
	CurrentQuantity  int
	Threshold        int
	StoreReferenceID string
}
