package ai_insights

import (
	"strconv"

	"github.com/gofiber/fiber/v2"
	"titansystem-backend/internal/ai_insights"
)

// ListarProdutosParados expõe a análise de produtos parados.
//
// Query:
// - dias (padrão 30)
// - limite (padrão 100)
func ListarProdutosParados(c *fiber.Ctx) error {
	dias, _ := strconv.Atoi(c.Query("dias", "30"))
	limite, _ := strconv.Atoi(c.Query("limite", "100"))

	resultado, err := ai_insights.ListarProdutosParados(dias, limite)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"erro": "falha ao executar análise",
		})
	}
	return c.Status(fiber.StatusOK).JSON(resultado)
}

