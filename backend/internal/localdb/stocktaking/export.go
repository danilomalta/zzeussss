package stocktaking

import (
	"encoding/csv"
	"io"
	"strconv"
	"strings"
)

func spreadsheetText(s string) string {
	first := strings.TrimLeft(s, " \t\r\n")
	if first != "" && strings.ContainsRune("=+-@", rune(first[0])) {
		return "'" + s
	}
	return s
}
func intText(p *int64) string {
	if p == nil {
		return ""
	}
	return strconv.FormatInt(*p, 10)
}

// Export writes a bounded, already-authorized snapshot. Uncounted values stay
// empty; every quantity column explicitly names the milliunit convention.
func Export(w io.Writer, d Detail) error {
	cw := csv.NewWriter(w)
	if err := cw.Write([]string{"inventory_id", "status", "revision", "location_id", "product_id", "unit", "counted_milli", "previous_milli", "difference_milli", "counted_by", "counted_at", "movement_id"}); err != nil {
		return err
	}
	for _, l := range d.Lines {
		actor, at, movement := "", "", ""
		if l.CountedBy != nil {
			actor = *l.CountedBy
		}
		if l.CountedAt != nil {
			at = *l.CountedAt
		}
		if l.MovementID != nil {
			movement = *l.MovementID
		}
		row := []string{spreadsheetText(d.Session.ID), d.Session.Status, strconv.FormatInt(d.Session.Revision, 10), spreadsheetText(d.Session.LocationID), spreadsheetText(l.ProductID), l.Unit, intText(l.CountedMilli), intText(l.PreviousMilli), intText(l.DifferenceMilli), spreadsheetText(actor), at, spreadsheetText(movement)}
		if err := cw.Write(row); err != nil {
			return err
		}
	}
	cw.Flush()
	return cw.Error()
}
