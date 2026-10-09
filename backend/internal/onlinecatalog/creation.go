package onlinecatalog

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
	"titansystem-backend/db/migrations"
)

func creationHash(b []byte) string { return fmt.Sprintf("%x", sha256.Sum256(b)) }

type NewProduct struct {
	Name        string `json:"nome"`
	Description string `json:"descricao"`
	SKU         string `json:"sku"`
	PriceCents  int64  `json:"price_cents"`
	Stock       int64  `json:"estoque"`
}
type CreatedProduct struct {
	Product
	Stock     int64     `json:"estoque"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
type Creation struct {
	OperationID string         `json:"operation_id"`
	ActorID     string         `json:"actor_id"`
	Snapshot    CreatedProduct `json:"product"`
	CreatedAt   time.Time      `json:"created_at"`
}

// Separate namespace: creation receipts are never interpreted as edit receipts.
const creationSQL = `SELECT operation_id::text,actor_id::text,snapshot,created_at,payload_hash FROM online_catalog_creations WHERE tenant_id=$1 AND operation_id=$2`

func scanCreation(row scanner) (Creation, string, error) {
	var c Creation
	var b []byte
	var hash string
	e := row.Scan(&c.OperationID, &c.ActorID, &b, &c.CreatedAt, &hash)
	if errors.Is(e, sql.ErrNoRows) {
		return c, hash, ErrMissing
	}
	if e != nil || json.Unmarshal(b, &c.Snapshot) != nil || c.Snapshot.ID < 1 || c.Snapshot.ID > MaxVersion || c.Snapshot.Version != 1 || c.Snapshot.PriceCents < 0 || c.Snapshot.PriceCents > MaxPrice || c.Snapshot.Stock < 0 || c.Snapshot.Stock > 2147483647 {
		return Creation{}, "", ErrUnavailable
	}
	return c, hash, nil
}
func (s Store) Creation(ctx context.Context, a Actor, op string) (Creation, error) {
	if s.DB == nil {
		return Creation{}, ErrUnavailable
	}
	c, _, e := scanCreation(s.DB.QueryRowContext(ctx, creationSQL+` AND actor_id=$3`, a.Tenant, op, a.User))
	return c, e
}
func (s Store) Create(ctx context.Context, a Actor, op string, input NewProduct) (Creation, error) {
	input.Name = strings.TrimSpace(input.Name)
	input.SKU = strings.TrimSpace(input.SKU)
	if !a.allowed("details") {
		return Creation{}, ErrDenied
	}
	if !ValidUUID(op) || !clean(input.Name, 255, true) || !clean(input.Description, 2000, false) || !clean(input.SKU, 100, true) || input.PriceCents < 0 || input.PriceCents > MaxPrice || input.Stock < 0 || input.Stock > 2147483647 {
		return Creation{}, ErrInput
	}
	if s.DB == nil {
		return Creation{}, ErrUnavailable
	}
	b, _ := json.Marshal(input)
	hash := fmt.Sprintf("%x", sha256.Sum256(b))
	tx, e := s.DB.BeginTx(ctx, nil)
	if e != nil {
		return Creation{}, ErrUnavailable
	}
	defer tx.Rollback()
	var live string
	e = tx.QueryRowContext(ctx, `SELECT s.id::text FROM online_sessions s JOIN users u ON u.id=s.user_id AND u.tenant_id=s.tenant_id JOIN tenants t ON t.id=s.tenant_id WHERE s.id=$1 AND s.tenant_id=$2 AND s.user_id=$3 AND s.role=$4 AND u.role=s.role AND t.status='active' AND s.revoked_at IS NULL AND s.expires_at>clock_timestamp() FOR SHARE OF s,u,t`, a.Session, a.Tenant, a.User, a.Role).Scan(&live)
	if errors.Is(e, sql.ErrNoRows) {
		return Creation{}, ErrDenied
	}
	if e != nil {
		return Creation{}, ErrUnavailable
	}
	if _, e = tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, a.Tenant+":create:"+op); e != nil {
		return Creation{}, ErrUnavailable
	}
	prior, oldHash, e := scanCreation(tx.QueryRowContext(ctx, creationSQL, a.Tenant, op))
	if e == nil {
		if prior.ActorID != a.User || oldHash != hash {
			return Creation{}, ErrConflict
		}
		if tx.Commit() != nil {
			return Creation{}, ErrUnavailable
		}
		return prior, nil
	}
	if e != ErrMissing {
		return Creation{}, e
	}
	decimal := fmt.Sprintf("%d.%02d", input.PriceCents/100, input.PriceCents%100)
	var p CreatedProduct
	var storedPrice string
	e = tx.QueryRowContext(ctx, `INSERT INTO products(tenant_id,nome,descricao,sku,preco,estoque,ativo) VALUES($1,$2,$3,$4,$5::numeric,$6,true) RETURNING id,nome,COALESCE(descricao,''),sku,preco::text,ativo,catalog_version,estoque,created_at,updated_at`, a.Tenant, input.Name, input.Description, input.SKU, decimal, input.Stock).Scan(&p.ID, &p.Name, &p.Description, &p.SKU, &storedPrice, &p.Active, &p.Version, &p.Stock, &p.CreatedAt, &p.UpdatedAt)
	if e != nil {
		return Creation{}, conflict(e)
	}
	p.PriceCents, e = cents(storedPrice)
	if e != nil || p.ID < 1 || p.ID > MaxVersion || p.Name != input.Name || p.Description != input.Description || p.SKU != input.SKU || p.PriceCents != input.PriceCents || p.Stock != input.Stock || p.Version != 1 || !p.Active {
		return Creation{}, ErrUnavailable
	}
	snapshot, _ := json.Marshal(p)
	result := Creation{OperationID: op, ActorID: a.User, Snapshot: p}
	e = tx.QueryRowContext(ctx, `INSERT INTO online_catalog_creations(tenant_id,operation_id,actor_id,product_id,payload_hash,snapshot) VALUES($1,$2,$3,$4,$5,$6::jsonb) RETURNING created_at`, a.Tenant, op, a.User, p.ID, hash, string(snapshot)).Scan(&result.CreatedAt)
	if e != nil {
		return Creation{}, conflict(e)
	}
	event, _ := json.Marshal(result)
	r, e := tx.ExecContext(ctx, `INSERT INTO online_catalog_creation_outbox(tenant_id,operation_id,event) VALUES($1,$2,$3::jsonb)`, a.Tenant, op, string(event))
	if e != nil {
		return Creation{}, ErrUnavailable
	}
	n, e := r.RowsAffected()
	if e != nil || n != 1 {
		return Creation{}, ErrUnavailable
	}
	if tx.Commit() != nil {
		return Creation{}, ErrUnavailable
	}
	return result, nil
}
func creationChecksum() string {
	return fmt.Sprintf("%x", sha256.Sum256([]byte(migrations.CatalogCreationSQL)))
}
func MigrateCreation(ctx context.Context, db *sql.DB) error {
	if db == nil {
		return ErrUnavailable
	}
	if CheckSchema(ctx, db) != nil {
		return ErrUnavailable
	}
	tx, e := db.BeginTx(ctx, nil)
	if e != nil {
		return ErrUnavailable
	}
	defer tx.Rollback()
	if _, e = tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(748203082)`); e != nil {
		return ErrUnavailable
	}
	if _, e = tx.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS online_catalog_creation_migrations(version INTEGER PRIMARY KEY,checksum TEXT NOT NULL)`); e != nil {
		return ErrUnavailable
	}
	rows, e := tx.QueryContext(ctx, `SELECT version,checksum FROM online_catalog_creation_migrations ORDER BY version`)
	if e != nil {
		return ErrUnavailable
	}
	n := 0
	for rows.Next() {
		var v int
		var h string
		if rows.Scan(&v, &h) != nil || v != 9 || h != creationChecksum() {
			rows.Close()
			return ErrUnavailable
		}
		n++
	}
	e = rows.Err()
	rows.Close()
	if e != nil || n > 1 {
		return ErrUnavailable
	}
	if n == 0 {
		if _, e = tx.ExecContext(ctx, migrations.CatalogCreationSQL); e != nil {
			return ErrUnavailable
		}
		if _, e = tx.ExecContext(ctx, `INSERT INTO online_catalog_creation_migrations(version,checksum) VALUES(9,$1)`, creationChecksum()); e != nil {
			return ErrUnavailable
		}
	}
	if checkCreation(ctx, tx) != nil {
		return ErrUnavailable
	}
	if tx.Commit() != nil {
		return ErrUnavailable
	}
	return nil
}
func checkCreation(ctx context.Context, db schemaReader) error {
	var n int
	var v int
	var h string
	if db.QueryRowContext(ctx, `SELECT count(*),COALESCE(min(version),0),COALESCE(min(checksum),'') FROM online_catalog_creation_migrations`).Scan(&n, &v, &h) != nil || n != 1 || v != 9 || h != creationChecksum() {
		return ErrUnavailable
	}
	var ok bool
	if db.QueryRowContext(ctx, `SELECT to_regclass('online_catalog_creations') IS NOT NULL AND to_regclass('online_catalog_creation_outbox') IS NOT NULL`).Scan(&ok) != nil || !ok {
		return ErrUnavailable
	}
	return nil
}
func CheckCreation(ctx context.Context, db *sql.DB) error {
	if db == nil {
		return ErrUnavailable
	}
	return checkCreation(ctx, db)
}
