package incoming

import (
	"bytes"
	"context"
	"crypto/ecdh"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"errors"
	"testing"

	"titansystem-backend/internal/localdb/outgoing"
)

func encryptionKey(t *testing.T) *ecdh.PrivateKey {
	t.Helper()
	key, err := ecdh.X25519().GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	return key
}

func encrypted(t *testing.T, f *fixture, key *ecdh.PrivateKey) SealedEnvelope {
	t.Helper()
	envelope, err := Seal(f.message(t), key.PublicKey())
	if err != nil {
		t.Fatal(err)
	}
	return envelope
}

func cloneEnvelope(t *testing.T, envelope SealedEnvelope) SealedEnvelope {
	t.Helper()
	body, err := json.Marshal(envelope)
	if err != nil {
		t.Fatal(err)
	}
	var copy SealedEnvelope
	if err := json.Unmarshal(body, &copy); err != nil {
		t.Fatal(err)
	}
	return copy
}

func TestSealedMessagePreservesExactSignedPayloadAndHidesContent(t *testing.T) {
	f := setup(t)
	f.event.Payload = []byte(" {\n  \"sale_id\" : \"sale-one\", \"private_note\" : \"confidential-business-data\"\n } ")
	key := encryptionKey(t)
	envelope := encrypted(t, f, key)
	wire, err := json.Marshal(envelope)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(wire, []byte("confidential-business-data")) || bytes.Contains(wire, []byte("sale_id")) || bytes.Contains(wire, f.event.Payload) {
		t.Fatal("envelope revelou conteudo")
	}
	if bytes.Contains(wire, key.Bytes()) {
		t.Fatal("envelope incluiu chave privada")
	}
	opened, err := OpenSealed(envelope, f.receiver, key)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(opened.Event.Payload, f.event.Payload) || opened.Event.PayloadSHA256() != f.event.PayloadSHA256() {
		t.Fatal("conteudo assinado foi reformatado")
	}
	f.grant(t)
	if _, err := ReceiveSealed(context.Background(), f.receiverDB, f.receiver, f.receiverKey, key, envelope); err != nil {
		t.Fatal(err)
	}
	receivedCount(t, f, 1)
}

func TestEncryptedReceptionAndReceiptConfirmExistingOutbox(t *testing.T) {
	f := setup(t)
	f.grant(t)
	key := encryptionKey(t)
	ctx := context.Background()
	result, err := ReceiveSealed(ctx, f.receiverDB, f.receiver, f.receiverKey, key, encrypted(t, f, key))
	if err != nil {
		t.Fatal(err)
	}
	verifier, err := NewReceiptVerifier(f.receiver, f.receiverKey.Public().(ed25519.PublicKey))
	if err != nil {
		t.Fatal(err)
	}
	if err := outgoing.Confirm(ctx, f.senderDB, f.sender, result.Receipt, verifier); err != nil {
		t.Fatal(err)
	}
	pending, err := outgoing.Pending(ctx, f.senderDB, f.sender, 10)
	if err != nil || len(pending) != 0 {
		t.Fatalf("confirmacao: %v %v", pending, err)
	}
	receivedCount(t, f, 1)
}

func TestSealedRetriesUseFreshCiphertextButSameDurableReceipt(t *testing.T) {
	f := setup(t)
	f.grant(t)
	key := encryptionKey(t)
	first := encrypted(t, f, key)
	second := encrypted(t, f, key)
	if bytes.Equal(first.EphemeralPublic, second.EphemeralPublic) || bytes.Equal(first.Nonce, second.Nonce) || bytes.Equal(first.Ciphertext, second.Ciphertext) {
		t.Fatal("criptografia reutilizou aleatoriedade")
	}
	ctx := context.Background()
	a, err := ReceiveSealed(ctx, f.receiverDB, f.receiver, f.receiverKey, key, first)
	if err != nil {
		t.Fatal(err)
	}
	b, err := ReceiveSealed(ctx, f.receiverDB, f.receiver, f.receiverKey, key, second)
	if err != nil || !b.Repeated || a.Receipt.ReceiptID != b.Receipt.ReceiptID || !bytes.Equal(a.Receipt.Proof, b.Receipt.Proof) {
		t.Fatalf("repeticao criptografada: %+v %v", b, err)
	}
	receivedCount(t, f, 1)
}

