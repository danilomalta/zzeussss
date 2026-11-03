package fiscalcore

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/url"
	"strings"
	"time"
)

// ProductionBinding is trusted process configuration. The browser cannot
// supply endpoints, keys, CSC, certificate trust or deployment identity.
// References record the responsible operator's provisioning evidence; they do
// not magically prove official accreditation or legal classification.
type ProductionBinding struct {
	ID                        string `json:"id"`
	TenantID                  string `json:"tenant_id"`
	StoreID                   string `json:"store_id"`
	DeviceID                  string `json:"device_id"`
	ProfileID                 string `json:"profile_id"`
	ProfileRevision           int    `json:"profile_revision"`
	Issuer                    string `json:"issuer"`
	UF                        string `json:"uf"`
	Timezone                  string `json:"timezone"`
	Model                     string `json:"model"`
	Series                    int    `json:"series"`
	InitialNumber             int64  `json:"initial_number"`
	SoleAuthorityReference    string `json:"sole_authority_reference"`
	EndpointRegistryReference string `json:"endpoint_registry_reference"`
	HomologationReference     string `json:"homologation_reference"`
	DeveloperReference        string `json:"developer_reference"`
	CancellationURL           string `json:"cancellation_url,omitempty"`
	AuthorizationURL          string `json:"authorization_url"`
	ConsultationURL           string `json:"consultation_url"`
	CertificateSHA256         string `json:"certificate_sha256"`
	SupplementSHA256          string `json:"supplement_sha256,omitempty"`
	ValidFrom                 string `json:"valid_from"`
	ValidUntil                string `json:"valid_until"`
}

func ProductionEndpoint(raw string) bool {
	u, e := url.Parse(raw)
	return e == nil && len(raw) <= 2048 && strings.TrimSpace(raw) == raw && u.Scheme == "https" && u.User == nil && u.Hostname() != "" && strings.HasSuffix(strings.ToLower(u.Hostname()), ".gov.br") && u.Port() == "" && u.RawQuery == "" && u.Fragment == "" && u.Path != ""
}
func (b ProductionBinding) Valid(now time.Time) bool {
	for _, v := range []string{b.ID, b.TenantID, b.StoreID, b.DeviceID, b.ProfileID, b.SoleAuthorityReference, b.EndpointRegistryReference, b.HomologationReference, b.DeveloperReference} {
		if len(v) < 1 || len(v) > 255 || strings.TrimSpace(v) != v || strings.ContainsAny(v, "\r\n\t\x00") {
			return false
		}
	}
	for _, v := range []string{b.SoleAuthorityReference, b.EndpointRegistryReference, b.HomologationReference, b.DeveloperReference} {
		if len(v) < 8 {
			return false
		}
	}
	_, ok := ufCodes[b.UF]
	from, e1 := time.Parse(time.RFC3339, b.ValidFrom)
	until, e2 := time.Parse(time.RFC3339, b.ValidUntil)
	fp, e3 := hex.DecodeString(b.CertificateSHA256)
	return ok && ValidatePrintOptions(PrintOptions{WidthMM: 210, Timezone: b.Timezone}) == nil && docRE.MatchString(b.Issuer) && (b.Model == "55" || b.Model == "65") && b.Series >= 0 && b.Series <= 889 && b.InitialNumber >= 1 && b.InitialNumber <= 999999999 && b.ProfileRevision >= 1 && e1 == nil && e2 == nil && !now.Before(from) && now.Before(until) && until.After(from) && e3 == nil && len(fp) == 32 && strings.ToLower(b.CertificateSHA256) == b.CertificateSHA256 && ProductionEndpoint(b.AuthorizationURL) && ProductionEndpoint(b.ConsultationURL) && b.AuthorizationURL != b.ConsultationURL && (b.CancellationURL == "" || ProductionEndpoint(b.CancellationURL) && b.CancellationURL != b.AuthorizationURL && b.CancellationURL != b.ConsultationURL)
}
func (b ProductionBinding) SHA256() string {
	raw, _ := json.Marshal(b)
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

func ProductionSupplementSHA256(qr QRConfiguration, consultation string) string {
	raw, _ := json.Marshal([]any{qr.BaseURL, qr.Version, qr.CSCID, consultation})
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

func PublicFiscalConsultationURL(raw string) bool { return publicFiscalURL(raw, false) }
