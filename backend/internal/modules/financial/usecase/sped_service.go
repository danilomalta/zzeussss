package usecase

import (
	"errors"
	"github.com/google/uuid"
	"time"
	"titansystem-backend/internal/modules/financial/domain"
)

var ErrSpedUnavailable = errors.New("SPED generation is not implemented")

// GenerateJobID remains available for callers, without seeding global PRNG state.
func GenerateJobID() string { return "SPED-" + uuid.NewString() }

// RequestSpedGeneration must not publish a fabricated file or completed job.
// Existing HTTP routes remain unavailable. No database write or goroutine runs.
func RequestSpedGeneration(startDate, endDate time.Time, requestedBy string) (*domain.SpedJob, error) {
	return nil, ErrSpedUnavailable
}
