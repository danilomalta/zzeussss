package main

import "testing"

func TestUnknownMaintenanceCommandDoesNotConnectOrLoadEnvironment(t *testing.T) {
	for _, args := range [][]string{nil, {"init"}, {"migrate-sessions", "extra"}, {"check-sessions", "--db", "real"}} {
		if run(args) == nil {
			t.Fatal("comando inválido aceito")
		}
	}
}
