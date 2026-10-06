package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"titansystem-backend/internal/localdb/localauth"
	"titansystem-backend/internal/localdb/stationfile"
)

type recoveryFile struct {
	Version  int    `json:"version"`
	TenantID string `json:"tenant_id"`
	StoreID  string `json:"store_id"`
	DeviceID string `json:"device_id"`
	OwnerID  string `json:"owner_id"`
	Key      string `json:"recovery_key"`
}

func main() {
	if err := run(os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "Titan access: operação não concluída; confira credenciais, estação e arquivos privados.")
		os.Exit(1)
	}
}

// Passwords are read only from a private file, never arguments, env or stdout.
func run(args []string, out io.Writer) error {
	if len(args) == 0 {
		return errors.New("command required")
	}
	command := args[0]
	if command != "recovery-init" && command != "recover" {
		return errors.New("unknown command")
	}
	fs := flag.NewFlagSet("titan-access", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	dbPath := fs.String("db", "", "existing database")
	stationPath := fs.String("station", "", "private station file")
	passwordFile := fs.String("password-file", "", "private file with current or new password")
	var ownerID, destination, recoveryPath *string
	if command == "recovery-init" {
		ownerID = fs.String("owner", "", "owner identity")
		destination = fs.String("out", "", "new private recovery file")
	} else {
		recoveryPath = fs.String("recovery-file", "", "existing private recovery file")
	}
	if err := fs.Parse(args[1:]); err != nil || fs.NArg() != 0 || *dbPath == "" || *stationPath == "" || *passwordFile == "" {
		return errors.New("invalid flags")
	}
	if command == "recovery-init" && (*ownerID == "" || *destination == "") {
		return errors.New("missing flags")
	}
	if command == "recover" && *recoveryPath == "" {
		return errors.New("missing flags")
	}
	password, err := privateRead(*passwordFile, 73)
	if err != nil {
		return err
	}
	password = bytes.TrimSuffix(password, []byte("\n"))
	if len(password) < 12 || len(password) > 72 {
		return errors.New("invalid credential file")
	}
	defer func() {
		for i := range password {
			password[i] = 0
		}
	}()
	var file recoveryFile
	if command == "recover" {
		body, err := privateRead(*recoveryPath, 2048)
		if err != nil {
			return err
		}
		if err := parseRecovery(body, &file); err != nil {
			return err
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	db, device, _, err := stationfile.OpenVerified(ctx, *dbPath, *stationPath)
	if err != nil {
		return err
	}
	defer db.Close()
	if command == "recovery-init" {
		secret := make([]byte, 32)
		if _, err := rand.Read(secret); err != nil {
			return err
		}
		file = recoveryFile{Version: 1, TenantID: device.TenantID, StoreID: device.StoreID, DeviceID: device.DeviceID, OwnerID: *ownerID, Key: base64.RawURLEncoding.EncodeToString(secret)}
		body, err := json.Marshal(file)
		if err != nil {
			return err
		}
		err = localauth.IssueOwnerRecovery(ctx, db, device, *ownerID, string(password), secret, func() error { return writePrivate(*destination, body) })
		if err != nil {
			return err
		}
		_, err = fmt.Fprintln(out, "Chave de recuperação preparada em arquivo privado. Guarde-a separadamente; ela permite redefinir o acesso do dono.")
		return err
	}
	if file.TenantID != device.TenantID || file.StoreID != device.StoreID || file.DeviceID != device.DeviceID {
		return errors.New("foreign recovery file")
	}
	if err := localauth.RecoverOwner(ctx, db, device, file.OwnerID, file.Key, string(password)); err != nil {
		return err
	}
	_, err = fmt.Fprintln(out, "Conta local recuperada. Sessões antigas encerradas e chave consumida. Faça novo login e prepare outra chave de emergência.")
	return err
}

func privateRead(path string, max int64) ([]byte, error) {
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || info.Mode().Perm() != 0600 || info.Size() > max {
		return nil, errors.New("invalid private file")
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	opened, err := file.Stat()
	if err != nil || !os.SameFile(info, opened) || opened.Mode().Perm() != 0600 {
		return nil, errors.New("private file changed")
	}
	body, err := io.ReadAll(io.LimitReader(file, max+1))
	if err != nil || int64(len(body)) > max {
		return nil, errors.New("invalid private file")
	}
	return body, nil
}

func writePrivate(path string, body []byte) error {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	if _, err = file.Write(body); err == nil {
		err = file.Sync()
	}
	closeErr := file.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	dir, err := os.Open(filepath.Dir(path))
	if err != nil {
		return err
	}
	defer dir.Close()
	return dir.Sync()
}

func parseRecovery(body []byte, out *recoveryFile) error {
	dec := json.NewDecoder(bytes.NewReader(body))
	start, err := dec.Token()
	if err != nil || start != json.Delim('{') {
		return errors.New("invalid recovery file")
	}
	allowed := map[string]bool{"version": true, "tenant_id": true, "store_id": true, "device_id": true, "owner_id": true, "recovery_key": true}
	seen := make(map[string]bool)
	for dec.More() {
		key, err := dec.Token()
		name, ok := key.(string)
		if err != nil || !ok || !allowed[name] || seen[name] {
			return errors.New("invalid recovery fields")
		}
		seen[name] = true
		var raw json.RawMessage
		if err := dec.Decode(&raw); err != nil || bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
			return errors.New("invalid recovery value")
		}
	}
	end, err := dec.Token()
	if err != nil || end != json.Delim('}') || len(seen) != len(allowed) {
		return errors.New("invalid recovery file")
	}
	if _, err := dec.Token(); err != io.EOF {
		return errors.New("trailing recovery data")
	}
	if err := json.Unmarshal(body, out); err != nil || out.Version != 1 || out.TenantID == "" || out.StoreID == "" || out.DeviceID == "" || out.OwnerID == "" || len(out.Key) != 43 {
		return errors.New("invalid recovery file")
	}
	return nil
}
