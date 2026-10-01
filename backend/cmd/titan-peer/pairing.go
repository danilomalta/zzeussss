package main

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"titansystem-backend/internal/localdb/identity"
	"titansystem-backend/internal/localdb/incoming"
	"titansystem-backend/internal/localdb/localauth"
	"titansystem-backend/internal/localdb/stationfile"
)

func runPeerPairing(ctx context.Context, args []string, password io.Reader, output io.Writer) error {
	if ctx == nil || len(args) == 0 || output == nil {
		return errPeerConfig
	}
	command := args[0]
	if command != "pair-start" && command != "pair-answer" && command != "pair-finish" {
		return errPeerConfig
	}
	options := flag.NewFlagSet(command, flag.ContinueOnError)
	options.SetOutput(io.Discard)
	stationPath := options.String("station", "", "chave privada que permanece neste aparelho")
	inputPath := options.String("in", "", "arquivo público de entrada")
	var dbPath, ownerID, outPath, name, signingPrint, encryptionPrint, requesterPrint string
	if command != "pair-answer" {
		options.StringVar(&dbPath, "db", "", "SQLite existente")
		options.StringVar(&ownerID, "owner", "", "dono; senha pelo stdin")
		options.StringVar(&signingPrint, "signing-sha256", "", "impressão Ed25519 do parceiro conferida")
		options.StringVar(&encryptionPrint, "encryption-sha256", "", "impressão X25519 do parceiro conferida")
	} else {
		options.StringVar(&requesterPrint, "requester-sha256", "", "impressão Ed25519 do solicitante conferida")
	}
	if command != "pair-finish" {
		options.StringVar(&outPath, "out", "", "arquivo público novo")
	}
	if command == "pair-start" {
		options.StringVar(&name, "name", "", "nome do parceiro")
	}
	if err := options.Parse(args[1:]); err != nil || options.NArg() != 0 || *stationPath == "" || *inputPath == "" {
		return errPeerConfig
	}
	if command == "pair-answer" && requesterPrint == "" || command != "pair-answer" && (dbPath == "" || ownerID == "" || signingPrint == "" || encryptionPrint == "" || password == nil) || command != "pair-finish" && outPath == "" || command == "pair-start" && name == "" {
		return errPeerConfig
	}
	if command != "pair-finish" {
		if _, err := os.Lstat(outPath); !os.IsNotExist(err) {
			return errPeerConfig
		}
	}
	raw, err := readPeerPublic(*inputPath)
	if err != nil {
		return err
	}
	if command == "pair-answer" {
		request, err := incoming.ParsePairChallenge(raw)
		if err != nil {
			return err
		}
		now := time.Now().Unix()
		if incoming.PairRequesterFingerprint(request) != requesterPrint || request.ExpiresUnix <= now || request.ExpiresUnix > now+300 {
			return errPeerConfig
		}
		proof, err := stationfile.SignPairingForStation(*stationPath, request.Target.Binding.Device, ed25519.PublicKey(request.Target.SigningPublicKey), request.Challenge)
		if err != nil {
			return err
		}
		answer := incoming.PairAnswer{Version: 1, Request: request, Proof: proof}
		body, err := json.Marshal(answer)
		if err != nil {
			return err
		}
		if _, err := incoming.ParsePairAnswer(body); err != nil {
			return err
		}
		if err := writePairPublic(outPath, body); err != nil {
			return err
		}
		_, err = fmt.Fprintln(output, "Prova pública escrita. Nenhum aparelho foi aprovado e nenhuma chave privada foi exportada.")
		return err
	}
	var descriptor incoming.PublicDescriptor
	var answer incoming.PairAnswer
	if command == "pair-start" {
		descriptor, _, err = incoming.ParsePublicDescriptor(raw)
	} else {
		answer, err = incoming.ParsePairAnswer(raw)
		descriptor = answer.Request.Target
	}
	if err != nil {
		return err
	}
	targetRaw, err := json.Marshal(descriptor)
	if err != nil {
		return err
	}
	_, prints, err := incoming.ParsePublicDescriptor(targetRaw)
	if err != nil || prints.SigningSHA256 != signingPrint || prints.EncryptionSHA256 != encryptionPrint {
		return errPeerConfig
	}
	db, local, key, err := stationfile.OpenVerified(ctx, dbPath, *stationPath)
	if err != nil {
		return err
	}
	defer db.Close()
	target := descriptor.Binding.Device
	if target.TenantID != local.TenantID || target.StoreID != local.StoreID || target.DeviceID == local.DeviceID {
		return errPeerConfig
	}
	session, err := pairingHumanSession(ctx, db, local, ownerID, password)
	if err != nil {
		return err
	}
	defer func() {
		cleanup, cancel := context.WithTimeout(context.WithoutCancel(ctx), 2*time.Second)
		defer cancel()
		_ = localauth.Logout(cleanup, db, session.Token)
	}()
	if command == "pair-start" {
		challenge, err := identity.RequestOwnedPairing(ctx, db, session.Actor, local, target.DeviceID, name, ed25519.PublicKey(descriptor.SigningPublicKey))
		if err != nil {
			return err
		}
		request, err := incoming.SignPairChallenge(local, key, descriptor, challenge)
		if err != nil {
			return err
		}
		body, err := json.Marshal(request)
		if err != nil {
			return err
		}
		if err := writePairPublic(outPath, body); err != nil {
			return err
		}
		_, err = fmt.Fprintf(output, "Desafio público escrito; solicitação pendente por até cinco minutos.\nEd25519 SHA-256 do solicitante: %s\n", incoming.PairRequesterFingerprint(request))
		return err
	}
	if answer.Request.Requester != local || !bytes.Equal(answer.Request.RequesterKey, key.Public().(ed25519.PublicKey)) {
		return errPeerConfig
	}
	if err := identity.CompleteOwnedPairing(ctx, db, session.Actor, local, target, ed25519.PublicKey(descriptor.SigningPublicKey), answer.Request.Challenge, answer.Proof, answer.Request.ExpiresUnix); err != nil {
		return err
	}
	_, err = fmt.Fprintln(output, "Posse da chave comprovada e pareamento aprovado pelo dono. Confiança de criptografia e permissões de eventos continuam separadas na 9K.")
	return err
}

func pairingHumanSession(ctx context.Context, db *sql.DB, device identity.DeviceContext, ownerID string, password io.Reader) (localauth.Session, error) {
	raw, err := io.ReadAll(io.LimitReader(password, 74))
	if err != nil {
		return localauth.Session{}, errPeerConfig
	}
	if len(raw) > 0 && raw[len(raw)-1] == '\n' {
		raw = raw[:len(raw)-1]
	}
	if len(raw) > 72 {
		return localauth.Session{}, errPeerConfig
	}
	session, err := localauth.Login(ctx, db, device, ownerID, string(raw))
	if err != nil {
		return localauth.Session{}, err
	}
	resolved, err := localauth.Resolve(ctx, db, session.Token)
	if err != nil {
		_ = localauth.Logout(ctx, db, session.Token)
		return localauth.Session{}, err
	}
	return resolved, nil
}
func writePairPublic(path string, body []byte) error {
	if len(body) == 0 || len(body) > 16*1024 {
		return errPeerConfig
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return err
	}
	defer file.Close()
	if _, err := file.Write(body); err != nil {
		return err
	}
	if err := file.Sync(); err != nil {
		return err
	}
	return file.Close()
}
