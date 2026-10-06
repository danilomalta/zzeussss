package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"titansystem-backend/internal/localdb"
	"titansystem-backend/internal/localdb/backup"
	"titansystem-backend/internal/localdb/identity"
	"titansystem-backend/internal/localdb/stationfile"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := runContext(ctx, os.Args[1:], os.Stdout); err != nil && !errors.Is(err, context.Canceled) {
		fmt.Fprintln(os.Stderr, "Titan backup: operação não concluída; confira arquivos, versão, chave e contexto.")
		os.Exit(1)
	}
}

func run(args []string, out io.Writer) error {
	return runContext(context.Background(), args, out)
}

func runContext(parent context.Context, args []string, out io.Writer) error {
	if len(args) == 0 {
		return errors.New("command required")
	}
	command := args[0]
	fs := flag.NewFlagSet("titan-backup", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	keyPath := fs.String("key", "", "private backup key file")
	var dbPath, stationPath, archive, destination, tenant, store, device *string
	var interval *time.Duration
	switch command {
	case "key-init":
	case "create", "watch":
		dbPath = fs.String("db", "", "existing database")
		stationPath = fs.String("station", "", "private station file")
		destination = fs.String("out", "", "new encrypted backup file")
		if command == "watch" {
			interval = fs.Duration("interval", time.Hour, "backup interval, 1m to 24h")
		}
	case "verify", "restore":
		archive = fs.String("archive", "", "encrypted backup file")
		tenant = fs.String("tenant", "", "expected tenant")
		store = fs.String("store", "", "expected store")
		device = fs.String("device", "", "expected device")
		if command == "restore" {
			destination = fs.String("out", "", "new database path")
		}
	default:
		return errors.New("unknown command")
	}
	if err := fs.Parse(args[1:]); err != nil || fs.NArg() != 0 || *keyPath == "" {
		return errors.New("invalid flags")
	}
	if err := parent.Err(); err != nil {
		return err
	}
	if command == "key-init" {
		if err := backup.InitKey(*keyPath); err != nil {
			return err
		}
		_, err := fmt.Fprintln(out, "Chave de backup criada em arquivo privado. Preserve uma cópia segura separada.")
		return err
	}
	key, err := backup.ReadKey(*keyPath)
	if err != nil {
		return err
	}
	defer func() {
		for i := range key {
			key[i] = 0
		}
	}()
	if command == "watch" {
		if *dbPath == "" || *stationPath == "" || *destination == "" || *interval < time.Minute || *interval > 24*time.Hour {
			return errors.New("invalid backup schedule")
		}
		info, err := os.Lstat(*destination)
		if err != nil || !info.IsDir() || info.Mode().Perm() != 0700 || info.Mode()&os.ModeSymlink != 0 {
			return backup.ErrBackup
		}
		for {
			if err := parent.Err(); err != nil {
				return err
			}
			id, err := localdb.NewID()
			if err != nil {
				return err
			}
			path := filepath.Join(*destination, "backup-"+id+".tytbak")
			// Re-prove the station on each iteration. A revoked station stops.
			if err := runContext(parent, []string{"create", "--db", *dbPath, "--station", *stationPath, "--key", *keyPath, "--out", path}, out); err != nil {
				return err
			}
			timer := time.NewTimer(*interval)
			select {
			case <-parent.Done():
				timer.Stop()
				return parent.Err()
			case <-timer.C:
			}
		}
	}
	ctx, cancel := context.WithTimeout(parent, 5*time.Minute)
	defer cancel()
	if command == "create" {
		if *dbPath == "" || *stationPath == "" || *destination == "" {
			return errors.New("missing flags")
		}
		db, scope, _, err := stationfile.OpenVerified(ctx, *dbPath, *stationPath)
		if err != nil {
			return err
		}
		defer db.Close()
		if err := backup.Create(ctx, db, scope, key, *destination); err != nil {
			return err
		}
	} else {
		if *archive == "" || *tenant == "" || *store == "" || *device == "" {
			return errors.New("missing flags")
		}
		scope := identity.DeviceContext{TenantID: *tenant, StoreID: *store, DeviceID: *device}
		if command == "verify" {
			err = backup.Verify(ctx, *archive, scope, key)
		} else {
			err = backup.Restore(ctx, *archive, *destination, scope, key)
		}
		if err != nil {
			return err
		}
	}
	_, err = fmt.Fprintln(out, "Operação concluída. Nenhum arquivo existente foi substituído.")
	return err
}
