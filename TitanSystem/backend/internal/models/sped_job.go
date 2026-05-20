package models

import (
	"time"

	"gorm.io/gorm"
)

// SpedJob status constants
const (
	SpedJobStatusPending   = "PENDING"
	SpedJobStatusProcessing = "PROCESSING"
	SpedJobStatusCompleted  = "COMPLETED"
	SpedJobStatusFailed     = "FAILED"
)

// SpedJob representa uma solicitação assíncrona para geração de arquivos EFD/Contábeis
type SpedJob struct {
	gorm.Model
	JobID       string     `json:"job_id" gorm:"uniqueIndex"`
	Type        string     `json:"type"` // e.g., "EFD-ICMS/IPI"
	Status      string     `json:"status"`
	StartDate   time.Time  `json:"start_date"`
	EndDate     time.Time  `json:"end_date"`
	RequestedBy string     `json:"requested_by"` // user ID or name
	FileURL     string     `json:"file_url"`     // Download link once completed
	ErrorMsg    string     `json:"error_msg"`
	CompletedAt *time.Time `json:"completed_at"`
}
