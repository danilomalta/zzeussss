// Package fiscal persists fiscal preparation, never a fabricated authorization.
package fiscal

import (
	"errors"
	"math/big"
	"regexp"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

var (
	ErrInvalid     = errors.New("invalid fiscal input")
	ErrConflict    = errors.New("fiscal revision or operation conflict")
	ErrNotFound    = errors.New("fiscal record not found")
	ErrIntegrity   = errors.New("fiscal record integrity failure")
	idPattern      = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]{0,127}$`)
	cnpjPattern    = regexp.MustCompile(`^[A-Z0-9]{12}[0-9]{2}$`)
	decimalPattern = regexp.MustCompile(`^(0|[1-9][0-9]{0,17})(\.[0-9]{1,10})?$`)
)

var stateCodes = map[string]string{
	"RO": "11", "AC": "12", "AM": "13", "RR": "14", "PA": "15", "AP": "16", "TO": "17",
	"MA": "21", "PI": "22", "CE": "23", "RN": "24", "PB": "25", "PE": "26", "AL": "27", "SE": "28", "BA": "29",
	"MG": "31", "ES": "32", "RJ": "33", "SP": "35", "PR": "41", "SC": "42", "RS": "43",
	"MS": "50", "MT": "51", "GO": "52", "DF": "53",
}

// ValidCNPJ follows RFB's numeric/alphanumeric modulo-11 algorithm (ASCII-48).
// It checks syntax/DVs, not existence, registration or authorization to emit.
func ValidCNPJ(s string) bool {
	if !cnpjPattern.MatchString(s) || s == strings.Repeat(s[:1], 14) {
		return false
	}
	for n := 12; n < 14; n++ {
		sum, weight := 0, 2
		for j := n - 1; j >= 0; j-- {
			sum += int(s[j]-48) * weight
			weight++
			if weight == 10 {
				weight = 2
			}
		}
		digit := 11 - sum%11
		if digit >= 10 {
			digit = 0
		}
		if int(s[n]-'0') != digit {
			return false
		}
	}
	return true
}

// RoundDecimal provides explicit nonnegative half-up decimal rounding without
// binary floating point. Fiscal rules must choose each field's scale separately.
// This helper is not a tax engine and does not choose rates or tax applicability.
func RoundDecimal(s string, scale int) (string, error) {
	if !decimalPattern.MatchString(s) || scale < 0 || scale > 10 {
		return "", ErrInvalid
	}
	r, ok := new(big.Rat).SetString(s)
	if !ok {
		return "", ErrInvalid
	}
	factor := new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(scale)), nil)
	num := new(big.Int).Mul(r.Num(), factor)
	q, rem := new(big.Int), new(big.Int)
	q.QuoRem(num, r.Denom(), rem)
	if new(big.Int).Lsh(rem, 1).Cmp(r.Denom()) >= 0 {
		q.Add(q, big.NewInt(1))
	}
	result := q.String()
	if scale == 0 {
		return result, nil
	}
	if len(result) <= scale {
		result = strings.Repeat("0", scale+1-len(result)) + result
	}
	return result[:len(result)-scale] + "." + result[len(result)-scale:], nil
}

type ProfileInput struct {
	OperationID       string `json:"operation_id"`
	ProfileID         string `json:"profile_id"`
	ExpectedRevision  int    `json:"expected_revision"`
	CNPJ              string `json:"cnpj"`
	LegalName         string `json:"legal_name"`
	State             string `json:"state"`
	Municipality      string `json:"municipality_ibge"`
	StateRegistration string `json:"state_registration"`
	CRT               string `json:"crt"`
	Model             string `json:"model"`
	Environment       string `json:"environment"`
	Series            int    `json:"series"`
}

func (p ProfileInput) valid() bool {
	code, ok := stateCodes[p.State]
	if !ok || len(p.Municipality) != 7 || !strings.HasPrefix(p.Municipality, code) {
		return false
	}
	if _, err := strconv.Atoi(p.Municipality); err != nil {
		return false
	}
	if !idPattern.MatchString(p.OperationID) || !idPattern.MatchString(p.ProfileID) || p.ExpectedRevision < 0 || p.ExpectedRevision >= 1000000 {
		return false
	}
	if !ValidCNPJ(p.CNPJ) || !utf8.ValidString(p.LegalName) || strings.TrimSpace(p.LegalName) != p.LegalName || len(p.LegalName) < 2 || len(p.LegalName) > 255 {
		return false
	}
	for _, c := range p.LegalName {
		if unicode.IsControl(c) {
			return false
		}
	}
	if len(p.StateRegistration) < 2 || len(p.StateRegistration) > 20 {
		return false
	}
	for _, c := range p.StateRegistration {
		if !(c >= '0' && c <= '9' || c >= 'A' && c <= 'Z') {
			return false
		}
	}
	return (p.CRT == "1" || p.CRT == "2" || p.CRT == "3" || p.CRT == "4") && (p.Model == "55" || p.Model == "65") && (p.Environment == "homologation" || p.Environment == "production") && p.Series >= 0 && p.Series <= 889
}

type Readiness struct {
	IssuanceAvailable     bool     `json:"issuance_available"`
	OfficialAuthorization bool     `json:"official_authorization"`
	NumberAllocated       bool     `json:"number_allocated"`
	Blockers              []string `json:"blockers"`
}

func Unavailable() Readiness {
	return Readiness{Blockers: []string{"tax_rules_not_validated", "issuer_credentials_not_configured", "xml_schema_and_signature_not_validated", "official_transport_not_configured", "uf_developer_requirements_not_validated"}}
}
