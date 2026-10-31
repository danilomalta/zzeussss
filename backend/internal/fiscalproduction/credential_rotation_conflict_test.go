package fiscalproduction

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"titansystem-backend/internal/localdb/fiscal"
)

// Strengthens the loser contract without changing the original rotation test:
// independent attempts must return revision conflicts, not generic unavailable.
func TestCustomerCredentialRotationLosersAreRevisionConflicts(t *testing.T) {
	r, a, _ := customerFixture(t)
	ctx := context.Background()
	if _, err := r.SaveCustomerCredentials(ctx, a, "profile", CustomerCredentialInput{OperationID: "initial-conflict-test", ProfileRevision: 1, Token: []byte("SyntheticInitialToken")}); err != nil {
		t.Fatal(err)
	}
	type result struct {
		input CustomerCredentialInput
		err   error
	}
	results := make(chan result, 4)
	start := make(chan struct{})
	var wg sync.WaitGroup
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			in := CustomerCredentialInput{OperationID: fmt.Sprintf("competing-%d", i), ProfileRevision: 1, ExpectedRevision: 1, Token: []byte(fmt.Sprintf("SyntheticDistinctToken%d", i)), CSC: []byte(fmt.Sprintf("SyntheticDistinctCSC%d", i))}
			_, err := r.SaveCustomerCredentials(ctx, a, "profile", in)
			results <- result{in, err}
		}(i)
	}
	close(start)
	wg.Wait()
	close(results)
	winners := 0
	var winner CustomerCredentialInput
	for res := range results {
		if res.err == nil {
			winners++
			winner = res.input
			continue
		}
		if !errors.Is(res.err, fiscal.ErrConflict) || errors.Is(res.err, ErrUnavailable) {
			t.Fatal("loser did not return a revision conflict")
		}
	}
	if winners != 1 {
		t.Fatalf("want one winner, got %d", winners)
	}
	status, err := r.ReadCustomerCredentials(ctx, a, "profile")
	if err != nil || status.Revision != 2 || status.OperationID != winner.OperationID {
		t.Fatal("wrong committed revision")
	}
	provider, _, err := r.Resolve(a)
	if err != nil {
		t.Fatal(err)
	}
	p := provider.(*FocusProvider)
	if string(p.token) != string(winner.Token) || string(p.csc) != string(winner.CSC) {
		t.Fatal("credentials from competing operations were mixed")
	}
}
