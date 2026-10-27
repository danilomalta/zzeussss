package fiscal

import "testing"

func TestCNPJOfficialAlphanumericAndNumericDV(t *testing.T) {
	// RFB manual's synthetic alphanumeric example; not a client registration.
	for _, s := range []string{"12ABC34501DE35", "11222333000181"} {
		if !ValidCNPJ(s) {
			t.Errorf("rejected %s", s)
		}
	}
	for _, s := range []string{"", "00000000000000", "11111111111111", "12ABC34501DE36", "12abc34501DE35", "12.ABC.345/01DE-35", "12ABC34501DE3A", "11222333000180"} {
		if ValidCNPJ(s) {
			t.Errorf("accepted %s", s)
		}
	}
}
func TestExactDecimalRoundingWithoutFloat(t *testing.T) {
	for _, x := range []struct {
		s     string
		scale int
		want  string
	}{
		{"0.1", 2, "0.10"}, {"1.005", 2, "1.01"}, {"1.0049999999", 2, "1.00"}, {"999999999999999999.9999999999", 2, "1000000000000000000.00"}, {"0.0000000001", 10, "0.0000000001"}, {"12.5", 0, "13"}, {"0", 3, "0.000"},
	} {
		out, err := RoundDecimal(x.s, x.scale)
		if err != nil || out != x.want {
			t.Errorf("%s %d: %s %v", x.s, x.scale, out, err)
		}
	}
	for _, s := range []string{"1e2", "-1", "NaN", "1,20", "01.00", " 1.2", "1.", "0.00000000001", "9999999999999999999"} {
		if _, err := RoundDecimal(s, 2); err == nil {
			t.Errorf("accepted %s", s)
		}
	}
	for _, scale := range []int{-1, 11} {
		if _, err := RoundDecimal("1", scale); err == nil {
			t.Error("accepted scale", scale)
		}
	}
}
func TestProfileValidationAll27StatesNoAutomaticAuthorization(t *testing.T) {
	if len(stateCodes) != 27 {
		t.Fatal(len(stateCodes))
	}
	for state, code := range stateCodes {
		p := ProfileInput{OperationID: "configure", ProfileID: "issuer", CNPJ: "12ABC34501DE35", LegalName: "Synthetic issuer", State: state, Municipality: code + "00000", StateRegistration: "123456789", CRT: "1", Model: "65", Environment: "homologation", Series: 1}
		if !p.valid() {
			t.Fatal(state)
		}
		p.Municipality = "9900000"
		if p.valid() {
			t.Fatal("foreign state prefix accepted")
		}
	}
	r := Unavailable()
	if r.IssuanceAvailable || r.OfficialAuthorization || r.NumberAllocated || len(r.Blockers) < 5 {
		t.Fatal(r)
	}
}

func TestDocumentedRequirementsCoverAll27StatesWithoutEnablingProduction(t *testing.T) {
	for state := range stateCodes {
		r, err := Requirements(state)
		if err != nil || r.UF != state || r.ReferenceDate != "2026-10-26" || r.ProductionValidated || len(r.Sources) == 0 {
			t.Fatal(state, r, err)
		}
	}
	if _, err := Requirements("XX"); err == nil {
		t.Fatal("accepted unknown state")
	}
	sc, err := Requirements("SC")
	if err != nil || sc.DeveloperRequirements == "" {
		t.Fatal("lost developer requirements")
	}
}
