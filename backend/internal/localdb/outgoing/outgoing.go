// Package outgoing consulta a outbox local e guarda recibos autenticados.
// O chamador deve provar previamente a identidade criptográfica do aparelho.
package outgoing

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"titansystem-backend/internal/localdb/identity"
)

var (
	ErrDenied          = errors.New("aparelho sem autorização local")
	ErrInvalid         = errors.New("recibo de sincronização inválido")
	ErrNotFound        = errors.New("evento pendente não encontrado")
	ErrReceiptConflict = errors.New("evento confirmado com outro recibo")
)

type Event struct {
	EventID       string
	TenantID      string
	StoreID       string
	DeviceID      string
	OperationID   string
	AggregateID   string
	EventType     string
	SchemaVersion int64
	Payload       json.RawMessage
	Attempts      int64
}

func (e Event) PayloadSHA256() string {
	return fmt.Sprintf("%x", sha256.Sum256(e.Payload))
}

// Receipt só pode vir de um destino autenticado que confirmou persistência.
// Verifier deve verificar a assinatura/prova do destinatário esperado.
type Receipt struct {
	ReceiptID     string
	EventID       string
	TenantID      string
	StoreID       string
	DeviceID      string
	PayloadSHA256 string
	Proof         []byte
}

type Verifier interface {
	Verify(context.Context, Event, Receipt) error
}

