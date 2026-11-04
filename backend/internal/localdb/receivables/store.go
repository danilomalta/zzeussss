// Package receivables records manual external commercial documents, never a fiscal invoice.
package receivables

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"regexp"
	"strings"
	"time"
	"titansystem-backend/internal/core/modules"
	"titansystem-backend/internal/localdb/entitlementstore"
	"titansystem-backend/internal/localdb/fiscal"
	"titansystem-backend/internal/localdb/identity"
	"titansystem-backend/internal/localdb/register"
	"titansystem-backend/internal/localdb/retail"
	"titansystem-backend/internal/localdb/sale"
	"titansystem-backend/internal/localdb/supplycycle"
	"titansystem-backend/internal/sqlitewrite"
	"unicode/utf8"
)

const MaxSafe int64 = 9007199254740991

var ErrInvalid = errors.New("invalid receivable input")
var ErrConflict = errors.New("receivable revision or operation conflict")
var ErrNotFound = errors.New("receivable not found")
var validID = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]{0,127}$`).MatchString

func text(s string, n int) bool {
	return s != "" && strings.TrimSpace(s) == s && utf8.ValidString(s) && utf8.RuneCountInString(s) <= n && !strings.ContainsAny(s, "\x00\r\n")
}
func date(s string) bool {
	t, e := time.Parse("2006-01-02", s)
	return e == nil && t.Format("2006-01-02") == s
}

type Installment struct {
	ID            string `json:"id"`
	DueOn         string `json:"due_on"`
	AmountCents   int64  `json:"amount_cents"`
	ReceivedCents int64  `json:"received_cents"`
}
type CreditSaleInput struct {
	SaleID             string      `json:"sale_id"`
	SessionID          string      `json:"session_id"`
	Items              []sale.Item `json:"items"`
	TaxQuoteOperations []string    `json:"tax_quote_operations,omitempty"`
}
type CreateInput struct {
	CreditSale        *CreditSaleInput    `json:"credit_sale,omitempty"`
	CustomerOrder     *CustomerOrderInput `json:"customer_order,omitempty"`
	OperationID       string              `json:"operation_id"`
	ID                string              `json:"id"`
	CustomerReference string              `json:"customer_reference"`
	DocumentReference string              `json:"document_reference"`
	Description       string              `json:"description"`
	IssuedOn          string              `json:"issued_on"`
	TotalCents        int64               `json:"total_cents"`
	Installments      []Installment       `json:"installments"`
}
type ReceiptInput struct {
	OperationID           string `json:"operation_id"`
	InstallmentID         string `json:"installment_id"`
	SessionID             string `json:"session_id"`
	AmountCents           int64  `json:"amount_cents"`
	ExpectedReceivedCents int64  `json:"expected_received_cents"`
}
type CancelInput struct {
	OperationID string `json:"operation_id"`
	Reason      string `json:"reason"`
}
type Reversal struct {
	OperationID         string `json:"operation_id"`
	OriginalDeviceID    string `json:"original_device_id"`
	OriginalOperationID string `json:"original_operation_id"`
	AmountCents         int64  `json:"amount_cents"`
}
type Receipt struct {
	DeviceID      string `json:"device_id"`
	OperationID   string `json:"operation_id"`
	InstallmentID string `json:"installment_id"`
	SessionID     string `json:"session_id"`
	MovementID    string `json:"movement_id"`
	AmountCents   int64  `json:"amount_cents"`
	ActorID       string `json:"actor_id"`
	CreatedAt     string `json:"created_at"`
}
type Event struct {
	DeviceID    string `json:"device_id"`
	OperationID string `json:"operation_id"`
	Action      string `json:"action"`
	ActorID     string `json:"actor_id"`
	Reason      string `json:"reason"`
	CreatedAt   string `json:"created_at"`
}
type Document struct {
	Reversals         []Reversal           `json:"reversals,omitempty"`
	CreditSaleID      string               `json:"credit_sale_id,omitempty"`
	ReversedCents     int64                `json:"reversed_cents"`
	CustomerOrder     *CustomerOrderOrigin `json:"customer_order,omitempty"`
	History           []Event              `json:"history"`
	ID                string               `json:"id"`
	CustomerReference string               `json:"customer_reference"`
	DocumentReference string               `json:"document_reference"`
	Description       string               `json:"description"`
	IssuedOn          string               `json:"issued_on"`
	TotalCents        int64                `json:"total_cents"`
	ReceivedCents     int64                `json:"received_cents"`
	OutstandingCents  int64                `json:"outstanding_cents"`
	State             string               `json:"state"`
	CreatedBy         string               `json:"created_by"`
	CreatedAt         string               `json:"created_at"`
	Installments      []Installment        `json:"installments"`
	Receipts          []Receipt            `json:"receipts"`
}
type Result struct {
	Resolution  string   `json:"resolution,omitempty"`
	DocumentID  string   `json:"document_id,omitempty"`
	OperationID string   `json:"operation_id"`
	Action      string   `json:"action"`
	Repeated    bool     `json:"repeated"`
	Document    Document `json:"document"`
}
type Page struct {
	Items  []Document `json:"items"`
	Total  int        `json:"total"`
	Limit  int        `json:"limit"`
	Offset int        `json:"offset"`
}

func digest(v any) string {
	b, _ := json.Marshal(v)
	s := sha256.Sum256(b)
	return hex.EncodeToString(s[:])
}
func readTx(ctx context.Context, tx *sql.Tx, a identity.Scope, id string) (Document, error) {
	var out Document
	out.Installments = []Installment{}
	out.Receipts = []Receipt{}
	out.History = []Event{}
	e := tx.QueryRowContext(ctx, `SELECT id,customer_reference,document_reference,description,issued_on,total_cents,state,created_by,created_at FROM receivable_documents WHERE tenant_id=? AND store_id=? AND id=?`, a.TenantID, a.StoreID, id).Scan(&out.ID, &out.CustomerReference, &out.DocumentReference, &out.Description, &out.IssuedOn, &out.TotalCents, &out.State, &out.CreatedBy, &out.CreatedAt)
	if errors.Is(e, sql.ErrNoRows) {
		return out, ErrNotFound
	}
	if e != nil {
		return out, e
	}
	rows, e := tx.QueryContext(ctx, `SELECT i.id,i.due_on,i.amount_cents,COALESCE((SELECT sum(r.amount_cents) FROM receivable_cash_receipts r WHERE r.tenant_id=i.tenant_id AND r.store_id=i.store_id AND r.document_id=i.document_id AND r.installment_id=i.id),0)-COALESCE((SELECT sum(v.amount_cents) FROM receivable_cash_reversals v WHERE v.tenant_id=i.tenant_id AND v.store_id=i.store_id AND v.document_id=i.document_id AND v.installment_id=i.id),0) FROM receivable_installments i WHERE i.tenant_id=? AND i.store_id=? AND i.document_id=? ORDER BY i.due_on,i.id`, a.TenantID, a.StoreID, id)
	if e != nil {
		return out, e
	}
	var total int64
	for rows.Next() {
		var v Installment
		if e = rows.Scan(&v.ID, &v.DueOn, &v.AmountCents, &v.ReceivedCents); e != nil {
			rows.Close()
			return out, e
		}
		if v.AmountCents < 1 || v.ReceivedCents < 0 || v.ReceivedCents > v.AmountCents || total > MaxSafe-v.AmountCents {
			rows.Close()
			return out, ErrConflict
		}
		total += v.AmountCents
		out.ReceivedCents += v.ReceivedCents
		out.Installments = append(out.Installments, v)
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return out, e
	}
	if total != out.TotalCents {
		return out, ErrConflict
	}
	rows, e = tx.QueryContext(ctx, `SELECT device_id,operation_id,installment_id,session_id,movement_id,amount_cents,actor_id,created_at FROM receivable_cash_receipts WHERE tenant_id=? AND store_id=? AND document_id=? ORDER BY created_at,operation_id`, a.TenantID, a.StoreID, id)
	if e != nil {
		return out, e
	}
	for rows.Next() {
		var v Receipt
		if e = rows.Scan(&v.DeviceID, &v.OperationID, &v.InstallmentID, &v.SessionID, &v.MovementID, &v.AmountCents, &v.ActorID, &v.CreatedAt); e != nil {
			rows.Close()
			return out, e
		}
		out.Receipts = append(out.Receipts, v)
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return out, e
	}
	rows, e = tx.QueryContext(ctx, `SELECT operation_id,action,actor_id,device_id,reason,created_at FROM receivable_operations WHERE tenant_id=? AND store_id=? AND document_id=? ORDER BY created_at,operation_id`, a.TenantID, a.StoreID, id)
	if e != nil {
		return out, e
	}
	for rows.Next() {
		var v Event
		if e = rows.Scan(&v.OperationID, &v.Action, &v.ActorID, &v.DeviceID, &v.Reason, &v.CreatedAt); e != nil {
			rows.Close()
			return out, e
		}
		out.History = append(out.History, v)
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return out, e
	}
	var reversals int64
	if e = tx.QueryRowContext(ctx, `SELECT COALESCE(sum(amount_cents),0) FROM receivable_cash_reversals WHERE tenant_id=? AND store_id=? AND document_id=?`, a.TenantID, a.StoreID, id).Scan(&reversals); e != nil {
		return out, e
	}
	out.ReversedCents = reversals
	reverseRows, err := tx.QueryContext(ctx, `SELECT operation_id,original_device_id,original_operation_id,amount_cents FROM receivable_cash_reversals WHERE tenant_id=? AND store_id=? AND document_id=? ORDER BY operation_id`, a.TenantID, a.StoreID, id)
	if err != nil {
		return out, err
	}
	for reverseRows.Next() {
		var v Reversal
		if err = reverseRows.Scan(&v.OperationID, &v.OriginalDeviceID, &v.OriginalOperationID, &v.AmountCents); err != nil {
			reverseRows.Close()
			return out, err
		}
		out.Reversals = append(out.Reversals, v)
	}
	err = reverseRows.Err()
	reverseRows.Close()
	if err != nil {
		return out, err
	}
	var creditSale string
	e = tx.QueryRowContext(ctx, `SELECT sale_id FROM receivable_credit_sales WHERE tenant_id=? AND store_id=? AND document_id=?`, a.TenantID, a.StoreID, id).Scan(&creditSale)
	if e != nil && !errors.Is(e, sql.ErrNoRows) {
		return out, e
	}
	out.CreditSaleID = creditSale
	out.OutstandingCents = out.TotalCents - out.ReceivedCents
	if out.State == "cancelled" {
		if out.ReceivedCents != 0 {
			return out, ErrConflict
		}
		out.OutstandingCents = 0
	} else if out.OutstandingCents == 0 {
		out.State = "settled"
	}
	out.CustomerOrder, e = readCustomerOrderTx(ctx, tx, a, id)
	if e != nil {
		return out, e
	}
	return out, nil
}
func Read(ctx context.Context, db *sql.DB, a identity.Scope, d identity.DeviceContext, id string) (Document, error) {
	if db == nil || !validID(id) {
		return Document{}, ErrInvalid
	}
	tx, e := db.BeginTx(ctx, nil)
	if e != nil {
		return Document{}, e
	}
	defer tx.Rollback()
	if e = identity.CanOperateTx(ctx, tx, a, d, identity.ViewAccounting); e != nil {
		return Document{}, e
	}
	out, e := readTx(ctx, tx, a, id)
	if e != nil {
		return out, e
	}
	return out, tx.Commit()
}
func List(ctx context.Context, db *sql.DB, a identity.Scope, d identity.DeviceContext, limit, offset int) (Page, error) {
	out := Page{Items: []Document{}, Limit: limit, Offset: offset}
	if db == nil || limit < 1 || limit > 50 || offset < 0 || offset > 1000000 {
		return out, ErrInvalid
	}
	tx, e := db.BeginTx(ctx, nil)
	if e != nil {
		return out, e
	}
	defer tx.Rollback()
	if e = identity.CanOperateTx(ctx, tx, a, d, identity.ViewAccounting); e != nil {
		return out, e
	}
	if e = tx.QueryRowContext(ctx, `SELECT count(*) FROM receivable_documents WHERE tenant_id=? AND store_id=?`, a.TenantID, a.StoreID).Scan(&out.Total); e != nil {
		return out, e
	}
	rows, e := tx.QueryContext(ctx, `SELECT id FROM receivable_documents WHERE tenant_id=? AND store_id=? ORDER BY created_at DESC,id LIMIT ? OFFSET ?`, a.TenantID, a.StoreID, limit, offset)
	if e != nil {
		return out, e
	}
	var ids []string
	for rows.Next() {
		var id string
		if e = rows.Scan(&id); e != nil {
			rows.Close()
			return out, e
		}
		ids = append(ids, id)
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return out, e
	}
	for _, id := range ids {
		v, e := readTx(ctx, tx, a, id)
		if e != nil {
			return out, e
		}
		out.Items = append(out.Items, v)
	}
	return out, tx.Commit()
}
func Operation(ctx context.Context, db *sql.DB, a identity.Scope, d identity.DeviceContext, op string) (Result, error) {
	var out Result
	if db == nil || !validID(op) {
		return out, ErrInvalid
	}
	tx, e := db.BeginTx(ctx, nil)
	if e != nil {
		return out, e
	}
	defer tx.Rollback()
	if e = identity.CanOperateTx(ctx, tx, a, d, identity.ViewAccounting); e != nil {
		return out, e
	}
	var raw string
	e = tx.QueryRowContext(ctx, `SELECT result_json FROM receivable_operations WHERE tenant_id=? AND store_id=? AND device_id=? AND actor_id=? AND operation_id=?`, a.TenantID, a.StoreID, d.DeviceID, a.IdentityID, op).Scan(&raw)
	if errors.Is(e, sql.ErrNoRows) {
		out, e = reconciliationTx(ctx, tx, a, d, op)
		if e != nil {
			return out, e
		}
		return out, tx.Commit()
	}
	if e != nil {
		return out, e
	}
	if json.Unmarshal([]byte(raw), &out) != nil {
		return out, ErrConflict
	}
	out.Repeated = true
	return out, tx.Commit()
}
func mutate(ctx context.Context, db *sql.DB, contracts *entitlementstore.Store, a identity.Scope, d identity.DeviceContext, token, op, id, action string, input any, work func(*sql.Tx) error) (Result, error) {
	var out Result
	if db == nil || contracts == nil || !validID(op) || !validID(id) {
		return out, ErrInvalid
	}
	h := digest(struct {
		ID, Action string
		Input      any
	}{id, action, input})
	e := sqlitewrite.Run(ctx, db, ErrConflict, func(tx *sql.Tx) error {
		if e := fiscal.ValidateSaleSessionTx(ctx, tx, a, d, token); e != nil {
			return e
		}
		if e := contracts.RequireTx(ctx, tx, a, d, identity.ManageCash, modules.Finance); e != nil {
			return e
		}
		var raw, prior, actor string
		e := tx.QueryRowContext(ctx, `SELECT result_json,request_sha256,actor_id FROM receivable_operations WHERE tenant_id=? AND store_id=? AND device_id=? AND operation_id=?`, a.TenantID, a.StoreID, d.DeviceID, op).Scan(&raw, &prior, &actor)
		if e == nil {
			if prior != h || actor != a.IdentityID || json.Unmarshal([]byte(raw), &out) != nil {
				return ErrConflict
			}
			out.Repeated = true
			return nil
		}
		if !errors.Is(e, sql.ErrNoRows) {
			return e
		}
		var retired int
		if e = tx.QueryRowContext(ctx, `SELECT count(*) FROM (SELECT * FROM receivable_reconciliations UNION ALL SELECT * FROM receivable_reversal_reconciliations) WHERE tenant_id=? AND store_id=? AND device_id=? AND operation_id=?`, a.TenantID, a.StoreID, d.DeviceID, op).Scan(&retired); e != nil {
			return e
		}
		if retired != 0 {
			return ErrConflict
		}
		if e = work(tx); e != nil {
			return e
		}
		doc, e := readTx(ctx, tx, a, id)
		if e != nil {
			return e
		}
		now := time.Now().UTC().Format(time.RFC3339Nano)
		reason := ""
		if cancellation, ok := input.(CancelInput); ok {
			reason = cancellation.Reason
		}
		if reversal, ok := input.(ReversalInput); ok {
			reason = reversal.Reason
		}
		doc.History = append(doc.History, Event{OperationID: op, Action: action, ActorID: a.IdentityID, DeviceID: d.DeviceID, Reason: reason, CreatedAt: now})
		out = Result{OperationID: op, Action: action, Document: doc}
		body, _ := json.Marshal(out)
		if e = checked(ctx, tx, `INSERT INTO receivable_operations VALUES(?,?,?,?,?,?,?,?,?,?,?)`, a.TenantID, a.StoreID, d.DeviceID, op, a.IdentityID, id, action, reason, h, string(body), now); e != nil {
			return e
		}
		// Local audit only: no new message type is sent to unaware peers.
		metadata, _ := json.Marshal(map[string]string{"operation_id": op, "document_id": id, "action": action, "actor_id": a.IdentityID})
		e = checked(ctx, tx, `INSERT INTO receivable_events VALUES(?,?,?,?,?,?)`, a.TenantID, a.StoreID, d.DeviceID, op, string(metadata), now)
		return e
	})
	return out, e
}
func Create(ctx context.Context, db *sql.DB, c *entitlementstore.Store, a identity.Scope, d identity.DeviceContext, token string, in CreateInput) (Result, error) {
	if !text(in.CustomerReference, 128) || !text(in.DocumentReference, 128) || !text(in.Description, 500) || !date(in.IssuedOn) || in.TotalCents < 1 || in.TotalCents > MaxSafe || len(in.Installments) < 1 || len(in.Installments) > 120 {
		return Result{}, ErrInvalid
	}
	if in.CreditSale != nil && (in.CustomerOrder != nil || !validID(in.CreditSale.SaleID) || !validID(in.CreditSale.SessionID)) {
		return Result{}, ErrInvalid
	}
	if in.CustomerOrder != nil && (!validID(in.CustomerOrder.MessageID) || !text(in.CustomerOrder.TermsReference, 500) || !in.CustomerOrder.Confirmed) {
		return Result{}, ErrInvalid
	}
	var sum int64
	seen := map[string]bool{}
	for _, v := range in.Installments {
		if !validID(v.ID) || seen[v.ID] || !date(v.DueOn) || v.DueOn < in.IssuedOn || v.AmountCents < 1 || v.AmountCents > MaxSafe-sum || v.ReceivedCents != 0 {
			return Result{}, ErrInvalid
		}
		seen[v.ID] = true
		sum += v.AmountCents
	}
	if sum != in.TotalCents {
		return Result{}, ErrInvalid
	}
	return mutate(ctx, db, c, a, d, token, in.OperationID, in.ID, "create", in, func(tx *sql.Tx) error {
		if in.CreditSale != nil {
			if e := retail.CreditTx(ctx, tx, a, in.CustomerReference, in.TotalCents); e != nil {
				return e
			}
			saleInput := sale.Input{OperationID: in.OperationID, SaleID: in.CreditSale.SaleID, CashSessionID: in.CreditSale.SessionID, Items: in.CreditSale.Items, TaxQuoteOperations: in.CreditSale.TaxQuoteOperations, Payments: []sale.Payment{{Method: "receivable", AmountCents: in.TotalCents}}}
			result, e := sale.CompleteCreditTx(ctx, tx, c, a, d, saleInput)
			if e != nil {
				return e
			}
			if result.Repeated {
				return ErrConflict
			}
		}
		var origin *CustomerOrderOrigin
		if in.CustomerOrder != nil {
			if e := c.RequireTx(ctx, tx, a, d, identity.ViewOrders, modules.Orders); e != nil {
				return e
			}
			source, e := supplycycle.CustomerBillingOrderTx(ctx, tx, a, d, in.CustomerOrder.MessageID)
			if e != nil {
				return billingOrderError(e)
			}
			if source.CustomerReference != in.CustomerReference {
				return ErrConflict
			}
			var n int
			if e = tx.QueryRowContext(ctx, `SELECT count(*) FROM receivable_customer_orders WHERE tenant_id=? AND store_id=? AND customer_tenant_id=? AND customer_store_id=? AND order_id=? AND active=1`, a.TenantID, a.StoreID, source.CustomerTenantID, source.CustomerStoreID, source.OrderID).Scan(&n); e != nil {
				return e
			}
			if n != 0 {
				return ErrConflict
			}
			origin = &CustomerOrderOrigin{Source: source, TermsReference: in.CustomerOrder.TermsReference}
		}
		var n int
		if e := tx.QueryRowContext(ctx, `SELECT count(*) FROM receivable_documents WHERE tenant_id=? AND store_id=? AND (id=? OR (customer_reference=? AND document_reference=?))`, a.TenantID, a.StoreID, in.ID, in.CustomerReference, in.DocumentReference).Scan(&n); e != nil {
			return e
		}
		if n != 0 {
			return ErrConflict
		}
		e := checked(ctx, tx, `INSERT INTO receivable_documents VALUES(?,?,?,?,?,?,?,?,'open',?,?)`, a.TenantID, a.StoreID, in.ID, in.CustomerReference, in.DocumentReference, in.Description, in.IssuedOn, in.TotalCents, a.IdentityID, time.Now().UTC().Format(time.RFC3339Nano))
		if e != nil {
			return e
		}
		if in.CreditSale != nil {
			if e = checked(ctx, tx, `INSERT INTO receivable_credit_sales VALUES(?,?,?,?)`, a.TenantID, a.StoreID, in.ID, in.CreditSale.SaleID); e != nil {
				return e
			}
		}
		if origin != nil {
			raw, _ := json.Marshal(origin)
			if e = checked(ctx, tx, `INSERT INTO receivable_customer_orders VALUES(?,?,?,?,?,?,?,?,?,?,1)`, a.TenantID, a.StoreID, in.ID, origin.Source.MessageID, origin.Source.DecisionID, origin.TermsReference, origin.Source.CustomerTenantID, origin.Source.CustomerStoreID, origin.Source.OrderID, string(raw)); e != nil {
				return e
			}
		}
		for _, v := range in.Installments {
			if e = checked(ctx, tx, `INSERT INTO receivable_installments VALUES(?,?,?,?,?,?)`, a.TenantID, a.StoreID, in.ID, v.ID, v.DueOn, v.AmountCents); e != nil {
				return e
			}
		}
		return nil
	})
}
func ReceiveCash(ctx context.Context, db *sql.DB, c *entitlementstore.Store, a identity.Scope, d identity.DeviceContext, token, id string, in ReceiptInput) (Result, error) {
	if !validID(in.InstallmentID) || !validID(in.SessionID) || in.AmountCents < 1 || in.AmountCents > MaxSafe || in.ExpectedReceivedCents < 0 || in.ExpectedReceivedCents > MaxSafe {
		return Result{}, ErrInvalid
	}
	return mutate(ctx, db, c, a, d, token, in.OperationID, id, "receive_cash", in, func(tx *sql.Tx) error {
		doc, e := readTx(ctx, tx, a, id)
		if e != nil {
			return e
		}
		if doc.State != "open" || len(doc.Receipts) >= 1000 {
			return ErrConflict
		}
		var part *Installment
		for i := range doc.Installments {
			if doc.Installments[i].ID == in.InstallmentID {
				part = &doc.Installments[i]
			}
		}
		if part == nil {
			return ErrNotFound
		}
		if part.ReceivedCents != in.ExpectedReceivedCents || in.AmountCents > part.AmountCents-part.ReceivedCents {
			return ErrConflict
		}
		movementOp := "ar-" + digest([]string{a.TenantID, a.StoreID, d.DeviceID, in.OperationID})
		movement, e := register.MoveTxWithContract(ctx, tx, c, a, d, register.MovementInput{SessionID: in.SessionID, OperationID: movementOp, Kind: "supply", AmountCents: in.AmountCents, Reason: "Receivable cash collection " + id})
		if e != nil {
			return e
		}
		if movement.Repeated {
			return ErrConflict
		}
		var balance int64
		if e = tx.QueryRowContext(ctx, `SELECT opening_cents+COALESCE((SELECT sum(amount_cents) FROM cash_movements WHERE tenant_id=? AND store_id=? AND cash_session_id=?),0) FROM cash_sessions WHERE tenant_id=? AND store_id=? AND id=?`, a.TenantID, a.StoreID, in.SessionID, a.TenantID, a.StoreID, in.SessionID).Scan(&balance); e != nil {
			return e
		}
		if balance < 0 || balance > MaxSafe {
			return ErrConflict
		}

		e = checked(ctx, tx, `INSERT INTO receivable_cash_receipts VALUES(?,?,?,?,?,?,?,?,?,?,?,?)`, a.TenantID, a.StoreID, d.DeviceID, id, in.InstallmentID, in.OperationID, movementOp, movement.MovementID, in.SessionID, in.AmountCents, a.IdentityID, movement.CreatedAt)
		return e
	})
}
func Cancel(ctx context.Context, db *sql.DB, c *entitlementstore.Store, a identity.Scope, d identity.DeviceContext, token, id string, in CancelInput) (Result, error) {
	if !text(in.Reason, 500) {
		return Result{}, ErrInvalid
	}
	return mutate(ctx, db, c, a, d, token, in.OperationID, id, "cancel", in, func(tx *sql.Tx) error {
		doc, e := readTx(ctx, tx, a, id)
		if e != nil {
			return e
		}
		if doc.State != "open" || doc.ReceivedCents != 0 || doc.CreditSaleID != "" {
			return ErrConflict
		}
		e = checked(ctx, tx, `UPDATE receivable_documents SET state='cancelled' WHERE tenant_id=? AND store_id=? AND id=? AND state='open'`, a.TenantID, a.StoreID, id)
		if e != nil {
			return e
		}
		if doc.CustomerOrder != nil {
			return checked(ctx, tx, `UPDATE receivable_customer_orders SET active=0 WHERE tenant_id=? AND store_id=? AND document_id=? AND active=1`, a.TenantID, a.StoreID, id)
		}
		return nil
	})
}

func checked(ctx context.Context, tx *sql.Tx, query string, args ...any) error {
	result, e := tx.ExecContext(ctx, query, args...)
	if e != nil {
		return e
	}
	n, e := result.RowsAffected()
	if e != nil {
		return e
	}
	if n != 1 {
		return ErrConflict
	}
	return nil
}
