// Package sale registra uma venda à vista local em uma transação SQLite.
// Somente dinheiro é aceito até existir confirmação real de cartão e PIX.
package sale

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"math/big"
	"strings"
	"time"
	"titansystem-backend/internal/localdb/catalog"

	"titansystem-backend/internal/localdb"
	"titansystem-backend/internal/localdb/identity"
	"titansystem-backend/internal/localdb/stockreservation"
)

var (
	ErrNotFound          = errors.New("venda local nao encontrada")
	ErrCashOverflow      = errors.New("saldo de caixa excede limite inteiro")
	ErrInvalid           = errors.New("venda inválida")
	ErrNoStock           = errors.New("estoque insuficiente na gôndola")
	ErrConflict          = errors.New("ID de venda ou operação reutilizado com outros dados")
	ErrCashClosed        = errors.New("caixa fechado ou de outro operador")
	ErrPaymentUnverified = errors.New("pagamento eletrônico ainda não verificado")
)

type Item struct {
	ProductID     string `json:"product_id"`
	LocationID    string `json:"location_id"`
	QuantityMilli int64  `json:"quantity_milli"`
}

type Payment struct {
	Method      string `json:"method"`
	AmountCents int64  `json:"amount_cents"`
}

type Input struct {
	OperationID   string    `json:"operation_id"`
	SaleID        string    `json:"sale_id"`
	CashSessionID string    `json:"cash_session_id"`
	Items         []Item    `json:"items"`
	Payments      []Payment `json:"payments"`
}

type Result struct {
	SaleID     string `json:"sale_id"`
	TotalCents int64  `json:"total_cents"`
	Repeated   bool   `json:"repeated"`
}

// Complete recebe Scope e DeviceContext já provados pela camada de sessão.
// O preço vem somente do catálogo persistido, nunca da requisição.
type authorization func(context.Context, *sql.Tx, identity.Scope, identity.DeviceContext) error

// Complete preserves the internal permission-only path. HTTP uses CompleteWithContract.
func Complete(ctx context.Context, db *sql.DB, actor identity.Scope, device identity.DeviceContext, in Input) (Result, error) {
	return complete(ctx, db, actor, device, in, authorizeLegacy)
}

