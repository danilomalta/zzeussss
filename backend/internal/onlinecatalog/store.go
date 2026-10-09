package onlinecatalog

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5/pgconn"
	"time"
)

var (
	ErrInput       = errors.New("entrada de catálogo inválida")
	ErrMissing     = errors.New("registro de catálogo não encontrado")
	ErrConflict    = errors.New("versão, SKU ou operação em conflito")
	ErrDenied      = errors.New("sessão sem autorização")
	ErrUnavailable = errors.New("catálogo indisponível ou migração necessária")
)

type Actor struct{ Tenant, User, Session, Role string }
type Product struct {
	ID          int64  `json:"id"`
	Name        string `json:"nome"`
	Description string `json:"descricao"`
	SKU         string `json:"sku"`
	PriceCents  int64  `json:"price_cents"`
	Active      bool   `json:"ativo"`
	Version     int64  `json:"version"`
}
type Receipt struct {
	OperationID string    `json:"operation_id"`
	ProductID   int64     `json:"product_id"`
	ActorID     string    `json:"actor_id"`
	Action      string    `json:"action"`
	Before      Product   `json:"before"`
	After       Product   `json:"after"`
	CreatedAt   time.Time `json:"created_at"`
}
type Store struct{ DB *sql.DB }

const productSQL = `SELECT id,nome,COALESCE(descricao,''),sku,preco::text,ativo,catalog_version FROM products WHERE tenant_id=$1 AND id=$2 AND deleted_at IS NULL`

type scanner interface{ Scan(...interface{}) error }

func scanProduct(row scanner) (Product, error) {
	var p Product
	var price string
	e := row.Scan(&p.ID, &p.Name, &p.Description, &p.SKU, &price, &p.Active, &p.Version)
	if errors.Is(e, sql.ErrNoRows) {
		return p, ErrMissing
	}
	if e != nil {
		return p, ErrUnavailable
	}
	p.PriceCents, e = cents(price)
	if e != nil || p.ID < 1 || p.ID > MaxVersion || p.Version < 1 || p.Version > MaxVersion {
		return Product{}, ErrUnavailable
	}
	return p, nil
}
func (s Store) Product(ctx context.Context, a Actor, id int64) (Product, error) {
	if s.DB == nil {
		return Product{}, ErrUnavailable
	}
	return scanProduct(s.DB.QueryRowContext(ctx, productSQL, a.Tenant, id))
}

const receiptSQL = `SELECT operation_id::text,product_id,actor_id::text,action,before_state,after_state,created_at,payload_hash FROM online_catalog_operations WHERE tenant_id=$1 AND operation_id=$2`

func scanReceipt(row scanner) (Receipt, string, error) {
	var r Receipt
	var before, after []byte
	var h string
	e := row.Scan(&r.OperationID, &r.ProductID, &r.ActorID, &r.Action, &before, &after, &r.CreatedAt, &h)
	if errors.Is(e, sql.ErrNoRows) {
		return r, h, ErrMissing
	}
	if e != nil {
		return r, h, ErrUnavailable
	}
	if json.Unmarshal(before, &r.Before) != nil || json.Unmarshal(after, &r.After) != nil || r.Before.ID != r.ProductID || r.After.ID != r.ProductID || r.After.Version != r.Before.Version+1 {
		return Receipt{}, "", ErrUnavailable
	}
	return r, h, nil
}

