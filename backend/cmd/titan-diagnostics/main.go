// titan-diagnostics is an explicit local operator command, never an HTTP server.
package main

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
	"titansystem-backend/internal/deployment"
	"titansystem-backend/internal/fiscalproduction"
	"titansystem-backend/internal/localdb"
	"titansystem-backend/internal/localdb/backup"
	"titansystem-backend/internal/localdb/identity"
)

var version = "development"
var errDiagnostic = errors.New("diagnóstico recusado; confira os caminhos privados e o contexto na instalação")

type report struct {
	Version  int    `json:"version"`
	Build    string `json:"build"`
	Schema   int    `json:"schema_version"`
	Observed int64  `json:"observed_unix"`
	Backup   int64  `json:"backup_unix"`
	Interval int64  `json:"backup_interval_seconds"`
	Vault    string `json:"vault"`
	Exported bool   `json:"key_exported"`
	TLS      string `json:"tls"`
}

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	if run(ctx, os.Args[1:], os.Stdout) != nil {
		fmt.Fprintln(os.Stderr, errDiagnostic)
		os.Exit(1)
	}
}
func privatePath(path string, dir bool) bool {
	if !filepath.IsAbs(path) || filepath.Clean(path) != path {
		return false
	}
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil || resolved != path {
		return false
	}
	i, err := os.Lstat(path)
	if err != nil {
		return false
	}
	if dir {
		return i.IsDir() && i.Mode().Perm() == 0700
	}
	return i.Mode().IsRegular() && i.Mode().Perm() == 0600
}
func run(ctx context.Context, args []string, out io.Writer) error {
	fs := flag.NewFlagSet("titan-diagnostics", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	dbPath := fs.String("db", "", "existing private database")
	profile := fs.String("deployment", "", "private deployment profile (omitted: loopback default)")
	proxy := fs.Bool("reverse-proxy-tls", false, "declared customer proxy, loopback only")
	vault := fs.String("vault", "", "existing customer vault")
	master := fs.String("key-file", "", "private customer master key")
	copy := fs.String("key-copy", "", "separate exported master key")
	dir := fs.String("backup-dir", "", "customer backup directory")
	keyFile := fs.String("backup-key", "", "customer backup key")
	keyCopy := fs.String("backup-key-copy", "", "separate private backup key copy")
	interval := fs.Duration("interval", time.Hour, "configured backup interval")
	tenant := fs.String("tenant", "", "expected tenant")
	store := fs.String("store", "", "expected store")
	device := fs.String("device", "", "expected device")
	if fs.Parse(args) != nil || fs.NArg() != 0 || out == nil || !privatePath(*dbPath, false) || *interval < time.Minute || *interval > 24*time.Hour || *tenant == "" || *store == "" || *device == "" {
		return errDiagnostic
	}
	scope := identity.DeviceContext{TenantID: *tenant, StoreID: *store, DeviceID: *device}
	u := url.URL{Scheme: "file", Path: *dbPath}
	db, err := sql.Open("sqlite", u.String()+"?mode=ro")
	if err != nil {
		return errDiagnostic
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	if localdb.ValidateSchema(ctx, db) != nil {
		return errDiagnostic
	}
	var schema, approved int
	if db.QueryRowContext(ctx, `SELECT count(*) FROM schema_migrations`).Scan(&schema) != nil || db.QueryRowContext(ctx, `SELECT count(*) FROM device_pairings WHERE tenant_id=? AND store_id=? AND device_id=? AND status='approved'`, *tenant, *store, *device).Scan(&approved) != nil || approved != 1 {
		return errDiagnostic
	}
	r := report{Version: 1, Build: version, Schema: schema, Observed: time.Now().Unix(), Interval: int64(interval.Seconds()), Vault: "unavailable", TLS: "invalid"}
	p := deployment.Local(8181)
	if *profile != "" {
		p, err = deployment.Load(*profile)
	}
	if err == nil {
		if tls, e := p.TLS(time.Now()); e == nil {
			if tls != nil {
				r.TLS = "direct_tls"
			} else {
				r.TLS = "loopback"
			}
			if *proxy {
				if p.Mode == "local" {
					r.TLS = "proxy_declared"
				} else {
					r.TLS = "invalid"
				}
			}
		}
	}
	if privatePath(*vault, false) {
		if available, exported := fiscalproduction.CustomerVaultDiagnostic(ctx, *vault, *master, *copy); available {
			r.Vault = "available"
			r.Exported = exported
		}
	}
	var key []byte
	e := errDiagnostic
	if privatePath(*keyFile, false) {
		key, e = backup.ReadKey(*keyFile)
	}
	defer clear(key)
	if e == nil {
		var copied []byte
		ce := errDiagnostic
		if privatePath(*keyCopy, false) {
			copied, ce = backup.ReadKey(*keyCopy)
		}
		defer clear(copied)
		sourceInfo, se := os.Stat(*keyFile)
		copyInfo, cse := os.Stat(*keyCopy)
		r.Exported = r.Exported && ce == nil && se == nil && cse == nil && !os.SameFile(sourceInfo, copyInfo) && bytes.Equal(key, copied)
		if privatePath(*dir, true) {
			f, e := os.Open(*dir)
			if e != nil {
				return errDiagnostic
			}
			entries, re := f.ReadDir(1001)
			f.Close()
			if re != nil && re != io.EOF {
				return errDiagnostic
			}
			if len(entries) > 1000 {
				return errDiagnostic
			}
			var newest string
			var stamp int64
			for _, entry := range entries {
				if !strings.HasSuffix(entry.Name(), ".tytbak") {
					continue
				}
				path := filepath.Join(*dir, entry.Name())
				if !privatePath(path, false) {
					continue
				}
				info, e := os.Stat(path)
				if e != nil {
					return errDiagnostic
				}
				t := info.ModTime().Unix()
				if t > stamp {
					stamp = t
					newest = path
				}
			}
			// An invalid latest archive is a warning, never hidden by an older valid one.
			if newest != "" && stamp <= r.Observed && backup.Verify(ctx, newest, scope, key) == nil {
				r.Backup = stamp
			}
		}
	} else {
		r.Exported = false
	}
	if ctx.Err() != nil {
		return errDiagnostic
	}
	return json.NewEncoder(out).Encode(r)
}
