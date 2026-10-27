package fiscal

import (
	"context"
	"math/big"
	"titansystem-backend/internal/fiscalcore"
)

// ValidateInvoice binds the trusted homologation orchestrator to the frozen
// sale. This first flow supports cash with no discounts/additional charges;
// unknown economic semantics are refused, not inferred from a fiscal form.
// It does not approve NCM/CFOP, rates, legal classification or tax policy.
func (r *HomologationRecorder) ValidateInvoice(ctx context.Context, in fiscalcore.Invoice) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err = r.sendPermissionTx(ctx, tx); err != nil {
		return err
	}
	intent, err := intentTx(ctx, tx, r.actor, r.intentID)
	if err != nil {
		return err
	}
	s := intent.Snapshot
	p := s.Profile
	key, err := fiscalcore.AccessKey(in.Key)
	if err != nil || key != r.key || in.Environment != 2 || in.Key.Emission != 1 || in.CRT != p.CRT || in.Issuer.CNPJ != p.CNPJ || in.Issuer.Name != p.LegalName || in.Issuer.IE != p.StateRegistration || in.Issuer.Address.CityCode != p.Municipality || in.Issuer.Address.UF != p.State || in.Direction != "1" || in.Nature == "" || len(in.Lines) != len(s.Items) || len(in.Payments) != len(s.Payments) {
		return ErrInvalid
	}
	// Same positional ordering as the immutable snapshot (sale_item ID order),
	// allowing multiple lines for the same product without collapsing them.
	total := new(big.Int)
	for i, item := range s.Items {
		line := in.Lines[i]
		if line.ProductID != item.ProductID || item.QuantityMilli <= 0 || item.UnitPriceCents < 0 || item.TotalCents < 0 || !equalScaled(line.Quantity, item.QuantityMilli, 1000) || !equalScaled(line.UnitPrice, item.UnitPriceCents, 100) {
			return ErrInvalid
		}
		for _, value := range []string{line.Discount, line.Freight, line.Insurance, line.Other} {
			if !equalScaled(value, 0, 1) {
				return fiscalcore.ErrUnsupported
			}
		}
		gross := new(big.Int).Mul(big.NewInt(item.QuantityMilli), big.NewInt(item.UnitPriceCents))
		gross.Add(gross, big.NewInt(500))
		gross.Quo(gross, big.NewInt(1000))
		if gross.Cmp(big.NewInt(item.TotalCents)) != 0 {
			return fiscalcore.ErrUnsupported
		}
		total.Add(total, big.NewInt(item.TotalCents))
	}
	if total.Cmp(big.NewInt(s.TotalCents)) != 0 {
		return ErrIntegrity
	}
	paid := new(big.Int)
	for i, payment := range s.Payments {
		if payment.Method != "cash" || payment.Status != "confirmed" || payment.AmountCents <= 0 || in.Payments[i].Method != "01" || !equalScaled(in.Payments[i].Amount, payment.AmountCents, 100) {
			return fiscalcore.ErrUnsupported
		}
		paid.Add(paid, big.NewInt(payment.AmountCents))
	}
	// Snapshot represents the applied amount, not the banknotes handed over.
	if paid.Cmp(total) != 0 {
		return fiscalcore.ErrUnsupported
	}
	for _, name := range []string{"vST", "vFCPST", "vII", "vIPI", "vIPIDevol", "vICMSDeson"} {
		if !equalScaled(in.TaxTotals[name], 0, 1) {
			return fiscalcore.ErrUnsupported
		}
	}
	return tx.Commit()
}

func equalScaled(text string, value, scale int64) bool {
	if !decimalPattern.MatchString(text) {
		return false
	}
	n, ok := new(big.Rat).SetString(text)
	return ok && n.Cmp(new(big.Rat).SetFrac(big.NewInt(value), big.NewInt(scale))) == 0
}
