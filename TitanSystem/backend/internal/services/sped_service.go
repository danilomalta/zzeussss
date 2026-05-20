package services

import (
	"fmt"
	"math/rand"
	"time"

	"titansystem-backend/internal/database"
	"titansystem-backend/internal/models"
)

func init() {
	rand.Seed(time.Now().UnixNano())
}

// GenerateJobID generates a random job ID for tracking SPED processing
func GenerateJobID() string {
	return fmt.Sprintf("SPED-%d-%d", time.Now().Unix(), rand.Intn(10000))
}

// RequestSpedGeneration creates a job and starts processing
func RequestSpedGeneration(startDate, endDate time.Time, requestedBy string) (*models.SpedJob, error) {
	job := &models.SpedJob{
		JobID:       GenerateJobID(),
		Type:        "EFD-ICMS/IPI",
		Status:      models.SpedJobStatusPending,
		StartDate:   startDate,
		EndDate:     endDate,
		RequestedBy: requestedBy,
	}

	if err := database.DB.Create(job).Error; err != nil {
		return nil, err
	}

	// Update status to processing and save
	job.Status = models.SpedJobStatusProcessing
	database.DB.Save(job)

	// Launch async rendering worker
	go processSpedJob(job.ID)

	return job, nil
}

// processSpedJob mock of heavy processing
func processSpedJob(jobID uint) {
	// Simulate delay 5 to 15 seconds
	delay := time.Duration(rand.Intn(10)+5) * time.Second
	time.Sleep(delay)

	var job models.SpedJob
	if err := database.DB.First(&job, jobID).Error; err != nil {
		return // Job not found probably
	}

	// Make sure it doesn't fail sporadically
	if rand.Float32() < 0.05 { // 5% chance of failure (mocking real-world issues)
		job.Status = models.SpedJobStatusFailed
		job.ErrorMsg = "Falha ao consultar NFe's no banco de dados."
	} else {
		job.Status = models.SpedJobStatusCompleted
		job.FileURL = fmt.Sprintf("https://s3.titansystem.com/sped/download/%s.txt", job.JobID)
		now := time.Now()
		job.CompletedAt = &now
	}

	database.DB.Save(&job)
}
