package fiscal

import (
	"archive/zip"
	"bytes"
	"context"
	"database/sql"
	"encoding/csv"
	"errors"
	"fmt"
	"strings"
	"time"
	"titansystem-backend/internal/fiscalcore"
	"titansystem-backend/internal/localdb/entitlementstore"
	"titansystem-backend/internal/localdb/identity"
)

type exportRow struct {
	Kind, ID, Key, State, File string
	XML                        []byte
}
type exportReference struct{ kind, id string }

var exportNotice = map[string]string{
	"pt-BR": "HOMOLOGAÇÃO — SEM VALOR FISCAL. Período pela data UTC de registro local. Resumo lista XMLs ausentes/incertos; não é SPED/EFD/PVA nem cobertura completa da Focus.",
	"en":    "HOMOLOGATION — NO FISCAL VALUE. Period uses local-record UTC date. Summary lists missing/uncertain XML; this is not SPED/EFD/PVA or complete Focus coverage.",
	"es":    "HOMOLOGACIÓN — SIN VALOR FISCAL. Período por fecha UTC del registro local. Resumen incluye XML ausente/incierto; no es SPED/EFD/PVA ni cobertura completa de Focus.",
	"fr":    "HOMOLOGATION — SANS VALEUR FISCALE. Période selon la date UTC du registre local. XML manquants/incertains au résumé ; ni SPED/EFD/PVA ni couverture complète Focus.",
	"zh-CN": "测试环境 — 无税务效力。期间按本地登记的 UTC 时间筛选。摘要列出缺失或不确定的 XML；不是 SPED/EFD/PVA，也不保证覆盖全部 Focus 文档。",
}

