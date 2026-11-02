package main

import (
	"context"
	"errors"
	"io"
	"testing"
	"titansystem-backend/internal/fiscalproduction"
)

func TestServeFiscalProvidersRejectUnsafeConfigurationBeforeDatabase(t *testing.T) {
	for _, args := range [][]string{
		{"--db", "not-an-existing-database", "--station", "not-an-existing-station", "--fiscal-providers", "relative-private-config"},
		{"--db", "not-an-existing-database", "--station", "not-an-existing-station", "--fiscal-providers", "/missing/private-config", "--fiscal-production", "/missing/old-config"},
	} {
		if err := serveStationContext(context.Background(), args, io.Discard); !errors.Is(err, fiscalproduction.ErrUnavailable) {
			t.Fatalf("unsafe provider config was not rejected before db: %v", err)
		}
	}
}
