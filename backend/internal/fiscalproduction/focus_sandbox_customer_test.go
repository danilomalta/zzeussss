package fiscalproduction

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"encoding/xml"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"
)

var errCustomerSandbox = errors.New("customer sandbox configuration rejected")

// Only test infrastructure: no installed credential is read by ordinary tests.
func privateSandboxFile(name string) ([]byte, error) {
	if !filepath.IsAbs(name) {
		return nil, errCustomerSandbox
	}
	info, err := os.Lstat(name)
	if err != nil || !info.Mode().IsRegular() || info.Mode().Perm()&0077 != 0 || info.Size() > 1024*1024 {
		return nil, errCustomerSandbox
	}
	f, err := os.Open(name)
	if err != nil {
		return nil, errCustomerSandbox
	}
	defer f.Close()
	opened, err := f.Stat()
	if err != nil || !os.SameFile(info, opened) {
		return nil, errCustomerSandbox
	}
	data, err := io.ReadAll(io.LimitReader(f, 1024*1024+1))
	if err != nil || len(data) > 1024*1024 {
		clear(data)
		return nil, errCustomerSandbox
	}
	return data, nil
}

// This file is a durable pre-send fence, not a replacement for issuer recovery.
// It is never deleted by this test, including on failure or timeout.
func prepareCustomerSandbox(payloadPath, attemptPath, cnpj, sale string) ([]byte, string, bool, error) {
	reject := func() ([]byte, string, bool, error) { return nil, "", false, errCustomerSandbox }
	if sale == "" || len(sale) > 128 || !filepath.IsAbs(attemptPath) || attemptPath == payloadPath {
		return reject()
	}
	body, err := privateSandboxFile(payloadPath)
	if err != nil {
		return reject()
	}
	ok := false
	defer func() {
		if !ok {
			clear(body)
		}
	}()
	var input struct {
		CNPJ     string            `json:"cnpj_emitente"`
		Items    []json.RawMessage `json:"items"`
		Payments []json.RawMessage `json:"formas_pagamento"`
	}
	if json.Unmarshal(body, &input) != nil || input.CNPJ != cnpj || len(input.Items) == 0 || len(input.Payments) == 0 {
		return reject()
	}
	ref := focusReference(cnpj, sale)
	digest := sha256.Sum256(body)
	// No payload, CNPJ, token, key or XML is written to the fence.
	marker, _ := json.Marshal(struct{ Ref, Digest string }{ref, hex.EncodeToString(digest[:])})
	f, err := os.OpenFile(attemptPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if errors.Is(err, os.ErrExist) {
		previous, readErr := privateSandboxFile(attemptPath)
		if readErr != nil || string(previous) != string(marker) {
			return reject()
		}
		ok = true
		return body, ref, false, nil
	}
	if err != nil {
		return reject()
	}
	_, writeErr := f.Write(marker)
	syncErr := f.Sync()
	closeErr := f.Close()
	dir, dirErr := os.Open(filepath.Dir(attemptPath))
	if dirErr == nil {
		dirErr = dir.Sync()
		dir.Close()
	}
	if writeErr != nil || syncErr != nil || closeErr != nil || dirErr != nil {
		return reject()
	}
	ok = true
	return body, ref, true, nil
}

func sandboxProtocolPresent(data []byte) bool {
	var doc struct {
		Protocol struct {
			Info struct {
				Number string `xml:"nProt"`
			} `xml:"infProt"`
		} `xml:"protNFe"`
	}
	return xml.Unmarshal(data, &doc) == nil && doc.Protocol.Info.Number != ""
}

func TestCustomerSandboxPreparation(t *testing.T) {
	dir := t.TempDir()
	payload := filepath.Join(dir, "request.json")
	attempt := filepath.Join(dir, "attempt.json")
	// Synthetic data only; no network and no installed credential.
	body := []byte(`{"cnpj_emitente":"synthetic-issuer","items":[{}],"formas_pagamento":[{}]}`)
	if err := os.WriteFile(payload, body, 0600); err != nil {
		t.Fatal(err)
	}
	_, ref, first, err := prepareCustomerSandbox(payload, attempt, "synthetic-issuer", "sale-1")
	if err != nil || !first || ref == "" {
		t.Fatal("first attempt must be recorded before send")
	}
	_, again, first, err := prepareCustomerSandbox(payload, attempt, "synthetic-issuer", "sale-1")
	if err != nil || first || ref != again {
		t.Fatal("retry must only query original reference")
	}
	if _, _, _, err = prepareCustomerSandbox(payload, attempt, "another-issuer", "sale-1"); err == nil {
		t.Fatal("cross issuer accepted")
	}
	if _, _, _, err = prepareCustomerSandbox(payload, attempt, "synthetic-issuer", "sale-2"); err == nil {
		t.Fatal("changed origin accepted")
	}
	if err = os.WriteFile(payload, append(body, ' '), 0600); err != nil {
		t.Fatal(err)
	}
	if _, _, _, err = prepareCustomerSandbox(payload, attempt, "synthetic-issuer", "sale-1"); err == nil {
		t.Fatal("changed snapshot accepted")
	}
}

func TestCustomerSandboxInterruptedAttemptNeverPermitsNewSend(t *testing.T) {
	dir := t.TempDir()
	payload := filepath.Join(dir, "request")
	attempt := filepath.Join(dir, "attempt")
	if err := os.WriteFile(payload, []byte(`{"cnpj_emitente":"issuer","items":[{}],"formas_pagamento":[{}]}`), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(attempt, nil, 0600); err != nil {
		t.Fatal(err)
	}
	_, _, first, err := prepareCustomerSandbox(payload, attempt, "issuer", "sale")
	if err == nil || first {
		t.Fatal("interrupted durable fence must refuse, never permit another POST")
	}
	if err.Error() != errCustomerSandbox.Error() {
		t.Fatal("diagnostic must not contain input data")
	}
}

func TestCustomerSandboxRejectsUnsafeFilesAndMissingData(t *testing.T) {
	dir := t.TempDir()
	payload := filepath.Join(dir, "payload")
	attempt := filepath.Join(dir, "attempt")
	for _, body := range []string{`{`, `{}`, `{"cnpj_emitente":"issuer","items":[{}]}`, `{"cnpj_emitente":"issuer","items":[{}],"formas_pagamento":[]}`} {
		if err := os.WriteFile(payload, []byte(body), 0600); err != nil {
			t.Fatal(err)
		}
		if _, _, _, err := prepareCustomerSandbox(payload, attempt, "issuer", "sale"); err == nil {
			t.Fatal("incomplete payload accepted")
		}
	}
	if err := os.Chmod(payload, 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := privateSandboxFile(payload); err == nil {
		t.Fatal("public file accepted")
	}
	link := filepath.Join(dir, "link")
	if err := os.Symlink(payload, link); err != nil {
		t.Fatal(err)
	}
	if _, err := privateSandboxFile(link); err == nil {
		t.Fatal("symlink accepted")
	}
	if _, err := privateSandboxFile("relative"); err == nil {
		t.Fatal("relative path accepted")
	}
	if sandboxProtocolPresent([]byte(`<nfeProc><protNFe><infProt/></protNFe></nfeProc>`)) {
		t.Fatal("missing protocol accepted")
	}
	if !sandboxProtocolPresent([]byte(`<nfeProc><protNFe><infProt><nProt>synthetic</nProt></infProt></protNFe></nfeProc>`)) {
		t.Fatal("protocol missing")
	}
}
