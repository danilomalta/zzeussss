package fiscal

import (
	"context"
	"crypto/rand"
	"database/sql"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"titansystem-backend/internal/core/modules"
	"titansystem-backend/internal/fiscalcore"
	"titansystem-backend/internal/localdb/entitlementstore"
	"titansystem-backend/internal/localdb/identity"
)

var ErrNumberExhausted = errors.New("fiscal series requires explicit reconciliation after number exhaustion")

type SeriesOriginInput struct {
	OperationID             string `json:"operation_id"`
	ProfileID               string `json:"profile_id"`
	ProfileRevision         int    `json:"profile_revision"`
	InitialNumber           int64  `json:"initial_number"`
	ReconciliationReference string `json:"reconciliation_reference"`
}

type NumberInput struct {
	OperationID string    `json:"operation_id"`
	IssuedAt    time.Time `json:"issued_at"`
}

type NumberAssignment struct {
	IntentID    string `json:"intent_id"`
	Issuer      string `json:"issuer"`
	Model       string `json:"model"`
	Series      int    `json:"series"`
	Number      int64  `json:"number"`
	AccessKey   string `json:"access_key"`
	IssuedAt    string `json:"issued_at"`
	DeviceID    string `json:"device_id"`
	ActorID     string `json:"actor_id"`
	OperationID string `json:"operation_id"`
	CreatedAt   string `json:"created_at"`
	Repeated    bool   `json:"repeated"`
}

// Lock before reading mutable authorization state. A no-op UPDATE acquires the
// SQLite writer lock even across independent handles/processes. A denied call
// rolls back; it never leaves an allocation or advances a sequence.
func numberLockTx(ctx context.Context, tx *sql.Tx) error {
	result, err := tx.ExecContext(ctx, `UPDATE fiscal_number_lock SET marker=marker WHERE id=1`)
	if err != nil {
		return err
	}
	n, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if n != 1 {
		return ErrIntegrity
	}
	return nil
}

func numberBarrierTx(ctx context.Context, tx *sql.Tx, tenant string) error {
	var count int
	if err := tx.QueryRowContext(ctx, `SELECT count(*) FROM fiscal_restore_barriers WHERE tenant_id=?`, tenant).Scan(&count); err != nil {
		return err
	}
	if count != 0 {
		return ErrRestored
	}
	return nil
}

