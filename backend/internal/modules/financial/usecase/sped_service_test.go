package usecase

import (
	"errors"
	"testing"
	"time"
)

func TestSpedNeverFabricatesCompletedJobWithoutImplementation(t *testing.T) {
	// No initialized database, network or worker is needed to refuse the feature.
	job, err := RequestSpedGeneration(time.Now(), time.Now(), "synthetic-owner")
	if job != nil || !errors.Is(err, ErrSpedUnavailable) {
		t.Fatalf("fabricated fiscal result: %v %v", job, err)
	}
}
