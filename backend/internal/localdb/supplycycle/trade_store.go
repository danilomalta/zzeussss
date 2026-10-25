package supplycycle

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"database/sql"
	"encoding/json"
	"errors"
	"time"
	"titansystem-backend/internal/localdb"
	"titansystem-backend/internal/localdb/entitlementstore"
	"titansystem-backend/internal/localdb/identity"
	"titansystem-backend/internal/localdb/purchases"
)

func tradeOwner(ctx context.Context, tx *sql.Tx, a identity.Scope) error {
	var role string
	if e := tx.QueryRowContext(ctx, `SELECT role FROM memberships WHERE tenant_id=? AND identity_id=?`, a.TenantID, a.IdentityID).Scan(&role); e != nil {
		return e
	}
	if role != "owner" {
		return identity.ErrDenied
	}
	return nil
}
func tradePeerTx(ctx context.Context, tx *sql.Tx, a identity.Scope, id string, active bool) (TradePeer, error) {
	var p TradePeer
	var body, tenant, store, device, status string
	var revision int64
	e := tx.QueryRowContext(ctx, `SELECT remote_tenant,remote_store,remote_device,status,revision,payload_json FROM company_trade_peers WHERE tenant_id=? AND store_id=? AND id=?`, a.TenantID, a.StoreID, id).Scan(&tenant, &store, &device, &status, &revision, &body)
	if errors.Is(e, sql.ErrNoRows) {
		return p, purchases.ErrNotFound
	}
	if e != nil {
		return p, e
	}
	if json.Unmarshal([]byte(body), &p) != nil || p.ID != id || p.Remote != (TradeParty{tenant, store, device}) || p.Status != status || p.Revision != revision || len(p.PublicKey) != 32 || TradeFingerprint(p.PublicKey) != p.FingerprintSHA256 {
		return p, purchases.ErrConflict
	}
	if active && p.Status != "active" {
		return p, identity.ErrDenied
	}
	return p, nil
}
func tradePrior(ctx context.Context, tx *sql.Tx, a identity.Scope, d identity.DeviceContext, op, body string) (TradeResult, bool, error) {
	var r TradeResult
	var actor, saved, result string
	e := tx.QueryRowContext(ctx, `SELECT actor_id,request_json,result_json FROM company_trade_operations WHERE tenant_id=? AND store_id=? AND device_id=? AND operation_id=?`, a.TenantID, a.StoreID, d.DeviceID, op).Scan(&actor, &saved, &result)
	if errors.Is(e, sql.ErrNoRows) {
		return r, false, nil
	}
	if e != nil {
		return r, false, e
	}
	if actor != a.IdentityID {
		return r, false, purchases.ErrNotFound
	}
	if saved != body {
		return r, false, purchases.ErrConflict
	}
	if json.Unmarshal([]byte(result), &r) != nil || r.OperationID != op || r.Repeated {
		return r, false, purchases.ErrConflict
	}
	r.Repeated = true
	return r, true, nil
}
func tradeFinish(ctx context.Context, tx *sql.Tx, a identity.Scope, d identity.DeviceContext, body string, r TradeResult) (TradeResult, error) {
	b, e := json.Marshal(r)
	if e != nil {
		return TradeResult{}, e
	}
	e = one(ctx, tx, `INSERT INTO company_trade_operations VALUES(?,?,?,?,?,?,?,?)`, a.TenantID, a.StoreID, d.DeviceID, r.OperationID, a.IdentityID, body, string(b), time.Now().UTC().Format(time.RFC3339Nano))
	if e != nil {
		return TradeResult{}, e
	}
	return r, tx.Commit()
}
func tradeSaveMessage(ctx context.Context, tx *sql.Tx, a identity.Scope, peer, direction string, m TradeMessage, envelope TradeEnvelope) error {
	b, _ := json.Marshal(envelope)
	return one(ctx, tx, `INSERT INTO company_trade_messages VALUES(?,?,?,?,?,?,?,?,?)`, a.TenantID, a.StoreID, m.ID, peer, direction, m.Kind, m.OrderID, m.OfferSHA256, string(b))
}
func tradeMessageTx(ctx context.Context, tx *sql.Tx, a identity.Scope, id string) (TradeMessage, TradeEnvelope, TradePeer, string, error) {
	var envelope TradeEnvelope
	var m TradeMessage
	var body, peer, direction, kind, order, offer string
	e := tx.QueryRowContext(ctx, `SELECT peer_id,direction,kind,order_id,offer_sha256,envelope_json FROM company_trade_messages WHERE tenant_id=? AND store_id=? AND id=?`, a.TenantID, a.StoreID, id).Scan(&peer, &direction, &kind, &order, &offer, &body)
	if errors.Is(e, sql.ErrNoRows) {
		e = purchases.ErrNotFound
	}
	if e != nil {
		return m, envelope, TradePeer{}, direction, e
	}
	p, e := tradePeerTx(ctx, tx, a, peer, false)
	if e != nil {
		return m, envelope, p, direction, e
	}
	if json.Unmarshal([]byte(body), &envelope) != nil {
		return m, envelope, p, direction, purchases.ErrConflict
	}
	public := ed25519.PublicKey(p.PublicKey)
	if direction == "sent" {
		// A local key may only verify history belonging to this store, never the
		// remote signature. It is not supplied by a request or by the envelope.
		var header TradeMessage
		if json.Unmarshal(envelope.Payload, &header) != nil {
			return m, envelope, p, direction, purchases.ErrConflict
		}
		if header.Source.TenantID != a.TenantID || header.Source.StoreID != a.StoreID {
			return m, envelope, p, direction, purchases.ErrConflict
		}
		e = tx.QueryRowContext(ctx, `SELECT public_key FROM device_pairings WHERE tenant_id=? AND store_id=? AND device_id=?`, a.TenantID, a.StoreID, header.Source.DeviceID).Scan(&public)
		if e != nil {
			return m, envelope, p, direction, e
		}
	}
	m, e = ParseTradeEnvelope(envelope, public)
	if e == nil && (m.ID != id || m.Kind != kind || m.OrderID != order || m.OfferSHA256 != offer) {
		e = purchases.ErrConflict
	}
	if e == nil {
		if direction == "sent" && m.Target != p.Remote || direction == "received" && (m.Source != p.Remote || m.Target.TenantID != a.TenantID || m.Target.StoreID != a.StoreID) {
			e = purchases.ErrConflict
		}
	}
	return m, envelope, p, direction, e
}
func tradeSignerTx(ctx context.Context, tx *sql.Tx, d identity.DeviceContext, k ed25519.PrivateKey) error {
	if len(k) != ed25519.PrivateKeySize {
		return identity.ErrDenied
	}
	var public []byte
	e := tx.QueryRowContext(ctx, `SELECT public_key FROM device_pairings WHERE tenant_id=? AND store_id=? AND device_id=? AND status='approved'`, d.TenantID, d.StoreID, d.DeviceID).Scan(&public)
	if errors.Is(e, sql.ErrNoRows) {
		return identity.ErrDenied
	}
	if e != nil {
		return e
	}
	if !bytes.Equal(public, k.Public().(ed25519.PublicKey)) {
		return identity.ErrDenied
	}
	return nil
}

