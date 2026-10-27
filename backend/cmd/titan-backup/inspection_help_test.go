package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestBackupHelpExplainsInspectionWithoutReadingKeyOrDatabase(t *testing.T) {
	for _, arg := range []string{"help", "--help", "-h"} {
		var out bytes.Buffer
		if e := run([]string{arg}, &out); e != nil {
			t.Fatal(e)
		}
		for _, text := range []string{"inspect", "pilot_cash_v1", "128 MiB", "revoga sessões", "BANCO_NOVO"} {
			if !strings.Contains(out.String(), text) {
				t.Fatal("missing operational help", text)
			}
		}
	}
}
