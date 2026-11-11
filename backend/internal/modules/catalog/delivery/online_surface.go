package delivery

import (
	"github.com/gofiber/fiber/v2"
	"titansystem-backend/internal/onlinecatalog"
)

// RecusarOperacaoExterna recusa operação que não existe neste PostgreSQL, sem inventar dado.
func RecusarOperacaoExterna(code string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		c.Set("Cache-Control", "no-store")
		return c.Status(fiber.StatusConflict).JSON(fiber.Map{"error": code})
	}
}

func MetricasVisaoGeral(c *fiber.Ctx) error {
	if !noManagementQuery(c) || len(c.Body()) != 0 {
		return managementError(c, onlinecatalog.ErrInput)
	}
	ctx, cancel, s, a, e := managementContext(c)
	defer cancel()
	if e != nil {
		return managementError(c, e)
	}
	r, e := s.QualitySummary(ctx, a, onlinecatalog.QualityQuery{Active: "active", Issue: "any", Limit: 50})
	if e != nil {
		return managementError(c, e)
	}
	return c.JSON(fiber.Map{
		"scope":          "online_catalog",
		"pdv_available":  false,
		"totalProducts":  r.Products,
		"totalSales":     0,
		"activeClients":  0,
		"totalCosts":     0,
		"netProfit":      0,
		"criticalIssues": 0,
		"averageUptime":  "",
	})
}

func VendasRecentesOnline(c *fiber.Ctx) error {
	if !noManagementQuery(c) || len(c.Body()) != 0 {
		return managementError(c, onlinecatalog.ErrInput)
	}
	c.Set("Cache-Control", "no-store")
	c.Set("X-Titan-PDV-Available", "false")
	return c.JSON([]struct{}{})
}

func SessoesCaixaOnline(c *fiber.Ctx) error {
	if !noManagementQuery(c) || len(c.Body()) != 0 {
		return managementError(c, onlinecatalog.ErrInput)
	}
	c.Set("Cache-Control", "no-store")
	return c.JSON(fiber.Map{"items": []struct{}{}, "available": false, "scope": "online"})
}

func FuncionariosOnline(c *fiber.Ctx) error {
	if !noManagementQuery(c) || len(c.Body()) != 0 {
		return managementError(c, onlinecatalog.ErrInput)
	}
	c.Set("Cache-Control", "no-store")
	return c.JSON(fiber.Map{"items": []struct{}{}, "available": false, "scope": "online"})
}
