package handlers

import "github.com/gofiber/fiber/v2"

type RespostaSaude struct {
	Status string `json:"status"`
}

func Saude(c *fiber.Ctx) error {
	return c.Status(fiber.StatusOK).JSON(RespostaSaude{Status: "ativo"})
}

