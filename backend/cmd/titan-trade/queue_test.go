package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestQueueCLIHelpDoesNotOpenFiles(t *testing.T) {
	var out bytes.Buffer
	if e := run(context.Background(), []string{"queue", "help"}, &out); e != nil || !strings.Contains(out.String(), "Não reenvia automaticamente") {
		t.Fatal("queue help unavailable")
	}
}
func TestQueueCLIInitStatusAndGracefulWatchCancellation(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "queue")
	var out bytes.Buffer
	if e := run(context.Background(), []string{"queue", "init", "--queue", dir}, &out); e != nil {
		t.Fatal(e)
	}
	out.Reset()
	if e := run(context.Background(), []string{"queue", "status", "--queue", dir}, &out); e != nil {
		t.Fatal(e)
	}
	var v struct {
		Total int `json:"total"`
	}
	if e := json.Unmarshal(out.Bytes(), &v); e != nil || v.Total != 0 {
		t.Fatal("invalid empty view")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	out.Reset()
	if e := run(ctx, []string{"queue", "watch", "--queue", dir}, &out); e != nil {
		t.Fatal("watch did not stop gracefully")
	}
}
func TestQueueCLIRefusesInvalidOperationsAndSchedules(t *testing.T) {
	for _, args := range [][]string{{"queue"}, {"queue", "unknown"}, {"queue", "status", "--job", "/not/read"}, {"queue", "watch", "--queue", "/not/read", "--interval", "0s"}, {"queue", "watch", "--queue", "/not/read", "--limit", "101"}} {
		var out bytes.Buffer
		if e := run(context.Background(), args, &out); e == nil {
			t.Fatal("unsafe invocation accepted")
		}
		if out.Len() != 0 {
			t.Fatal("printed private data or falsely announced startup")
		}
	}
}

func TestQueueCLIBackupRestoreAndValidation(t *testing.T) {
	base := t.TempDir()
	dir := filepath.Join(base, "queue")
	archive := filepath.Join(base, "backup.enc")
	key := filepath.Join(base, "backup.key")
	dest := filepath.Join(base, "restored")
	if e := os.WriteFile(key, bytes.Repeat([]byte{7}, 32), 0600); e != nil {
		t.Fatal(e)
	}
	for _, args := range [][]string{
		{"queue", "init", "--queue", dir},
		{"queue", "backup", "--queue", dir, "--key", key, "--out", archive},
		{"queue", "verify-backup", "--archive", archive, "--key", key},
		{"queue", "restore", "--archive", archive, "--key", key, "--queue", dest},
	} {
		var out bytes.Buffer
		if e := run(context.Background(), args, &out); e != nil {
			t.Fatalf("CLI %s: %v", args[1], e)
		}
		if bytes.Contains(out.Bytes(), []byte(key)) {
			t.Fatal("private key path leaked")
		}
	}
	for _, args := range [][]string{
		{"queue", "backup", "--queue", dir, "--key", key, "--out", archive},
		{"queue", "restore", "--archive", archive, "--key", key, "--queue", dest},
		{"queue", "verify-backup", "--archive", archive, "--key", key, "--queue", dir},
		{"queue", "backup", "--queue", dir, "--key", key},
	} {
		var out bytes.Buffer
		if e := run(context.Background(), args, &out); e == nil || out.Len() != 0 {
			t.Fatal("invalid CLI accepted or false success")
		}
	}
}