// ConfigureHomologationSeries records an explicit, owner-approved starting
// point; it never guesses that an existing issuer may restart at 1. The
// reference is a non-secret audit identifier, not proof of official clearance.
// This database must be the sole authority for this issuer/model/series.
func ConfigureHomologationSeries(ctx context.Context, db *sql.DB, license *entitlementstore.Store, a identity.Scope, d identity.DeviceContext, in SeriesOriginInput, token string) error {
	ref := in.ReconciliationReference
	if db == nil || license == nil || !idPattern.MatchString(in.OperationID) || !idPattern.MatchString(in.ProfileID) || in.ProfileRevision < 1 || in.InitialNumber < 1 || in.InitialNumber > 999999999 || len(ref) < 8 || len(ref) > 255 || !utf8.ValidString(ref) || strings.TrimSpace(ref) != ref {
		return ErrInvalid
	}
	for _, c := range ref {
		if unicode.IsControl(c) {
			return ErrInvalid
		}
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err = numberLockTx(ctx, tx); err != nil {
		return err
	}
	if err = fiscalSessionTx(ctx, tx, a, d, token); err != nil {
		return err
	}
	if err = license.RequireTx(ctx, tx, a, d, identity.ViewAccounting, modules.Fiscal); err != nil {
		return err
	}
	var role string
	if err = tx.QueryRowContext(ctx, `SELECT role FROM memberships WHERE tenant_id=? AND identity_id=? AND status='active'`, a.TenantID, a.IdentityID).Scan(&role); err != nil {
		return err
	}
	if role != "owner" {
		return identity.ErrDenied
	}
	if err = numberBarrierTx(ctx, tx, a.TenantID); err != nil {
		return err
	}
	profile, err := profileTx(ctx, tx, a, in.ProfileID, 0)
	if err != nil {
		return err
	}
	p := profile.Input
	if profile.Revision != in.ProfileRevision {
		return ErrConflict
	}
	if p.Environment != "homologation" {
		return fiscalcore.ErrUnsupported
	}
	var tenant, store, profileID, device, actor, op, reference string
	var revision int
	var first int64
	err = tx.QueryRowContext(ctx, `SELECT tenant_id,store_id,profile_id,profile_revision,device_id,actor_id,operation_id,initial_number,reconciliation_reference FROM fiscal_number_series WHERE issuer=? AND model=? AND series=?`, p.CNPJ, p.Model, p.Series).Scan(&tenant, &store, &profileID, &revision, &device, &actor, &op, &first, &reference)
	if err == nil {
		if tenant != a.TenantID || store != a.StoreID || profileID != in.ProfileID || revision != in.ProfileRevision || device != d.DeviceID || actor != a.IdentityID || op != in.OperationID || first != in.InitialNumber || reference != ref {
			return ErrConflict
		}
		return tx.Commit()
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	var last int64
	if err = tx.QueryRowContext(ctx, `SELECT COALESCE(CAST(max(substr(access_key,26,9)) AS INTEGER),0) FROM fiscal_transmission_requests WHERE substr(access_key,7,14)=? AND substr(access_key,21,2)=? AND substr(access_key,23,3)=?`, p.CNPJ, p.Model, formatSeries(p.Series)).Scan(&last); err != nil {
		return err
	}
	if in.InitialNumber <= last {
		return ErrConflict
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO fiscal_number_series VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?)`, p.CNPJ, p.Model, p.Series, a.TenantID, a.StoreID, in.ProfileID, in.ProfileRevision, d.DeviceID, a.IdentityID, in.OperationID, in.InitialNumber, ref, time.Now().UTC().Format(time.RFC3339Nano))
	if err != nil {
		return err
	}
	return tx.Commit()
}

// AllocateHomologationNumber commits the number, issue instant and random cNF
// atomically before signing/network. A retry with the same request returns the
// ORIGINAL key. There is no expiration, release or automatic number reuse.
func AllocateHomologationNumber(ctx context.Context, db *sql.DB, license *entitlementstore.Store, a identity.Scope, d identity.DeviceContext, intentID string, in NumberInput, token string) (NumberAssignment, error) {
	var out NumberAssignment
	if db == nil || license == nil || !idPattern.MatchString(intentID) || !idPattern.MatchString(in.OperationID) || in.IssuedAt.IsZero() {
		return out, ErrInvalid
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return out, err
	}
	defer tx.Rollback()
	if err = numberLockTx(ctx, tx); err != nil {
		return out, err
	}
	r := &HomologationRecorder{db: db, license: license, actor: a, device: d, intentID: intentID, token: token}
	intent, err := r.intentAuthorityTx(ctx, tx)
	if err != nil {
		return out, err
	}
	out, err = numberAssignmentTx(ctx, tx, a, intentID)
	if err == nil {
		if out.OperationID != in.OperationID || out.ActorID != a.IdentityID || out.DeviceID != d.DeviceID || out.IssuedAt != in.IssuedAt.Format(time.RFC3339Nano) {
			return NumberAssignment{}, ErrConflict
		}
		out.Repeated = true
		return out, tx.Commit()
	}
	if !errors.Is(err, ErrNotFound) {
		return NumberAssignment{}, err
	}
	var transmitted int
	if err = tx.QueryRowContext(ctx, `SELECT count(*) FROM fiscal_transmission_requests WHERE tenant_id=? AND store_id=? AND intent_id=?`, a.TenantID, a.StoreID, intentID).Scan(&transmitted); err != nil {
		return NumberAssignment{}, err
	}
	if transmitted != 0 {
		return NumberAssignment{}, fiscalcore.ErrRecorded
	}
	p := intent.Snapshot.Profile
	var first int64
	var tenant, store string
	if err = tx.QueryRowContext(ctx, `SELECT initial_number,tenant_id,store_id FROM fiscal_number_series WHERE issuer=? AND model=? AND series=?`, p.CNPJ, p.Model, p.Series).Scan(&first, &tenant, &store); errors.Is(err, sql.ErrNoRows) {
		return NumberAssignment{}, ErrNotFound
	} else if err != nil {
		return NumberAssignment{}, err
	}
	if tenant != a.TenantID || store != a.StoreID {
		return NumberAssignment{}, identity.ErrDenied
	}
	var number int64
	err = tx.QueryRowContext(ctx, `SELECT max(n)+1 FROM (SELECT ? AS n UNION ALL SELECT COALESCE(max(number),0) AS n FROM fiscal_number_allocations WHERE issuer=? AND model=? AND series=? UNION ALL SELECT COALESCE(CAST(max(substr(access_key,26,9)) AS INTEGER),0) AS n FROM fiscal_transmission_requests WHERE substr(access_key,7,14)=? AND substr(access_key,21,2)=? AND substr(access_key,23,3)=?)`, first-1, p.CNPJ, p.Model, p.Series, p.CNPJ, p.Model, formatSeries(p.Series)).Scan(&number)
	if err != nil {
		return NumberAssignment{}, err
	}
	if number > 999999999 {
		return NumberAssignment{}, ErrNumberExhausted
	}
	numeric, err := rand.Int(rand.Reader, big.NewInt(100000000))
	if err != nil {
		return NumberAssignment{}, err
	}
	key, err := fiscalcore.AccessKey(fiscalcore.KeyInput{UF: p.State, IssuedAt: in.IssuedAt, CNPJ: p.CNPJ, Model: p.Model, Series: p.Series, Number: number, Emission: 1, NumericCode: fmt.Sprintf("%08d", numeric.Int64())})
	if err != nil {
		return NumberAssignment{}, err
	}
	out = NumberAssignment{IntentID: intentID, Issuer: p.CNPJ, Model: p.Model, Series: p.Series, Number: number, AccessKey: key, IssuedAt: in.IssuedAt.Format(time.RFC3339Nano), DeviceID: d.DeviceID, ActorID: a.IdentityID, OperationID: in.OperationID, CreatedAt: time.Now().UTC().Format(time.RFC3339Nano)}
	_, err = tx.ExecContext(ctx, `INSERT INTO fiscal_number_allocations VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?)`, a.TenantID, a.StoreID, intentID, out.Issuer, out.Model, out.Series, out.Number, out.AccessKey, out.IssuedAt, out.DeviceID, out.ActorID, out.OperationID, out.CreatedAt)
	if err != nil {
		return NumberAssignment{}, err
	}
	return out, tx.Commit()
}

func numberAssignmentTx(ctx context.Context, tx *sql.Tx, a identity.Scope, intentID string) (NumberAssignment, error) {
	var out NumberAssignment
	err := tx.QueryRowContext(ctx, `SELECT intent_id,issuer,model,series,number,access_key,issued_at,device_id,actor_id,operation_id,created_at FROM fiscal_number_allocations WHERE tenant_id=? AND store_id=? AND intent_id=?`, a.TenantID, a.StoreID, intentID).Scan(&out.IntentID, &out.Issuer, &out.Model, &out.Series, &out.Number, &out.AccessKey, &out.IssuedAt, &out.DeviceID, &out.ActorID, &out.OperationID, &out.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return out, ErrNotFound
	}
	if err != nil {
		return out, err
	}
	issued, err := time.Parse(time.RFC3339Nano, out.IssuedAt)
	if err != nil || out.Series < 0 || out.Series > 889 || out.Number < 1 || out.Number > 999999999 || !fiscalcore.ValidAccessKey(out.AccessKey) || out.AccessKey[6:20] != out.Issuer || out.AccessKey[20:22] != out.Model || out.AccessKey[22:25] != formatSeries(out.Series) || out.AccessKey[25:34] != fmt.Sprintf("%09d", out.Number) || out.AccessKey[34:35] != "1" || out.AccessKey[2:6] != issued.Format("0601") || !idPattern.MatchString(out.OperationID) {
		return NumberAssignment{}, ErrIntegrity
	}
	return out, nil
}

// ReadHomologationNumber is a current-session recovery operation. It does not
// require the original session or a currently valid license, and never allocates.
func ReadHomologationNumber(ctx context.Context, db *sql.DB, a identity.Scope, d identity.DeviceContext, intentID, token string) (NumberAssignment, error) {
	if db == nil || !idPattern.MatchString(intentID) {
		return NumberAssignment{}, ErrInvalid
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return NumberAssignment{}, err
	}
	defer tx.Rollback()
	if err = fiscalSessionTx(ctx, tx, a, d, token); err != nil {
		return NumberAssignment{}, err
	}
	all, err := readPermission(ctx, tx, a, d)
	if err != nil {
		return NumberAssignment{}, err
	}
	out, err := numberAssignmentTx(ctx, tx, a, intentID)
	if err != nil {
		return NumberAssignment{}, err
	}
	if !all && (out.ActorID != a.IdentityID || out.DeviceID != d.DeviceID) {
		return NumberAssignment{}, ErrNotFound
	}
	return out, tx.Commit()
}
