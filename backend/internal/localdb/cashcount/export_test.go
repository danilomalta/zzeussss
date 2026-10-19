package cashcount

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestExportBoundsContextAndUnsupportedText(t *testing.T) {
	var b boundedCSV
	if _, err := b.Write(make([]byte, MaxCSVBytes)); err != nil {
		t.Fatal(err)
	}
	if _, err := b.Write([]byte("x")); !errors.Is(err, ErrRange) {
		t.Fatal(err)
	}
	for _, s := range []string{"=SUM(1)", "+1", "-1", "@name", " leading", "\ufeffhidden", "line\nbreak", "tab\tcell", strings.Repeat("a", 4097)} {
		if err := exportText(s); !errors.Is(err, ErrText) {
			t.Fatal(s, err)
		}
	}
	for _, s := range []string{"", "Cofre São Paulo", "123-id", "cofre_2", "nome;com;separador", "nome\"aspas"} {
		if err := exportText(s); err != nil {
			t.Fatal(s, err)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := encodeCSV(ctx, Report{Items: []ReportItem{{}}}); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
}
