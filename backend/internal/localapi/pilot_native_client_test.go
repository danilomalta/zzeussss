package localapi

import (
	"bufio"
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

var pilotElectron = flag.String("pilot-electron", "", "Explicit existing Electron binary for native acceptance; no installation")
var pilotNativeScript = flag.String("pilot-native-script", "", "Explicit desktop/tests/pilot-api-client.cjs for native acceptance")
var pilotDesktopStage = flag.String("pilot-desktop-stage", "", "Explicit compiled desktop stage for native acceptance")

type pilotNativeTransport struct {
	t         *testing.T
	origin    string
	cmd       *exec.Cmd
	stdin     io.WriteCloser
	reader    *bufio.Scanner
	processes int
	requests  int
}
type pilotNativeMessage struct {
	Type             string            `json:"type"`
	Status           int               `json:"status"`
	Headers          map[string]string `json:"headers"`
	Body             string            `json:"body"`
	Origin           string            `json:"origin"`
	Sandbox          bool              `json:"sandbox"`
	ContextIsolation bool              `json:"contextIsolation"`
	NodeIntegration  bool              `json:"nodeIntegration"`
}

func pilotAcceptanceTransport(t *testing.T, fallback http.RoundTripper) http.RoundTripper {
	t.Helper()
	if *pilotElectron == "" && *pilotNativeScript == "" && *pilotDesktopStage == "" {
		return fallback
	}
	for _, p := range []string{*pilotElectron, *pilotNativeScript, *pilotDesktopStage} {
		if !filepath.IsAbs(p) {
			t.Fatal("all three native acceptance paths must be explicit and absolute")
		}
		if _, e := os.Stat(p); e != nil {
			t.Fatal(e)
		}
	}
	n := &pilotNativeTransport{t: t}
	t.Cleanup(func() {
		if e := n.stop(); e != nil {
			t.Error(e)
		}
		t.Logf("Native Electron production transport: %d requests, %d isolated application lifetimes", n.requests, n.processes)
	})
	return n
}
func (n *pilotNativeTransport) next() (pilotNativeMessage, error) {
	for n.reader.Scan() {
		line := n.reader.Text()
		if !strings.HasPrefix(line, "TITAN_PILOT_RPC:") {
			continue
		}
		var v pilotNativeMessage
		if e := json.Unmarshal([]byte(strings.TrimPrefix(line, "TITAN_PILOT_RPC:")), &v); e != nil {
			return v, e
		}
		if v.Type == "failure" {
			return v, errors.New("native application reported failure (credentials omitted)")
		}
		return v, nil
	}
	return pilotNativeMessage{}, errors.New("native application ended without protocol response")
}
func (n *pilotNativeTransport) stop() error {
	if n.cmd == nil {
		return nil
	}
	_ = n.stdin.Close()
	e := n.cmd.Wait()
	n.cmd = nil
	return e
}
func (n *pilotNativeTransport) start(origin string) error {
	if e := n.stop(); e != nil {
		return e
	}
	args := []string{}
	if runtime.GOOS == "linux" {
		args = append(args, "--ozone-platform=headless")
	}
	args = append(args, *pilotNativeScript, *pilotDesktopStage, origin)
	cmd := exec.Command(*pilotElectron, args...)
	for _, name := range []string{"PATH", "HOME", "TMPDIR", "SYSTEMROOT"} {
		if v, ok := os.LookupEnv(name); ok {
			cmd.Env = append(cmd.Env, name+"="+v)
		}
	}
	cmd.Stderr = io.Discard
	out, e := cmd.StdoutPipe()
	if e != nil {
		return e
	}
	input, e := cmd.StdinPipe()
	if e != nil {
		return e
	}
	if e = cmd.Start(); e != nil {
		return e
	}
	n.cmd = cmd
	n.stdin = input
	n.reader = bufio.NewScanner(out)
	n.reader.Buffer(make([]byte, 4096), 6<<20)
	v, e := n.next()
	if e != nil {
		return e
	}
	if v.Type != "ready" || v.Origin != "titan://app" || !v.Sandbox || !v.ContextIsolation || v.NodeIntegration {
		return errors.New("desktop origin or security preferences invalid")
	}
	n.origin = origin
	n.processes++
	return nil
}
func (n *pilotNativeTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if req.URL.Scheme != "http" || req.URL.Hostname() != "127.0.0.1" || !strings.HasPrefix(req.URL.Path, "/local/v1/") {
		return nil, errors.New("acceptance only permits disposable loopback API")
	}
	origin := req.URL.Scheme + "://" + req.URL.Host
	if n.cmd == nil || n.origin != origin {
		if e := n.start(origin); e != nil {
			return nil, e
		}
	}
	var body []byte
	var e error
	if req.Body != nil {
		body, e = io.ReadAll(io.LimitReader(req.Body, 1<<20))
		req.Body.Close()
		if e != nil {
			return nil, e
		}
	}
	headers := map[string]string{}
	for _, key := range []string{"Authorization", "Content-Type"} {
		if v := req.Header.Get(key); v != "" {
			headers[key] = v
		}
	}
	raw, e := json.Marshal(map[string]any{"type": "request", "method": req.Method, "path": req.URL.RequestURI(), "headers": headers, "body": base64.StdEncoding.EncodeToString(body)})
	if e != nil {
		return nil, e
	}
	if _, e = n.stdin.Write(append(raw, '\n')); e != nil {
		return nil, e
	}
	v, e := n.next()
	if e != nil {
		return nil, e
	}
	if v.Type != "response" || v.Status < 100 || v.Status > 599 {
		return nil, errors.New("invalid native HTTP result")
	}
	data, e := base64.StdEncoding.DecodeString(v.Body)
	if e != nil {
		return nil, e
	}
	n.requests++
	h := http.Header{}
	for k, value := range v.Headers {
		h.Set(k, value)
	}
	return &http.Response{StatusCode: v.Status, Status: fmt.Sprintf("%d", v.Status), Header: h, Body: io.NopCloser(bytes.NewReader(data)), ContentLength: int64(len(data)), Request: req}, nil
}
