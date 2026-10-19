// Package cashjournal persists operational cash postings in their source
// transactions. Control counterparts are not a statutory chart of accounts.
package cashjournal

import (
	"context"
	"database/sql"
	"errors"
	"math/big"
	"strings"

	"titansystem-backend/internal/localdb/identity"
)

var (
	ErrInvalid      = errors.New("consulta de journal invalida")
	ErrNotFound     = errors.New("caixa nao encontrado neste escopo")
	ErrOpen         = errors.New("journal disponivel depois do encerramento")
	ErrInconsistent = errors.New("journal e fontes de caixa nao conferem")
	ErrRange        = errors.New("journal excede inteiros exatos do cliente")
)

const maxSafe int64 = 9007199254740991

// CaptureTx runs AFTER the source's authoritative audit metadata exists and
// BEFORE its outer transaction commits. No HTTP route accepts journal postings.
func CaptureTx(ctx context.Context, tx *sql.Tx, actor identity.Scope, device identity.DeviceContext, kind, id string) error {
	if tx == nil || (kind != "opening" && kind != "movement") || id == "" {
		return ErrInvalid
	}
	var category string
	var sourceActor sql.NullString
	err := tx.QueryRowContext(ctx, `SELECT category,actor_identity_id FROM cash_journal_sources WHERE tenant_id=? AND store_id=? AND device_id=? AND source_kind=? AND source_id=?`, actor.TenantID, actor.StoreID, device.DeviceID, kind, id).Scan(&category, &sourceActor)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrInconsistent
	}
	if err != nil {
		return err
	}
	if category == "unclassified" || !sourceActor.Valid || sourceActor.String != actor.IdentityID {
		return ErrInconsistent
	}
	w, err := tx.ExecContext(ctx, `INSERT INTO cash_journal_entries(tenant_id,store_id,device_id,session_id,entry_id,source_kind,source_id,movement_id,cash_delta_cents,occurred_at,category,operation_id,actor_identity_id,counter_account)
	 SELECT tenant_id,store_id,device_id,session_id,entry_id,source_kind,source_id,movement_id,cash_delta_cents,occurred_at,category,operation_id,actor_identity_id,counter_account FROM cash_journal_sources WHERE tenant_id=? AND store_id=? AND device_id=? AND source_kind=? AND source_id=?`, actor.TenantID, actor.StoreID, device.DeviceID, kind, id)
	if err != nil {
		return err
	}
	n, err := w.RowsAffected()
	if err != nil {
		return err
	}
	if n != 1 {
		return ErrInconsistent
	}
	return nil
}

type Leg struct {
	Account     string `json:"account"`
	Side        string `json:"side"`
	AmountCents int64  `json:"amount_cents"`
}
type Entry struct {
	ID             string  `json:"id"`
	SourceKind     string  `json:"source_kind"`
	SourceID       string  `json:"source_id"`
	Category       string  `json:"category"`
	OperationID    *string `json:"operation_id"`
	ActorID        *string `json:"actor_id"`
	OccurredAt     string  `json:"occurred_at"`
	CashDeltaCents int64   `json:"cash_delta_cents"`
	Legs           []Leg   `json:"legs"`
}
type Summary struct {
	CashBalanceCents  int64 `json:"cash_balance_cents"`
	ExpectedCents     int64 `json:"expected_cents"`
	DeclaredCents     int64 `json:"declared_cents"`
	DifferenceCents   int64 `json:"difference_cents"`
	DebitCents        int64 `json:"debit_cents"`
	CreditCents       int64 `json:"credit_cents"`
	UnclassifiedCount int64 `json:"unclassified_count"`
}
type Page struct {
	SessionID      string  `json:"session_id"`
	Scope          string  `json:"scope"`
	SourceComplete bool    `json:"source_complete"`
	Offset         int     `json:"offset"`
	Limit          int     `json:"limit"`
	TotalCount     int64   `json:"total_count"`
	HasMore        bool    `json:"has_more"`
	NextOffset     *int    `json:"next_offset"`
	Summary        Summary `json:"summary"`
	Items          []Entry `json:"items"`
}

func safe(n *big.Int) (int64, error) {
	if !n.IsInt64() || n.Cmp(big.NewInt(maxSafe)) > 0 || n.Cmp(big.NewInt(-maxSafe)) < 0 {
		return 0, ErrRange
	}
	return n.Int64(), nil
}

