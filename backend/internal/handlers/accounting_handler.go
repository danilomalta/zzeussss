package handlers

import (
	"time"

	"github.com/gofiber/fiber/v2"
	"titansystem-backend/internal/database"
	"titansystem-backend/internal/models"
	"titansystem-backend/internal/services"
)

// SpedRequest struct para validar payload de emissão SPED
type SpedRequest struct {
	StartDate string `json:"start_date"` // YYYY-MM-DD
	EndDate   string `json:"end_date"`   // YYYY-MM-DD
}

// RequestSpedGeneration lida com POST /api/v1/accounting/sped/request
func RequestSpedGeneration(c *fiber.Ctx) error {
	var req SpedRequest

	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Payload inválido"})
	}

	start, err := time.Parse("2006-01-02", req.StartDate)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Formato de data inválido. Use YYYY-MM-DD"})
	}
	end, err := time.Parse("2006-01-02", req.EndDate)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Formato de data inválido. Use YYYY-MM-DD"})
	}

	// TODO: Get real user from JWT Context in production
	username := "Gestor (Mock)"

	job, err := services.RequestSpedGeneration(start, end, username)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Falha ao enfileirar job SPED"})
	}

	return c.JSON(fiber.Map{
		"message": "Geração do SPED iniciada em background.",
		"job_id":  job.JobID,
		"status":  job.Status,
	})
}

// GetSpedStatus lida com GET /api/v1/accounting/sped/status/:job_id
func GetSpedStatus(c *fiber.Ctx) error {
	jobID := c.Params("job_id")
	if jobID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "ID do Job não informado"})
	}

	var job models.SpedJob
	if err := database.DB.Where("job_id = ?", jobID).First(&job).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Job não encontrado"})
	}

	return c.JSON(fiber.Map{
		"job_id":       job.JobID,
		"status":       job.Status,
		"started_at":   job.CreatedAt,
		"completed_at": job.CompletedAt,
		"file_url":     job.FileURL,
		"message":      job.ErrorMsg,
	})
}
