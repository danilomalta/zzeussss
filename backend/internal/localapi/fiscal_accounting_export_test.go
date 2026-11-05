package localapi

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/url"
	"strings"
	"testing"
	"time"
	"titansystem-backend/internal/fiscalcore"
)

func TestHTTPFiscalAccountingExportContainsExactArchivedXMLAndRequiresOwner(t *testing.T) {
	f, intent, original := danfeSQLFixture(t)
	r := querySQLRecorder(t, f, f.db, intent, "danfe-authorized", f.token)
	requestRecord, resultRecord, readErr := r.Read(context.Background())
	if readErr != nil {
		t.Fatal("archive fixture", readErr)
	}
	proof, e := fiscalcore.ReconcileHomologationNFe(original, requestRecord, resultRecord)
	if e != nil || len(proof.ProcessedXML) == 0 {
		t.Fatal("original proof", e)
	}
	args := url.Values{"from": {time.Now().Add(-time.Hour).UTC().Format(time.RFC3339)}, "until": {time.Now().Add(time.Hour).UTC().Format(time.RFC3339)}, "lang": {"pt-BR"}}
	path := "/local/v1/fiscal/accounting-export?" + args.Encode()
	code, body := request(t, f.app, "GET", path, "", f.token)
	if code != 200 {
		t.Fatal("export", code, string(body))
	}
	archive, e := zip.NewReader(bytes.NewReader(body), int64(len(body)))
	if e != nil {
		t.Fatal(e)
	}
	found := false
	summary := false
	for _, file := range archive.File {
		reader, _ := file.Open()
		data, _ := io.ReadAll(reader)
		reader.Close()
		if file.Name == "xml/"+original.Key+"-nfe.xml" {
			found = bytes.Equal(data, proof.ProcessedXML)
		}
		if file.Name == "summary.csv" {
			summary = bytes.Contains(data, []byte(intent.ID))
		}
	}
	if !found || !summary {
		t.Fatal("archive does not contain exact source and summary")
	}
	for _, role := range []string{"manager", "accountant", "cashier"} {
		code, _ := request(t, f.app, "POST", "/local/v1/staff", `{"operation_id":"export-`+role+`","identity_id":"export-`+role+`","name":"Synthetic","role":"`+role+`","password":"test-password-strong-1"}`, f.token)
		if code != 201 {
			t.Fatal("staff", code)
		}
		code, raw := request(t, f.app, "POST", "/local/v1/login", `{"identity_id":"export-`+role+`","password":"test-password-strong-1"}`, "")
		if code != 200 {
			t.Fatal("login", code)
		}
		var auth struct {
			Token string `json:"token"`
		}
		json.Unmarshal(raw, &auth)
		if code, _ := request(t, f.app, "GET", path, "", auth.Token); code != 403 {
			t.Fatal("owner boundary", role, code)
		}
	}
	if code, _ := request(t, f.app, "GET", path+"&destination=/tmp/not-authorized", "", f.token); code != 400 {
		t.Fatal("server destination accepted", code)
	}
	if code, _ := request(t, f.app, "GET", path, "", ""); code != 401 {
		t.Fatal("anonymous", code)
	}
}

func TestHTTPFiscalAccountingExportIncludesReconciledCancellationAndInvalidation(t *testing.T) {
	for _, kind := range []string{"cancellation", "invalidation"} {
		t.Run(kind, func(t *testing.T) {
			var f *stockHTTPFixture
			var expected []byte
			var suffix string
			if kind == "cancellation" {
				fixture, intent, input := cancellationSQLFixture(t)
				f = fixture
				r := cancellationSQLRecorder(t, f, f.db, intent, "export-cancel", f.token)
				if e := r.Reserve(context.Background(), cancellationSQLRequest(t, r, f, "export-cancel", input)); e != nil {
					t.Fatal(e)
				}
				if e := r.Complete(context.Background(), cancellationSQLResult("export-cancel", input)); e != nil {
					t.Fatal(e)
				}
				proof, e := fiscalcore.RecoverHomologationCancellation(context.Background(), r)
				if e != nil {
					t.Fatal(e)
				}
				expected = proof.ProcessedXML
				suffix = "-cancel.xml"
			} else {
				fixture, input := invalidationSQLFixture(t)
				f = fixture
				r := invalidationSQLRecorder(t, f, f.db, "export-inut", f.token, input)
				if e := r.Reserve(context.Background(), invalidationSQLRequest(t, r, "export-inut", input)); e != nil {
					t.Fatal(e)
				}
				if e := r.Complete(context.Background(), invalidationSQLResult("export-inut", input, "102")); e != nil {
					t.Fatal(e)
				}
				proof, e := fiscalcore.RecoverHomologationInvalidation(context.Background(), r)
				if e != nil {
					t.Fatal(e)
				}
				expected = proof.ProcessedXML
				suffix = "-inut.xml"
			}
			args := url.Values{"from": {time.Now().Add(-time.Hour).UTC().Format(time.RFC3339)}, "until": {time.Now().Add(time.Hour).UTC().Format(time.RFC3339)}, "lang": {"en"}}
			code, body := request(t, f.app, "GET", "/local/v1/fiscal/accounting-export?"+args.Encode(), "", f.token)
			if code != 200 {
				t.Fatal(code)
			}
			archive, e := zip.NewReader(bytes.NewReader(body), int64(len(body)))
			if e != nil {
				t.Fatal(e)
			}
			found := false
			for _, file := range archive.File {
				if strings.HasSuffix(file.Name, suffix) {
					reader, _ := file.Open()
					data, _ := io.ReadAll(reader)
					reader.Close()
					found = bytes.Equal(data, expected)
				}
			}
			if !found || len(expected) == 0 {
				t.Fatal("event archive mismatch")
			}
		})
	}
}
