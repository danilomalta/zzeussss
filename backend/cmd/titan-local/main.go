package main

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"titansystem-backend/internal/localdb"
	"titansystem-backend/internal/localdb/localsetup"
)

type stationFile struct {
	TenantID   string `json:"tenant_id"`
	StoreID    string `json:"store_id"`
	DeviceID   string `json:"device_id"`
	PrivateKey string `json:"private_key"`
}

func main() {
	if len(os.Args) < 2 || os.Args[1] != "init" {
		fmt.Fprintln(os.Stderr, "Uso: titan-local init --db CAMINHO_NOVO.sqlite --station CAMINHO_NOVO.station --empresa NOME --loja NOME --dono NOME")
		os.Exit(2)
	}
	if err := initStation(os.Args[2:], os.Stdin, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "Instalação local:", err)
		os.Exit(1)
	}
}

func initStation(args []string, passwordReader io.Reader, output io.Writer) error {
	options := flag.NewFlagSet("init", flag.ContinueOnError)
	options.SetOutput(io.Discard)
	dbPath := options.String("db", "", "novo arquivo SQLite")
	stationPath := options.String("station", "", "arquivo privado do aparelho")
	tenantName := options.String("empresa", "", "nome da empresa")
	storeName := options.String("loja", "", "nome da loja")
	ownerName := options.String("dono", "", "nome do dono")
	if err := options.Parse(args); err != nil {
		return err
	}
	if *dbPath == "" || *stationPath == "" || *tenantName == "" || *storeName == "" || *ownerName == "" || options.NArg() != 0 {
		return errors.New("informe todos os caminhos e nomes")
	}
	if _, err := os.Lstat(*dbPath); err == nil {
		return errors.New("arquivo SQLite já existe; não sobrescreva instalações")
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if _, err := os.Lstat(*stationPath); err == nil {
		return errors.New("arquivo privado já existe; não sobrescreva chaves")
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	passwordBytes, err := io.ReadAll(io.LimitReader(passwordReader, 73))
	if err != nil {
		return err
	}
	password := string(passwordBytes)
	if len(password) > 0 && password[len(password)-1] == '\n' {
		password = password[:len(password)-1]
	}
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return err
	}
	if err = os.MkdirAll(filepath.Dir(*stationPath), 0700); err != nil {
		return err
	}
	station, err := os.OpenFile(*stationPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return err
	}
	closeStation := func() error { return station.Close() }
	// A chave permanece em disco mesmo se o banco falhar. Não reutilizar o
	// caminho em nova instalação sem investigação da falha.
	db, err := localdb.Open(context.Background(), *dbPath)
	if err != nil {
		_ = closeStation()
		return err
	}
	defer db.Close()
	result, err := localsetup.Initialize(context.Background(), db, localsetup.Input{
		TenantName: *tenantName, StoreName: *storeName, OwnerName: *ownerName,
		DeviceName: "Caixa principal", Password: password, PublicKey: public,
	})
	if err != nil {
		_ = closeStation()
		return err
	}
	bytes, err := json.Marshal(stationFile{result.TenantID, result.StoreID, result.DeviceID, base64.RawURLEncoding.EncodeToString(private)})
	if err != nil {
		_ = closeStation()
		return err
	}
	if _, err = station.Write(bytes); err != nil {
		_ = closeStation()
		return err
	}
	if err = station.Sync(); err != nil {
		_ = closeStation()
		return err
	}
	if err = closeStation(); err != nil {
		return err
	}
	_, err = fmt.Fprintf(output, "Empresa: %s\nLoja: %s\nDono (ID para login): %s\nAparelho: %s\n", result.TenantID, result.StoreID, result.OwnerID, result.DeviceID)
	return err
}
