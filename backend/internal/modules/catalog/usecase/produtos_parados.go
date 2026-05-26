package usecase

import (
	"time"

	"titansystem-backend/internal/core/database"
	"titansystem-backend/internal/modules/catalog/domain"
)

type ProdutoParado struct {
	ProdutoID uint       `json:"produto_id"`
	SKU       string     `json:"sku"`
	Nome      string     `json:"nome"`
	Estoque   int        `json:"estoque"`
	UltimaVendaEm *time.Time `json:"ultima_venda_em"`
	DiasSemVenda int      `json:"dias_sem_venda"`
}

// ListarProdutosParados encontra produtos com estoque e sem venda recente.
//
// Definição:
// - Produto parado = possui estoque > 0 e não teve venda nos últimos `diasSemVenda`.
//
// Observação:
// - Essa consulta é a base para recomendações automáticas (ex.: sugerir desconto).
func ListarProdutosParados(diasSemVenda int, limite int) ([]ProdutoParado, error) {
	if diasSemVenda <= 0 {
		diasSemVenda = 30
	}
	if limite <= 0 || limite > 500 {
		limite = 100
	}

	agora := time.Now()
	limiteData := agora.AddDate(0, 0, -diasSemVenda)

	type linha struct {
		ProdutoID     uint
		SKU           string
		Nome          string
		Estoque       int
		UltimaVendaEm *time.Time
	}

	var linhas []linha

	// Estratégia:
	// - produtos com estoque > 0
	// - junta com itens de venda e vendas para descobrir a última data de venda por produto
	// - filtra os que não venderam desde `limiteData` (ou nunca venderam)
	err := database.DB.
		Table("products p").
		Select(`
			p.id as produto_id,
			p.sku as sku,
			p.nome as nome,
			p.estoque as estoque,
			MAX(s.vendida_em) as ultima_venda_em
		`).
		Joins("LEFT JOIN sale_items si ON si.product_id = p.id").
		Joins("LEFT JOIN sales s ON s.id = si.sale_id AND s.status = ?", "concluida").
		Where("p.estoque > 0 AND p.ativo = ?", true).
		Group("p.id, p.sku, p.nome, p.estoque").
		Having("MAX(s.vendida_em) IS NULL OR MAX(s.vendida_em) < ?", limiteData).
		Order("p.estoque DESC, p.id DESC").
		Limit(limite).
		Scan(&linhas).Error
	if err != nil {
		return nil, err
	}

	resultado := make([]ProdutoParado, 0, len(linhas))
	for _, l := range linhas {
		dias := diasSemVenda
		if l.UltimaVendaEm != nil {
			dias = int(agora.Sub(*l.UltimaVendaEm).Hours() / 24)
			if dias < 0 {
				dias = 0
			}
		}
		resultado = append(resultado, ProdutoParado{
			ProdutoID:     l.ProdutoID,
			SKU:           l.SKU,
			Nome:          l.Nome,
			Estoque:       l.Estoque,
			UltimaVendaEm: l.UltimaVendaEm,
			DiasSemVenda:  dias,
		})
	}

	// Mantém uma referência para evitar “import não usado” caso o compilador/IDE fique agressivo
	// em refatorações futuras; o modelo é parte do domínio desta análise.
	_ = domain.Product{}

	return resultado, nil
}

