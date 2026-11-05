package fiscalproduction

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"encoding/xml"
	"errors"
	"io"
	"net/http"
	"net/url"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"titansystem-backend/internal/fiscalcore"
	"titansystem-backend/internal/localdb"
	"titansystem-backend/internal/localdb/backup"
	"titansystem-backend/internal/localdb/fiscal"
	"titansystem-backend/internal/sqlitewrite"
)

// Official reference: consultar_nfe, consultar_nfce, cancelar_nfe,
// cancelar_nfce and emitir_carta_correcao on doc.focusnfe.com.br.
// Private paths are never returned to an HTTP caller or logged.
type focusArchiveLinks struct {
	Invoice          string `json:"invoice_xml_path"`
	Cancellation     string `json:"cancellation_xml_path"`
	Correction       string `json:"correction_xml_path"`
	CorrectionNumber int    `json:"correction_number"`
	DANFE            string `json:"danfe_path"`
}
type focusArchiveRecord struct {
	Version    int    `json:"version"`
	Sale       string `json:"sale_id"`
	Intent     string `json:"intent_id"`
	Key        string `json:"key"`
	Kind       string `json:"kind"`
	Number     int    `json:"number"`
	SHA256     string `json:"sha256"`
	Ciphertext []byte `json:"ciphertext"`
}

func archiveAAD(a RequestContext, r focusArchiveRecord) []byte {
	b, _ := json.Marshal([]string{"focus-xml-archive-v1", a.Actor.TenantID, a.Actor.StoreID, r.Sale, r.Intent, r.Key, r.Kind, strconv.Itoa(r.Number), r.SHA256})
	return b
}
func archiveCrypt(keyFile string) (cipher.AEAD, error) {
	if !filepath.IsAbs(keyFile) {
		return nil, ErrUnavailable
	}
	key, e := backup.ReadKey(keyFile)
	if e != nil {
		return nil, ErrUnavailable
	}
	defer clear(key)
	block, e := aes.NewCipher(key)
	if e != nil {
		return nil, ErrUnavailable
	}
	return cipher.NewGCM(block)
}
func safeFocusXMLPath(resource, key, kind string, number int) bool {
	if !fiscalcore.ValidAccessKey(key) || path.Clean(resource) != resource {
		return false
	}
	u, e := url.Parse(resource)
	if e != nil || u.Scheme != "" || u.Host != "" || u.RawQuery != "" || u.Fragment != "" {
		return false
	}
	prefix := "/arquivos/" + key[6:20] + "/"
	dev := "/arquivos_development/" + key[6:20] + "/"
	if !strings.HasPrefix(resource, prefix) && !strings.HasPrefix(resource, dev) {
		return false
	}
	for _, c := range resource {
		if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || strings.ContainsRune("/_.-", c)) {
			return false
		}
	}
	// The API supplies the complete path. Do not infer event filenames.
	switch kind {
	case "invoice":
		if number != 0 {
			return false
		}
	case "cancellation":
		if number != 1 {
			return false
		}
	case "correction":
		if number < 1 || number > 20 {
			return false
		}
	default:
		return false
	}
	return strings.HasSuffix(path.Base(resource), ".xml")
}

