// Local-only import of accountant-provided rules. Never calls Focus or emits.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
	"titansystem-backend/internal/localdb/fiscal"
)

var errImport = errors.New("import refused; verify the file, local owner session and saved revisions")
var words = map[string]string{"pt-BR": "Regras publicadas em homologação. Aprovação contábil não é presumida; produção bloqueada.", "en": "Rules published for homologation. Accountant approval is never inferred; production blocked.", "es": "Reglas publicadas para homologación. No se presume aprobación contable; producción bloqueada.", "fr": "Règles publiées en homologation. Aucune approbation comptable présumée ; production bloquée.", "zh-CN": "规则已发布到测试环境。不默认会计师已批准；生产环境被阻止。"}

func main() {
	info, e := os.Stdin.Stat()
	if e != nil || info.Mode()&os.ModeCharDevice != 0 {
		fmt.Fprintln(os.Stderr, errImport)
		os.Exit(1)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Minute)
	defer cancel()
	if run(ctx, os.Args[1:], os.Stdin, os.Stdout) != nil {
		fmt.Fprintln(os.Stderr, errImport)
		os.Exit(1)
	}
}
func run(ctx context.Context, args []string, secret io.Reader, out io.Writer) error {
	fs := flag.NewFlagSet("titan-tax-import", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	input := fs.String("in", "", "versioned accountant file")
	base := fs.String("api", "", "loopback API origin on this customer host")
	lang := fs.String("lang", "pt-BR", "language")
	if fs.Parse(args) != nil || fs.NArg() != 0 || words[*lang] == "" || secret == nil || out == nil {
		return errImport
	}
	u, e := url.Parse(*base)
	if e != nil || u.User != nil || u.Host == "" || u.RawQuery != "" || u.Fragment != "" || (u.Path != "" && u.Path != "/") {
		return errImport
	}
	// No remote destination, DNS lookup, proxy, redirect or default authority.
	ip := net.ParseIP(u.Hostname())
	if ip == nil || !ip.IsLoopback() || (u.Scheme != "http" && u.Scheme != "https") {
		return errImport
	}
	info, e := os.Lstat(*input)
	if e != nil || !info.Mode().IsRegular() || info.Size() > 1024*1024 {
		return errImport
	}
	file, e := os.Open(*input)
	if e != nil {
		return errImport
	}
	defer file.Close()
	opened, e := file.Stat()
	if e != nil || !os.SameFile(info, opened) {
		return errImport
	}
	raw, e := io.ReadAll(io.LimitReader(file, 1024*1024+1))
	if e != nil {
		return errImport
	}
	table, e := fiscal.DecodeTaxTable(raw)
	if e != nil {
		return errImport
	}
	token, e := io.ReadAll(io.LimitReader(secret, 4097))
	if e != nil || len(token) == 0 || len(token) > 4096 {
		return errImport
	}
	defer clear(token)
	for _, b := range token {
		if b <= 32 || b >= 127 {
			return errImport
		}
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = nil
	defer transport.CloseIdleConnections()
	client := &http.Client{Transport: transport, Timeout: 20 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}

	req, e := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimSuffix(*base, "/")+"/local/v1/fiscal/tax-tables/import", bytes.NewReader(raw))
	if e != nil {
		return errImport
	}
	req.Header.Set("Authorization", "Bearer "+string(token))
	req.Header.Set("Content-Type", "application/json")
	response, e := client.Do(req)
	if e != nil {
		return errImport
	}
	defer response.Body.Close()
	payload, e := io.ReadAll(io.LimitReader(response.Body, 1024*1024+1))
	if e != nil || len(payload) > 1024*1024 || response.StatusCode != 201 {
		return errImport
	}
	var saved []fiscal.TaxPolicy
	if json.Unmarshal(payload, &saved) != nil || len(saved) != len(table.Policies) {
		return errImport
	}
	for i, p := range saved {
		in := table.Policies[i]
		if p.Input.Rule.ID != in.Rule.ID || p.Input.OperationID != in.OperationID || p.Revision != in.ExpectedRevision+1 || p.Input.AccountantApproved() || p.Input.Review == nil || p.Input.Review.FileSHA256 != in.Review.FileSHA256 {
			return errImport
		}
	}
	return json.NewEncoder(out).Encode(struct {
		Published int    `json:"published"`
		Message   string `json:"message"`
	}{len(saved), words[*lang]})
}
