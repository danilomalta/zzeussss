package fiscal

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"math/big"

	"titansystem-backend/internal/fiscalcore"
	"titansystem-backend/internal/localdb/identity"
)

func sameTaxJSON(a, b any) bool {
	left, _ := json.Marshal(a)
	right, _ := json.Marshal(b)
	return bytes.Equal(left, right)
}
func equalTaxDecimal(a, b string) bool {
	if !decimalPattern.MatchString(a) || !decimalPattern.MatchString(b) {
		return false
	}
	left, _ := new(big.Rat).SetString(a)
	right, _ := new(big.Rat).SetString(b)
	return left.Cmp(right) == 0
}

// Explicit opt-in. Existing manually reviewed invoices remain readable and do
// not pretend to carry a new policy proof. A quote-backed review is all-lines;
// it cannot mix an approved source with arbitrary client tax nodes/totals.
func applyProductionTaxQuotesTx(ctx context.Context, tx *sql.Tx, a identity.Scope, d identity.DeviceContext, in *fiscalcore.Invoice, profileID string, profileRevision int) error {
	quoted := false
	for _, line := range in.Lines {
		quoted = quoted || line.TaxQuoteOperationID != ""
	}
	if !quoted {
		return nil
	}
	totals := map[string]*big.Rat{}
	for i := range in.Lines {
		line := &in.Lines[i]
		if !idPattern.MatchString(line.TaxQuoteOperationID) {
			return ErrInvalid
		}
		quote, err := taxQuoteTx(ctx, tx, a, d, line.TaxQuoteOperationID)
		if err != nil {
			return err
		}
		p, err := taxPolicyTx(ctx, tx, a, quote.Input.PolicyID, 0)
		if err != nil {
			return err
		}
		if !p.Input.Active || p.Revision != quote.Input.PolicyRevision || p.SHA256 != quote.PolicySHA256 || p.Input.ProfileID != profileID || p.Input.ProfileRevision != profileRevision {
			return ErrConflict
		}
		q := quote.Input
		if q.Case.Operation != "sale" || q.Context.OriginUF != in.Issuer.Address.UF || q.Context.CRT != in.CRT || q.Context.Model != in.Key.Model || q.Context.Date != in.Key.IssuedAt.Format("2006-01-02") || q.Case.OriginMunicipality != in.Issuer.Address.CityCode {
			return ErrInvalid
		}
		recipientUF, municipality, status := in.Issuer.Address.UF, in.Issuer.Address.CityCode, "noncontributor"
		if in.Recipient != nil {
			switch in.Recipient.IEDestination {
			case "1":
				status = "contributor"
			case "2":
				status = "exempt_registration"
			case "9":
			default:
				return ErrInvalid
			}
			if in.Recipient.Address.UF != "" {
				recipientUF = in.Recipient.Address.UF
			}
			if in.Recipient.Address.CityCode != "" {
				municipality = in.Recipient.Address.CityCode
			}
		}
		final := "no"
		if in.FinalConsumer == "1" {
			final = "yes"
		} else if in.FinalConsumer != "0" {
			return ErrInvalid
		}
		destination := "1"
		if recipientUF != in.Issuer.Address.UF {
			destination = "2"
		}
		if q.Context.DestinationUF != recipientUF || q.Case.DestinationMunicipality != municipality || q.Case.RecipientStatus != status || q.Case.FinalConsumer != final || in.Destination != destination {
			return ErrInvalid
		}
		// Destination-sharing/consumer-final DIFAL is not represented by these
		// own-tax groups. Never silently omit a destination obligation.
		if destination == "2" {
			if status == "noncontributor" && final == "yes" {
				if p.Input.Rule.Goods == nil || p.Input.Rule.Goods.Destination == nil {
					return fiscalcore.ErrUnsupported
				}
			} else if status != "contributor" || final != "no" || !((q.Case.Purpose == "resale") || (q.Case.Purpose == "industrialization")) {
				return fiscalcore.ErrUnsupported
			}
		}
		if q.Item.ProductID != line.ProductID {
			return ErrInvalid
		}
		for _, v := range [][2]string{{q.Item.Quantity, line.Quantity}, {q.Item.UnitPrice, line.UnitPrice}, {q.Item.Discount, line.Discount}, {q.Item.Freight, line.Freight}, {q.Item.Insurance, line.Insurance}, {q.Item.Other, line.Other}} {
			if !equalTaxDecimal(v[0], v[1]) {
				return ErrInvalid
			}
		}
		mapped := quote.Result.GoodsXML
		if mapped != nil && line.FCI != "" && line.FCI != mapped.Classification.FCI {
			return ErrInvalid
		}
		if mapped == nil || mapped.Version != 1 {
			return fiscalcore.ErrUnsupported
		}
		if line.NCM != "" && line.NCM != quote.Result.NCM || line.CFOP != "" && line.CFOP != quote.Result.CFOP || line.CEST != "" && line.CEST != q.Case.CEST || line.BenefitCode != "" && line.BenefitCode != mapped.Classification.BenefitCode {
			return ErrInvalid
		}
		if len(line.Taxes) != 0 && !sameTaxJSON(line.Taxes, mapped.Taxes) {
			return ErrInvalid
		}
		line.NCM = quote.Result.NCM
		line.CFOP = quote.Result.CFOP
		line.CEST = q.Case.CEST
		line.BenefitCode = mapped.Classification.BenefitCode
		line.FCI = mapped.Classification.FCI
		line.Taxes = mapped.Taxes
		for k, value := range mapped.Totals {
			n, ok := new(big.Rat).SetString(value)
			if !ok {
				return ErrIntegrity
			}
			if totals[k] == nil {
				totals[k] = new(big.Rat)
			}
			totals[k].Add(totals[k], n)
		}
	}
	merged := map[string]string{}
	for k, value := range totals {
		merged[k] = value.FloatString(2)
	}
	if len(in.TaxTotals) != 0 && !sameTaxJSON(in.TaxTotals, merged) {
		return ErrInvalid
	}
	in.TaxTotals = merged
	return nil
}
