package cashaccounting

import (
	"context"
	"errors"
	"math/big"
	"strings"
	"testing"
)

func TestAccountingCSVAndAggregateExactBoundaries(t *testing.T) {
	var b csvBuffer
	if _, err := b.Write(make([]byte, MaxCSVBytes)); err != nil {
		t.Fatal(err)
	}
	if _, err := b.Write([]byte("x")); !errors.Is(err, ErrRange) {
		t.Fatal(err)
	}
	for _, text := range []string{"=1", "+1", "-1", "@name", " leading", "line\nbreak", "\ufeffhidden", strings.Repeat("a", 4097)} {
		if err := exportText(text); !errors.Is(err, ErrText) {
			t.Fatal(text, err)
		}
	}
	for _, n := range []int64{maxSafe, -maxSafe} {
		if v, err := exact(big.NewInt(n)); err != nil || v != n {
			t.Fatal(v, err)
		}
	}
	if _, err := exact(new(big.Int).Add(big.NewInt(maxSafe), big.NewInt(1))); !errors.Is(err, ErrRange) {
		t.Fatal(err)
	}
	// Signed differences may cancel without overflowing an intermediate int64.
	n := new(big.Int).Mul(big.NewInt(maxSafe), big.NewInt(10000))
	n.Sub(n, new(big.Int).Mul(big.NewInt(maxSafe), big.NewInt(10000)))
	if v, err := exact(n); err != nil || v != 0 {
		t.Fatal(v, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := encodeCSV(ctx, Report{}); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
}