// Reuses embedded XSD validation; checks identity/environment in the downloaded
// processed XML. This archive is provider evidence, not new official acceptance.
func validateFocusArchiveXML(data []byte, key, kind string, number int) error {
	if !fiscalcore.ValidAccessKey(key) {
		return ErrUnavailable
	}
	schema := "nfe_process"
	if kind != "invoice" {
		schema = "nfe_event_process"
	}
	if fiscalcore.ValidateXML(schema, data) != nil {
		return ErrUnavailable
	}
	if kind == "invoice" {
		var doc struct {
			XMLName xml.Name
			NFe     struct {
				Info struct {
					ID  string `xml:"Id,attr"`
					Ide struct {
						Env string `xml:"tpAmb"`
					} `xml:"ide"`
					Emit struct {
						CNPJ string `xml:"CNPJ"`
					} `xml:"emit"`
				} `xml:"infNFe"`
			} `xml:"NFe"`
			Prot struct {
				Info struct {
					Env    string `xml:"tpAmb"`
					Key    string `xml:"chNFe"`
					Status string `xml:"cStat"`
				} `xml:"infProt"`
			} `xml:"protNFe"`
		}
		if xml.Unmarshal(data, &doc) != nil || doc.XMLName.Local != "nfeProc" || doc.NFe.Info.ID != "NFe"+key || doc.NFe.Info.Ide.Env != "2" || doc.NFe.Info.Emit.CNPJ != key[6:20] || doc.Prot.Info.Env != "2" || doc.Prot.Info.Key != key || (doc.Prot.Info.Status != "100" && doc.Prot.Info.Status != "150") {
			return ErrUnavailable
		}
	} else {
		type info struct {
			Env      string `xml:"tpAmb"`
			Key      string `xml:"chNFe"`
			Event    string `xml:"tpEvento"`
			Sequence int    `xml:"nSeqEvento"`
			Status   string `xml:"cStat"`
		}
		var doc struct {
			XMLName xml.Name
			Event   struct {
				Info info `xml:"infEvento"`
			} `xml:"evento"`
			Result struct {
				Info info `xml:"infEvento"`
			} `xml:"retEvento"`
		}
		code, seq := "110111", 1
		if kind == "correction" {
			code = "110110"
			seq = number
		}
		if xml.Unmarshal(data, &doc) != nil || doc.XMLName.Local != "procEventoNFe" || doc.Event.Info.Env != "2" || doc.Result.Info.Env != "2" || doc.Event.Info.Key != key || doc.Result.Info.Key != key || doc.Event.Info.Event != code || doc.Result.Info.Event != code || doc.Event.Info.Sequence != seq || doc.Result.Info.Sequence != seq || (doc.Result.Info.Status != "135" && doc.Result.Info.Status != "155") {
			return ErrUnavailable
		}
	}
	return nil
}
func (p *FocusProvider) downloadArchiveXML(ctx context.Context, resource, key, kind string, number int) ([]byte, error) {
	if p == nil || p.configuration.Environment != "homologation" || p.configuration.FocusBaseURL != focusHomologation || p.configuration.CertificateType != "A1" {
		return nil, ErrCheckoutProduction
	}
	if !safeFocusXMLPath(resource, key, kind, number) || p.safeMessage(resource) != resource {
		return nil, ErrUnavailable
	}
	req, e := http.NewRequestWithContext(ctx, http.MethodGet, focusHomologation+resource, nil)
	if e != nil {
		return nil, ErrUnavailable
	}
	req.SetBasicAuth(string(p.token), "")
	req.Header.Set("Accept", "application/xml")
	resp, e := p.client.Do(req)
	if e != nil {
		return nil, ErrUnavailable
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, ErrUnavailable
	}
	data, e := io.ReadAll(io.LimitReader(resp.Body, 2*fiscalcore.MaxXML+1))
	if e != nil || len(data) > 2*fiscalcore.MaxXML {
		return nil, ErrUnavailable
	}
	for _, secret := range [][]byte{p.token, p.csc} {
		if len(secret) > 0 && bytes.Contains(data, secret) {
			clear(data)
			return nil, ErrUnavailable
		}
	}
	if validateFocusArchiveXML(data, key, kind, number) != nil {
		clear(data)
		return nil, ErrUnavailable
	}
	return data, nil
}
func (p *FocusProvider) storeArchive(ctx context.Context, a RequestContext, sale, intent, key, kind string, number int, data []byte) error {
	if p.configuration.Environment != "homologation" || p.configuration.FocusBaseURL != focusHomologation || p.configuration.CertificateType != "A1" || !fiscalcore.ValidAccessKey(key) || len(data) == 0 {
		return ErrUnavailable
	}
	if e := authorizeCustomerCredentials(ctx, a); e != nil {
		return e
	}
	crypt, e := archiveCrypt(p.configuration.XMLArchiveKeyFile)
	if e != nil {
		return e
	}
	sum := sha256.Sum256(data)
	record := focusArchiveRecord{Version: 1, Sale: sale, Intent: intent, Key: key, Kind: kind, Number: number, SHA256: hex.EncodeToString(sum[:])}
	nonce := make([]byte, crypt.NonceSize())
	if _, e = rand.Read(nonce); e != nil {
		return ErrUnavailable
	}
	record.Ciphertext = crypt.Seal(nonce, nonce, data, archiveAAD(a, record))
	raw, _ := json.Marshal(record)
	id, e := localdb.NewID()
	if e != nil {
		return ErrUnavailable
	}
	opSum := sha256.Sum256([]byte(key + "/" + kind + "/" + strconv.Itoa(number)))
	operation := "focus-xml-" + hex.EncodeToString(opSum[:])
	return sqlitewrite.Run(ctx, a.DB, fiscal.ErrConflict, func(tx *sql.Tx) error {
		if e := fiscal.ValidateSaleSessionTx(ctx, tx, a.Actor, a.Device, a.Token); e != nil {
			return e
		}
		var old string
		e := tx.QueryRowContext(ctx, `SELECT payload_json FROM outbox WHERE tenant_id=? AND store_id=? AND device_id=? AND operation_id=? AND event_type='checkout.focus_xml_archive'`, a.Actor.TenantID, a.Actor.StoreID, a.Device.DeviceID, operation).Scan(&old)
		if e == nil {
			var prior focusArchiveRecord
			if json.Unmarshal([]byte(old), &prior) != nil || prior.SHA256 != record.SHA256 || prior.Sale != sale || prior.Intent != intent {
				return fiscal.ErrConflict
			}
			n := crypt.NonceSize()
			if len(prior.Ciphertext) < n {
				return fiscal.ErrConflict
			}
			plain, e := crypt.Open(nil, prior.Ciphertext[:n], prior.Ciphertext[n:], archiveAAD(a, prior))
			defer clear(plain)
			if e != nil || !bytes.Equal(plain, data) {
				return fiscal.ErrConflict
			}
			return nil
		}
		if !errors.Is(e, sql.ErrNoRows) {
			return e
		}
		result, e := tx.ExecContext(ctx, `INSERT INTO outbox(event_id,tenant_id,store_id,device_id,operation_id,aggregate_id,event_type,schema_version,payload_json,created_at)VALUES(?,?,?,?,?,?,'checkout.focus_xml_archive',1,?,?)`, id, a.Actor.TenantID, a.Actor.StoreID, a.Device.DeviceID, operation, sale, string(raw), time.Now().UTC().Format(time.RFC3339Nano))
		if e != nil {
			return e
		}
		written, e := result.RowsAffected()
		if e != nil {
			return e
		}
		if written != 1 {
			return fiscal.ErrConflict
		}
		return nil
	})
}
func (p *FocusProvider) archiveResult(ctx context.Context, a RequestContext, sale, intent string, out DocumentResult) string {
	if p.configuration.XMLArchiveKeyFile == "" {
		return "unavailable"
	}
	var links focusArchiveLinks
	if json.Unmarshal(out.Evidence, &links) != nil {
		return "pending"
	}
	type target struct {
		kind, path string
		number     int
	}
	targets := []target{}
	if out.State == "authorized_homologation" || links.Invoice != "" {
		targets = append(targets, target{"invoice", links.Invoice, 0})
	}
	if out.State == fiscal.CheckoutCancelled {
		targets = append(targets, target{"cancellation", links.Cancellation, 1})
	}
	if links.Correction != "" {
		targets = append(targets, target{"correction", links.Correction, links.CorrectionNumber})
	}
	for _, target := range targets {
		data, e := p.downloadArchiveXML(ctx, target.path, out.Key, target.kind, target.number)
		if e == nil {
			e = p.storeArchive(ctx, a, sale, intent, out.Key, target.kind, target.number, data)
		}
		clear(data)
		if e != nil {
			return "pending"
		}
	}
	return "archived"
}

