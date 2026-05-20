package b2blogistics

/*
SUPPLIER MATCHING SERVICE
=========================
Este arquivo concentra a lógica estratégica de conexão B2B. Ele atua como um "Tinder" 
entre o mercado (que teve o alerta gerado pelo stock_monitor) e o produtor/fornecedor.

Regra de Negócio:
1. Recebe os alertas gerados pelo Stock Monitor.
2. Com base na geolocalização e preço praticado, busca na base de Fornecedores parceiros (Titan B2B Network) os que podem suprir aquela demanda.
3. Dispara uma notificação (push/email/fila) para os produtores correspondentes informando da oportunidade de venda.
*/

// SupplierMatcher descreve a assinatura para o pareamento de fornecedores.
type SupplierMatcher interface {
	FindBestSuppliers(alert ProductStockAlert) ([]SupplierCandidate, error)
	NotifySuppliers(candidates []SupplierCandidate, alert ProductStockAlert) error
}

// SupplierCandidate é o DTO para representar um possível fornecedor para um alerta de estoque.
type SupplierCandidate struct {
	SupplierID string
	Score      float64 // Score de match baseado em distância e preço
	DistanceKM float64
}
