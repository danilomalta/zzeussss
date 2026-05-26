package usecase

import (
	"log"
	"titansystem-backend/internal/core/database"
	catalogDomain "titansystem-backend/internal/modules/catalog/domain"
	posDomain "titansystem-backend/internal/modules/pos/domain"
)

// RunDiscountEngine analisa todos os produtos e gera sugestões de desconto.
// Critérios utilizados: Estoque alto, tempo de reposição longo sem giro, variação de demanda.
func RunDiscountEngine() ([]posDomain.DiscountSuggestion, error) {
	log.Println("Inciando Motor de Descontos...")
	
	var produtos []catalogDomain.Product
	if err := database.DB.Find(&produtos).Error; err != nil {
		return nil, err
	}

	var suggestions []posDomain.DiscountSuggestion

	for _, p := range produtos {
		// Pular inativos
		if !p.Ativo {
			continue
		}

		suggestion := evaluateProduct(p)
		if suggestion != nil {
			// Save pending suggestion
			if err := database.DB.Create(suggestion).Error; err != nil {
				log.Printf("Failed to save suggestion for product %d: %v", p.ID, err)
				continue
			}
			suggestions = append(suggestions, *suggestion)
		}
	}

	return suggestions, nil
}

func evaluateProduct(p catalogDomain.Product) *posDomain.DiscountSuggestion {
	// Regras hipotéticas do motor para definir sugestão e faixa
	
	// Critério 1: Produto Parado e com grande estoque
	if p.Estoque > 50 && p.DemandaMediaDiaria < 0.5 {
		return &posDomain.DiscountSuggestion{
			ProductID:         p.ID,
			SuggestedDiscount: 15.0,
			SuggestedRange:    "10%% - 20%%",
			Reason:            "Produto ocioso com alto estoque e baixíssimo giro",
			Criteria:          "PRODUTO_PARADO",
			Status:            posDomain.DiscountStatusPending,
		}
	}

	// Critério 2: Estoque acima do max recomendado
	maxRecomendado := float64(p.DemandaMediaDiaria) * float64(p.TempoReposicaoDias) * 1.5 // buffer de 50%
	if maxRecomendado > 0 && float64(p.Estoque) > maxRecomendado*2 { // Dobro do recomendado
		return &posDomain.DiscountSuggestion{
			ProductID:         p.ID,
			SuggestedDiscount: 10.0,
			SuggestedRange:    "5%% - 10%%",
			Reason:            "Excesso de estoque frente ao tempo de giro usual",
			Criteria:          "EXCESSO_ESTOQUE_VS_GIRO",
			Status:            posDomain.DiscountStatusPending,
		}
	}

	return nil
}

// ApproveSuggestion altera o status da sugestão e (em um sistema completo) pode alterar o preço ou gerar um cupom
func ApproveSuggestion(suggestionID uint, userID string) error {
	var suggestion posDomain.DiscountSuggestion
	if err := database.DB.First(&suggestion, suggestionID).Error; err != nil {
		return err
	}

	suggestion.Status = posDomain.DiscountStatusApproved
	suggestion.ReviewedBy = userID
	
	return database.DB.Save(&suggestion).Error
}

func RejectSuggestion(suggestionID uint, userID string) error {
	var suggestion posDomain.DiscountSuggestion
	if err := database.DB.First(&suggestion, suggestionID).Error; err != nil {
		return err
	}

	suggestion.Status = posDomain.DiscountStatusRejected
	suggestion.ReviewedBy = userID
	
	return database.DB.Save(&suggestion).Error
}
