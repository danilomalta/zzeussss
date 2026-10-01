package incoming

import (
	"bytes"
	"context"
	"crypto/ecdh"
	"crypto/ed25519"
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"time"

	"titansystem-backend/internal/localdb/identity"
	"titansystem-backend/internal/localdb/outgoing"
)

const SealedReceiverPath = "/sync/v1/events"
const sealedHTTPBodyLimit = 512 * 1024

// HTTPReceiptResult confirms durable reception, not business application.
// Receipt keeps the existing outgoing.Receipt wire field names.
type HTTPReceiptResult struct {
	Receipt  outgoing.Receipt `json:"receipt"`
	Repeated bool             `json:"repeated"`
}

// NewSealedReceiverHTTP builds a handler only: it does not open a listener,
// mount a business API or start an automatic sender. Device and private keys
// must come from verified local startup, never from request fields.
func NewSealedReceiverHTTP(ctx context.Context, db *sql.DB, device identity.DeviceContext, signing ed25519.PrivateKey, encryption *ecdh.PrivateKey) (http.Handler, error) {
	if db == nil || !validID(device.TenantID) || !validID(device.StoreID) || !validID(device.DeviceID) ||
		!validKey(signing) || encryption == nil || encryption.Curve() != ecdh.X25519() {
		return nil, ErrInvalid
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	public, err := deviceKey(ctx, tx, device)
	if err != nil {
		return nil, err
	}
	if !bytes.Equal(public, signing.Public().(ed25519.PublicKey)) {
		return nil, ErrDenied
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	// Prevent mutation of the caller's signing slice after construction.
	key := append(ed25519.PrivateKey(nil), signing...)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		if r.URL.Path != SealedReceiverPath || r.URL.RawQuery != "" {
			receiverHTTPError(w, http.StatusNotFound, "not_found")
			return
		}
		if r.Method != http.MethodPost {
			w.Header().Set("Allow", http.MethodPost)
			receiverHTTPError(w, http.StatusMethodNotAllowed, "method_not_allowed")
			return
		}
		media, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
		if err != nil || media != "application/json" || r.Header.Get("Content-Encoding") != "" {
			receiverHTTPError(w, http.StatusUnsupportedMediaType, "unsupported_media_type")
			return
		}
		if r.ContentLength > sealedHTTPBodyLimit {
			receiverHTTPError(w, http.StatusRequestEntityTooLarge, "body_too_large")
			return
		}
		body := http.MaxBytesReader(w, r.Body, sealedHTTPBodyLimit)
		defer body.Close()
		raw, err := io.ReadAll(body)
		if err != nil {
			var large *http.MaxBytesError
			if errors.As(err, &large) {
				receiverHTTPError(w, http.StatusRequestEntityTooLarge, "body_too_large")
			} else {
				receiverHTTPError(w, http.StatusBadRequest, "invalid_request")
			}
			return
		}
		envelope, err := decodeHTTPEnvelope(raw)
		if err != nil {
			receiverHTTPError(w, http.StatusBadRequest, "invalid_request")
			return
		}
		requestContext, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()
		result, err := ReceiveSealed(requestContext, db, device, key, encryption, envelope)
		if err != nil {
			switch {
			case errors.Is(err, ErrDenied), errors.Is(err, identity.ErrDenied):
				receiverHTTPError(w, http.StatusForbidden, "denied")
			case errors.Is(err, ErrInvalid):
				receiverHTTPError(w, http.StatusBadRequest, "invalid_request")
			case errors.Is(err, ErrConflict):
				receiverHTTPError(w, http.StatusConflict, "conflict")
			default:
				receiverHTTPError(w, http.StatusInternalServerError, "unavailable")
			}
			return
		}
		status := http.StatusCreated
		if result.Repeated {
			status = http.StatusOK
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		// If delivery of the response fails, the sender retries the same event;
		// Receive returns its persisted receipt instead of inserting again.
		_ = json.NewEncoder(w).Encode(HTTPReceiptResult{Receipt: result.Receipt, Repeated: result.Repeated})
	}), nil
}

func receiverHTTPError(w http.ResponseWriter, status int, code string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(struct {
		Error string `json:"error"`
	}{code})
}

func decodeHTTPEnvelope(raw []byte) (SealedEnvelope, error) {
	fields, err := exactKeyObject(raw, []string{"version", "destination", "recipient_key_sha256", "ephemeral_public", "salt", "nonce", "ciphertext"})
	if err != nil {
		return SealedEnvelope{}, ErrInvalid
	}
	if _, err := exactKeyObject(fields["destination"], []string{"TenantID", "StoreID", "DeviceID"}); err != nil {
		return SealedEnvelope{}, ErrInvalid
	}
	var envelope SealedEnvelope
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return SealedEnvelope{}, ErrInvalid
	}
	return envelope, nil
}