// Read returns a page and whole-session cash summary from one consistent
// snapshot. It is restricted to the original operator's closed local drawer.
// Pagination does not mean that every page has already been retrieved.
func Read(ctx context.Context, db *sql.DB, actor identity.Scope, device identity.DeviceContext, id string, limit, offset int) (Page, error) {
	if limit < 1 || limit > 100 {
		return Page{}, ErrInvalid
	}
	return readSnapshot(ctx, db, actor, device, id, limit, offset, 0)
}

// maxRows is an export-only bound. Summary, source validation and entry rows
// share this transaction; exports never concatenate separately read pages.
func readSnapshot(ctx context.Context, db *sql.DB, actor identity.Scope, device identity.DeviceContext, id string, limit, offset, maxRows int) (Page, error) {
	if db == nil || id == "" || id != strings.TrimSpace(id) || len(id) > 128 || strings.ContainsRune(id, 0) || limit < 1 || offset < 0 || offset > 1000000000 {
		return Page{}, ErrInvalid
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return Page{}, err
	}
	defer tx.Rollback()
	if err = identity.CanOperateTx(ctx, tx, actor, device, identity.Sell); err != nil {
		return Page{}, err
	}
	var closed sql.NullString
	err = tx.QueryRowContext(ctx, `SELECT c.closed_at FROM cash_sessions c JOIN cash_session_operators o ON o.tenant_id=c.tenant_id AND o.store_id=c.store_id AND o.session_id=c.id WHERE c.tenant_id=? AND c.store_id=? AND c.device_id=? AND c.id=? AND c.operator_id=? AND o.identity_id=?`, actor.TenantID, actor.StoreID, device.DeviceID, id, actor.IdentityID, actor.IdentityID).Scan(&closed)
	if errors.Is(err, sql.ErrNoRows) {
		return Page{}, ErrNotFound
	}
	if err != nil {
		return Page{}, err
	}
	if !closed.Valid {
		return Page{}, ErrOpen
	}
	page := Page{SessionID: id, Scope: "cash_operational", Items: make([]Entry, 0), Limit: limit, Offset: offset}
	var closureTime string
	err = tx.QueryRowContext(ctx, `SELECT expected_cents,declared_cents,closed_at FROM cash_closures WHERE tenant_id=? AND store_id=? AND session_id=? AND identity_id=?`, actor.TenantID, actor.StoreID, id, actor.IdentityID).Scan(&page.Summary.ExpectedCents, &page.Summary.DeclaredCents, &closureTime)
	if errors.Is(err, sql.ErrNoRows) {
		return Page{}, ErrInconsistent
	}
	if err != nil {
		return Page{}, err
	}
	if closureTime != closed.String {
		return Page{}, ErrInconsistent
	}
	if page.Summary.ExpectedCents < 0 || page.Summary.DeclaredCents < 0 || page.Summary.ExpectedCents > maxSafe || page.Summary.DeclaredCents > maxSafe {
		return Page{}, ErrRange
	}
	if err = checkSources(ctx, tx, actor, device, id); err != nil {
		return Page{}, err
	}
	rows, err := tx.QueryContext(ctx, `SELECT cash_delta_cents,category FROM cash_journal_entries WHERE tenant_id=? AND store_id=? AND device_id=? AND session_id=? ORDER BY sequence`, actor.TenantID, actor.StoreID, device.DeviceID, id)
	if err != nil {
		return Page{}, err
	}
	balance, turnover := new(big.Int), new(big.Int)
	for rows.Next() {
		var delta int64
		var category string
		if err = rows.Scan(&delta, &category); err != nil {
			rows.Close()
			return Page{}, err
		}
		balance.Add(balance, big.NewInt(delta))
		turnover.Add(turnover, new(big.Int).Abs(big.NewInt(delta)))
		page.TotalCount++
		if maxRows > 0 && page.TotalCount > int64(maxRows) {
			rows.Close()
			return Page{}, ErrExportLimit
		}
		if category == "unclassified" {
			page.Summary.UnclassifiedCount++
		}
	}
	err = rows.Err()
	closeErr := rows.Close()
	if err != nil {
		return Page{}, err
	}
	if closeErr != nil {
		return Page{}, closeErr
	}
	if page.TotalCount > maxSafe {
		return Page{}, ErrRange
	}
	page.Summary.CashBalanceCents, err = safe(balance)
	if err != nil {
		return Page{}, err
	}
	page.Summary.DebitCents, err = safe(turnover)
	if err != nil {
		return Page{}, err
	}
	page.Summary.CreditCents = page.Summary.DebitCents
	if page.Summary.CashBalanceCents != page.Summary.ExpectedCents {
		return Page{}, ErrInconsistent
	}
	page.Summary.DifferenceCents = page.Summary.DeclaredCents - page.Summary.ExpectedCents
	rows, err = tx.QueryContext(ctx, `SELECT entry_id,source_kind,source_id,category,operation_id,actor_identity_id,occurred_at,cash_delta_cents,counter_account FROM cash_journal_entries WHERE tenant_id=? AND store_id=? AND device_id=? AND session_id=? ORDER BY sequence LIMIT ? OFFSET ?`, actor.TenantID, actor.StoreID, device.DeviceID, id, limit, offset)
	if err != nil {
		return Page{}, err
	}
	for rows.Next() {
		var item Entry
		var op, who sql.NullString
		var counter string
		if err = rows.Scan(&item.ID, &item.SourceKind, &item.SourceID, &item.Category, &op, &who, &item.OccurredAt, &item.CashDeltaCents, &counter); err != nil {
			rows.Close()
			return Page{}, err
		}
		if op.Valid {
			s := op.String
			item.OperationID = &s
		}
		if who.Valid {
			s := who.String
			item.ActorID = &s
		}
		amount, e := safe(new(big.Int).Abs(big.NewInt(item.CashDeltaCents)))
		if e != nil {
			rows.Close()
			return Page{}, e
		}
		debit, credit := "cash_drawer", counter
		if item.CashDeltaCents < 0 {
			debit, credit = counter, "cash_drawer"
		}
		item.Legs = []Leg{{Account: debit, Side: "debit", AmountCents: amount}, {Account: credit, Side: "credit", AmountCents: amount}}
		page.Items = append(page.Items, item)
	}
	err = rows.Err()
	closeErr = rows.Close()
	if err != nil {
		return Page{}, err
	}
	if closeErr != nil {
		return Page{}, closeErr
	}
	page.HasMore = int64(offset+len(page.Items)) < page.TotalCount
	if page.HasMore {
		n := offset + len(page.Items)
		if n > 1000000000 {
			return Page{}, ErrRange
		}
		page.NextOffset = &n
	}
	page.SourceComplete = true
	if err = tx.Commit(); err != nil {
		return Page{}, err
	}
	return page, nil
}

