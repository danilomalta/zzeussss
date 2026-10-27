package fiscalcore

import "sort"

// SupportedUFs returns an independent copy, never a mutable shared registry.
// Coverage here means jurisdiction identifiers, not legal approval of rates.
func SupportedUFs() []string {
	out := make([]string, 0, len(ufCodes))
	for uf := range ufCodes {
		out = append(out, uf)
	}
	sort.Strings(out)
	return out
}

type Capabilities struct {
	UFs                     []string `json:"uf_identifiers"`
	Schemas                 []string `json:"pinned_schemas"`
	LocalComponents         []string `json:"local_components"`
	OfficialIssuance        bool     `json:"official_issuance"`
	OfficialHomologation    bool     `json:"official_homologation"`
	NationalTaxRuleCoverage bool     `json:"national_tax_rule_coverage"`
	NFSeIssuance            bool     `json:"nfse_issuance"`
	DANFEOfficial           bool     `json:"danfe_official"`
	SPEDOfficial            bool     `json:"sped_official"`
	Pending                 []string `json:"pending"`
}

func ComponentCapabilities() Capabilities {
	return Capabilities{UFs: SupportedUFs(), Schemas: []string{"NF-e 4.00 PL_010g_v1.03", "NF-e event 1.00", "national NFS-e 1.01 production", "national NFS-e 1.01 homologation 2026-10-20"}, LocalComponents: []string{"exact arithmetic with explicit dated policies", "NF-e/NFC-e serialization", "issuer-bound RSA XML signature and XSD validation", "cancellation XML", "homologation cancellation bound to original authorization with immutable journal and validated event process", "NFC-e QR 2/3 and PNG", "one-attempt mTLS transport with durable technical journal", "national DPS homologation XML/signature/compression with 2026-10-20 schema", "homologation status lookup and exact reconciliation with validated nfeProc", "schema root and includes loaded from embedded files independently of working directory"}, Pending: []string{
		"production API/outbox and multi-installation numbering authority",
		"approved tax classification/rules and RTC totals by issuer and effective date",
		"official endpoint registry by UF/model/environment/effective date",
		"certificate provisioning, complete ICP-Brasil chain/revocation and issuer accreditation",
		"NFS-e DPS serie pattern interoperability with pinned national schema",
		"municipal providers and NFS-e event author binding",
		"official homologation of status/receipt reconciliation, other fiscal events and contingency lifecycle",
		"official DANFE layout, SPED bookkeeping/PVA and official homologation",
	}}
}
