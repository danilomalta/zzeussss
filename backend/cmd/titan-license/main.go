// titan-license is an offline issuer tool. Do not ship its private key to clients.
package main

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"titansystem-backend/internal/core/entitlements"
	"titansystem-backend/internal/core/modules"
)

type issuerKey struct {
	Version    int    `json:"version"`
	KeyID      string `json:"key_id"`
	PrivateKey []byte `json:"private_key"`
}

func main() {
	if err := run(os.Args[1:], os.Stdout, time.Now().UTC()); err != nil {
		fmt.Fprintln(os.Stderr, "Titan emissor:", err)
		os.Exit(1)
	}
}

func run(args []string, output io.Writer, now time.Time) error {
	if len(args) == 0 {
		return errors.New("uso: titan-license init|public|sign (emissor administrativo offline)")
	}
	options := flag.NewFlagSet(args[0], flag.ContinueOnError)
	options.SetOutput(io.Discard)
	keyPath := options.String("private-key", "", "arquivo privado emissor")
	var keyID, outPath, tenant, expires, selected *string
	var revision *int64
	switch args[0] {
	case "init":
		keyID = options.String("key-id", "", "identificador do emissor")
	case "public":
		outPath = options.String("out", "", "novo arquivo publico")
	case "sign":
		outPath = options.String("out", "", "novo arquivo de contrato")
		tenant = options.String("tenant", "", "ID exato da empresa")
		revision = options.Int64("revision", 0, "revisao positiva explicitamente escolhida")
		expires = options.String("expires", "", "vencimento RFC3339")
		selected = options.String("modules", "", "modulos separados por virgula")
	default:
		return errors.New("comando emissor desconhecido")
	}
	if err := options.Parse(args[1:]); err != nil {
		return errors.New("argumentos do emissor invalidos")
	}
	if *keyPath == "" || options.NArg() != 0 {
		return errors.New("informe --private-key e somente argumentos reconhecidos")
	}
	if args[0] == "init" {
		if _, err := entitlements.NewVerifier(map[string]ed25519.PublicKey{*keyID: make([]byte, ed25519.PublicKeySize)}); err != nil {
			return errors.New("identificador emissor invalido")
		}
		_, private, err := ed25519.GenerateKey(rand.Reader)
		if err != nil {
			return errors.New("falha ao gerar chave emissora")
		}
		data, err := json.Marshal(issuerKey{Version: 1, KeyID: *keyID, PrivateKey: private})
		if err != nil {
			return err
		}
		if err := createFile(*keyPath, data); err != nil {
			return err
		}
		_, err = fmt.Fprintln(output, "Chave privada emissora criada. Mantenha fora dos aparelhos dos clientes e do Git.")
		return err
	}
	if *outPath == "" {
		return errors.New("informe --out para um novo arquivo")
	}
	key, err := readKey(*keyPath)
	if err != nil {
		return err
	}
	public := ed25519.PrivateKey(key.PrivateKey).Public().(ed25519.PublicKey)
	if args[0] == "public" {
		data, err := json.Marshal(map[string][]byte{key.KeyID: public})
		if err != nil {
			return err
		}
		if err := createFile(*outPath, data); err != nil {
			return err
		}
		_, err = fmt.Fprintln(output, "Configuracao PUBLICA exportada. Distribua somente por canal confiavel.")
		return err
	}
	if *tenant == "" || strings.TrimSpace(*tenant) != *tenant || *revision < 1 || *selected == "" {
		return errors.New("informe empresa, revisao positiva e modulos explicitamente")
	}
	deadline, err := time.Parse(time.RFC3339, *expires)
	if err != nil || now.Unix() <= 0 || deadline.Unix() <= now.Unix() {
		return errors.New("vencimento deve ser RFC3339 e posterior ao horario de emissao")
	}
	ids := make([]modules.ID, 0)
	seen := make(map[modules.ID]bool)
	for _, part := range strings.Split(*selected, ",") {
		id := modules.ID(strings.TrimSpace(part))
		if id == "" || seen[id] {
			return errors.New("selecao de modulos vazia ou duplicada")
		}
		seen[id] = true
		ids = append(ids, id)
	}
	active, err := modules.Required(ids)
	if err != nil {
		return errors.New("selecao contem modulo desconhecido")
	}
	payload, err := json.Marshal(entitlements.Claims{Version: 1, TenantID: *tenant, Revision: *revision,
		IssuedAt: now.Unix(), NotBefore: now.Unix(), ExpiresAt: deadline.Unix(), Modules: active})
	if err != nil {
		return err
	}
	envelope := entitlements.Envelope{KeyID: key.KeyID, Payload: payload,
		Signature: ed25519.Sign(ed25519.PrivateKey(key.PrivateKey), entitlements.SigningMessage(key.KeyID, payload))}
	verifier, err := entitlements.NewVerifier(map[string]ed25519.PublicKey{key.KeyID: public})
	if err != nil {
		return err
	}
	if _, err := verifier.Verify(envelope, *tenant, now); err != nil {
		return errors.New("contrato emitido falhou na verificacao interna")
	}
	data, err := json.Marshal(envelope)
	if err != nil {
		return err
	}
	if err := createFile(*outPath, data); err != nil {
		return err
	}
	_, err = fmt.Fprintf(output, "Contrato emitido. Revisao: %d; modulos incluindo dependencias: %v\n", *revision, active)
	return err
}

