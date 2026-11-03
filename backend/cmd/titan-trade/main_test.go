package main

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

func TestHelpDoesNotReadFilesOrAuthenticate(t *testing.T) {
	for _, arg := range []string{"help", "--help", "-h"} {
		var out bytes.Buffer
		e := run(context.Background(), []string{arg}, &out)
		if arg == "-h" {
			if e == nil {
				t.Fatal("unexpected short flag accepted")
			}
			continue
		}
		if e != nil || !strings.Contains(out.String(), "nunca faz POST") {
			t.Fatal("help unavailable")
		}
	}
}
func TestInvalidArgumentsNeverPrintCredentialsOrTryNetwork(t *testing.T) {
	for _, args := range [][]string{{}, {"--token", "never-print-this"}, {"--mode", "automatic", "--source-token", "/not/read"}, {"--job", "/absent", "unexpected"}} {
		var out bytes.Buffer
		if e := run(context.Background(), args, &out); e == nil {
			t.Fatal("invalid invocation accepted")
		}
		if out.Len() != 0 {
			t.Fatal("unexpected credential or data output")
		}
	}
}