// The owner verifies the fingerprint and business identity through a separate
// authenticated channel. A self-signed key alone is never accepted as trust.
func TrustTradePeer(ctx context.Context, db *sql.DB, c *entitlementstore.Store, a identity.Scope, d identity.DeviceContext, in TradePeerInput) (TradeResult, error) {
	if !key(in.OperationID) || !key(in.ID) || !in.Remote.valid() || in.Remote.TenantID == a.TenantID || len(in.PublicKey) != 32 || bytes.Equal(in.PublicKey, make([]byte, 32)) || !tradeHash(in.FingerprintSHA256) || in.FingerprintSHA256 != TradeFingerprint(in.PublicKey) || !text(in.VerificationReference, 255, false) || (in.Role != "supplier" && in.Role != "customer") || (in.Role == "supplier" && !key(in.SupplierID)) || (in.Role == "customer" && in.SupplierID != "") {
		return TradeResult{}, purchases.ErrInvalid
	}
	tx, e := begin(ctx, db, c, a, d, false)
	if e != nil {
		return TradeResult{}, e
	}
	defer tx.Rollback()
	if e = tradeOwner(ctx, tx, a); e != nil {
		return TradeResult{}, e
	}
	body := request(in.ID, "trust_trade_peer", in)
	if r, ok, e := tradePrior(ctx, tx, a, d, in.OperationID, body); ok || e != nil {
		return r, e
	}
	if in.Role == "supplier" {
		var count int
		e = tx.QueryRowContext(ctx, `SELECT count(*) FROM purchase_suppliers WHERE tenant_id=? AND store_id=? AND id=? AND status='active'`, a.TenantID, a.StoreID, in.SupplierID).Scan(&count)
		if e != nil {
			return TradeResult{}, e
		}
		if count != 1 {
			return TradeResult{}, purchases.ErrNotFound
		}
	}
	p := TradePeer{ID: in.ID, Remote: in.Remote, PublicKey: append([]byte(nil), in.PublicKey...), FingerprintSHA256: in.FingerprintSHA256, Role: in.Role, SupplierID: in.SupplierID, VerificationReference: in.VerificationReference, Status: "active", Revision: 1, ApprovedBy: a.IdentityID}
	b, _ := json.Marshal(p)
	e = one(ctx, tx, `INSERT INTO company_trade_peers VALUES(?,?,?,?,?,?,?,?,?)`, a.TenantID, a.StoreID, p.ID, p.Remote.TenantID, p.Remote.StoreID, p.Remote.DeviceID, p.Status, p.Revision, string(b))
	if e != nil {
		return TradeResult{}, e
	}
	return tradeFinish(ctx, tx, a, d, body, TradeResult{OperationID: in.OperationID, Peer: &p})
}
func RevokeTradePeer(ctx context.Context, db *sql.DB, c *entitlementstore.Store, a identity.Scope, d identity.DeviceContext, id string, in TradeRevokeInput) (TradeResult, error) {
	if !key(id) || !key(in.OperationID) || in.ExpectedRevision != 1 || !text(in.Reason, 500, false) {
		return TradeResult{}, purchases.ErrInvalid
	}
	tx, e := begin(ctx, db, c, a, d, false)
	if e != nil {
		return TradeResult{}, e
	}
	defer tx.Rollback()
	if e = tradeOwner(ctx, tx, a); e != nil {
		return TradeResult{}, e
	}
	body := request(id, "revoke_trade_peer", in)
	if r, ok, e := tradePrior(ctx, tx, a, d, in.OperationID, body); ok || e != nil {
		return r, e
	}
	p, e := tradePeerTx(ctx, tx, a, id, true)
	if e != nil {
		return TradeResult{}, e
	}
	if p.Revision != in.ExpectedRevision {
		return TradeResult{}, purchases.ErrConflict
	}
	p.Revision++
	p.Status = "revoked"
	b, _ := json.Marshal(p)
	e = one(ctx, tx, `UPDATE company_trade_peers SET status=?,revision=?,payload_json=? WHERE tenant_id=? AND store_id=? AND id=? AND revision=?`, p.Status, p.Revision, string(b), a.TenantID, a.StoreID, id, in.ExpectedRevision)
	if e != nil {
		return TradeResult{}, e
	}
	return tradeFinish(ctx, tx, a, d, body, TradeResult{OperationID: in.OperationID, Peer: &p})
}
func ExportTradeOrder(ctx context.Context, db *sql.DB, c *entitlementstore.Store, a identity.Scope, d identity.DeviceContext, signer ed25519.PrivateKey, id string, in TradeExportInput) (TradeResult, error) {
	if !key(id) || !key(in.OperationID) || !key(in.PeerID) || !text(in.Reason, 500, false) {
		return TradeResult{}, purchases.ErrInvalid
	}
	tx, e := begin(ctx, db, c, a, d, false)
	if e != nil {
		return TradeResult{}, e
	}
	defer tx.Rollback()
	if e = tradeReadyTx(ctx, tx, a); e != nil {
		return TradeResult{}, e
	}
	body := request(id, "export_trade_order", in)
	if r, ok, e := tradePrior(ctx, tx, a, d, in.OperationID, body); ok || e != nil {
		return r, e
	}
	p, e := tradePeerTx(ctx, tx, a, in.PeerID, true)
	if e != nil {
		return TradeResult{}, e
	}
	o, e := orderTx(ctx, tx, a, id)
	if e != nil {
		return TradeResult{}, e
	}
	if p.Role != "supplier" || p.SupplierID != o.Supplier || o.Cancelled || o.Accepted != 0 {
		return TradeResult{}, purchases.ErrConflict
	}
	dec, e := decisionTx(ctx, tx, a, id)
	if e != nil {
		return TradeResult{}, e
	}
	if dec != nil {
		return TradeResult{}, purchases.ErrConflict
	}
	if e = tradeSignerTx(ctx, tx, d, signer); e != nil {
		return TradeResult{}, e
	}
	mid, e := localdb.NewID()
	if e != nil {
		return TradeResult{}, e
	}
	now := time.Now().UTC()
	m := TradeMessage{Version: 1, Kind: "order", ID: mid, Source: tradeParty(d), Target: p.Remote, ActorID: a.IdentityID, CreatedAt: now.Format(time.RFC3339Nano), ExpiresAt: now.Add(7 * 24 * time.Hour).Format(time.RFC3339Nano), OrderID: id, ProductID: o.Product, Unit: o.Unit, QuantityMilli: o.Planned, Reason: in.Reason}
	e = tx.QueryRowContext(ctx, `SELECT sku,name FROM purchase_order_items WHERE tenant_id=? AND store_id=? AND order_id=?`, a.TenantID, a.StoreID, id).Scan(&m.SKU, &m.ProductName)
	if e != nil {
		return TradeResult{}, e
	}
	env, e := signTradeMessage(m, signer)
	if e != nil {
		return TradeResult{}, e
	}
	if e = tradeSaveMessage(ctx, tx, a, p.ID, "sent", m, env); e != nil {
		return TradeResult{}, e
	}
	return tradeFinish(ctx, tx, a, d, body, TradeResult{OperationID: in.OperationID, Message: &m, Envelope: &env})
}
func tradeIncoming(ctx context.Context, tx *sql.Tx, a identity.Scope, d identity.DeviceContext, in TradeImportInput, kind, role string) (TradeMessage, TradePeer, error) {
	p, e := tradePeerTx(ctx, tx, a, in.PeerID, true)
	if e != nil {
		return TradeMessage{}, p, e
	}
	if p.Role != role {
		return TradeMessage{}, p, identity.ErrDenied
	}
	m, e := ParseTradeEnvelope(in.Envelope, ed25519.PublicKey(p.PublicKey))
	if e != nil {
		return m, p, e
	}
	if m.Kind != kind || m.Source != p.Remote || m.Target != tradeParty(d) || !tradeFresh(m) {
		return m, p, identity.ErrDenied
	}
	return m, p, nil
}
func ImportTradeOrder(ctx context.Context, db *sql.DB, c *entitlementstore.Store, a identity.Scope, d identity.DeviceContext, in TradeImportInput) (TradeResult, error) {
	if !key(in.OperationID) || !key(in.PeerID) {
		return TradeResult{}, purchases.ErrInvalid
	}
	tx, e := begin(ctx, db, c, a, d, false)
	if e != nil {
		return TradeResult{}, e
	}
	defer tx.Rollback()
	if e = tradeReadyTx(ctx, tx, a); e != nil {
		return TradeResult{}, e
	}
	body := request("order", "import_trade_order", in)
	if r, ok, e := tradePrior(ctx, tx, a, d, in.OperationID, body); ok || e != nil {
		return r, e
	}
	m, p, e := tradeIncoming(ctx, tx, a, d, in, "order", "customer")
	if e != nil {
		return TradeResult{}, e
	}
	if e = tradeSaveMessage(ctx, tx, a, p.ID, "received", m, in.Envelope); e != nil {
		return TradeResult{}, e
	}
	return tradeFinish(ctx, tx, a, d, body, TradeResult{OperationID: in.OperationID, Message: &m, Envelope: &in.Envelope})
}
func DecideTradeOrder(ctx context.Context, db *sql.DB, c *entitlementstore.Store, a identity.Scope, d identity.DeviceContext, signer ed25519.PrivateKey, id string, in TradeDecisionInput) (TradeResult, error) {
	if !key(id) || !key(in.OperationID) || !validTradeDecision(in.Decision, in.AcceptedMilli, in.PromisedAt) || !text(in.Reason, 500, false) {
		return TradeResult{}, purchases.ErrInvalid
	}
	tx, e := begin(ctx, db, c, a, d, false)
	if e != nil {
		return TradeResult{}, e
	}
	defer tx.Rollback()
	if e = tradeReadyTx(ctx, tx, a); e != nil {
		return TradeResult{}, e
	}
	body := request(id, "decide_trade_order", in)
	if r, ok, e := tradePrior(ctx, tx, a, d, in.OperationID, body); ok || e != nil {
		return r, e
	}
	offer, env, p, direction, e := tradeMessageTx(ctx, tx, a, id)
	if e != nil {
		return TradeResult{}, e
	}
	if direction != "received" || offer.Kind != "order" || offer.Target != tradeParty(d) || p.Role != "customer" || p.Status != "active" || !tradeFresh(offer) || in.AcceptedMilli > offer.QuantityMilli {
		return TradeResult{}, purchases.ErrConflict
	}
	if e = tradeSignerTx(ctx, tx, d, signer); e != nil {
		return TradeResult{}, e
	}
	mid, e := localdb.NewID()
	if e != nil {
		return TradeResult{}, e
	}
	m := offer
	m.Kind = "decision"
	m.ID = mid
	m.Source = tradeParty(d)
	m.Target = offer.Source
	m.ActorID = a.IdentityID
	m.CreatedAt = time.Now().UTC().Format(time.RFC3339Nano)
	m.OfferSHA256 = TradeEnvelopeSHA256(env)
	m.Decision = in.Decision
	m.AcceptedMilli = in.AcceptedMilli
	m.PromisedAt = in.PromisedAt
	m.Reason = in.Reason
	signed, e := signTradeMessage(m, signer)
	if e != nil {
		return TradeResult{}, e
	}
	if e = tradeSaveMessage(ctx, tx, a, p.ID, "sent", m, signed); e != nil {
		return TradeResult{}, e
	}
	return tradeFinish(ctx, tx, a, d, body, TradeResult{OperationID: in.OperationID, Message: &m, Envelope: &signed})
}
func tradeOfferForDecision(ctx context.Context, tx *sql.Tx, a identity.Scope, m TradeMessage, p TradePeer) (TradeMessage, error) {
	var id string
	e := tx.QueryRowContext(ctx, `SELECT id FROM company_trade_messages WHERE tenant_id=? AND store_id=? AND order_id=? AND direction='sent' AND kind='order' AND peer_id=?`, a.TenantID, a.StoreID, m.OrderID, p.ID).Scan(&id)
	if errors.Is(e, sql.ErrNoRows) {
		e = purchases.ErrNotFound
	}
	if e != nil {
		return TradeMessage{}, e
	}
	offer, env, _, _, e := tradeMessageTx(ctx, tx, a, id)
	if e != nil {
		return offer, e
	}
	if m.OfferSHA256 != TradeEnvelopeSHA256(env) || m.Source != offer.Target || m.Target != offer.Source || m.ProductID != offer.ProductID || m.SKU != offer.SKU || m.ProductName != offer.ProductName || m.Unit != offer.Unit || m.QuantityMilli != offer.QuantityMilli || m.ExpiresAt != offer.ExpiresAt {
		return offer, purchases.ErrConflict
	}
	return offer, nil
}
func ApplyTradeDecision(ctx context.Context, db *sql.DB, c *entitlementstore.Store, a identity.Scope, d identity.DeviceContext, id string, in TradeImportInput) (TradeResult, error) {
	if !key(id) || !key(in.OperationID) || !key(in.PeerID) {
		return TradeResult{}, purchases.ErrInvalid
	}
	tx, e := begin(ctx, db, c, a, d, false)
	if e != nil {
		return TradeResult{}, e
	}
	defer tx.Rollback()
	if e = tradeReadyTx(ctx, tx, a); e != nil {
		return TradeResult{}, e
	}
	body := request(id, "apply_trade_decision", in)
	if r, ok, e := tradePrior(ctx, tx, a, d, in.OperationID, body); ok || e != nil {
		return r, e
	}
	m, p, e := tradeIncoming(ctx, tx, a, d, in, "decision", "supplier")
	if e != nil {
		return TradeResult{}, e
	}
	if m.OrderID != id {
		return TradeResult{}, purchases.ErrConflict
	}
	offer, e := tradeOfferForDecision(ctx, tx, a, m, p)
	if e != nil {
		return TradeResult{}, e
	}
	o, e := orderTx(ctx, tx, a, id)
	if e != nil {
		return TradeResult{}, e
	}
	if o.Cancelled || o.Accepted != 0 || o.Supplier != p.SupplierID || o.Product != offer.ProductID || o.Unit != offer.Unit || o.Planned != offer.QuantityMilli {
		return TradeResult{}, purchases.ErrConflict
	}
	previous, e := decisionTx(ctx, tx, a, id)
	if e != nil {
		return TradeResult{}, e
	}
	if previous != nil {
		return TradeResult{}, purchases.ErrConflict
	}
	if e = tradeSaveMessage(ctx, tx, a, p.ID, "received", m, in.Envelope); e != nil {
		return TradeResult{}, e
	}
	hash := TradeEnvelopeSHA256(in.Envelope)
	e = one(ctx, tx, `INSERT INTO company_trade_applied VALUES(?,?,?,?,?,1)`, a.TenantID, a.StoreID, id, m.ID, hash)
	if e != nil {
		return TradeResult{}, e
	}
	v := Decision{DecisionInput: DecisionInput{OperationID: in.OperationID, Decision: m.Decision, AcceptedMilli: m.AcceptedMilli, PromisedAt: m.PromisedAt, Contact: m.ActorID, EvidenceReference: "sha256:" + hash, ObservedAt: m.CreatedAt, Reason: m.Reason}, OrderID: id, Revision: 1, EvidenceMode: "authenticated_company", ExternalVerified: true, RecordedBy: a.IdentityID, RecordedAt: time.Now().UTC().Format(time.RFC3339Nano)}
	b, _ := json.Marshal(v)
	e = one(ctx, tx, `INSERT INTO purchase_supplier_decisions VALUES(?,?,?,?,?)`, a.TenantID, a.StoreID, id, 1, string(b))
	if e != nil {
		return TradeResult{}, e
	}
	result := Result{OperationID: in.OperationID, Decision: &v}
	if e = saveCycleOperationTx(ctx, tx, a, d, "decision", id, body, 1, result); e != nil {
		return TradeResult{}, e
	}
	return tradeFinish(ctx, tx, a, d, body, TradeResult{OperationID: in.OperationID, Message: &m, Envelope: &in.Envelope, Cycle: &result})
}

