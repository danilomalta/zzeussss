package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"titansystem-backend/internal/localdb"
	"titansystem-backend/internal/localdb/identity"
	"titansystem-backend/internal/localdb/incoming"
	"titansystem-backend/internal/localdb/stationfile"
)

// A candidate has its own keys, but no database, company membership or permission.
// Only the existing owner's challenge/proof/approval can admit it in that owner's DB.
func createCandidate(args []string, output io.Writer) error {
	fs := flag.NewFlagSet("candidate-init", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	parent := fs.String("parent", "", "descritor público do PC principal")
	dir := fs.String("dir", "", "diretório privado novo neste aparelho")
	signing := fs.String("signing-sha256", "", "impressão do principal conferida por canal autenticado")
	encryption := fs.String("encryption-sha256", "", "impressão de transporte do principal conferida")
	if fs.Parse(args) != nil || fs.NArg() != 0 || *parent == "" || !filepath.IsAbs(*dir) || filepath.Clean(*dir) != *dir || output == nil {
		return errPeerConfig
	}
	raw, err := readPeerPublic(*parent)
	if err != nil {
		return err
	}
	descriptor, prints, err := incoming.ParsePublicDescriptor(raw)
	if err != nil || prints.SigningSHA256 != *signing || prints.EncryptionSHA256 != *encryption {
		return errPeerConfig
	}
	// A new leaf only. Do not follow redirects, replace keys or reuse partial state.
	for p := filepath.Dir(filepath.Clean(*dir)); ; p = filepath.Dir(p) {
		info, err := os.Lstat(p)
		if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return errPeerConfig
		}
		if p == filepath.Dir(p) {
			break
		}
	}
	if err := os.Mkdir(*dir, 0700); err != nil {
		return err
	}
	deviceID, err := localdb.NewID()
	if err != nil {
		return err
	}
	device := identity.DeviceContext{TenantID: descriptor.Binding.Device.TenantID, StoreID: descriptor.Binding.Device.StoreID, DeviceID: deviceID}
	_, key, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return err
	}
	defer clear(key)
	station := stationfile.Station{TenantID: device.TenantID, StoreID: device.StoreID, DeviceID: device.DeviceID, PrivateKey: base64.RawURLEncoding.EncodeToString(key)}
	body, err := json.Marshal(station)
	if err != nil {
		return err
	}
	if err := writePairPublic(filepath.Join(*dir, "device.station"), body); err != nil {
		return err
	}
	transport, err := incoming.CreateEncryptionKey(filepath.Join(*dir, "peer.encryption"), device)
	if err != nil {
		return err
	}
	public, err := incoming.CreatePublicDescriptor(device, 1, transport, key)
	if err != nil {
		return err
	}
	if err := writePublicDescriptor(filepath.Join(*dir, "peer-public.json"), public, output); err != nil {
		return err
	}
	_, err = fmt.Fprintln(output, "Candidato com chaves próprias criado. Sem SQLite, aprovação ou permissão de eventos. Compartilhe somente peer-public.json.")
	return err
}

func inspectPairFile(args []string, output io.Writer) error {
	if len(args) != 2 || args[0] != "--in" || output == nil {
		return errPeerConfig
	}
	raw, err := readPeerPublic(args[1])
	if err != nil {
		return err
	}
	var request incoming.PairChallenge
	kind := "challenge"
	if answer, e := incoming.ParsePairAnswer(raw); e == nil {
		request, kind = answer.Request, "answer"
	} else {
		request, err = incoming.ParsePairChallenge(raw)
		if err != nil {
			return err
		}
	}
	target, err := json.Marshal(request.Target)
	if err != nil {
		return err
	}
	_, prints, err := incoming.ParsePublicDescriptor(target)
	if err != nil {
		return err
	}
	// Parsing verifies cryptographic structure. It does not approve a peer.
	return json.NewEncoder(output).Encode(struct {
		Kind             string `json:"kind"`
		RequesterSHA256  string `json:"requester_sha256"`
		SigningSHA256    string `json:"target_signing_sha256"`
		EncryptionSHA256 string `json:"target_encryption_sha256"`
		ExpiresUnix      int64  `json:"expires_unix"`
		Expired          bool   `json:"expired"`
		Approved         bool   `json:"approved"`
	}{kind, incoming.PairRequesterFingerprint(request), prints.SigningSHA256, prints.EncryptionSHA256, request.ExpiresUnix, request.ExpiresUnix <= time.Now().Unix(), false})
}
