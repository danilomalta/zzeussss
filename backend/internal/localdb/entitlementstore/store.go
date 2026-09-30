// Package entitlementstore persiste contratos verificados no SQLite local.
// Nao fornece rota publica de ativacao e nao altera casos de uso existentes.
package entitlementstore

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"time"

	"titansystem-backend/internal/core/entitlements"
	"titansystem-backend/internal/core/modules"
	"titansystem-backend/internal/localdb/identity"
)

var (
	ErrNotInstalled  = errors.New("contrato nao instalado")
	ErrStale         = errors.New("revisao de contrato antiga")
	ErrConflict      = errors.New("revisao reutilizada com outro contrato")
	ErrClockRollback = errors.New("horario anterior a observacao persistida")
	ErrCorrupt       = errors.New("estado do contrato inconsistente")
)

type Store struct {
	db       *sql.DB
	verifier *entitlements.Verifier
	clock    func() time.Time
}

type InstallResult struct {
	Revision int64
	Repeated bool
}

// New recebe uma conexao local migrada e um verificador de chaves confiaveis.
// clock pertence a configuracao do backend; nunca e recebido em pedido HTTP.
func New(db *sql.DB, verifier *entitlements.Verifier, clock func() time.Time) (*Store, error) {
	if db == nil || verifier == nil {
		return nil, errors.New("armazenamento de contratos sem banco ou verificador")
	}
	if clock == nil {
		clock = time.Now
	}
	return &Store{db: db, verifier: verifier, clock: clock}, nil
}

type record struct {
	revision int64
	observed int64
	envelope entitlements.Envelope
}

func load(ctx context.Context, tx *sql.Tx, tenantID string) (record, error) {
	var r record
	err := tx.QueryRowContext(ctx, `SELECT s.revision,s.last_observed_unix,h.key_id,h.payload,h.signature
		FROM module_contract_state s JOIN module_contract_history h
		ON h.tenant_id=s.tenant_id AND h.revision=s.revision WHERE s.tenant_id=?`, tenantID).
		Scan(&r.revision, &r.observed, &r.envelope.KeyID, &r.envelope.Payload, &r.envelope.Signature)
	if errors.Is(err, sql.ErrNoRows) {
		return record{}, ErrNotInstalled
	}
	return r, err
}

// Install exige sessao humana e aparelho previamente provados pelo chamador.
// Somente owner ativo pode instalar; ser dono nao substitui a assinatura emissora.
// Historico e ponteiro vigente sao gravados na mesma transacao.
func (s *Store) Install(ctx context.Context, actor identity.Scope, device identity.DeviceContext, envelope entitlements.Envelope) (InstallResult, error) {
	if s == nil || s.db == nil || s.verifier == nil || s.clock == nil {
		return InstallResult{}, ErrNotInstalled
	}
	now := s.clock().UTC()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return InstallResult{}, err
	}
	defer tx.Rollback()
	if err := identity.CanOperateTx(ctx, tx, actor, device, identity.ManageStaff); err != nil {
		return InstallResult{}, err
	}
	var role string
	if err := tx.QueryRowContext(ctx, `SELECT role FROM memberships WHERE tenant_id=? AND identity_id=?`, actor.TenantID, actor.IdentityID).Scan(&role); err != nil {
		return InstallResult{}, err
	}
	if role != "owner" {
		return InstallResult{}, identity.ErrDenied
	}
	claims, err := s.verifier.Verify(envelope, actor.TenantID, now)
	if err != nil {
		return InstallResult{}, err
	}
	previous, err := load(ctx, tx, actor.TenantID)
	if err != nil && !errors.Is(err, ErrNotInstalled) {
		return InstallResult{}, err
	}
	if err == nil {
		if now.Unix() < previous.observed {
			return InstallResult{}, ErrClockRollback
		}
		if claims.Revision < previous.revision {
			return InstallResult{}, ErrStale
		}
		if claims.Revision == previous.revision {
			if envelope.KeyID != previous.envelope.KeyID || !bytes.Equal(envelope.Payload, previous.envelope.Payload) || !bytes.Equal(envelope.Signature, previous.envelope.Signature) {
				return InstallResult{}, ErrConflict
			}
			if _, err := tx.ExecContext(ctx, `UPDATE module_contract_state SET last_observed_unix=? WHERE tenant_id=?`, now.Unix(), actor.TenantID); err != nil {
				return InstallResult{}, err
			}
			if err := tx.Commit(); err != nil {
				return InstallResult{}, err
			}
			return InstallResult{Revision: claims.Revision, Repeated: true}, nil
		}
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO module_contract_history
		(tenant_id,revision,key_id,payload,signature,installed_by,installed_at_unix) VALUES (?,?,?,?,?,?,?)`,
		actor.TenantID, claims.Revision, envelope.KeyID, envelope.Payload, envelope.Signature, actor.IdentityID, now.Unix()); err != nil {
		return InstallResult{}, err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO module_contract_state(tenant_id,revision,last_observed_unix) VALUES (?,?,?)
		ON CONFLICT(tenant_id) DO UPDATE SET revision=excluded.revision,last_observed_unix=excluded.last_observed_unix`,
		actor.TenantID, claims.Revision, now.Unix()); err != nil {
		return InstallResult{}, err
	}
	if err := tx.Commit(); err != nil {
		return InstallResult{}, err
	}
	return InstallResult{Revision: claims.Revision}, nil
}

// RequireTx exige autorizacao humana e do aparelho e revalida a assinatura
// armazenada antes de consultar capacidades. Deve compartilhar a transacao
// da operacao de negocio; o chamador decide commit ou rollback.
// permission e module sao escolhidos pelo caso de uso, nunca pelo pedido HTTP.
func (s *Store) RequireTx(ctx context.Context, tx *sql.Tx, actor identity.Scope, device identity.DeviceContext, permission identity.Permission, module modules.ID) error {
	if s == nil || s.db == nil || s.verifier == nil || s.clock == nil || tx == nil {
		return ErrNotInstalled
	}
	if err := identity.CanOperateTx(ctx, tx, actor, device, permission); err != nil {
		return err
	}
	now := s.clock().UTC()
	r, err := load(ctx, tx, actor.TenantID)
	if err != nil {
		return err
	}
	if now.Unix() < r.observed {
		return ErrClockRollback
	}
	claims, err := s.verifier.Verify(r.envelope, actor.TenantID, now)
	if err != nil {
		return err
	}
	if claims.Revision != r.revision {
		return ErrCorrupt
	}
	if err := modules.Require(module, claims.Modules); err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `UPDATE module_contract_state SET last_observed_unix=? WHERE tenant_id=?`, now.Unix(), actor.TenantID)
	return err
}
