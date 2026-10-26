package localapi

import (
	"encoding/json"
	"flag"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"titansystem-backend/internal/traderelay"
)

var pilotTradeQueue = flag.Bool("pilot-trade-queue", os.Getenv("TITAN_TRADE_QUEUE") == "1", "Use real durable trade queue in the authenticated company cash pilots")

func pilotDispatchTrade(t *testing.T, binary string, source, target *pilotHTTP, j traderelay.Job, directory, mode string) traderelay.Result {
	if !*pilotTradeQueue {
		return pilotRunRelayCLI(t, binary, source, target, j, directory, mode)
	}
	return pilotRunQueueCLI(t, binary, source, target, j, directory, mode)
}
func pilotRunQueueCLI(t *testing.T, binary string, source, target *pilotHTTP, j traderelay.Job, directory, mode string) traderelay.Result {
	t.Helper()
	if e := os.Chmod(directory, 0700); e != nil {
		t.Fatal(e)
	}
	prefix := filepath.Join(directory, j.OperationID)
	job, st, tt, queue := prefix+".job.json", prefix+".source-token", prefix+".target-token", prefix+".queue"
	body, _ := json.Marshal(j)
	for p, b := range map[string][]byte{job: body, st: []byte(source.token), tt: []byte(target.token)} {
		if e := os.WriteFile(p, b, 0600); e != nil {
			t.Fatal(e)
		}
	}
	call := func(out any, args ...string) {
		t.Helper()
		cmd := exec.Command(binary, append([]string{"queue"}, args...)...)
		output, e := cmd.CombinedOutput()
		if e != nil {
			t.Fatalf("compiled queue %v %s", e, output)
		}
		if out != nil && json.Unmarshal(output, out) != nil {
			t.Fatal("invalid queue JSON")
		}
	}
	result := traderelay.Result{OperationID: j.OperationID, MessageID: j.MessageID}
	if mode == "send" {
		call(nil, "init", "--queue", queue)
		var add traderelay.QueueAction
		call(&add, "add", "--queue", queue, "--job", job, "--source-token", st, "--target-token", tt)
		if add.Item.OperationID != j.OperationID || add.Item.MessageID != j.MessageID {
			t.Fatal("queue changed identifiers")
		}
		result.SourceCalls += add.SourceCalls
		result.TargetCalls += add.TargetCalls
		var batch traderelay.QueueBatch
		call(&batch, "run", "--queue", queue)
		if batch.Confirmed != 1 || batch.Posts != 1 {
			t.Fatalf("delivery not confirmed %+v", batch)
		}
		result.SourceCalls += batch.SourceCalls
		result.TargetCalls += batch.TargetCalls
		result.Posts = batch.Posts
	} else if mode == "recover" {
		var view traderelay.QueueView
		call(&view, "status", "--queue", queue)
		if view.Total != 1 || view.Items[0].OperationID != j.OperationID || view.Items[0].MessageID != j.MessageID {
			t.Fatal("missing original queue attempt")
		}
		var batch traderelay.QueueBatch
		call(&batch, "verify", "--queue", queue, "--id", view.Items[0].ID)
		if batch.Confirmed != 1 || batch.Posts != 0 || batch.SourceCalls != 0 {
			t.Fatal("receipt verification wrote or contacted sender")
		}
		result.SourceCalls = batch.SourceCalls
		result.TargetCalls = batch.TargetCalls
	} else {
		t.Fatal("unsupported pilot queue mode")
	}
	result.Status = "confirmed"
	source.steps += result.SourceCalls
	target.steps += result.TargetCalls
	t.Logf("QUEUE CLI: %s, source%d HTTP destination%d HTTP posts%d; persisted original receipt confirmed", mode, result.SourceCalls, result.TargetCalls, result.Posts)
	return result
}
func TestTradeQueueCompiledExecutableBothPasses(t *testing.T) {
	b, s, j, _ := relaySetup(t)
	binary := relayExecutable(t)
	dir := t.TempDir()
	r := pilotRunQueueCLI(t, binary, b.p, s.p, j, dir, "send")
	if r.Posts != 1 {
		t.Fatal("missing initial delivery")
	}
	r = pilotRunQueueCLI(t, binary, b.p, s.p, j, dir, "recover")
	if r.Posts != 0 {
		t.Fatal("verification sent again")
	}
}
