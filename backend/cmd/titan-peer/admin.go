package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"titansystem-backend/internal/localdb/incoming"
	"titansystem-backend/internal/localdb/localauth"
	"titansystem-backend/internal/localdb/stationfile"
)

func runPeerAdmin(ctx context.Context, args []string, password io.Reader, output io.Writer) error {
	if ctx == nil || len(args) == 0 || password == nil || output == nil {
		return errPeerConfig
	}
	command := args[0]
	if command != "trust" && command != "approve" && command != "revoke" {
		return errPeerConfig
	}
	options := flag.NewFlagSet(command, flag.ContinueOnError)
	options.SetOutput(io.Discard)
	dbPath := options.String("db", "", "SQLite existente")
	stationPath := options.String("station", "", "aparelho privado existente")
	ownerID := options.String("owner", "", "dono; senha pelo stdin")
	descriptorPath := options.String("in", "", "descritor público do parceiro já pareado")
	signingPrint := options.String("signing-sha256", "", "impressão Ed25519 conferida por outro canal")
	encryptionPrint := options.String("encryption-sha256", "", "impressão X25519 conferida por outro canal")
	var eventsText string
	if command != "trust" {
		options.StringVar(&eventsText, "events", "", "tipos de evento separados por vírgula")
	}
	if err := options.Parse(args[1:]); err != nil || options.NArg() != 0 || *dbPath == "" || *stationPath == "" || *ownerID == "" || *descriptorPath == "" || *signingPrint == "" || *encryptionPrint == "" {
		return errPeerConfig
	}
	var events []string
	if command != "trust" {
		if eventsText == "" {
			return errPeerConfig
		}
		events = strings.Split(eventsText, ",")
		seen := make(map[string]bool)
		for _, event := range events {
			if event == "sale.cancelled" || event == "cash.movement" {
				if seen[event] {
					return errPeerConfig
				}
				seen[event] = true
				continue
			}
			switch event {
			case "sale.committed", "stock.operation", "cash.open", "cash.close":
			default:
				return errPeerConfig
			}
			if seen[event] {
				return errPeerConfig
			}
			seen[event] = true
		}
	}
	body, err := readPeerPublic(*descriptorPath)
	if err != nil {
		return err
	}
	_, prints, err := incoming.ParsePublicDescriptor(body)
	if err != nil || prints.SigningSHA256 != *signingPrint || prints.EncryptionSHA256 != *encryptionPrint {
		return errPeerConfig
	}
	db, device, _, err := stationfile.OpenVerified(ctx, *dbPath, *stationPath)
	if err != nil {
		return err
	}
	defer db.Close()
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
	session, err := localauth.Login(ctx, db, device, *ownerID, string(raw))
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
	if command == "revoke" {
		err = incoming.RevokePublicPeerEvents(ctx, db, session.Actor, device, body, *signingPrint, *encryptionPrint, events)
	} else {
		err = incoming.ApprovePublicPeer(ctx, db, session.Actor, device, body, *signingPrint, *encryptionPrint, events)
	}
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(output, "Decisão do dono registrada. Apenas os eventos explicitamente selecionados foram alterados; o pareamento do aparelho permanece separado.")
	return err
}

func readPeerPublic(path string) ([]byte, error) {
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || info.Size() > 16*1024 {
		return nil, errPeerConfig
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, errPeerConfig
	}
	defer file.Close()
	opened, err := file.Stat()
	if err != nil || !opened.Mode().IsRegular() || !os.SameFile(info, opened) {
		return nil, errPeerConfig
	}
	body, err := io.ReadAll(io.LimitReader(file, 16*1024+1))
	if err != nil || len(body) > 16*1024 {
		return nil, errPeerConfig
	}
	return body, nil
}
