package usecase

import (
	"errors"
	"strings"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"titansystem-backend/internal/core/database"
	catalogDomain "titansystem-backend/internal/modules/catalog/domain"
	posDomain "titansystem-backend/internal/modules/pos/domain"
)

var ErrSuggestionUnavailable = errors.New(
	"sugestão não encontrada ou já revisada",
)

// A synchronous request must not scan an unbounded company catalog.
const MaxDiscountAnalysisProducts = 1000

var ErrDiscountAnalysisTooLarge = errors.New("catálogo excede o limite de análise síncrona")

func RunDiscountEngine(tenantID string) ([]posDomain.DiscountSuggestion, error) {
	if strings.TrimSpace(tenantID) == "" {
		return nil, errors.New("empresa obrigatória")
	}

	suggestions := make([]posDomain.DiscountSuggestion, 0)

	err := database.DB.Transaction(func(tx *gorm.DB) error {
		var produtos []catalogDomain.Product
		if err := tx.
			Where("tenant_id = ? AND ativo = ?", tenantID, true).
			Order("id ASC").Limit(MaxDiscountAnalysisProducts + 1).
			Find(&produtos).Error; err != nil {
			return err
		}
		if len(produtos) > MaxDiscountAnalysisProducts {
			return ErrDiscountAnalysisTooLarge
		}
		for _, p := range produtos {
			if p.ID == 0 || p.TenantID != tenantID {
				return errors.New("contexto inválido na análise de produtos")
			}
		}

		for _, p := range produtos {
			suggestion := evaluateProduct(p)
			if suggestion == nil {
				continue
			}

			// A empresa vem do contexto autenticado.
			suggestion.TenantID = tenantID

			// O índice da migração impede duas sugestões pendentes
			// para o mesmo produto, inclusive em chamadas concorrentes.
			result := tx.Clauses(clause.OnConflict{
				Columns: []clause.Column{{Name: "tenant_id"}, {Name: "product_id"}},
				TargetWhere: clause.Where{Exprs: []clause.Expression{
					clause.Expr{SQL: "status = 'PENDING' AND deleted_at IS NULL"},
				}},
				DoNothing: true,
			}).Create(suggestion)

			if result.Error != nil {
				return result.Error
			}
			if result.RowsAffected > 0 {
				suggestions = append(suggestions, *suggestion)
			}
		}
		return nil
	})

	if err != nil {
		return nil, err
	}
	return suggestions, nil
}

func evaluateProduct(p catalogDomain.Product) *posDomain.DiscountSuggestion {
	if !p.Ativo {
		return nil
	}

	if p.Estoque > 50 && p.DemandaMediaDiaria < 0.5 {
		return &posDomain.DiscountSuggestion{
			TenantID:          p.TenantID,
			ProductID:         p.ID,
			SuggestedDiscount: 15,
			SuggestedRange:    "10% - 20%",
			Reason:            "Produto ocioso com alto estoque e baixíssimo giro",
			Criteria:          "PRODUTO_PARADO",
			Status:            posDomain.DiscountStatusPending,
		}
	}

	maxRecomendado := p.DemandaMediaDiaria *
		float64(p.TempoReposicaoDias) * 1.5

	if maxRecomendado > 0 && float64(p.Estoque) > maxRecomendado*2 {
		return &posDomain.DiscountSuggestion{
			TenantID:          p.TenantID,
			ProductID:         p.ID,
			SuggestedDiscount: 10,
			SuggestedRange:    "5% - 10%",
			Reason:            "Excesso de estoque frente ao tempo de giro usual",
			Criteria:          "EXCESSO_ESTOQUE_VS_GIRO",
			Status:            posDomain.DiscountStatusPending,
		}
	}

	return nil
}

func ApproveSuggestion(tenantID string, suggestionID uint, userID string) error {
	return reviewSuggestion(
		tenantID, suggestionID, userID, posDomain.DiscountStatusApproved,
	)
}

func RejectSuggestion(tenantID string, suggestionID uint, userID string) error {
	return reviewSuggestion(
		tenantID, suggestionID, userID, posDomain.DiscountStatusRejected,
	)
}

func reviewSuggestion(
	tenantID string,
	suggestionID uint,
	userID string,
	status string,
) error {
	if strings.TrimSpace(tenantID) == "" ||
		strings.TrimSpace(userID) == "" ||
		suggestionID == 0 {
		return errors.New("empresa, operador e sugestão são obrigatórios")
	}

	// Atualiza somente sugestões pendentes da empresa autenticada.
	// A condição também impede duas revisões concorrentes.
	result := database.DB.
		Model(&posDomain.DiscountSuggestion{}).
		Where(
			"tenant_id = ? AND id = ? AND status = ?",
			tenantID, suggestionID, posDomain.DiscountStatusPending,
		).
		Updates(map[string]interface{}{
			"status":      status,
			"reviewed_by": userID,
		})

	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrSuggestionUnavailable
	}
	return nil
}
