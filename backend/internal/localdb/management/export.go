package management

import (
	"encoding/csv"
	"io"
	"strconv"
	"strings"
)

func safeCell(s string) string {
	v := strings.TrimLeft(s, " \t\r\n")
	if v != "" && strings.ContainsRune("=+-@", rune(v[0])) {
		return "'" + s
	}
	return s
}

// ExportSales exports only the returned page, retaining its period and basis.
func ExportSales(w io.Writer, r SalesReport) error {
	c := csv.NewWriter(w)
	if err := c.Write([]string{"product_id", "sku", "name", "unit", "quantity_milli", "revenue_cents", "previous_quantity_milli", "previous_revenue_cents", "revenue_delta_cents", "change_basis", "abc", "from_inclusive", "to_exclusive", "compare_from_inclusive", "compare_to_exclusive", "basis"}); err != nil {
		return err
	}
	for _, v := range r.Items {
		row := []string{safeCell(v.ProductID), safeCell(v.SKU), safeCell(v.Name), safeCell(v.Unit), strconv.FormatInt(v.QuantityMilli, 10), strconv.FormatInt(v.RevenueCents, 10), strconv.FormatInt(v.PreviousQuantityMilli, 10), strconv.FormatInt(v.PreviousRevenueCents, 10), strconv.FormatInt(v.RevenueDeltaCents, 10), v.ChangeBasis, v.ABCCategory, r.From, r.To, r.CompareFrom, r.CompareTo, r.Basis}
		if err := c.Write(row); err != nil {
			return err
		}
	}
	c.Flush()
	return c.Error()
}
