package sale

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestPaymentProofExportBoundsAndExactAggregation(t *testing.T) {
	var b proofCSVBuffer
	if _, err := b.Write(make([]byte, MaxProofCSVBytes)); err != nil {
		t.Fatal(err)
	}
	if _, err := b.Write([]byte("x")); !errors.Is(err, ErrProofRange) {
		t.Fatal(err)
	}
	for _, s := range []string{"=1", "+1", "-1", "@ref", " leading", "line\nbreak", "\ufeffhidden", strings.Repeat("a", 4097)} {
		if err := proofExportText(s); !errors.Is(err, ErrProofText) {
			t.Fatal(s, err)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := encodeProofCSV(ctx, ProofReport{}); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	n := checkoutMaxSafe - 1
	if err := proofAdd(&n, 1); err != nil || n != checkoutMaxSafe {
		t.Fatal(n, err)
	}
	if err := proofAdd(&n, 1); !errors.Is(err, ErrProofRange) || n != checkoutMaxSafe {
		t.Fatal(n, err)
	}
}
