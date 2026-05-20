package handlers

import (
	"strconv"

	"github.com/gofiber/fiber/v2"
	"titansystem-backend/internal/database"
	"titansystem-backend/internal/models"
	"titansystem-backend/internal/services"
)

// SuggestDiscounts POST /api/v1/discounts/suggest
// Trigga o motor de análise para gerar um relatório em banco.
func SuggestDiscounts(c *fiber.Ctx) error {
	sugestoes, err := services.RunDiscountEngine()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Falha ao executar IA / Motor de Descontos"})
	}

	return c.JSON(fiber.Map{
		"message":       "Motor finalizado com sucesso",
		"items_gerados": len(sugestoes),
		"sugestoes":     sugestoes,
	})
}

// GetSuggestions GET /api/v1/discounts/suggestions
// Lista sugestões pendentes para o gestor
func GetSuggestions(c *fiber.Ctx) error {
	var sugestoes []models.DiscountSuggestion

	statusFilter := c.Query("status", models.DiscountStatusPending)

	if err := database.DB.Where("status = ?", statusFilter).Find(&sugestoes).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Falha ao buscar sugestões."})
	}

	return c.JSON(fiber.Map{
		"count":     len(sugestoes),
		"sugestoes": sugestoes,
	})
}

// ReviewSuggestion POST /api/v1/discounts/suggestions/:id/review
func ReviewSuggestion(c *fiber.Ctx) error {
	idParam := c.Params("id")
	id, err := strconv.ParseUint(idParam, 10, 32)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "ID inválido"})
	}

	var payload struct {
		Aprovado bool `json:"aprovado"`
	}

	if err := c.BodyParser(&payload); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Payload inválido"})
	}

	// TODO: Get user from context
	user := "Gerente (Mock)"

	// Aprovar ou rejeitar
	if payload.Aprovado {
		err = services.ApproveSuggestion(uint(id), user)
	} else {
		err = services.RejectSuggestion(uint(id), user)
	}

	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Falha ao salvar revisão."})
	}

	return c.JSON(fiber.Map{
		"message": "Sugestão analisada com sucesso.",
	})
}