// All outputs are exclusive files. A failed write is preserved for investigation;
// it never overwrites, deletes or automatically reuses key/contract paths.
func createFile(path string, data []byte) error {
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return errors.New("nao foi possivel criar arquivo novo; confira caminho e existencia")
	}
	if _, err := file.Write(append(data, '\n')); err != nil {
		_ = file.Close()
		return errors.New("falha ao gravar arquivo; preserve-o para investigar")
	}
	if err := file.Sync(); err != nil {
		_ = file.Close()
		return errors.New("falha ao sincronizar arquivo; preserve-o para investigar")
	}
	if err := file.Close(); err != nil {
		return errors.New("falha ao fechar arquivo; preserve-o para investigar")
	}
	return nil
}

func readKey(path string) (issuerKey, error) {
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || info.Mode().Perm() != 0600 {
		return issuerKey{}, errors.New("chave emissora deve ser arquivo regular privado 0600")
	}
	file, err := os.Open(path)
	if err != nil {
		return issuerKey{}, errors.New("nao foi possivel abrir chave emissora")
	}
	defer file.Close()
	info, err = file.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Mode().Perm() != 0600 {
		return issuerKey{}, errors.New("arquivo privado emissor invalido")
	}
	body, err := io.ReadAll(io.LimitReader(file, 8193))
	if err != nil || len(body) > 8192 {
		return issuerKey{}, errors.New("arquivo privado emissor invalido")
	}
	decoder := json.NewDecoder(bytes.NewReader(body))
	opening, err := decoder.Token()
	if err != nil || opening != json.Delim('{') {
		return issuerKey{}, errors.New("arquivo privado emissor invalido")
	}
	seen := make(map[string]bool)
	for decoder.More() {
		token, err := decoder.Token()
		name, ok := token.(string)
		if err != nil || !ok || seen[name] || (name != "version" && name != "key_id" && name != "private_key") {
			return issuerKey{}, errors.New("arquivo privado emissor invalido")
		}
		seen[name] = true
		var value json.RawMessage
		if err := decoder.Decode(&value); err != nil {
			return issuerKey{}, errors.New("arquivo privado emissor invalido")
		}
	}
	closing, err := decoder.Token()
	if err != nil || closing != json.Delim('}') || len(seen) != 3 {
		return issuerKey{}, errors.New("arquivo privado emissor invalido")
	}
	if _, err := decoder.Token(); err != io.EOF {
		return issuerKey{}, errors.New("arquivo privado emissor invalido")
	}
	var key issuerKey
	if err := json.Unmarshal(body, &key); err != nil || key.Version != 1 || len(key.PrivateKey) != ed25519.PrivateKeySize {
		return issuerKey{}, errors.New("arquivo privado emissor invalido")
	}
	rebuilt := ed25519.NewKeyFromSeed(key.PrivateKey[:ed25519.SeedSize])
	if !bytes.Equal(rebuilt, key.PrivateKey) {
		return issuerKey{}, errors.New("chave emissora inconsistente")
	}
	if _, err := entitlements.NewVerifier(map[string]ed25519.PublicKey{key.KeyID: rebuilt.Public().(ed25519.PublicKey)}); err != nil {
		return issuerKey{}, errors.New("identificador emissor invalido")
	}
	return key, nil
}
