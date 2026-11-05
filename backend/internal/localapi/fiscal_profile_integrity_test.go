package localapi

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"sync"
	"testing"

	"titansystem-backend/internal/localdb/fiscal"
	"titansystem-backend/internal/localdb/sale"
)

func TestFiscalProfileIgnoredRevisionCannotReportSuccess(t *testing.T) {
	for _, path := range []string{"/local/v1/fiscal/profiles", "/local/v1/fiscal/onboarding"} {
		t.Run(strings.TrimPrefix(path, "/local/v1/fiscal/"), func(t *testing.T) {
			f := fiscalFixture(t)
			input := strings.Replace(fiscalProfileBody, `"operation_id":"issuer-create"`, `"operation_id":"profile-revision-two"`, 1)
			input = strings.Replace(input, `"expected_revision":0`, `"expected_revision":1`, 1)
			if _, e := f.db.Exec(`CREATE TRIGGER synthetic_ignore_profile BEFORE INSERT ON fiscal_profiles BEGIN SELECT RAISE(IGNORE); END`); e != nil {
				t.Fatal(e)
			}
			if code, _ := request(t, f.app, "POST", path, input, f.token); code != 409 {
				t.Errorf("missing revision reported as success: %d", code)
			}
			code, body := request(t, f.app, "GET", "/local/v1/fiscal/profiles/issuer-one", "", f.token)
			var original fiscal.Profile
			if code != 200 || json.Unmarshal(body, &original) != nil || original.Revision != 1 || original.Input.OperationID != "issuer-create" {
				t.Fatal("original profile changed", code)
			}
			var n int
			if e := f.db.QueryRow(`SELECT count(*) FROM fiscal_profiles`).Scan(&n); e != nil || n != 1 {
				t.Fatal("partial revision", n, e)
			}
			if _, e := f.db.Exec(`DROP TRIGGER synthetic_ignore_profile`); e != nil {
				t.Fatal(e)
			}
			for _, want := range []int{201, 200} {
				code, body = request(t, f.app, "POST", path, input, f.token)
				var saved fiscal.Profile
				if code != want || json.Unmarshal(body, &saved) != nil || saved.Revision != 2 || saved.Input.OperationID != "profile-revision-two" {
					t.Fatal("recovery changed identity/revision", code)
				}
			}
			if e := f.db.QueryRow(`SELECT count(*) FROM fiscal_profiles`).Scan(&n); e != nil || n != 2 {
				t.Fatal("duplicate revision", n, e)
			}
		})
	}
}

func TestFiscalProfileConcurrentManualRevision(t *testing.T) {
	f := fiscalFixture(t)
	const n = 6
	type result struct {
		code int
		body []byte
	}
	results := make(chan result, n)
	inputs := map[string]fiscal.ProfileInput{}
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		var in fiscal.ProfileInput
		if e := json.Unmarshal([]byte(fiscalProfileBody), &in); e != nil {
			t.Fatal(e)
		}
		in.OperationID = fmt.Sprintf("p17-revision-%d", i)
		in.ExpectedRevision = 1
		in.LegalName = fmt.Sprintf("Synthetic revision %d", i)
		inputs[in.OperationID] = in
		body, e := json.Marshal(in)
		if e != nil {
			t.Fatal(e)
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			code, raw := request(t, f.app, "POST", "/local/v1/fiscal/onboarding", string(body), f.token)
			results <- result{code, raw}
		}()
	}
	wg.Wait()
	close(results)
	wins := 0
	var winner fiscal.Profile
	for r := range results {
		switch r.code {
		case 201:
			wins++
			if e := json.Unmarshal(r.body, &winner); e != nil {
				t.Fatal(e)
			}
		case 409:
		default:
			t.Fatal("not revision conflict", r.code)
		}
	}
	if wins != 1 || winner.Revision != 2 {
		t.Fatal("winners/revision", wins, winner.Revision)
	}
	if !reflect.DeepEqual(winner.Input, inputs[winner.Input.OperationID]) {
		t.Fatal("mixed profile")
	}
	replay, _ := json.Marshal(winner.Input)
	code, raw := request(t, f.app, "POST", "/local/v1/fiscal/onboarding", string(replay), f.token)
	var recovered fiscal.Profile
	if code != 200 || json.Unmarshal(raw, &recovered) != nil || recovered.Revision != 2 || !reflect.DeepEqual(recovered.Input, winner.Input) {
		t.Fatal("replay lost original revision", code)
	}
	var count int
	if e := f.db.QueryRow(`SELECT count(*) FROM fiscal_profiles`).Scan(&count); e != nil || count != 2 {
		t.Fatal("duplicate revision", count, e)
	}
}

func TestFiscalProviderCannotBeSelectedByCheckoutBody(t *testing.T) {
	f := checkoutSetup(t)
	in := checkoutInput(f)
	in.Parts = []sale.PlanPartInput{{PartID: "cash", Method: "cash", AmountCents: in.ExpectedTotalCents}}
	planRequest(t, f, "POST", planPath, planJSON(t, in), 201)
	for _, key := range []string{"provider", "tenant_id", "token", "csc", "certificate_reference", "focus_base_url"} {
		body := map[string]any{"operation_id": "blocked-selection", "received_cents": in.ExpectedTotalCents, key: "untrusted-client-choice"}
		if code, _ := request(t, f.app, "POST", planPath+"/"+in.CheckoutID+"/finalize-cash", planJSON(t, body), f.token); code != 400 {
			t.Fatal("client selected fiscal configuration", key, code)
		}
	}
	var n int
	if e := f.db.QueryRow(`SELECT count(*) FROM sales`).Scan(&n); e != nil || n != 0 {
		t.Fatal("invalid selection committed sale", n, e)
	}
}
