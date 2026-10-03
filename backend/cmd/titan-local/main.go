package main

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"titansystem-backend/internal/localapi"
	"titansystem-backend/internal/localdb"
	"titansystem-backend/internal/localdb/identity"
	"titansystem-backend/internal/localdb/localsetup"
)

type stationFile struct {
	TenantID   string `json:"tenant_id"`
	StoreID    string `json:"store_id"`
	DeviceID   string `json:"device_id"`
	PrivateKey string `json:"private_key"`
}

func main() {
	if len(os.Args) < 2 || (os.Args[1] != "init" && os.Args[1] != "serve") {
		fmt.Fprintln(os.Stderr, "Uso: titan-local init|serve --db CAMINHO.sqlite --station CAMINHO.station")
		os.Exit(2)
	}
	var err error
	if os.Args[1] == "init" {
		err = initStation(os.Args[2:], os.Stdin, os.Stdout)
	} else {
		err = serveStation(os.Args[2:])
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "Titan local:", err)
		os.Exit(1)
	}
}

func serveStation(args []string) error {
	options := flag.NewFlagSet("serve", flag.ContinueOnError)
	options.SetOutput(io.Discard)
	dbPath := options.String("db", "", "SQLite existente")
	stationPath := options.String("station", "", "arquivo privado do aparelho")
	issuerKeys := options.String("issuer-keys", "", "JSON local de chaves publicas emissoras confiaveis")
	port := options.Int("port", 8181, "porta local entre 1 e 65535; escuta somente em 127.0.0.1")
	if err := options.Parse(args); err != nil {
		return err
	}
	if *dbPath == "" || *stationPath == "" || options.NArg() != 0 {
		return errors.New("informe banco e aparelho")
	}
	if *port < 1 || *port > 65535 {
		return errors.New("porta local deve estar entre 1 e 65535")
	}
	verifier, err := readIssuerVerifier(*issuerKeys)
	if err != nil {
		return err
	}
	db, device, err := openVerified(context.Background(), *dbPath, *stationPath)
	if err != nil {
		return err
	}
	defer db.Close()
	app, err := localapi.NewWithVerifier(db, device, verifier)
	if err != nil {
		return err
	}
	address := fmt.Sprintf("127.0.0.1:%d", *port)
	fmt.Fprintf(os.Stdout, "API local em http://%s/local/v1/health\n", address)
	if verifier == nil {
		fmt.Fprintln(os.Stdout, "Cadastros indisponiveis: configure --issuer-keys e instale um contrato valido. Login e consultas continuam disponiveis.")
	}
	return app.Listen(address)
}

func openVerified(ctx context.Context, dbPath, stationPath string) (*sql.DB, identity.DeviceContext, error) {
	stat, err := os.Lstat(dbPath)
	if err != nil {
		return nil, identity.DeviceContext{}, err
	}
	if !stat.Mode().IsRegular() || stat.Mode().Perm() != 0600 {
		return nil, identity.DeviceContext{}, errors.New("banco não é arquivo regular 0600")
	}
	stat, err = os.Lstat(stationPath)
	if err != nil {
		return nil, identity.DeviceContext{}, err
	}
	if !stat.Mode().IsRegular() || stat.Mode().Perm() != 0600 {
		return nil, identity.DeviceContext{}, errors.New("arquivo privado não é regular 0600")
	}
	bytes, err := os.ReadFile(stationPath)
	if err != nil {
		return nil, identity.DeviceContext{}, err
	}
	var station stationFile
	if err = json.Unmarshal(bytes, &station); err != nil {
		return nil, identity.DeviceContext{}, errors.New("arquivo do aparelho inválido")
	}
	key, err := base64.RawURLEncoding.DecodeString(station.PrivateKey)
	if err != nil || len(key) != ed25519.PrivateKeySize {
		return nil, identity.DeviceContext{}, errors.New("chave do aparelho inválida")
	}
	db, err := localdb.Open(ctx, dbPath)
	if err != nil {
		return nil, identity.DeviceContext{}, err
	}
	device := identity.DeviceContext{TenantID: station.TenantID, StoreID: station.StoreID, DeviceID: station.DeviceID}
	challenge, err := identity.IssueDeviceChallenge(ctx, db, device)
	if err == nil {
		_, err = identity.CompleteDeviceChallenge(ctx, db, device, challenge.ID, ed25519.Sign(ed25519.PrivateKey(key), identity.DeviceAuthMessage(device, challenge)))
	}
	if err != nil {
		_ = db.Close()
		return nil, identity.DeviceContext{}, err
	}
	return db, device, nil
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
