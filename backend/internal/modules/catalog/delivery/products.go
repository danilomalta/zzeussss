package delivery

import (
	"titansystem-backend/internal/apicontract"

	"titansystem-backend/internal/core/database"
	"titansystem-backend/internal/modules/catalog/domain"
	"titansystem-backend/pkg/middleware"

	"github.com/gofiber/fiber/v2"
)

type CreateProductRequest struct {
	Nome      string  `json:"nome"`
	Descricao string  `json:"descricao"`
	Preco     float64 `json:"preco"`
	SKU       string  `json:"sku"`
	Estoque   int     `json:"estoque"`
}

func ListarProdutos(c *fiber.Ctx) error {
	tenantID, err := middleware.TenantID(c)
	if err != nil {
		return err
	}

	limit, offset, err := apicontract.PageQuery(c)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"erro": "paginação inválida"})
	}
	produtos := make([]domain.Product, 0)
	if err := database.DB.
		Where("tenant_id = ?", tenantID).
		Order("id desc").
		Limit(limit).Offset(offset).
		Find(&produtos).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"erro": "falha ao listar produtos",
		})
	}

	return c.Status(fiber.StatusOK).JSON(produtos)
}

func CriarProduto(c *fiber.Ctx) error {
	tenantID, err := middleware.TenantID(c)
	if err != nil {
		return err
	}

	req, err := decodeProductInput(c.Get("Content-Type"), c.Body())
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"erro": "cadastro de produto inválido"})
	}

	p := domain.Product{
		TenantID:  tenantID,
		Nome:      req.Nome,
		Descricao: req.Descricao,
		Preco:     req.Preco,
		SKU:       req.SKU,
		Estoque:   req.Estoque,
		Ativo:     true,
	}

	if err := database.DB.Create(&p).Error; err != nil {
		return c.Status(fiber.StatusConflict).JSON(fiber.Map{
			"erro": "falha ao criar produto (sku pode já existir)",
		})
	}

	return c.Status(fiber.StatusCreated).JSON(p)
}
