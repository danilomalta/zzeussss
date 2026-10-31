package fiscalproduction

import (
	"context"
	"errors"
	"fmt"
	"time"

	sqlite "modernc.org/sqlite"
	"titansystem-backend/internal/localdb/fiscal"
)

// Only SQLite contention messages are safe to retain: no SQL, paths or values.
// Other database errors stay private. Callers can use errors.As for the code.
var errVaultKeyChanged = errors.New("vault key changed")

func vaultDatabaseError(stage string, err error) error {
	if errors.Is(err, fiscal.ErrConflict) || errors.Is(err, errVaultKeyChanged) {
		return err
	}
	var cause *sqlite.Error
	if errors.As(err, &cause) && (cause.Code()&255 == 5 || cause.Code()&255 == 6) {
		return errors.Join(ErrUnavailable, fmt.Errorf("fiscal vault %s: %w", stage, cause))
	}
	return ErrUnavailable
}

func vaultContention(err error) bool {
	var cause *sqlite.Error
	return errors.As(err, &cause) && (cause.Code()&255 == 5 || cause.Code()&255 == 6)
}

// Retries cover opening, key control, BEGIN IMMEDIATE and commit. Every retry
// reopens and rereads under a fresh writer transaction; no stale revision survives.
func (r *ProviderRegistry) rotateCustomerCredentials(ctx context.Context, a RequestContext, profile string, in CustomerCredentialInput, p fiscal.Profile) (CustomerCredentialStatus, error) {
	var last error
	for attempt := 0; attempt < 4; attempt++ {
		out, err := r.saveCustomerCredentialAttempt(ctx, a, profile, in, p)
		if !vaultContention(err) {
			return out, err
		}
		last = err
		if attempt == 3 {
			break
		}
		timer := time.NewTimer(time.Duration(attempt+1) * 25 * time.Millisecond)
		select {
		case <-ctx.Done():
			timer.Stop()
			return CustomerCredentialStatus{}, ctx.Err()
		case <-timer.C:
		}
	}
	var cause *sqlite.Error
	errors.As(last, &cause)
	// Exhausted contention is a retryable revision conflict, never unavailable.
	return CustomerCredentialStatus{}, errors.Join(fiscal.ErrConflict, fmt.Errorf("credential rotation write contention: %w", cause))
}
