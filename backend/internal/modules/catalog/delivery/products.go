package delivery

import (
	"strings"

	"github.com/gofiber/fiber/v2"
	"titansystem-backend/internal/core/database"
	"titansystem-backend/internal/modules/catalog/domain"
)

type CreateProductRequest struct {
	Nome      string  `json:"nome"`
	Descricao string  `json:"descricao"`
	Preco     float64 `json:"preco"`
	SKU         string  `json:"sku"`
	Estoque     int     `json:"estoque"`
}

func ListarProdutos(c *fiber.Ctx) error {
	var produtos []domain.Product
	if err := database.DB.Order("id desc").Find(&produtos).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"erro": "falha ao listar produtos",
		})
	}
	return c.Status(fiber.StatusOK).JSON(produtos)
}

func CriarProduto(c *fiber.Ctx) error {
	var req CreateProductRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"erro": "json inválido",
		})
	}

	req.Nome = strings.TrimSpace(req.Nome)
	req.SKU = strings.TrimSpace(req.SKU)
	if req.Nome == "" || req.SKU == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"erro": "campos obrigatórios: nome, sku",
		})
	}
	if req.Preco < 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"erro": "preco não pode ser negativo",
		})
	}
	if req.Estoque < 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"erro": "estoque não pode ser negativo",
		})
	}

	p := domain.Product{
		Nome:      req.Nome,
		Descricao: req.Descricao,
		Preco:     req.Preco,
		SKU:         req.SKU,
		Estoque:     req.Estoque,
		Ativo:       true,
	}

	if err := database.DB.Create(&p).Error; err != nil {
		return c.Status(fiber.StatusConflict).JSON(fiber.Map{
			"erro": "falha ao criar produto (sku pode já existir)",
		})
	}

	return c.Status(fiber.StatusCreated).JSON(p)
}