func complete(ctx context.Context, db *sql.DB, actor identity.Scope, device identity.DeviceContext, in Input, authorize authorization) (Result, error) {
	if db == nil {
		return Result{}, errors.New("banco local indisponível")
	}
	if !valid(in) {
		return Result{}, ErrInvalid
	}
	encoded, err := json.Marshal(in)
	if err != nil {
		return Result{}, err
	}
	hash := fmt.Sprintf("%x", sha256.Sum256(encoded))
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return Result{}, err
	}
	defer tx.Rollback()
	if err = authorize(ctx, tx, actor, device); err != nil {
		return Result{}, err
	}
	var cancelledOperation int
	if err = tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM sale_cancellations WHERE tenant_id=? AND device_id=? AND operation_id=?`, actor.TenantID, device.DeviceID, in.OperationID).Scan(&cancelledOperation); err != nil {
		return Result{}, err
	}
	if cancelledOperation != 0 {
		return Result{}, ErrConflict
	}
	var priorSale, priorHash, priorStore, priorActor string
	err = tx.QueryRowContext(ctx, `SELECT sale_id, request_hash, store_id, actor_identity_id FROM sale_operations
		WHERE tenant_id = ? AND device_id = ? AND operation_id = ?`, actor.TenantID, device.DeviceID, in.OperationID).
		Scan(&priorSale, &priorHash, &priorStore, &priorActor)
	if err == nil {
		if priorSale != in.SaleID || priorHash != hash || priorStore != actor.StoreID || priorActor != actor.IdentityID {
			return Result{}, ErrConflict
		}
		var total int64
		var saleStatus string
		if err = tx.QueryRowContext(ctx, `SELECT total_cents,status FROM sales WHERE tenant_id = ? AND id = ?`, actor.TenantID, priorSale).Scan(&total, &saleStatus); err != nil {
			return Result{}, err
		}
		if saleStatus != "committed" {
			return Result{}, ErrConflict
		}
		if err = tx.Commit(); err != nil {
			return Result{}, err
		}
		return Result{SaleID: priorSale, TotalCents: total, Repeated: true}, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return Result{}, err
	}
	var existingSale int
	if err = tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM sales WHERE tenant_id=? AND id=?`, actor.TenantID, in.SaleID).Scan(&existingSale); err != nil {
		return Result{}, err
	}
	if existingSale != 0 {
		return Result{}, ErrConflict
	}
	var operator, sessionDevice string
	err = tx.QueryRowContext(ctx, `SELECT b.identity_id, c.device_id FROM cash_sessions c
		JOIN cash_session_operators b ON b.tenant_id = c.tenant_id AND b.session_id = c.id
		WHERE c.tenant_id = ? AND c.store_id = ? AND c.id = ? AND c.closed_at IS NULL`,
		actor.TenantID, actor.StoreID, in.CashSessionID).Scan(&operator, &sessionDevice)
	if errors.Is(err, sql.ErrNoRows) {
		return Result{}, ErrCashClosed
	}
	if err != nil {
		return Result{}, err
	}
	if operator != actor.IdentityID || sessionDevice != device.DeviceID {
		return Result{}, identity.ErrDenied
	}
	prices := make([]int64, len(in.Items))
	lineTotals := make([]int64, len(in.Items))
	type pricedItem struct {
		Item
		UnitPriceCents int64 `json:"unit_price_cents"`
		TotalCents     int64 `json:"total_cents"`
	}
	pricedItems := make([]pricedItem, 0, len(in.Items))
	consumed := make(map[string]int64)
	var total int64
	for i, item := range in.Items {
		var price int64
		var unit, kind string
		err = tx.QueryRowContext(ctx, `SELECT p.price_cents, p.unit, l.kind FROM products p
			JOIN stock_locations l ON l.tenant_id = p.tenant_id
			WHERE p.tenant_id = ? AND p.id = ? AND l.store_id = ? AND l.id = ?`,
			actor.TenantID, item.ProductID, actor.StoreID, item.LocationID).Scan(&price, &unit, &kind)
		if errors.Is(err, sql.ErrNoRows) {
			return Result{}, ErrInvalid
		}
		if err != nil {
			return Result{}, err
		}
		if e := catalog.RequireActiveProductTx(ctx, tx, actor.TenantID, item.ProductID); e != nil {
			return Result{}, e
		}
		if kind != "shelf" {
			return Result{}, ErrInvalid
		}
		if unit == "unit" && item.QuantityMilli%1000 != 0 {
			return Result{}, ErrInvalid
		}
		line, err := roundedCents(price, item.QuantityMilli)
		if err != nil || line > math.MaxInt64-total {
			return Result{}, ErrInvalid
		}
		prices[i], lineTotals[i] = price, line
		pricedItems = append(pricedItems, pricedItem{item, price, line})
		total += line
		key := item.ProductID + "\x00" + item.LocationID
		if consumed[key] > math.MaxInt64-item.QuantityMilli {
			return Result{}, ErrInvalid
		}
		consumed[key] += item.QuantityMilli
		var balance int64
		if err := tx.QueryRowContext(ctx, `SELECT COALESCE(SUM(quantity_milli),0) FROM stock_movements
			WHERE tenant_id=? AND store_id=? AND product_id=? AND location_id=?`,
			actor.TenantID, actor.StoreID, item.ProductID, item.LocationID).Scan(&balance); err != nil {
			return Result{}, err
		}
		free, err := stockreservation.FreeTx(ctx, tx, actor.TenantID, actor.StoreID, item.ProductID, item.LocationID, balance)
		if errors.Is(err, stockreservation.ErrUnavailable) {
			return Result{}, ErrNoStock
		}
		if err != nil {
			return Result{}, err
		}
		if free < consumed[key] {
			return Result{}, ErrNoStock
		}
	}
	if total <= 0 {
		return Result{}, ErrInvalid
	}
	var paid int64
	for _, payment := range in.Payments {
		if payment.Method != "cash" {
			return Result{}, ErrPaymentUnverified
		}
		if payment.AmountCents > math.MaxInt64-paid {
			return Result{}, ErrInvalid
		}
		paid += payment.AmountCents
	}
	if paid != total {
		return Result{}, ErrInvalid
	}
	var opening, movements int64
	if err = tx.QueryRowContext(ctx, `SELECT opening_cents FROM cash_sessions WHERE tenant_id=? AND id=?`, actor.TenantID, in.CashSessionID).Scan(&opening); err != nil {
		return Result{}, err
	}
	if err = tx.QueryRowContext(ctx, `SELECT COALESCE(SUM(amount_cents),0) FROM cash_movements WHERE tenant_id=? AND store_id=? AND cash_session_id=?`, actor.TenantID, actor.StoreID, in.CashSessionID).Scan(&movements); err != nil {
		return Result{}, err
	}
	if movements > 0 && opening > math.MaxInt64-movements {
		return Result{}, ErrCashOverflow
	}
	current := opening + movements
	if current < 0 || total > math.MaxInt64-current {
		return Result{}, ErrCashOverflow
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err = tx.ExecContext(ctx, `INSERT INTO sales(tenant_id,store_id,device_id,id,cash_session_id,status,total_cents,committed_at)
		VALUES (?,?,?,?,?,'committed',?,?)`, actor.TenantID, actor.StoreID, device.DeviceID, in.SaleID, in.CashSessionID, total, now)
	if err != nil {
		return Result{}, fmt.Errorf("registrar venda: %w", err)
	}
	for i, item := range in.Items {
		itemID, err := localdb.NewID()
		if err != nil {
			return Result{}, err
		}
		_, err = tx.ExecContext(ctx, `INSERT INTO sale_items VALUES (?,?,?,?,?,?,?)`, actor.TenantID, in.SaleID, itemID, item.ProductID, item.QuantityMilli, prices[i], lineTotals[i])
		if err != nil {
			return Result{}, err
		}
		movementID, err := localdb.NewID()
		if err != nil {
			return Result{}, err
		}
		_, err = tx.ExecContext(ctx, `INSERT INTO stock_movements VALUES (?,?,?,?,?,?,?,?,?)`, movementID, actor.TenantID, actor.StoreID, device.DeviceID, item.ProductID, item.LocationID, -item.QuantityMilli, "venda:"+in.SaleID, now)
		if err != nil {
			return Result{}, err
		}
		_, err = tx.ExecContext(ctx, `INSERT INTO sale_item_stock VALUES (?,?,?,?)`, actor.TenantID, in.SaleID, itemID, movementID)
		if err != nil {
			return Result{}, err
		}
	}
	for _, payment := range in.Payments {
		id, err := localdb.NewID()
		if err != nil {
			return Result{}, err
		}
		_, err = tx.ExecContext(ctx, `INSERT INTO sale_payments VALUES (?,?,?,'cash','confirmed',?)`, actor.TenantID, in.SaleID, id, payment.AmountCents)
		if err != nil {
			return Result{}, err
		}
	}
	cashID, err := localdb.NewID()
	if err != nil {
		return Result{}, err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO cash_movements VALUES (?,?,?,?,?,?,?)`, cashID, actor.TenantID, actor.StoreID, in.CashSessionID, total, "venda:"+in.SaleID, now)
	if err != nil {
		return Result{}, err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO sale_operations VALUES (?,?,?,?,?,?,?)`, actor.TenantID, actor.StoreID, device.DeviceID, in.OperationID, actor.IdentityID, in.SaleID, hash)
	if err != nil {
		return Result{}, err
	}
	payload, err := json.Marshal(struct {
		TenantID   string       `json:"tenant_id"`
		StoreID    string       `json:"store_id"`
		DeviceID   string       `json:"device_id"`
		ActorID    string       `json:"actor_id"`
		SaleID     string       `json:"sale_id"`
		SessionID  string       `json:"cash_session_id"`
		TotalCents int64        `json:"total_cents"`
		Items      []pricedItem `json:"items"`
		Payments   []Payment    `json:"payments"`
	}{actor.TenantID, actor.StoreID, device.DeviceID, actor.IdentityID, in.SaleID, in.CashSessionID, total, pricedItems, in.Payments})
	if err != nil {
		return Result{}, err
	}
	eventID, err := localdb.NewID()
	if err != nil {
		return Result{}, err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO outbox(event_id,tenant_id,store_id,device_id,operation_id,aggregate_id,event_type,schema_version,payload_json,created_at)
		VALUES (?,?,?,?,?,?,'sale.committed',1,?,?)`, eventID, actor.TenantID, actor.StoreID, device.DeviceID, in.OperationID, in.SaleID, string(payload), now)
	if err != nil {
		return Result{}, err
	}
	if err = tx.Commit(); err != nil {
		return Result{}, err
	}
	return Result{SaleID: in.SaleID, TotalCents: total}, nil
}

func valid(in Input) bool {
	if strings.TrimSpace(in.OperationID) == "" || strings.TrimSpace(in.SaleID) == "" || strings.TrimSpace(in.CashSessionID) == "" || len(in.Items) == 0 || len(in.Items) > 500 || len(in.Payments) == 0 || len(in.Payments) > 8 || len(in.OperationID) > 128 || len(in.SaleID) > 128 || len(in.CashSessionID) > 128 {
		return false
	}
	for _, item := range in.Items {
		if strings.TrimSpace(item.ProductID) == "" || strings.TrimSpace(item.LocationID) == "" || item.QuantityMilli <= 0 || len(item.ProductID) > 128 || len(item.LocationID) > 128 {
			return false
		}
	}
	for _, payment := range in.Payments {
		if payment.AmountCents <= 0 {
			return false
		}
	}
	return true
}

func roundedCents(price, quantity int64) (int64, error) {
	if price < 0 || quantity <= 0 {
		return 0, ErrInvalid
	}
	n := new(big.Int).Mul(big.NewInt(price), big.NewInt(quantity))
	n.Add(n, big.NewInt(500))
	n.Div(n, big.NewInt(1000))
	if !n.IsInt64() {
		return 0, ErrInvalid
	}
	return n.Int64(), nil
}

func authorizeLegacy(ctx context.Context, tx *sql.Tx, actor identity.Scope, device identity.DeviceContext) error {
	return identity.CanOperateTx(ctx, tx, actor, device, identity.Sell)
}