// Historical proof remains readable after revocation. Revocation blocks new
// imports/decisions; it must not rewrite an acceptance already recorded.
func verifiedTradeDecisionTx(ctx context.Context, tx *sql.Tx, a identity.Scope, v Decision) error {
	var id, hash string
	var revision int64
	e := tx.QueryRowContext(ctx, `SELECT message_id,envelope_sha256,revision FROM company_trade_applied WHERE tenant_id=? AND store_id=? AND order_id=?`, a.TenantID, a.StoreID, v.OrderID).Scan(&id, &hash, &revision)
	if e != nil {
		return purchases.ErrConflict
	}
	m, env, p, direction, e := tradeMessageTx(ctx, tx, a, id)
	if e != nil {
		return e
	}
	if direction != "received" || m.Kind != "decision" || m.OrderID != v.OrderID || revision != v.Revision || v.ExpectedRevision != 0 || hash != TradeEnvelopeSHA256(env) || v.EvidenceReference != "sha256:"+hash || v.Contact != m.ActorID || v.ObservedAt != m.CreatedAt || v.Decision != m.Decision || v.AcceptedMilli != m.AcceptedMilli || v.PromisedAt != m.PromisedAt || v.Reason != m.Reason {
		return purchases.ErrConflict
	}
	_, e = tradeOfferForDecision(ctx, tx, a, m, p)
	return e
}
func TradeOperation(ctx context.Context, db *sql.DB, a identity.Scope, d identity.DeviceContext, id string) (TradeResult, error) {
	if !key(id) {
		return TradeResult{}, purchases.ErrInvalid
	}
	tx, e := readTx(ctx, db, a, d, false)
	if e != nil {
		return TradeResult{}, e
	}
	defer tx.Rollback()
	var actor, body string
	e = tx.QueryRowContext(ctx, `SELECT actor_id,result_json FROM company_trade_operations WHERE tenant_id=? AND store_id=? AND device_id=? AND operation_id=?`, a.TenantID, a.StoreID, d.DeviceID, id).Scan(&actor, &body)
	if errors.Is(e, sql.ErrNoRows) || e == nil && actor != a.IdentityID {
		return TradeResult{}, purchases.ErrNotFound
	}
	if e != nil {
		return TradeResult{}, e
	}
	var r TradeResult
	if json.Unmarshal([]byte(body), &r) != nil || r.OperationID != id || r.Repeated {
		return r, purchases.ErrConflict
	}
	if r.Envelope != nil && r.Message != nil {
		m, env, _, _, e := tradeMessageTx(ctx, tx, a, r.Message.ID)
		if e != nil {
			return r, e
		}
		b, _ := json.Marshal(m)
		saved, _ := json.Marshal(r.Message)
		if !bytes.Equal(b, saved) || TradeEnvelopeSHA256(env) != TradeEnvelopeSHA256(*r.Envelope) {
			return r, purchases.ErrConflict
		}
	}
	if r.Cycle != nil && r.Cycle.Decision != nil {
		if e = verifiedTradeDecisionTx(ctx, tx, a, *r.Cycle.Decision); e != nil {
			return r, e
		}
	}
	return r, tx.Commit()
}