// Receipt lookup is scoped to the authenticated actor, including after re-login.
func (s Store) Receipt(ctx context.Context, a Actor, op string) (Receipt, error) {
	if s.DB == nil {
		return Receipt{}, ErrUnavailable
	}
	r, _, e := scanReceipt(s.DB.QueryRowContext(ctx, receiptSQL+` AND actor_id=$3`, a.Tenant, op, a.User))
	return r, e
}
func (s Store) History(ctx context.Context, a Actor, id int64, limit, offset int) ([]Receipt, error) {
	if _, e := s.Product(ctx, a, id); e != nil {
		return nil, e
	}
	rows, e := s.DB.QueryContext(ctx, `SELECT operation_id::text,product_id,actor_id::text,action,before_state,after_state,created_at,payload_hash FROM online_catalog_operations WHERE tenant_id=$1 AND product_id=$2 ORDER BY created_at DESC,operation_id DESC LIMIT $3 OFFSET $4`, a.Tenant, id, limit, offset)
	if e != nil {
		return nil, ErrUnavailable
	}
	defer rows.Close()
	result := make([]Receipt, 0)
	for rows.Next() {
		r, _, e := scanReceipt(rows)
		if e != nil {
			return nil, e
		}
		result = append(result, r)
	}
	if rows.Err() != nil {
		return nil, ErrUnavailable
	}
	return result, nil
}
func conflict(err error) error {
	var p *pgconn.PgError
	if errors.As(err, &p) && (p.Code == "23505" || p.Code == "23514") {
		return ErrConflict
	}
	return ErrUnavailable
}
func (a Actor) allowed(action string) bool {
	if !ValidUUID(a.Tenant) || !ValidUUID(a.User) || !ValidUUID(a.Session) {
		return false
	}
	return a.Role == "owner" || a.Role == "admin" || a.Role == "manager" || (a.Role == "stock" && action == "details")
}

// Mutate locks live membership/session, operation identity and product in that
// order. Product + immutable receipt commit together. A failed commit is unknown.
func (s Store) Mutate(ctx context.Context, a Actor, id int64, action string, c Change) (Receipt, error) {
	if !a.allowed(action) {
		return Receipt{}, ErrDenied
	}
	if s.DB == nil {
		return Receipt{}, ErrUnavailable
	}
	// Public callers must use the same validation as HTTP callers.
	input := map[string]interface{}{"operation_id": c.OperationID, "expected_version": c.ExpectedVersion}
	switch action {
	case "details":
		input["nome"] = c.Name
		input["descricao"] = c.Description
		input["sku"] = c.SKU
	case "price":
		input["price_cents"] = c.PriceCents
	case "active":
		input["ativo"] = c.Active
	default:
		return Receipt{}, ErrInput
	}
	raw, _ := json.Marshal(input)
	valid, e := Decode("application/json", raw, action)
	if e != nil || id < 1 || id > MaxVersion {
		return Receipt{}, ErrInput
	}
	c = valid
	tx, e := s.DB.BeginTx(ctx, nil)
	if e != nil {
		return Receipt{}, ErrUnavailable
	}
	defer tx.Rollback()
	r, e := s.mutateTx(ctx, tx, a, id, action, c)
	if e != nil {
		return Receipt{}, e
	}
	if tx.Commit() != nil {
		return Receipt{}, ErrUnavailable
	}
	return r, nil
}