func TestWrongEncryptionRecipientAndAlteredHeadersCannotReceive(t *testing.T) {
	f := setup(t)
	f.grant(t)
	key := encryptionKey(t)
	envelope := encrypted(t, f, key)
	if _, err := ReceiveSealed(context.Background(), f.receiverDB, f.receiver, f.receiverKey, encryptionKey(t), envelope); err == nil {
		t.Fatal("chave de outro destinatario abriu")
	}
	for _, change := range []func(*SealedEnvelope){
		func(e *SealedEnvelope) { e.Version = 2 },
		func(e *SealedEnvelope) { e.Destination.DeviceID = "different" },
		func(e *SealedEnvelope) { e.Destination.TenantID = "other" },
		func(e *SealedEnvelope) { e.RecipientKeySHA256 = "different" },
		func(e *SealedEnvelope) { e.EphemeralPublic[0] ^= 1 },
		func(e *SealedEnvelope) { e.Salt[0] ^= 1 },
		func(e *SealedEnvelope) { e.Nonce[0] ^= 1 },
		func(e *SealedEnvelope) { e.Ciphertext[0] ^= 1 },
		func(e *SealedEnvelope) { e.Ciphertext = e.Ciphertext[:len(e.Ciphertext)-1] },
	} {
		copy := cloneEnvelope(t, envelope)
		change(&copy)
		result, err := ReceiveSealed(context.Background(), f.receiverDB, f.receiver, f.receiverKey, key, copy)
		if err == nil || result.Receipt.ReceiptID != "" {
			t.Fatalf("adulteracao gerou recibo: %+v %v", result, err)
		}
	}
	receivedCount(t, f, 0)
}

func TestEncryptionDoesNotReplaceSenderSignatureOrPeerGrant(t *testing.T) {
	f := setup(t)
	key := encryptionKey(t)
	valid := encrypted(t, f, key)
	if _, err := ReceiveSealed(context.Background(), f.receiverDB, f.receiver, f.receiverKey, key, valid); !errors.Is(err, ErrDenied) {
		t.Fatalf("sem concessao: %v", err)
	}
	f.grant(t)
	message := f.message(t)
	message.Signature[0] ^= 1
	forged, err := Seal(message, key.PublicKey())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ReceiveSealed(context.Background(), f.receiverDB, f.receiver, f.receiverKey, key, forged); !errors.Is(err, ErrDenied) {
		t.Fatalf("assinatura falsa: %v", err)
	}
	receivedCount(t, f, 0)
}

func TestSealedRejectsInvalidCurvesLowOrderKeysAndSizes(t *testing.T) {
	f := setup(t)
	message := f.message(t)
	if _, err := Seal(message, nil); !errors.Is(err, ErrInvalid) {
		t.Fatalf("sem chave: %v", err)
	}
	wrongCurve, err := ecdh.P256().GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Seal(message, wrongCurve.PublicKey()); !errors.Is(err, ErrInvalid) {
		t.Fatalf("curva errada: %v", err)
	}
	zero, err := ecdh.X25519().NewPublicKey(make([]byte, 32))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Seal(message, zero); !errors.Is(err, ErrInvalid) {
		t.Fatalf("chave baixa ordem: %v", err)
	}
	key := encryptionKey(t)
	envelope := encrypted(t, f, key)
	for _, change := range []func(*SealedEnvelope){
		func(e *SealedEnvelope) { e.EphemeralPublic = nil },
		func(e *SealedEnvelope) { e.EphemeralPublic = make([]byte, 32) },
		func(e *SealedEnvelope) { e.Salt = nil },
		func(e *SealedEnvelope) { e.Nonce = nil },
		func(e *SealedEnvelope) { e.Ciphertext = nil },
		func(e *SealedEnvelope) { e.Ciphertext = make([]byte, sealedLimit+17) },
	} {
		copy := cloneEnvelope(t, envelope)
		change(&copy)
		if _, err := OpenSealed(copy, f.receiver, key); err == nil {
			t.Fatal("formato invalido aceito")
		}
	}
	if _, err := OpenSealed(envelope, f.receiver, nil); !errors.Is(err, ErrInvalid) {
		t.Fatalf("sem chave privada: %v", err)
	}
	if _, err := OpenSealed(envelope, f.receiver, wrongCurve); !errors.Is(err, ErrInvalid) {
		t.Fatalf("curva privada errada: %v", err)
	}
}

func TestInnerSealedContentRejectsAmbiguousJSON(t *testing.T) {
	f := setup(t)
	m := f.message(t)
	e := m.Event
	body, err := json.Marshal(sealedContent{e.EventID, e.TenantID, e.StoreID, e.DeviceID, e.OperationID, e.AggregateID, e.EventType, e.SchemaVersion, []byte(e.Payload), m.Signature})
	if err != nil {
		t.Fatal(err)
	}
	for _, bad := range [][]byte{
		[]byte("null"), []byte("{}"), append(append([]byte(nil), body...), []byte(" {}")...),
		bytes.Replace(body, []byte(`"event_id":"event-one"`), []byte(`"event_id":"event-one","event_id":"different"`), 1),
		bytes.Replace(body, []byte(`"schema_version":1`), []byte(`"schema_version":null`), 1),
		bytes.Replace(body, []byte(`"schema_version":1`), []byte(`"schema_version":1,"unknown":true`), 1),
	} {
		if _, err := decodeSealedContent(bad); err == nil {
			t.Fatal("JSON interno ambiguo aceito")
		}
	}
}
