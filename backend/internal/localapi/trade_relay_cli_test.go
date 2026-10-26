package localapi

import (
	"encoding/json"
	"flag"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
	"titansystem-backend/internal/traderelay"
)

var pilotTradeCLI = flag.String("pilot-trade-cli", os.Getenv("TITAN_TRADE_CLI"), "Explicit compiled titan-trade executable for real company-message transport")

func relayExecutable(t *testing.T) string {
	t.Helper()
	if *pilotTradeCLI != "" {
		if !filepath.IsAbs(*pilotTradeCLI) {
			t.Fatal("absolute trade executable required")
		}
		return *pilotTradeCLI
	}
	binary := filepath.Join(t.TempDir(), "titan-trade")
	cmd := exec.Command(filepath.Join(runtime.GOROOT(), "bin", "go"), "build", "-trimpath", "-o", binary, "../../cmd/titan-trade")
	if output, e := cmd.CombinedOutput(); e != nil {
		t.Fatalf("build relay: %v %s", e, output)
	}
	return binary
}
func pilotRunRelayCLI(t *testing.T, binary string, source, target *pilotHTTP, j traderelay.Job, directory, mode string) traderelay.Result {
	t.Helper()
	if e := os.Chmod(directory, 0700); e != nil {
		t.Fatal(e)
	}
	prefix := filepath.Join(directory, j.OperationID)
	jobFile := prefix + ".job.json"
	sourceFile := prefix + ".source-token"
	targetFile := prefix + ".target-token"
	state := prefix + ".state.json"
	body, e := json.Marshal(j)
	if e != nil {
		t.Fatal(e)
	}
	for path, contents := range map[string][]byte{jobFile: body, sourceFile: []byte(source.token), targetFile: []byte(target.token)} {
		if e = os.WriteFile(path, contents, 0600); e != nil {
			t.Fatal(e)
		}
	}
	cmd := exec.Command(binary, "--job", jobFile, "--state", state, "--source-token", sourceFile, "--target-token", targetFile, "--mode", mode)
	output, e := cmd.CombinedOutput()
	if e != nil {
		t.Fatalf("compiled relay failed: %v %s", e, output)
	}
	var out traderelay.Result
	if e = json.Unmarshal(output, &out); e != nil || out.Status != "confirmed" || out.OperationID != j.OperationID || out.MessageID != j.MessageID {
		t.Fatal("invalid compiled relay receipt")
	}
	source.steps += out.SourceCalls
	target.steps += out.TargetCalls
	t.Logf("TRADE CLI: %s, source %d HTTP, destination %d HTTP, posts%d, receipt confirmed", mode, out.SourceCalls, out.TargetCalls, out.Posts)
	return out
}
func TestTradeRelayCompiledExecutableRealHTTP(t *testing.T) {
	b, s, j, _ := relaySetup(t)
	binary := relayExecutable(t)
	dir := t.TempDir()
	out := pilotRunRelayCLI(t, binary, b.p, s.p, j, dir, "send")
	if out.Posts != 1 {
		t.Fatal("missing explicit send")
	}
	out = pilotRunRelayCLI(t, binary, b.p, s.p, j, dir, "recover")
	if out.Posts != 0 || out.SourceCalls != 0 {
		t.Fatal("recovery contacted source or posted")
	}
}
