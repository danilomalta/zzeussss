// Package retail stores only customer-controlled, explicit opt-in retail policies.
package retail

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
	"titansystem-backend/internal/sqlitewrite"
)

var ErrInvalid = errors.New("invalid retail input")
var ErrConflict = errors.New("retail revision conflict")
var ErrDisabled = errors.New("retail feature disabled or customer not eligible")
var ID = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]{0,127}$`).MatchString

const Max int64 = 9007199254740991

type Config struct {
	RefundEnabled     bool     `json:"refund_enabled"`
	Revision          int64    `json:"revision"`
	CreditEnabled     bool     `json:"credit_enabled"`
	LoyaltyEnabled    bool     `json:"loyalty_enabled"`
	MultistoreEnabled bool     `json:"multistore_enabled"`
	CreditRoles       []string `json:"credit_roles"`
	RefundRoles       []string `json:"refund_roles"`
	PointsRoles       []string `json:"points_roles"`
	// No rates, points or expiry defaults: both ratio operands and days are explicit.
	EarnCents    int64 `json:"earn_cents"`
	EarnPoints   int64 `json:"earn_points"`
	RedeemPoints int64 `json:"redeem_points"`
	RedeemCents  int64 `json:"redeem_cents"`
	ValidDays    int64 `json:"valid_days"`
}
type Customer struct {
	ID               string `json:"id"`
	Revision         int64  `json:"revision"`
	Name             string `json:"name"`
	CPF              string `json:"cpf"`
	Phone            string `json:"phone"`
	ConsentReference string `json:"consent_reference"`
	ConsentAt        string `json:"consent_at"`
	Anonymized       bool   `json:"anonymized"`
	CreditLimitCents int64  `json:"credit_limit_cents"`
	BlockOverdue     bool   `json:"block_overdue"`
	CreditEnabled    bool   `json:"credit_enabled"`
	LoyaltyConsent   bool   `json:"loyalty_consent"`
}
type Service struct {
	Clock     func() time.Time
	DB        *sql.DB
	Contracts *entitlementstore.Store
	Actor     identity.Scope
	Device    identity.DeviceContext
	Token     string
}

func Hash(v any) string {
	b, _ := json.Marshal(v)
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}
func (s Service) Authorize(ctx context.Context, tx *sql.Tx) error {
	if e := fiscal.ValidateSaleSessionTx(ctx, tx, s.Actor, s.Device, s.Token); e != nil {
		return e
	}
	return s.Contracts.RequireTx(ctx, tx, s.Actor, s.Device, identity.ManageCash, modules.Finance)
}
func Owner(ctx context.Context, tx *sql.Tx, a identity.Scope) error {
	var role, status string
	e := tx.QueryRowContext(ctx, `SELECT role,status FROM memberships WHERE tenant_id=? AND identity_id=?`, a.TenantID, a.IdentityID).Scan(&role, &status)
	if e != nil {
		return e
	}
	if role != "owner" || status != "active" {
		return identity.ErrDenied
	}
	return nil
}
func ConfigTx(ctx context.Context, tx *sql.Tx, tenant string) (Config, error) {
	c := Config{CreditRoles: []string{"owner"}, RefundRoles: []string{"owner"}, PointsRoles: []string{"owner"}}
	var raw string
	e := tx.QueryRowContext(ctx, `SELECT body FROM retail_config WHERE tenant_id=?`, tenant).Scan(&raw)
	if errors.Is(e, sql.ErrNoRows) {
		return c, nil
	}
	if e != nil {
		return c, e
	}
	e = json.Unmarshal([]byte(raw), &c)
	return c, e
}
func CustomerTx(ctx context.Context, tx *sql.Tx, tenant, id string) (Customer, error) {
	var c Customer
	var raw string
	e := tx.QueryRowContext(ctx, `SELECT body FROM retail_customers WHERE tenant_id=? AND id=?`, tenant, id).Scan(&raw)
	if errors.Is(e, sql.ErrNoRows) {
		return c, ErrDisabled
	}
	if e != nil {
		return c, e
	}
	e = json.Unmarshal([]byte(raw), &c)
	return c, e
}
func RoleTx(ctx context.Context, tx *sql.Tx, a identity.Scope, roles []string) error {
	var role, status string
	if e := tx.QueryRowContext(ctx, `SELECT role,status FROM memberships WHERE tenant_id=? AND identity_id=?`, a.TenantID, a.IdentityID).Scan(&role, &status); e != nil {
		return e
	}
	if status == "active" {
		for _, r := range roles {
			if role == r {
				return nil
			}
		}
	}
	return identity.ErrDenied
}
func rolesOK(v []string) bool {
	if len(v) == 0 {
		return false
	}
	seen := map[string]bool{}
	for _, s := range v {
		if seen[s] || (s != "owner" && s != "manager" && s != "accountant" && s != "cashier") {
			return false
		}
		seen[s] = true
	}
	return true
}

// Run acquires the shared immediate writer before any business read. Audit contains IDs/hash only.
func (s Service) Run(ctx context.Context, op, action, id string, input any, work func(*sql.Tx) error) error {
	if s.DB == nil || s.Contracts == nil || !ID(op) || !ID(id) {
		return ErrInvalid
	}
	hash := Hash(input)
	return sqlitewrite.Run(ctx, s.DB, ErrConflict, func(tx *sql.Tx) error {
		if e := s.Authorize(ctx, tx); e != nil {
			return e
		}
		var prior, actor, store, device, act, subject string
		e := tx.QueryRowContext(ctx, `SELECT request_hash,actor_id,store_id,device_id,action,subject_id FROM retail_operations WHERE tenant_id=? AND operation_id=?`, s.Actor.TenantID, op).Scan(&prior, &actor, &store, &device, &act, &subject)
		if e == nil {
			if prior != hash || actor != s.Actor.IdentityID || store != s.Actor.StoreID || device != s.Device.DeviceID || act != action || subject != id {
				return ErrConflict
			}
			return nil
		}
		if !errors.Is(e, sql.ErrNoRows) {
			return e
		}
		if e = work(tx); e != nil {
			return e
		}
		_, e = tx.ExecContext(ctx, `INSERT INTO retail_operations VALUES(?,?,?,?,?,?,?,?,?)`, s.Actor.TenantID, s.Actor.StoreID, s.Device.DeviceID, op, s.Actor.IdentityID, action, id, hash, time.Now().UTC().Format(time.RFC3339Nano))
		return e
	})
}
func (s Service) Configure(ctx context.Context, op string, c Config) error {
	if c.EarnCents < 0 || c.EarnPoints < 0 || c.RedeemCents < 0 || c.RedeemPoints < 0 || c.ValidDays < 0 {
		return ErrInvalid
	}
	if c.Revision < 0 || c.Revision >= Max || !rolesOK(c.CreditRoles) || !rolesOK(c.RefundRoles) || !rolesOK(c.PointsRoles) {
		return ErrInvalid
	}
	if c.LoyaltyEnabled && (c.EarnCents < 1 || c.EarnPoints < 1 || c.RedeemCents < 1 || c.RedeemPoints < 1 || c.ValidDays < 1 || c.ValidDays > 36500 || c.EarnCents > Max || c.EarnPoints > Max || c.RedeemCents > Max || c.RedeemPoints > Max) {
		return ErrInvalid
	}
	return s.Run(ctx, op, "configure", "config", c, func(tx *sql.Tx) error {
		if e := Owner(ctx, tx, s.Actor); e != nil {
			return e
		}
		old, e := ConfigTx(ctx, tx, s.Actor.TenantID)
		if e != nil {
			return e
		}
		if c.Revision != old.Revision {
			return ErrConflict
		}
		c.Revision++
		b, _ := json.Marshal(c)
		_, e = tx.ExecContext(ctx, `INSERT INTO retail_config VALUES(?,?,?) ON CONFLICT(tenant_id) DO UPDATE SET revision=excluded.revision,body=excluded.body`, s.Actor.TenantID, c.Revision, string(b))
		return e
	})
}
func (s Service) SaveCustomer(ctx context.Context, op string, c Customer) error {
	if !ID(c.ID) || c.Revision < 0 || c.Revision >= Max || c.Anonymized || strings.TrimSpace(c.Name) == "" || len(c.Name) > 200 || len(c.CPF) > 14 || len(c.Phone) > 40 || len(c.ConsentReference) > 200 || c.ConsentReference == "" || c.CreditLimitCents < 0 || c.CreditLimitCents > Max {
		return ErrInvalid
	}
	// Consent is an explicit local act; server time is recorded, never a default commercial term.
	if c.ConsentAt != "" {
		return ErrInvalid
	}
	return s.Run(ctx, op, "customer", c.ID, c, func(tx *sql.Tx) error {
		if e := Owner(ctx, tx, s.Actor); e != nil {
			return e
		}
		old, e := CustomerTx(ctx, tx, s.Actor.TenantID, c.ID)
		if e != nil && !errors.Is(e, ErrDisabled) {
			return e
		}
		if old.Revision != c.Revision || old.Anonymized {
			return ErrConflict
		}
		c.Revision++
		c.ConsentAt = time.Now().UTC().Format(time.RFC3339Nano)
		raw, _ := json.Marshal(c)
		_, e = tx.ExecContext(ctx, `INSERT INTO retail_customers VALUES(?,?,?,?) ON CONFLICT(tenant_id,id) DO UPDATE SET revision=excluded.revision,body=excluded.body`, s.Actor.TenantID, c.ID, c.Revision, string(raw))
		return e
	})
}
func (s Service) Privacy(ctx context.Context, op, id string, revision int64, anonymize bool) (Customer, error) {
	var out Customer
	e := s.Run(ctx, op, map[bool]string{true: "anonymize", false: "export"}[anonymize], id, struct {
		ID        string
		Revision  int64
		Anonymize bool
	}{id, revision, anonymize}, func(tx *sql.Tx) error {
		if e := Owner(ctx, tx, s.Actor); e != nil {
			return e
		}
		c, e := CustomerTx(ctx, tx, s.Actor.TenantID, id)
		if e != nil {
			return e
		}
		if c.Revision != revision {
			return ErrConflict
		}
		out = c
		if anonymize {
			c.Name = ""
			c.CPF = ""
			c.Phone = ""
			c.ConsentReference = ""
			c.Anonymized = true
			c.CreditEnabled = false
			c.LoyaltyConsent = false
			c.Revision++
			out = c
			raw, _ := json.Marshal(c)
			_, e = tx.ExecContext(ctx, `UPDATE retail_customers SET revision=?,body=? WHERE tenant_id=? AND id=?`, c.Revision, string(raw), s.Actor.TenantID, id)
			return e
		}
		return nil
	})
	// A repeated export reads only the current record: anonymized data never resurrects from a receipt.
	if e == nil {
		tx, err := s.DB.BeginTx(ctx, nil)
		if err != nil {
			return out, err
		}
		defer tx.Rollback()
		if err = s.Authorize(ctx, tx); err != nil {
			return out, err
		}
		if err = Owner(ctx, tx, s.Actor); err != nil {
			return out, err
		}
		out, e = CustomerTx(ctx, tx, s.Actor.TenantID, id)
	}
	return out, e
}
func CreditTx(ctx context.Context, tx *sql.Tx, a identity.Scope, customer string, amount int64) error {
	c, e := ConfigTx(ctx, tx, a.TenantID)
	if e != nil {
		return e
	}
	if !c.CreditEnabled {
		return ErrDisabled
	}
	if e = RoleTx(ctx, tx, a, c.CreditRoles); e != nil {
		return e
	}
	person, e := CustomerTx(ctx, tx, a.TenantID, customer)
	if e != nil {
		return e
	}
	if !person.CreditEnabled || person.Anonymized {
		return ErrDisabled
	}
	var balance, overdue int64
	e = tx.QueryRowContext(ctx, `SELECT COALESCE(sum(i.amount_cents-COALESCE((SELECT sum(r.amount_cents) FROM receivable_cash_receipts r WHERE r.tenant_id=i.tenant_id AND r.store_id=i.store_id AND r.document_id=i.document_id AND r.installment_id=i.id),0)+COALESCE((SELECT sum(v.amount_cents) FROM receivable_cash_reversals v WHERE v.tenant_id=i.tenant_id AND v.store_id=i.store_id AND v.document_id=i.document_id AND v.installment_id=i.id),0)),0),COALESCE(sum(CASE WHEN i.due_on<? AND i.amount_cents>COALESCE((SELECT sum(r.amount_cents) FROM receivable_cash_receipts r WHERE r.tenant_id=i.tenant_id AND r.store_id=i.store_id AND r.document_id=i.document_id AND r.installment_id=i.id),0)-COALESCE((SELECT sum(v.amount_cents) FROM receivable_cash_reversals v WHERE v.tenant_id=i.tenant_id AND v.store_id=i.store_id AND v.document_id=i.document_id AND v.installment_id=i.id),0) THEN 1 ELSE 0 END),0) FROM receivable_installments i JOIN receivable_documents d ON d.tenant_id=i.tenant_id AND d.store_id=i.store_id AND d.id=i.document_id WHERE d.tenant_id=? AND d.customer_reference=? AND d.state='open'`, time.Now().UTC().Format("2006-01-02"), a.TenantID, customer).Scan(&balance, &overdue)
	if e != nil {
		return e
	}
	if amount < 1 || amount > person.CreditLimitCents || balance > person.CreditLimitCents-amount || (person.BlockOverdue && overdue > 0) {
		return ErrDisabled
	}
	return nil
}

func (s Service) now() time.Time {
	if s.Clock != nil {
		return s.Clock().UTC()
	}
	return time.Now().UTC()
}
