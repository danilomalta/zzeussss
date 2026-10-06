package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"time"

	"titansystem-backend/internal/localdb/stationfile"
)

// checkStation is maintenance: it migrates an EXISTING database, proves the
// approved station and checks integrity. It is not a read-only health endpoint.
func checkStation(args []string, out io.Writer) error {
	fs := flag.NewFlagSet("check", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	path := fs.String("db", "", "banco existente")
	station := fs.String("station", "", "aparelho privado existente")
	if err := fs.Parse(args); err != nil || fs.NArg() != 0 || *path == "" || *station == "" || out == nil {
		return errors.New("configuração de verificação inválida")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	db, _, key, err := stationfile.OpenVerified(ctx, *path, *station)
	if err != nil {
		return errors.New("banco, versão ou aparelho não aprovado; verificação interrompida")
	}
	defer db.Close()
	defer clear(key)
	rows, err := db.QueryContext(ctx, "PRAGMA integrity_check")
	if err != nil {
		return errors.New("integridade não confirmada")
	}
	count := 0
	valid := true
	for rows.Next() {
		var value string
		if err := rows.Scan(&value); err != nil || value != "ok" {
			valid = false
		}
		count++
	}
	queryErr := rows.Err()
	closeErr := rows.Close()
	if queryErr != nil || closeErr != nil || !valid || count != 1 {
		return errors.New("integridade não confirmada")
	}
	rows, err = db.QueryContext(ctx, "PRAGMA foreign_key_check")
	if err != nil {
		return errors.New("referências não confirmadas")
	}
	hasViolation := rows.Next()
	queryErr = rows.Err()
	closeErr = rows.Close()
	if hasViolation || queryErr != nil || closeErr != nil {
		return errors.New("referências não confirmadas")
	}
	_, err = fmt.Fprintln(out, "Banco existente verificado, migrações compatíveis e aparelho aprovado.")
	return err
}
