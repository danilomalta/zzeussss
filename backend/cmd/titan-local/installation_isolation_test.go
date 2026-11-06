package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestIndependentInstallationsNeverShareStationKeys(t *testing.T) {
	type installation struct {
		db, station string
		config      stationFile
	}
	var installations [2]installation
	for i := range installations {
		dir := t.TempDir()
		x := &installations[i]
		x.db, x.station = filepath.Join(dir, "local.sqlite"), filepath.Join(dir, "station.json")
		var output bytes.Buffer
		if err := initStation([]string{"--db", x.db, "--station", x.station, "--empresa", "Disposable", "--loja", "Store", "--dono", "Owner"}, strings.NewReader("disposable-owner-password\n"), &output); err != nil {
			t.Fatal(err)
		}
		raw, err := os.ReadFile(x.station)
		if err != nil {
			t.Fatal(err)
		}
		if err = json.Unmarshal(raw, &x.config); err != nil {
			t.Fatal(err)
		}
		if x.config.PrivateKey == "" || bytes.Contains(output.Bytes(), []byte(x.config.PrivateKey)) {
			t.Fatal("missing or disclosed station key")
		}
		db, _, err := openVerified(context.Background(), x.db, x.station)
		if err != nil {
			t.Fatal(err)
		}
		if err = db.Close(); err != nil {
			t.Fatal(err)
		}
	}
	if installations[0].config.PrivateKey == installations[1].config.PrivateKey || installations[0].config.TenantID == installations[1].config.TenantID {
		t.Fatal("installations share keys or tenant")
	}
	// Even knowing the public IDs cannot turn the other client's key into a
	// station credential. Only disposable files created above are changed.
	foreign := installations[0].config
	foreign.PrivateKey = installations[1].config.PrivateKey
	raw, err := json.Marshal(foreign)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(installations[0].station, raw, 0600); err != nil {
		t.Fatal(err)
	}
	if db, _, err := openVerified(context.Background(), installations[0].db, installations[0].station); err == nil {
		_ = db.Close()
		t.Fatal("another installation's key authenticated")
	}
}
