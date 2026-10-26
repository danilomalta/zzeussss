// Package traderelay transports only explicitly exported, signed documents.
// It never approves a purchase, creates a decision or reads a business database.
package traderelay

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"titansystem-backend/internal/localdb/supplycycle"
)

var ErrInvalid = errors.New("invalid private job, destination or saved attempt")
var ErrContext = errors.New("authenticated identity or pinned station does not match job")
var ErrUncertain = errors.New("delivery not confirmed; preserve state and recover before explicit retry")
var ErrLocked = errors.New("another process holds the private attempt lock")
var ErrTransport = errors.New("transport failed; no credentials or response bodies logged")

type HTTPError struct{ Status int }

func (e HTTPError) Error() string { return fmt.Sprintf("API refused request: HTTP %d", e.Status) }

var idPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:-]{0,127}$`)

func key(s string) bool { return idPattern.MatchString(s) && s != "." && s != ".." }

type Endpoint struct {
	URL         string                 `json:"url"`
	Party       supplycycle.TradeParty `json:"party"`
	ActorID     string                 `json:"actor_id"`
	Fingerprint string                 `json:"fingerprint_sha256"`
}
type Job struct {
	Version     int      `json:"version"`
	Source      Endpoint `json:"source"`
	Target      Endpoint `json:"target"`
	MessageID   string   `json:"message_id"`
	PeerID      string   `json:"target_peer_id"`
	OperationID string   `json:"operation_id"`
}
type State struct {
	Version         int    `json:"version"`
	JobSHA256       string `json:"job_sha256"`
	EnvelopeSHA256  string `json:"envelope_sha256"`
	Status          string `json:"status"`
	SourcePublicKey []byte `json:"source_public_key"`
}
type Result struct {
	Status      string `json:"status"`
	OperationID string `json:"operation_id"`
	MessageID   string `json:"message_id"`
	Posts       int    `json:"posts"`
	SourceCalls int    `json:"source_http_calls"`
	TargetCalls int    `json:"target_http_calls"`
}

func validEndpoint(e Endpoint) bool {
	u, err := url.Parse(e.URL)
	if err != nil || u.User != nil || u.RawQuery != "" || u.Fragment != "" || u.Opaque != "" || u.RawPath != "" || u.Path != "/local/v1" || u.Hostname() == "" || strings.ContainsAny(e.URL, "\r\n\t\\") {
		return false
	}
	if u.Scheme != "https" && (u.Scheme != "http" || (u.Hostname() != "127.0.0.1" && u.Hostname() != "::1")) {
		return false
	}
	h, err := hex.DecodeString(e.Fingerprint)
	return err == nil && len(h) == 32 && hex.EncodeToString(h) == e.Fingerprint && key(e.ActorID) && key(e.Party.TenantID) && key(e.Party.StoreID) && key(e.Party.DeviceID)
}
func (j Job) Validate() error {
	if j.Version != 1 || !validEndpoint(j.Source) || !validEndpoint(j.Target) || j.Source.URL == j.Target.URL || j.Source.Party.TenantID == j.Target.Party.TenantID || !key(j.MessageID) || !key(j.PeerID) || !key(j.OperationID) {
		return ErrInvalid
	}
	return nil
}
func digest(v any) string {
	b, _ := json.Marshal(v)
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}

// Reject duplicate keys at every depth, unknown fields and trailing JSON.
func decode(b []byte, out any) error {
	d := json.NewDecoder(bytes.NewReader(b))
	var walk func(int) error
	walk = func(depth int) error {
		if depth > 32 {
			return ErrInvalid
		}
		v, e := d.Token()
		if e != nil {
			return e
		}
		if x, ok := v.(json.Delim); ok {
			switch x {
			case '{':
				seen := map[string]bool{}
				for d.More() {
					k, e := d.Token()
					if e != nil {
						return e
					}
					s, ok := k.(string)
					if !ok || seen[s] {
						return ErrInvalid
					}
					seen[s] = true
					if e = walk(depth + 1); e != nil {
						return e
					}
				}
			case '[':
				for d.More() {
					if e = walk(depth + 1); e != nil {
						return e
					}
				}
			default:
				return ErrInvalid
			}
			_, e = d.Token()
			return e
		}
		return nil
	}
	if walk(0) != nil {
		return ErrInvalid
	}
	if _, e := d.Token(); e != io.EOF {
		return ErrInvalid
	}
	d = json.NewDecoder(bytes.NewReader(b))
	d.DisallowUnknownFields()
	if d.Decode(out) != nil {
		return ErrInvalid
	}
	return nil
}
func readPrivate(name string, limit int64) ([]byte, error) {
	info, e := os.Lstat(name)
	if e != nil || !info.Mode().IsRegular() || info.Mode().Perm() != 0600 || info.Size() > limit {
		return nil, ErrInvalid
	}
	f, e := os.Open(name)
	if e != nil {
		return nil, ErrInvalid
	}
	defer f.Close()
	opened, e := f.Stat()
	if e != nil || !os.SameFile(info, opened) {
		return nil, ErrInvalid
	}
	b, e := io.ReadAll(io.LimitReader(f, limit+1))
	if e != nil || int64(len(b)) > limit {
		return nil, ErrInvalid
	}
	return b, nil
}
func ReadJob(name string) (Job, error) {
	var j Job
	b, e := readPrivate(name, 16384)
	if e != nil {
		return j, e
	}
	if e = decode(b, &j); e != nil {
		return j, e
	}
	return j, j.Validate()
}
func ReadToken(name string) (string, error) {
	b, e := readPrivate(name, 4096)
	if e != nil {
		return "", e
	}
	s := strings.TrimSpace(string(b))
	if s == "" {
		return "", ErrInvalid
	}
	for _, c := range s {
		if c < 33 || c > 126 {
			return "", ErrInvalid
		}
	}
	return s, nil
}

type api struct {
	e      Endpoint
	token  string
	client *http.Client
	calls  *int
}

func (a api) call(ctx context.Context, method, path string, body, out any) error {
	*a.calls++
	var reader io.Reader
	if body != nil {
		b, e := json.Marshal(body)
		if e != nil {
			return ErrInvalid
		}
		reader = bytes.NewReader(b)
	}
	req, e := http.NewRequestWithContext(ctx, method, a.e.URL+path, reader)
	if e != nil {
		return ErrInvalid
	}
	req.Header.Set("Authorization", "Bearer "+a.token)
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, e := a.client.Do(req)
	if e != nil {
		return ErrTransport
	}
	defer resp.Body.Close()
	b, e := io.ReadAll(io.LimitReader(resp.Body, 128*1024+1))
	if e != nil || len(b) > 128*1024 {
		return ErrTransport
	}
	if resp.StatusCode != 200 && resp.StatusCode != 201 {
		return HTTPError{resp.StatusCode}
	}
	if out != nil && json.Unmarshal(b, out) != nil {
		return ErrTransport
	}
	return nil
}
func (a api) identity(ctx context.Context) (ed25519.PublicKey, error) {
	var me struct {
		TenantID string `json:"tenant_id"`
		StoreID  string `json:"store_id"`
		DeviceID string `json:"device_id"`
		ActorID  string `json:"identity_id"`
	}
	if e := a.call(ctx, "GET", "/me", nil, &me); e != nil {
		return nil, e
	}
	if (supplycycle.TradeParty{TenantID: me.TenantID, StoreID: me.StoreID, DeviceID: me.DeviceID}) != a.e.Party || me.ActorID != a.e.ActorID {
		return nil, ErrContext
	}
	var station struct {
		Party       supplycycle.TradeParty `json:"party"`
		Public      []byte                 `json:"public_key"`
		Fingerprint string                 `json:"fingerprint_sha256"`
	}
	if e := a.call(ctx, "GET", "/company-trade/identity", nil, &station); e != nil {
		return nil, e
	}
	if station.Party != a.e.Party || len(station.Public) != ed25519.PublicKeySize || supplycycle.TradeFingerprint(station.Public) != a.e.Fingerprint || station.Fingerprint != a.e.Fingerprint {
		return nil, ErrContext
	}
	return station.Public, nil
}
func (a api) ready(ctx context.Context) error {
	var r struct {
		Required *bool `json:"required"`
	}
	if e := a.call(ctx, "GET", "/company-trade/recovery", nil, &r); e != nil {
		return e
	}
	if r.Required == nil {
		return ErrTransport
	}
	if *r.Required {
		return ErrContext
	}
	return nil
}

// Run accepts a private existing job and private state directory. Each invocation
// issues at most one POST. recover never posts; uncertain send never retries.
func Run(ctx context.Context, j Job, sourceToken, targetToken, statePath, mode string) (Result, error) {
	return runBound(ctx, j, sourceToken, targetToken, statePath, mode, "")
}
func runBound(ctx context.Context, j Job, sourceToken, targetToken, statePath, mode, expectedHash string) (Result, error) {
	return runPolicy(ctx, j, sourceToken, targetToken, statePath, mode, expectedHash, false)
}
func runPolicy(ctx context.Context, j Job, sourceToken, targetToken, statePath, mode, expectedHash string, recoveryOnly bool) (Result, error) {
	out := Result{OperationID: j.OperationID, MessageID: j.MessageID}
	if recoveryOnly && mode == "retry" {
		return out, ErrInvalid
	}
	if (expectedHash != "" && !validHash(expectedHash)) || ctx == nil || j.Validate() != nil || (sourceToken == "" && mode != "recover") || targetToken == "" || (mode != "send" && mode != "recover" && mode != "retry") || !filepath.IsAbs(statePath) {
		return out, ErrInvalid
	}
	parent := filepath.Dir(statePath)
	pi, e := os.Lstat(parent)
	if e != nil || !pi.IsDir() || pi.Mode().Perm() != 0700 || pi.Mode()&os.ModeSymlink != 0 {
		return out, ErrInvalid
	}
	unlock, e := lockState(statePath + ".lock")
	if e != nil {
		return out, e
	}
	defer unlock()
	saved := State{Version: 1, JobSHA256: digest(j), Status: "prepared"}
	hasState := false
	if _, e = os.Lstat(statePath); e == nil {
		b, e := readPrivate(statePath, 4096)
		if e != nil || decode(b, &saved) != nil || saved.Version != 1 || saved.JobSHA256 != digest(j) || (saved.Status != "uncertain" && saved.Status != "confirmed") || len(saved.EnvelopeSHA256) != 64 {
			return out, ErrInvalid
		}
		hasState = true
	} else if !errors.Is(e, os.ErrNotExist) {
		return out, ErrInvalid
	}
	if hasState && expectedHash != "" && saved.EnvelopeSHA256 != expectedHash {
		return out, ErrInvalid
	}
	if mode == "send" && hasState {
		return out, ErrUncertain
	}
	if mode != "send" && !hasState {
		return out, ErrInvalid
	}
	client, closeClient := newClient()
	defer closeClient()
	source, target := api{j.Source, sourceToken, client, &out.SourceCalls}, api{j.Target, targetToken, client, &out.TargetCalls}
	// Recovery is a read at the destination; the sender may be offline. The
	// sender public key was pinned and durably captured before the first POST.
	if mode == "recover" {
		if len(saved.SourcePublicKey) != ed25519.PublicKeySize || supplycycle.TradeFingerprint(saved.SourcePublicKey) != j.Source.Fingerprint {
			return out, ErrInvalid
		}
		if _, e = target.identity(ctx); e != nil {
			return out, e
		}
		var receipt supplycycle.TradeResult
		if e = target.call(ctx, "GET", "/company-trade/operations/"+url.PathEscape(j.OperationID), nil, &receipt); e != nil {
			var code HTTPError
			if errors.As(e, &code) && code.Status == 404 {
				out.Status = "not_found"
				return out, ErrUncertain
			}
			return out, e
		}
		if receipt.Envelope == nil || receipt.Message == nil || receipt.OperationID != j.OperationID || supplycycle.TradeEnvelopeSHA256(*receipt.Envelope) != saved.EnvelopeSHA256 {
			return out, ErrUncertain
		}
		m, e := supplycycle.ParseTradeEnvelope(*receipt.Envelope, ed25519.PublicKey(saved.SourcePublicKey))
		if e != nil || m != *receipt.Message || m.ID != j.MessageID || m.Source != j.Source.Party || m.Target != j.Target.Party {
			return out, ErrUncertain
		}
		saved.Status = "confirmed"
		if e = save(statePath, saved); e != nil {
			return out, e
		}
		out.Status = "confirmed"
		return out, nil
	}
	public, e := source.identity(ctx)
	if e != nil {
		return out, e
	}
	if _, e = target.identity(ctx); e != nil {
		return out, e
	}
	var record supplycycle.TradeMessageRecord
	if e = source.call(ctx, "GET", "/company-trade/messages/"+url.PathEscape(j.MessageID), nil, &record); e != nil {
		return out, e
	}
	message, e := supplycycle.ParseTradeEnvelope(record.Envelope, public)
	if e != nil || record.Direction != "sent" || message.ID != j.MessageID || message != record.Message || message.Source != j.Source.Party || message.Target != j.Target.Party {
		return out, ErrContext
	}
	hash := supplycycle.TradeEnvelopeSHA256(record.Envelope)
	if expectedHash != "" && hash != expectedHash {
		return out, ErrContext
	}
	if hasState && saved.EnvelopeSHA256 != hash {
		return out, ErrInvalid
	}
	saved.EnvelopeSHA256 = hash
	saved.SourcePublicKey = append([]byte(nil), public...)
	confirm := func(receipt supplycycle.TradeResult) error {
		if receipt.OperationID != j.OperationID || receipt.Message == nil || *receipt.Message != message || receipt.Envelope == nil || supplycycle.TradeEnvelopeSHA256(*receipt.Envelope) != hash {
			return ErrUncertain
		}
		saved.Status = "confirmed"
		if e := save(statePath, saved); e != nil {
			return e
		}
		out.Status = "confirmed"
		return nil
	}
	var receipt supplycycle.TradeResult
	e = target.call(ctx, "GET", "/company-trade/operations/"+url.PathEscape(j.OperationID), nil, &receipt)
	if e == nil {
		e = confirm(receipt)
		return out, e
	}
	var code HTTPError
	if !errors.As(e, &code) || code.Status != 404 {
		return out, e
	}
	if mode == "recover" || recoveryOnly {
		out.Status = "not_found"
		return out, ErrUncertain
	}
	if saved.Status == "confirmed" {
		return out, ErrUncertain
	}
	if e = source.ready(ctx); e != nil {
		return out, e
	}
	if e = target.ready(ctx); e != nil {
		return out, e
	}
	// Bind the same authenticated actors immediately before the sole write.
	if _, e = source.identity(ctx); e != nil {
		return out, e
	}
	if _, e = target.identity(ctx); e != nil {
		return out, e
	}
	saved.Status = "uncertain"
	if e = save(statePath, saved); e != nil {
		return out, e
	}
	endpoint := "/company-trade/orders/import"
	if message.Kind == "decision" {
		endpoint = "/purchase-orders/" + url.PathEscape(message.OrderID) + "/company-trade/decision"
	}
	input := supplycycle.TradeImportInput{OperationID: j.OperationID, PeerID: j.PeerID, Envelope: record.Envelope}
	out.Posts = 1
	if e = target.call(ctx, "POST", endpoint, input, &receipt); e != nil {
		out.Status = "uncertain"
		return out, ErrUncertain
	}
	e = confirm(receipt)
	return out, e
}
func save(name string, v State) error { return saveJSON(name, v) }
func saveJSON(name string, v any) error {
	// Lock serializes this directory's cooperating invocations. Files contain no
	// token, envelope or personal/business document, only the immutable job hash.
	b, e := json.Marshal(v)
	if e != nil {
		return ErrInvalid
	}
	if fi, e := os.Lstat(name); e == nil && (!fi.Mode().IsRegular() || fi.Mode().Perm() != 0600) {
		return ErrInvalid
	}
	f, e := os.CreateTemp(filepath.Dir(name), ".titan-trade-")
	if e != nil {
		return e
	}
	tmp := f.Name()
	defer os.Remove(tmp)
	if _, e = f.Write(b); e == nil {
		e = f.Sync()
	}
	if closeErr := f.Close(); e == nil {
		e = closeErr
	}
	if e != nil {
		return e
	}
	if e = os.Rename(tmp, name); e != nil {
		return e
	}
	dir, e := os.Open(filepath.Dir(name))
	if e != nil {
		return e
	}
	defer dir.Close()
	return dir.Sync()
}

func newClient() (*http.Client, func()) {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = nil
	return &http.Client{Transport: transport, Timeout: 15 * time.Second, CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }}, transport.CloseIdleConnections
}