// AccountingExport reuses journal reconcilers. No transmission, filesystem path,
// tax calculation, signature or provider request participates in this read.
func AccountingExport(ctx context.Context, db *sql.DB, license *entitlementstore.Store, a identity.Scope, d identity.DeviceContext, token, from, until, lang string) ([]byte, error) {
	start, e := time.Parse(time.RFC3339, from)
	end, e2 := time.Parse(time.RFC3339, until)
	if e != nil || e2 != nil || !start.Before(end) || end.Sub(start) > 366*24*time.Hour || exportNotice[lang] == "" {
		return nil, ErrInvalid
	}
	tx, e := db.BeginTx(ctx, nil)
	if e != nil {
		return nil, e
	}
	defer tx.Rollback()
	if e = fiscalSessionTx(ctx, tx, a, d, token); e != nil {
		return nil, e
	}
	var role string
	if tx.QueryRowContext(ctx, `SELECT role FROM memberships WHERE tenant_id=? AND identity_id=? AND status='active'`, a.TenantID, a.IdentityID).Scan(&role) != nil || role != "owner" {
		return nil, identity.ErrDenied
	}
	if _, e = readPermission(ctx, tx, a, d); e != nil {
		return nil, e
	}
	// Inputs carry explicit timestamps, not an inferred fiscal competence/year.
	rows, e := tx.QueryContext(ctx, `SELECT 'invoice',id FROM fiscal_intents WHERE tenant_id=? AND store_id=? AND environment='homologation' AND julianday(created_at)>=julianday(?) AND julianday(created_at)<julianday(?) UNION ALL SELECT 'cancellation',intent_id FROM fiscal_cancellation_requests WHERE tenant_id=? AND store_id=? AND julianday(created_at)>=julianday(?) AND julianday(created_at)<julianday(?) UNION ALL SELECT 'invalidation',operation_id FROM fiscal_invalidation_requests WHERE tenant_id=? AND store_id=? AND julianday(created_at)>=julianday(?) AND julianday(created_at)<julianday(?) UNION ALL SELECT CASE event_type WHEN 'checkout.focus_cancel' THEN 'focus_cancellation' ELSE 'focus_correction' END,aggregate_id FROM outbox WHERE tenant_id=? AND store_id=? AND event_type IN ('checkout.focus_cancel','checkout.focus_correction') AND julianday(created_at)>=julianday(?) AND julianday(created_at)<julianday(?) LIMIT 501`, a.TenantID, a.StoreID, from, until, a.TenantID, a.StoreID, from, until, a.TenantID, a.StoreID, from, until, a.TenantID, a.StoreID, from, until)
	if e != nil {
		return nil, e
	}
	var refs []exportReference
	for rows.Next() {
		var ref exportReference
		if rows.Scan(&ref.kind, &ref.id) != nil {
			rows.Close()
			return nil, ErrIntegrity
		}
		refs = append(refs, ref)
	}
	e = rows.Err()
	rows.Close()
	if e != nil || len(refs) > 500 {
		return nil, ErrInvalid
	}
	if e = tx.Commit(); e != nil {
		return nil, e
	}
	var entries []exportRow
	totalBytes := 0
	for _, ref := range refs {
		before := len(entries)
		if ref.kind == "invoice" {
			row := exportInvoice(ctx, db, a, d, token, ref.id)
			entries = append(entries, row)
		} else if ref.kind == "cancellation" {
			var raw string
			if e := db.QueryRowContext(ctx, `SELECT request_json FROM fiscal_cancellation_requests WHERE tenant_id=? AND store_id=? AND intent_id=?`, a.TenantID, a.StoreID, ref.id).Scan(&raw); e == nil {
				var req fiscalcore.RecordedRequest
				event := exportRow{Kind: "cancellation", ID: ref.id, State: "xml_unavailable"}
				if decodeNativeJournal(a, ref.id, "cancel.request", &raw, &req) == nil {
					r := &HomologationCancellationRecorder{db: db, license: license, actor: a, device: d, token: token, intentID: ref.id, key: req.Key}
					if proof, e := fiscalcore.RecoverHomologationCancellation(ctx, r); e == nil && len(proof.ProcessedXML) > 0 {
						event.Key = req.Key
						event.State = "archived_homologation"
						event.XML = proof.ProcessedXML
						event.File = "events/" + req.Key + "-cancel.xml"
					}
				}
				entries = append(entries, event)
			} else if !errors.Is(e, sql.ErrNoRows) {
				return nil, ErrIntegrity
			}
		} else if ref.kind == "invalidation" {
			event := exportRow{Kind: "invalidation", ID: ref.id, State: "xml_unavailable"}
			var raw, profile, approval string
			var revision int
			e := db.QueryRowContext(ctx, `SELECT request_json,profile_id,profile_revision,approval_reference FROM fiscal_invalidation_requests WHERE tenant_id=? AND store_id=? AND operation_id=?`, a.TenantID, a.StoreID, ref.id).Scan(&raw, &profile, &revision, &approval)
			var req fiscalcore.RecordedRequest
			if e == nil && decodeNativeJournal(a, ref.id, "invalidate.request", &raw, &req) == nil {
				if original, e := fiscalcore.OriginalInvalidationFromRequest(req); e == nil {
					r, e := NewHomologationInvalidationRecorder(ctx, db, license, a, d, profile, revision, ref.id, req.URL, approval, token, original.Input)
					if e == nil {
						if proof, e := fiscalcore.RecoverHomologationInvalidation(ctx, r); e == nil && len(proof.ProcessedXML) > 0 {
							event.State = "archived_homologation"
							event.XML = proof.ProcessedXML
							event.File = "events/" + r.ID() + "-inut.xml"
						}
					}
				}
			}
			entries = append(entries, event)
		} else {
			entries = append(entries, exportRow{Kind: ref.kind, ID: ref.id, State: "xml_unavailable"})
		}
		for i := before; i < len(entries); i++ {
			totalBytes += len(entries[i].XML)
		}
		if totalBytes > 50*1024*1024 {
			return nil, ErrInvalid
		}
	}
	// A final authorization check prevents a revoked session from publishing ZIP.
	tx, e = db.BeginTx(ctx, nil)
	if e != nil {
		return nil, e
	}
	defer tx.Rollback()
	if e = fiscalSessionTx(ctx, tx, a, d, token); e != nil {
		return nil, e
	}
	if tx.QueryRowContext(ctx, `SELECT role FROM memberships WHERE tenant_id=? AND identity_id=? AND status='active'`, a.TenantID, a.IdentityID).Scan(&role) != nil || role != "owner" {
		return nil, identity.ErrDenied
	}
	if e = tx.Commit(); e != nil {
		return nil, e
	}
	return buildAccountingZIP(entries, exportNotice[lang])
}
func exportInvoice(ctx context.Context, db *sql.DB, a identity.Scope, d identity.DeviceContext, token, id string) exportRow {
	row := exportRow{Kind: "invoice", ID: id, State: "xml_unavailable"}
	tx, e := db.BeginTx(ctx, nil)
	if e != nil {
		return row
	}
	defer tx.Rollback()
	original, result, e := originalTransmissionTx(ctx, tx, a, id)
	if e != nil {
		return row
	}
	var query string
	e = tx.QueryRowContext(ctx, `SELECT query_id FROM fiscal_query_requests WHERE tenant_id=? AND store_id=? AND intent_id=? ORDER BY rowid DESC LIMIT 1`, a.TenantID, a.StoreID, id).Scan(&query)
	var xml []byte
	if errors.Is(e, sql.ErrNoRows) {
		if proof, e := fiscalcore.ReconcileHomologationIssuance(original, result); e == nil {
			xml = proof.ProcessedXML
		}
	} else if e == nil {
		if proof, _, e := cancellationAuthorizationTx(ctx, tx, a, d, id, query, token, true); e == nil {
			xml = proof.ProcessedXML
		}
	}
	if len(xml) == 0 {
		return row
	}
	row.Key = original.Key
	row.State = "archived_homologation"
	row.XML = xml
	row.File = "xml/" + original.Key + "-nfe.xml"
	return row
}
func csvData(s string) string {
	return "'" + strings.ReplaceAll(strings.ReplaceAll(s, "\r", " "), "\n", " ")
}
func buildAccountingZIP(entries []exportRow, notice string) ([]byte, error) {
	var buffer bytes.Buffer
	writer := zip.NewWriter(&buffer)
	var summary bytes.Buffer
	csvWriter := csv.NewWriter(&summary)
	csvWriter.Write([]string{"kind", "local_record_id", "key", "state", "file"})
	seen := map[string]bool{}
	size := 0
	for _, entry := range entries {
		if entry.File != "" {
			size += len(entry.XML)
			if size > 50*1024*1024 || seen[entry.File] || strings.Contains(entry.File, "..") || strings.ContainsAny(entry.File, "\\:") {
				return nil, ErrInvalid
			}
			seen[entry.File] = true
			file, e := writer.Create(entry.File)
			if e != nil {
				return nil, ErrInvalid
			}
			if _, e = file.Write(entry.XML); e != nil {
				return nil, ErrInvalid
			}
		}
		csvWriter.Write([]string{csvData(entry.Kind), csvData(entry.ID), csvData(entry.Key), csvData(entry.State), csvData(entry.File)})
	}
	csvWriter.Flush()
	if csvWriter.Error() != nil {
		return nil, ErrInvalid
	}
	for name, body := range map[string][]byte{"summary.csv": summary.Bytes(), "READ_ME.txt": []byte(notice + fmt.Sprintf("\nRecords: %d\n", len(entries)))} {
		file, e := writer.Create(name)
		if e != nil {
			return nil, ErrInvalid
		}
		if _, e = file.Write(body); e != nil {
			return nil, ErrInvalid
		}
	}
	if writer.Close() != nil {
		return nil, ErrInvalid
	}
	return buffer.Bytes(), nil
}