func checkSources(ctx context.Context, tx *sql.Tx, actor identity.Scope, device identity.DeviceContext, id string) error {
	var damaged int
	err := tx.QueryRowContext(ctx, `SELECT count(*) FROM cash_journal_sources s LEFT JOIN cash_journal_entries j ON j.tenant_id=s.tenant_id AND j.entry_id=s.entry_id WHERE s.tenant_id=? AND s.store_id=? AND s.device_id=? AND s.session_id=? AND (j.entry_id IS NULL OR j.store_id IS NOT s.store_id OR j.device_id IS NOT s.device_id OR j.session_id IS NOT s.session_id OR j.source_kind IS NOT s.source_kind OR j.source_id IS NOT s.source_id OR j.movement_id IS NOT s.movement_id OR j.cash_delta_cents IS NOT s.cash_delta_cents OR j.occurred_at IS NOT s.occurred_at OR j.category IS NOT s.category OR j.operation_id IS NOT s.operation_id OR j.actor_identity_id IS NOT s.actor_identity_id OR j.counter_account IS NOT s.counter_account)`, actor.TenantID, actor.StoreID, device.DeviceID, id).Scan(&damaged)
	if err != nil {
		return err
	}
	if damaged != 0 {
		return ErrInconsistent
	}
	err = tx.QueryRowContext(ctx, `SELECT count(*) FROM cash_journal_entries j
 LEFT JOIN cash_sessions c ON c.tenant_id=j.tenant_id AND c.store_id=j.store_id AND c.device_id=j.device_id AND c.id=j.session_id
 LEFT JOIN cash_movements m ON m.tenant_id=j.tenant_id AND m.store_id=j.store_id AND m.cash_session_id=j.session_id AND m.id=j.source_id
 WHERE j.tenant_id=? AND j.store_id=? AND j.device_id=? AND j.session_id=?
 AND (c.id IS NULL OR (j.source_kind='opening' AND c.id<>j.source_id) OR (j.source_kind='movement' AND m.id IS NULL))`, actor.TenantID, actor.StoreID, device.DeviceID, id).Scan(&damaged)
	if err != nil {
		return err
	}
	if damaged != 0 {
		return ErrInconsistent
	}
	return nil
}
