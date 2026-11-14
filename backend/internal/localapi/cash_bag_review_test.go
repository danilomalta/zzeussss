package localapi

import (
	"reflect"
	"strings"
	"testing"
)

func TestHTTPBagReceiveExplanationIsAtomicScopedAndImmutable(t *testing.T) {
	for _, rollback := range []bool{false, true} {
		t.Run(map[bool]string{false: "persist", true: "rollback"}[rollback], func(t *testing.T) {
			f, receiver := bagSetup(t)
			bagResult(t, f, f.token, "POST", bagsPath, createBagBody, 201)
			bagResult(t, f, f.token, "POST", bagsPath+"/bag-one/dispatch", dispatchBagBody, 200)
			body := strings.TrimSuffix(receiveBagBody, "}") + `,"reason":"Physical count reviewed; ten cents missing"}`
			bagResult(t, f, f.token, "POST", bagsPath+"/bag-one/receive", body, 403)
			bagResult(t, f, receiver, "POST", bagsPath+"/bag-one/receive", strings.Replace(body, "Physical count reviewed; ten cents missing", " ", 1), 400)
			before := bagCounts(t, f)
			if rollback {
				if _, err := f.db.Exec(`CREATE TRIGGER review_failure BEFORE INSERT ON cash_bag_outbox WHEN NEW.event_type='cash.bag.receive' BEGIN SELECT RAISE(ABORT,'injected review rollback'); END`); err != nil {
					t.Fatal(err)
				}
				bagResult(t, f, receiver, "POST", bagsPath+"/bag-one/receive", body, 500)
				if !reflect.DeepEqual(before, bagCounts(t, f)) {
					t.Fatal("review failure partially changed custody or cash")
				}
				if _, err := f.db.Exec(`DROP TRIGGER review_failure`); err != nil {
					t.Fatal(err)
				}
			}
			first := bagResult(t, f, receiver, "POST", bagsPath+"/bag-one/receive", body, 200)
			if first.Bag.History[2].Reason != "Physical count reviewed; ten cents missing" || first.Bag.History[2].ActorID == first.Bag.OperatorID || *first.Bag.DifferenceCents != -10 {
				t.Fatal("review identity, reason or difference lost")
			}
			after := bagCounts(t, f)
			repeated := bagResult(t, f, receiver, "POST", bagsPath+"/bag-one/receive", body, 200)
			if !repeated.Repeated || !reflect.DeepEqual(first.Bag, repeated.Bag) || !reflect.DeepEqual(after, bagCounts(t, f)) {
				t.Fatal("review replay duplicated or changed effect")
			}
			bagResult(t, f, receiver, "POST", bagsPath+"/bag-one/receive", strings.Replace(body, "ten cents missing", "changed explanation", 1), 409)
		})
	}
}
