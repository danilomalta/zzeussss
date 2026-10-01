package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"
	"time"

	"titansystem-backend/internal/localdb/incoming"
	"titansystem-backend/internal/localdb/localauth"
	"titansystem-backend/internal/localdb/stationfile"
)

var errPeerConfig = errors.New("configuração administrativa inválida")

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	if err := runPeer(ctx, os.Args[1:], os.Stdin, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "Titan peer: operação não concluída; confira credenciais, aprovações e arquivos. Não substitua arquivos privados existentes.")
		os.Exit(1)
	}
}

func runPeer(ctx context.Context, args []string, password io.Reader, output io.Writer) error {
	if ctx == nil || output == nil || len(args) == 0 {
		return errPeerConfig
	}
	if args[0] == "inspect" {
		return inspectPeer(args[1:], output)
	}
	if args[0] != "key-init" && args[0] != "export" {
		return errPeerConfig
	}
	options := flag.NewFlagSet(args[0], flag.ContinueOnError)
	options.SetOutput(io.Discard)
	dbPath := options.String("db", "", "SQLite existente")
	stationPath := options.String("station", "", "aparelho privado existente")
	encryptionPath := options.String("encryption", "", "arquivo X25519")
	var ownerID, outPath string
	var revision int64 = 1
	if args[0] == "key-init" {
		options.StringVar(&ownerID, "owner", "", "ID do dono; senha via stdin")
	} else {
		options.StringVar(&outPath, "out", "", "novo descritor público")
		options.Int64Var(&revision, "revision", 1, "revisão do vínculo proposto")
	}
	if err := options.Parse(args[1:]); err != nil || options.NArg() != 0 || *dbPath == "" || *stationPath == "" || *encryptionPath == "" {
		return errPeerConfig
	}
	if args[0] == "key-init" && ownerID == "" || args[0] == "export" && (outPath == "" || revision < 1) {
		return errPeerConfig
	}
	db, device, signing, err := stationfile.OpenVerified(ctx, *dbPath, *stationPath)
	if err != nil {
		return err
	}
	defer db.Close()
	if args[0] == "key-init" {
		if password == nil {
			return errPeerConfig
		}
		raw, err := io.ReadAll(io.LimitReader(password, 74))
		if err != nil {
			return errPeerConfig
		}
		if len(raw) > 0 && raw[len(raw)-1] == '\n' {
			raw = raw[:len(raw)-1]
		}
		if len(raw) > 72 {
			return errPeerConfig
		}
		session, err := localauth.Login(ctx, db, device, ownerID, string(raw))
		if err != nil {
			return err
		}
		defer func() {
			cleanup, cancel := context.WithTimeout(context.WithoutCancel(ctx), 2*time.Second)
			defer cancel()
			_ = localauth.Logout(cleanup, db, session.Token)
		}()
		session, err = localauth.Resolve(ctx, db, session.Token)
		if err != nil {
			return err
		}
		if err := createOwnerEncryption(ctx, db, session.Actor, device, signing, *encryptionPath); err != nil {
			return err
		}
		_, err = fmt.Fprintln(output, "Chave privada criada; vínculo público local aprovado pelo dono. Nenhum parceiro remoto foi autorizado.")
		return err
	}
	encryption, err := incoming.LoadEncryptionKey(*encryptionPath, device)
	if err != nil {
		return err
	}
	descriptor, err := incoming.CreatePublicDescriptor(device, revision, encryption, signing)
	if err != nil {
		return err
	}
	return writePublicDescriptor(outPath, descriptor, output)
}
