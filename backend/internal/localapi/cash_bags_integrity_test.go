package localapi

import (
	"encoding/json"
	"reflect"
	"strings"
	"sync"
	"testing"
)

func TestHTTPBagCompetingAllocationsAndTerminalActionsHaveOneWinner(t *testing.T) {
	f, receiver := bagSetup(t)
	var wg sync.WaitGroup
	statuses := make(chan int, 2)
	for _, body := range []string{createBagBody, strings.NewReplacer("prepare-one", "prepare-two", "bag-one", "bag-two", "seal-one", "seal-two").Replace(createBagBody)} {
		body := body
		wg.Add(1)
		go func() {
			defer wg.Done()
			status, _ := request(t, f.app, "POST", bagsPath, body, f.token)
			statuses <- status
		}()
	}
	wg.Wait()
	close(statuses)
	created, conflicts := 0, 0
	for s := range statuses {
		if s == 201 {
			created++
		} else if s == 409 {
			conflicts++
		} else {
			t.Fatal(s)
		}
	}
	if created != 1 || conflicts != 1 {
		t.Fatal(created, conflicts)
	}
	status, data := request(t, f.app, "GET", bagsPath, "", f.token)
	if status != 200 {
		t.Fatal(status)
	}
	// Read keys from the real JSON rather than assuming which request won.
	var raw struct{ Items []map[string]any }
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatal(err)
	}
	id := raw.Items[0]["id"].(string)
	seal := raw.Items[0]["seal_id"].(string)
	bagResult(t, f, f.token, "POST", bagsPath+"/"+id+"/dispatch", dispatchBagBody, 200)
	statuses = make(chan int, 2)
	for _, count := range []int{290, 310} {
		count := count
		wg.Add(1)
		go func() {
			defer wg.Done()
			body, _ := json.Marshal(map[string]any{"operation_id": "one-receipt", "seal_id": seal, "counted_cents": count})
			status, _ := request(t, f.app, "POST", bagsPath+"/"+id+"/receive", string(body), receiver)
			statuses <- status
		}()
	}
	wg.Wait()
	close(statuses)
	received := 0
	conflicts = 0
	for s := range statuses {
		if s == 200 {
			received++
		} else if s == 409 {
			conflicts++
		} else {
			t.Fatal(s)
		}
	}
	if received != 1 || conflicts != 1 {
		t.Fatal(received, conflicts)
	}
	if b := bagRead(t, f, receiver, id, 200); len(b.History) != 3 || b.State != "received" {
		t.Fatal(b)
	}
	if n := bagCounts(t, f); n[3] != 1 || n[4] != 3 {
		t.Fatal(n)
	}
}

func TestHTTPBagCountZeroAndExactAggregateOverflow(t *testing.T) {
	for _, amount := range []string{"0", "9007199254740991"} {
		t.Run(amount, func(t *testing.T) {
			f, receiver := bagSetup(t)
			bagResult(t, f, f.token, "POST", bagsPath, createBagBody, 201)
			bagResult(t, f, f.token, "POST", bagsPath+"/bag-one/dispatch", dispatchBagBody, 200)
			bagResult(t, f, receiver, "POST", bagsPath+"/bag-one/receive", strings.Replace(receiveBagBody, "290", amount, 1), 200)
			if amount == "9007199254740991" {
				cashRequest(t, f.httpContractFixture, "movements", strings.Replace(withdrawalBody, "withdraw-one", "withdraw-two", 1), 200)
				second := strings.NewReplacer("prepare-one", "prepare-two", "bag-one", "bag-two", "seal-one", "seal-two", "withdraw-one", "withdraw-two").Replace(createBagBody)
				bagResult(t, f, f.token, "POST", bagsPath, second, 201)
				bagResult(t, f, f.token, "POST", bagsPath+"/bag-two/dispatch", `{"operation_id":"dispatch-two"}`, 200)
				body := strings.NewReplacer("receive-one", "receive-two", "seal-one", "seal-two", "290", amount).Replace(receiveBagBody)
				bagResult(t, f, receiver, "POST", bagsPath+"/bag-two/receive", body, 200)
			}
			cashRequest(t, f.httpContractFixture, "close", cashCloseBody, 200)
			if amount == "0" {
				v := bagCoverage(t, f, f.token, 200)
				if v.ReceivedCountedCents != 0 || v.DifferenceCents != -300 || v.DivergentCount != 1 {
					t.Fatal(v)
				}
			} else {
				bagCoverage(t, f, f.token, 422)
			}
		})
	}
}

func TestHTTPBagCoverageRefusesMissingAuditSourceOrClosureWithoutRepair(t *testing.T) {
	for _, damage := range []string{"unassigned_audit", "bag_audit", "closure", "history_state"} {
		t.Run(damage, func(t *testing.T) {
			f, _ := bagSetup(t)
			if damage != "unassigned_audit" {
				bagResult(t, f, f.token, "POST", bagsPath, createBagBody, 201)
			}
			cashRequest(t, f.httpContractFixture, "close", cashCloseBody, 200)
			switch damage {
			case "unassigned_audit", "bag_audit":
				if _, err := f.db.Exec(`PRAGMA foreign_keys=OFF; DELETE FROM cash_adjustments; PRAGMA foreign_keys=ON`); err != nil {
					t.Fatal(err)
				}
			case "closure":
				if _, err := f.db.Exec(`DELETE FROM cash_closures`); err != nil {
					t.Fatal(err)
				}
			case "history_state":
				// This transition is structurally allowed by the SQL state guard,
				// but has no operation receipt. Reads must detect the lost audit.
				if _, err := f.db.Exec(`UPDATE cash_bags SET state='in_transit',updated_at='tampered'`); err != nil {
					t.Fatal(err)
				}
			}
			before := bagCounts(t, f)
			bagCoverage(t, f, f.token, 409)
			if !reflect.DeepEqual(before, bagCounts(t, f)) {
				t.Fatal("coverage repaired past")
			}
			if damage == "bag_audit" || damage == "history_state" {
				bagRead(t, f, f.token, "bag-one", 409)
			}
		})
	}
}