// Local export decrypts only selected tenant/store rows; it never contacts Focus.
// Ciphertext is in the selected business DB and therefore in its existing backup.
func (r *ProviderRegistry) AccountingExport(ctx context.Context, a RequestContext, from, until, lang string) ([]byte, error) {
	original, e := fiscal.AccountingExport(ctx, a.DB, a.License, a.Actor, a.Device, a.Token, from, until, lang)
	if e != nil {
		return nil, e
	}
	rows, e := a.DB.QueryContext(ctx, `SELECT payload_json FROM outbox WHERE tenant_id=? AND store_id=? AND event_type='checkout.focus_xml_archive' AND julianday(created_at)>=julianday(?) AND julianday(created_at)<julianday(?) ORDER BY created_at,event_id LIMIT 501`, a.Actor.TenantID, a.Actor.StoreID, from, until)
	if e != nil {
		return nil, ErrUnavailable
	}
	var records []focusArchiveRecord
	for rows.Next() {
		var raw string
		var record focusArchiveRecord
		if rows.Scan(&raw) != nil || json.Unmarshal([]byte(raw), &record) != nil {
			rows.Close()
			return nil, ErrUnavailable
		}
		records = append(records, record)
	}
	e = rows.Err()
	rows.Close()
	if e != nil || len(records) > 500 {
		return nil, ErrUnavailable
	}
	if len(records) == 0 {
		return original, nil
	}
	if r == nil {
		return nil, ErrUnavailable
	}
	entry, ok := r.entries[a.Actor.TenantID]
	if !ok {
		return nil, ErrUnavailable
	}
	crypt, e := archiveCrypt(entry.Configuration.XMLArchiveKeyFile)
	if e != nil {
		return nil, e
	}
	reader, e := zip.NewReader(bytes.NewReader(original), int64(len(original)))
	if e != nil {
		return nil, ErrUnavailable
	}
	var result bytes.Buffer
	writer := zip.NewWriter(&result)
	var csvRows [][]string
	total := 0
	for _, file := range reader.File {
		f, e := file.Open()
		if e != nil {
			return nil, ErrUnavailable
		}
		data, e := io.ReadAll(io.LimitReader(f, 50*1024*1024+1))
		f.Close()
		if e != nil || len(data) > 50*1024*1024 {
			return nil, ErrUnavailable
		}
		if file.Name == "summary.csv" {
			csvRows, e = csv.NewReader(bytes.NewReader(data)).ReadAll()
			if e != nil {
				return nil, ErrUnavailable
			}
			continue
		}
		out, e := writer.Create(file.Name)
		if e != nil {
			return nil, ErrUnavailable
		}
		out.Write(data)
		total += len(data)
	}
	for _, record := range records {
		n := crypt.NonceSize()
		if record.Version != 1 || len(record.Ciphertext) < n || !fiscalcore.ValidAccessKey(record.Key) {
			return nil, ErrUnavailable
		}
		data, e := crypt.Open(nil, record.Ciphertext[:n], record.Ciphertext[n:], archiveAAD(a, record))
		if e != nil {
			return nil, ErrUnavailable
		}
		sum := sha256.Sum256(data)
		if hex.EncodeToString(sum[:]) != record.SHA256 || validateFocusArchiveXML(data, record.Key, record.Kind, record.Number) != nil {
			clear(data)
			return nil, ErrUnavailable
		}
		name := "focus/" + record.Key + "-" + record.Kind + "-" + strconv.Itoa(record.Number) + ".xml"
		total += len(data)
		if total > 50*1024*1024 {
			clear(data)
			return nil, ErrUnavailable
		}
		out, e := writer.Create(name)
		if e != nil {
			clear(data)
			return nil, ErrUnavailable
		}
		_, e = out.Write(data)
		clear(data)
		if e != nil {
			return nil, ErrUnavailable
		}
		// Replace only the corresponding explicit missing row; preserve all other gaps.
		id, kind := record.Sale, "focus_"+record.Kind
		if record.Kind == "invoice" {
			id = record.Intent
			kind = "invoice"
		}
		filtered := csvRows[:0]
		for _, row := range csvRows {
			if len(row) == 5 && row[0] == "'"+kind && row[1] == "'"+id && row[3] == "'xml_unavailable" {
				continue
			}
			filtered = append(filtered, row)
		}
		csvRows = filtered
		row := []string{kind, id, record.Key, "archived_homologation", name}
		for i := range row {
			row[i] = "'" + strings.ReplaceAll(strings.ReplaceAll(row[i], "\r", " "), "\n", " ")
		}
		csvRows = append(csvRows, row)
		if len(csvRows) > 501 {
			return nil, ErrUnavailable
		}
	}
	out, e := writer.Create("summary.csv")
	if e != nil {
		return nil, ErrUnavailable
	}
	csvWriter := csv.NewWriter(out)
	csvWriter.WriteAll(csvRows)
	if csvWriter.Error() != nil || writer.Close() != nil {
		return nil, ErrUnavailable
	}
	if e = authorizeCustomerCredentials(ctx, a); e != nil {
		return nil, e
	}
	return result.Bytes(), nil
}

func validateArchiveKeys(entries []ProviderRegistration) error {
	seen := map[[32]byte]bool{}
	for _, entry := range entries {
		file := entry.Configuration.XMLArchiveKeyFile
		if file == "" {
			continue
		}
		if !filepath.IsAbs(file) {
			return ErrUnavailable
		}
		key, e := backup.ReadKey(file)
		if e != nil {
			return ErrUnavailable
		}
		digest := sha256.Sum256(key)
		clear(key)
		if seen[digest] {
			return ErrUnavailable
		}
		seen[digest] = true
	}
	return nil
}
