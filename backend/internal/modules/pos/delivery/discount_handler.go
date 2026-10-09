package delivery

import (
	"errors"
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v2"

	"titansystem-backend/internal/apicontract"
	"titansystem-backend/internal/core/database"
	"titansystem-backend/internal/modules/pos/domain"
	"titansystem-backend/internal/modules/pos/usecase"
	"titansystem-backend/pkg/middleware"
)

func SuggestDiscounts(c *fiber.Ctx) error {
	tenantID, err := middleware.TenantID(c)
	if err != nil {
		return err
	}

	sugestoes, err := usecase.RunDiscountEngine(tenantID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Falha ao gerar sugestões de desconto.",
		})
	}

	return c.JSON(fiber.Map{
		"message":       "Análise concluída.",
		"items_gerados": len(sugestoes),
		"sugestoes":     sugestoes,
	})
}

func GetSuggestions(c *fiber.Ctx) error {
	tenantID, err := middleware.TenantID(c)
	if err != nil {
		return err
	}

	limit, offset, err := apicontract.PageQuery(c, "status")
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Paginação inválida."})
	}
	statusFilter := c.Query("status", domain.DiscountStatusPending)
	switch statusFilter {
	case domain.DiscountStatusPending,
		domain.DiscountStatusApproved,
		domain.DiscountStatusRejected:
	default:
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Status inválido.",
		})
	}

	sugestoes := make([]domain.DiscountSuggestion, 0)
	if err := database.DB.
		Where("tenant_id = ? AND status = ?", tenantID, statusFilter).
		Order("id DESC").
		Limit(limit).Offset(offset).
		Find(&sugestoes).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Falha ao buscar sugestões.",
		})
	}

	return c.JSON(fiber.Map{
		"count":     len(sugestoes),
		"sugestoes": sugestoes,
	})
}

func ReviewSuggestion(c *fiber.Ctx) error {
	tenantID, err := middleware.TenantID(c)
	if err != nil {
		return err
	}

	userID := strings.TrimSpace(middleware.UserID(c))
	if userID == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "Operador ausente na sessão.",
		})
	}

	id, err := strconv.ParseUint(c.Params("id"), 10, strconv.IntSize)
	if err != nil || id == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "ID inválido.",
		})
	}

	var payload struct {
		Aprovado *bool `json:"aprovado"`
	}
	if err := c.BodyParser(&payload); err != nil || payload.Aprovado == nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Informe aprovado como true ou false.",
		})
	}

	if *payload.Aprovado {
		err = usecase.ApproveSuggestion(tenantID, uint(id), userID)
	} else {
		err = usecase.RejectSuggestion(tenantID, uint(id), userID)
	}

	if errors.Is(err, usecase.ErrSuggestionUnavailable) {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Sugestão não encontrada ou já revisada.",
		})
	}
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Falha ao salvar revisão.",
		})
	}

	return c.JSON(fiber.Map{
		"message": "Sugestão analisada com sucesso.",
	})
}