// mutateTx also serves atomic batches; only its caller commits the transaction.
func (s Store) mutateTx(ctx context.Context, tx *sql.Tx, a Actor, id int64, action string, c Change) (Receipt, error) {
	var live string
	e := tx.QueryRowContext(ctx, liveCatalogSQL, a.Session, a.Tenant, a.User, a.Role).Scan(&live)
	if errors.Is(e, sql.ErrNoRows) {
		return Receipt{}, ErrDenied
	}
	if e != nil {
		return Receipt{}, ErrUnavailable
	}
	if _, e = tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, a.Tenant+":"+c.OperationID); e != nil {
		return Receipt{}, ErrUnavailable
	}
	hash := c.hash(action, id)
	prior, priorHash, e := scanReceipt(tx.QueryRowContext(ctx, receiptSQL, a.Tenant, c.OperationID))
	if e == nil {
		if prior.ActorID != a.User || prior.ProductID != id || prior.Action != action || priorHash != hash {
			return Receipt{}, ErrConflict
		}
		return prior, nil
	}
	if !errors.Is(e, ErrMissing) {
		return Receipt{}, e
	}
	before, e := scanProduct(tx.QueryRowContext(ctx, productSQL+` FOR UPDATE`, a.Tenant, id))
	if e != nil {
		return Receipt{}, e
	}
	if before.Version != c.ExpectedVersion {
		return Receipt{}, ErrConflict
	}
	after := before
	after.Version++
	var result sql.Result
	switch action {
	case "details":
		after.Name = c.Name
		after.Description = c.Description
		after.SKU = c.SKU
		result, e = tx.ExecContext(ctx, `UPDATE products SET nome=$3,descricao=$4,sku=$5,catalog_version=catalog_version+1,updated_at=clock_timestamp() WHERE tenant_id=$1 AND id=$2 AND catalog_version=$6`, a.Tenant, id, c.Name, c.Description, c.SKU, c.ExpectedVersion)
	case "price":
		after.PriceCents = c.PriceCents
		decimal := fmt.Sprintf("%d.%02d", c.PriceCents/100, c.PriceCents%100)
		result, e = tx.ExecContext(ctx, `UPDATE products SET preco=$3::numeric,catalog_version=catalog_version+1,updated_at=clock_timestamp() WHERE tenant_id=$1 AND id=$2 AND catalog_version=$4`, a.Tenant, id, decimal, c.ExpectedVersion)
	case "active":
		after.Active = c.Active
		result, e = tx.ExecContext(ctx, `UPDATE products SET ativo=$3,catalog_version=catalog_version+1,updated_at=clock_timestamp() WHERE tenant_id=$1 AND id=$2 AND catalog_version=$4`, a.Tenant, id, c.Active, c.ExpectedVersion)
	}
	if e != nil {
		return Receipt{}, conflict(e)
	}
	n, e := result.RowsAffected()
	if e != nil || n != 1 {
		return Receipt{}, ErrUnavailable
	}
	// Read the persisted values: an existing DB trigger must not silently change
	// or suppress the operation while we publish an invented snapshot.
	persisted, e := scanProduct(tx.QueryRowContext(ctx, productSQL, a.Tenant, id))
	if e != nil || persisted != after {
		return Receipt{}, ErrUnavailable
	}
	beforeJSON, _ := json.Marshal(before)
	afterJSON, _ := json.Marshal(after)
	r := Receipt{OperationID: c.OperationID, ProductID: id, ActorID: a.User, Action: action, Before: before, After: after}
	e = tx.QueryRowContext(ctx, `INSERT INTO online_catalog_operations(tenant_id,operation_id,product_id,actor_id,action,payload_hash,before_state,after_state) VALUES($1,$2,$3,$4,$5,$6,$7::jsonb,$8::jsonb) RETURNING created_at`, a.Tenant, c.OperationID, id, a.User, action, hash, string(beforeJSON), string(afterJSON)).Scan(&r.CreatedAt)
	if e != nil {
		return Receipt{}, conflict(e)
	}
	event, _ := json.Marshal(r)
	eventResult, e := tx.ExecContext(ctx, `INSERT INTO online_catalog_outbox(tenant_id,operation_id,event) VALUES($1,$2,$3::jsonb)`, a.Tenant, c.OperationID, string(event))
	if e != nil {
		return Receipt{}, ErrUnavailable
	}
	if rows, err := eventResult.RowsAffected(); err != nil || rows != 1 {
		return Receipt{}, ErrUnavailable
	}
	return r, nil
}

const liveCatalogSQL = `SELECT s.id::text FROM online_sessions s JOIN users u ON u.id=s.user_id AND u.tenant_id=s.tenant_id JOIN tenants t ON t.id=s.tenant_id WHERE s.id=$1 AND s.tenant_id=$2 AND s.user_id=$3 AND s.role=$4 AND u.role=s.role AND t.status='active' AND s.revoked_at IS NULL AND s.expires_at>clock_timestamp() FOR SHARE OF s,u,t`
