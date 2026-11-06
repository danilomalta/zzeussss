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
	if err := c.Write([]string{"product_id", "sku", "name", "unit", "quantity_milli", "revenue_cents", "previous_quantity_milli", "previous_revenue_cents", "revenue_delta_cents", "change_basis", "abc", "from_inclusive", "to_exclusive", "compare_from_inclusive", "compare_to_exclusive", "basis", "row_type", "page_limit", "page_offset", "page_products", "total_products", "source_rows", "period_revenue_cents", "previous_period_revenue_cents", "page_complete"}); err != nil {
		return err
	}
	coverage := []string{strconv.Itoa(r.Limit), strconv.Itoa(r.Offset), strconv.Itoa(len(r.Items)), strconv.Itoa(r.TotalProducts), strconv.Itoa(r.SourceRows), strconv.FormatInt(r.RevenueCents, 10), strconv.FormatInt(r.PreviousRevenueCents, 10), strconv.FormatBool(r.Offset == 0 && len(r.Items) == r.TotalProducts)}
	// A coverage record also survives an empty/out-of-range page. Empty item
	// cells are deliberately NOT zero-valued sales or an official balance.
	metadata := make([]string, 16)
	metadata[11], metadata[12], metadata[13], metadata[14], metadata[15] = r.From, r.To, r.CompareFrom, r.CompareTo, r.Basis
	if err := c.Write(append(append(metadata, "coverage"), coverage...)); err != nil {
		return err
	}
	for _, v := range r.Items {
		row := []string{safeCell(v.ProductID), safeCell(v.SKU), safeCell(v.Name), safeCell(v.Unit), strconv.FormatInt(v.QuantityMilli, 10), strconv.FormatInt(v.RevenueCents, 10), strconv.FormatInt(v.PreviousQuantityMilli, 10), strconv.FormatInt(v.PreviousRevenueCents, 10), strconv.FormatInt(v.RevenueDeltaCents, 10), v.ChangeBasis, v.ABCCategory, r.From, r.To, r.CompareFrom, r.CompareTo, r.Basis}
		if err := c.Write(append(append(row, "item"), coverage...)); err != nil {
			return err
		}
	}
	c.Flush()
	return c.Error()
}