// Pending não faz HTTP e não altera a fila. Os IDs do aparelho precisam vir de
// sessão verificada fora deste pacote, nunca de parâmetros livres da API.
func Pending(ctx context.Context, db *sql.DB, device identity.DeviceContext, limit int) ([]Event, error) {
	if db == nil {
		return nil, errors.New("banco local indisponível")
	}
	if limit < 1 || limit > 100 {
		return nil, ErrInvalid
	}
	if err := authorized(ctx, db, device); err != nil {
		return nil, err
	}
	rows, err := db.QueryContext(ctx, `SELECT event_id,tenant_id,store_id,device_id,operation_id,aggregate_id,event_type,schema_version,payload_json,attempts
		FROM outbox WHERE tenant_id=? AND store_id=? AND device_id=? AND status='pending'
		ORDER BY created_at,event_id LIMIT ?`, device.TenantID, device.StoreID, device.DeviceID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	events := make([]Event, 0)
	for rows.Next() {
		var e Event
		var payload string
		if err = rows.Scan(&e.EventID, &e.TenantID, &e.StoreID, &e.DeviceID, &e.OperationID, &e.AggregateID, &e.EventType, &e.SchemaVersion, &payload, &e.Attempts); err != nil {
			return nil, err
		}
		e.Payload = json.RawMessage(payload)
		events = append(events, e)
	}
	return events, rows.Err()
}

// FailedAttempt registra falha de transporte sem remover ou confirmar o evento.
func FailedAttempt(ctx context.Context, db *sql.DB, device identity.DeviceContext, eventID string) error {
	if db == nil {
		return errors.New("banco local indisponível")
	}
	if strings.TrimSpace(eventID) == "" {
		return ErrInvalid
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err = authorizedTx(ctx, tx, device); err != nil {
		return err
	}
	result, err := tx.ExecContext(ctx, `UPDATE outbox SET attempts=attempts+1
		WHERE event_id=? AND tenant_id=? AND store_id=? AND device_id=? AND status='pending' AND attempts<9223372036854775807`,
		eventID, device.TenantID, device.StoreID, device.DeviceID)
	if err != nil {
		return err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if count != 1 {
		return ErrNotFound
	}
	return tx.Commit()
}

// Confirm só reconhece o evento após prova verificada do destino. Repetir o
// mesmo recibo é seguro, e recibo divergente preserva o histórico existente.
func Confirm(ctx context.Context, db *sql.DB, device identity.DeviceContext, receipt Receipt, verifier Verifier) error {
	if db == nil {
		return errors.New("banco local indisponível")
	}
	if verifier == nil || strings.TrimSpace(receipt.EventID) == "" || strings.TrimSpace(receipt.ReceiptID) == "" {
		return ErrInvalid
	}
	if err := authorized(ctx, db, device); err != nil {
		return err
	}
	event, status, err := load(ctx, db, device, receipt.EventID)
	if err != nil {
		return err
	}
	if !matches(event, receipt) {
		return ErrInvalid
	}
	if err = verifier.Verify(ctx, event, receipt); err != nil {
		return fmt.Errorf("prova do recebimento: %w", err)
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err = authorizedTx(ctx, tx, device); err != nil {
		return err
	}
	var currentStatus, payload string
	err = tx.QueryRowContext(ctx, `SELECT status,payload_json FROM outbox
		WHERE event_id=? AND tenant_id=? AND store_id=? AND device_id=?`,
		receipt.EventID, device.TenantID, device.StoreID, device.DeviceID).Scan(&currentStatus, &payload)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	if currentStatus != status && status == "pending" {
		return ErrReceiptConflict
	}
	if fmt.Sprintf("%x", sha256.Sum256([]byte(payload))) != receipt.PayloadSHA256 {
		return ErrReceiptConflict
	}
	if currentStatus == "acked" {
		var priorID, priorHash string
		err = tx.QueryRowContext(ctx, `SELECT receipt_id,payload_sha256 FROM outbox_receipts WHERE event_id=?`, receipt.EventID).Scan(&priorID, &priorHash)
		if err != nil {
			return err
		}
		if priorID != receipt.ReceiptID || priorHash != receipt.PayloadSHA256 {
			return ErrReceiptConflict
		}
		return tx.Commit()
	}
	if currentStatus != "pending" {
		return ErrReceiptConflict
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err = tx.ExecContext(ctx, `INSERT INTO outbox_receipts VALUES (?,?,?,?,?,?,?)`, receipt.EventID, device.TenantID, device.StoreID, device.DeviceID, receipt.ReceiptID, receipt.PayloadSHA256, now)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `UPDATE outbox SET status='acked',acked_at=? WHERE event_id=? AND tenant_id=? AND store_id=? AND device_id=? AND status='pending'`,
		now, receipt.EventID, device.TenantID, device.StoreID, device.DeviceID)
	if err != nil {
		return err
	}
	return tx.Commit()
}

func matches(e Event, r Receipt) bool {
	return e.EventID == r.EventID && e.TenantID == r.TenantID && e.StoreID == r.StoreID && e.DeviceID == r.DeviceID && e.PayloadSHA256() == r.PayloadSHA256
}

func load(ctx context.Context, db *sql.DB, device identity.DeviceContext, eventID string) (Event, string, error) {
	var e Event
	var payload, status string
	err := db.QueryRowContext(ctx, `SELECT event_id,tenant_id,store_id,device_id,operation_id,aggregate_id,event_type,schema_version,payload_json,attempts,status
		FROM outbox WHERE event_id=? AND tenant_id=? AND store_id=? AND device_id=?`,
		eventID, device.TenantID, device.StoreID, device.DeviceID).
		Scan(&e.EventID, &e.TenantID, &e.StoreID, &e.DeviceID, &e.OperationID, &e.AggregateID, &e.EventType, &e.SchemaVersion, &payload, &e.Attempts, &status)
	if errors.Is(err, sql.ErrNoRows) {
		return e, "", ErrNotFound
	}
	if err != nil {
		return e, "", err
	}
	e.Payload = json.RawMessage(payload)
	return e, status, nil
}

func authorized(ctx context.Context, db *sql.DB, d identity.DeviceContext) error {
	var approved int
	if d.TenantID == "" || d.StoreID == "" || d.DeviceID == "" {
		return ErrDenied
	}
	err := db.QueryRowContext(ctx, `SELECT 1 FROM device_pairings WHERE tenant_id=? AND store_id=? AND device_id=? AND status='approved'`, d.TenantID, d.StoreID, d.DeviceID).Scan(&approved)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrDenied
	}
	return err
}

func authorizedTx(ctx context.Context, tx *sql.Tx, d identity.DeviceContext) error {
	var approved int
	if d.TenantID == "" || d.StoreID == "" || d.DeviceID == "" {
		return ErrDenied
	}
	err := tx.QueryRowContext(ctx, `SELECT 1 FROM device_pairings WHERE tenant_id=? AND store_id=? AND device_id=? AND status='approved'`, d.TenantID, d.StoreID, d.DeviceID).Scan(&approved)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrDenied
	}
	return err
}
