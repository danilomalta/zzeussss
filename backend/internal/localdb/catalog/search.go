package catalog

import (
	"context"
	"database/sql"
	"strings"
	"unicode"
	"unicode/utf8"

	"golang.org/x/text/unicode/norm"
	"titansystem-backend/internal/localdb/identity"
)

type SearchInput struct {
	Query   string
	Unit    string
	Pending string
	Offset  int
}
type SearchProduct struct {
	Product
	MinimumConfigured bool `json:"minimum_configured"`
	Approximate       bool `json:"approximate"`
}
type SearchResult struct {
	Items       []SearchProduct `json:"items"`
	Total       int             `json:"total"`
	Offset      int             `json:"offset"`
	Limit       int             `json:"limit"`
	CostVisible bool            `json:"cost_visible"`
}

func searchFold(s string) string {
	return strings.Join(strings.Fields(strings.Map(func(r rune) rune {
		if unicode.Is(unicode.Mn, r) {
			return -1
		}
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			return unicode.ToLower(r)
		}
		return ' '
	}, norm.NFD.String(s))), " ")
}

// One substitution/insertion/deletion is allowed only for words of >= 5 runes.
func oneEdit(a, b []rune) bool {
	if len(a)-len(b) > 1 || len(b)-len(a) > 1 {
		return false
	}
	i, j, edits := 0, 0, 0
	for i < len(a) && j < len(b) {
		if a[i] == b[j] {
			i++
			j++
			continue
		}
		edits++
		if edits > 1 {
			return false
		}
		switch {
		case len(a) > len(b):
			i++
		case len(b) > len(a):
			j++
		default:
			i++
			j++
		}
	}
	if i < len(a) || j < len(b) {
		edits++
	}
	return edits <= 1
}
func searchMatch(p Product, query string) (bool, bool) {
	words := strings.Fields(searchFold(p.Name + " " + p.SKU + " " + p.Barcode))
	text := strings.Join(words, " ")
	approx := false
	for _, term := range strings.Fields(query) {
		if strings.Contains(text, term) {
			continue
		}
		found := false
		numeric := true
		for _, r := range term {
			if !unicode.IsDigit(r) {
				numeric = false
			}
		}
		if !numeric && utf8.RuneCountInString(term) >= 5 {
			for _, word := range words {
				if oneEdit([]rune(term), []rune(word)) {
					found = true
					approx = true
					break
				}
			}
		}
		if !found {
			return false, false
		}
	}
	return true, approx
}

// Search scans only this tenant's catalog; location policies are store scoped.
// Offset is applied after matching, so search is not limited to a loaded page.
func Search(ctx context.Context, db *sql.DB, a identity.Scope, d identity.DeviceContext, in SearchInput) (SearchResult, error) {
	out := SearchResult{Items: []SearchProduct{}, Offset: in.Offset, Limit: 50}
	if db == nil || in.Offset < 0 || in.Offset > 1000000000 || len(in.Query) > 240 || !utf8.ValidString(in.Query) || in.Unit != "" && !validUnit(in.Unit) || in.Pending != "" && in.Pending != "barcode" && in.Pending != "cost" && in.Pending != "minimum" {
		return out, ErrInvalidCatalog
	}
	query := searchFold(in.Query)
	if len(strings.Fields(query)) > 8 {
		return out, ErrInvalidCatalog
	}
	tx, e := db.BeginTx(ctx, nil)
	if e != nil {
		return out, e
	}
	defer tx.Rollback()
	if e = identity.CanOperateTx(ctx, tx, a, d, identity.ViewCatalog); e != nil {
		return out, e
	}
	var role string
	if e = tx.QueryRowContext(ctx, `SELECT role FROM memberships WHERE tenant_id=? AND identity_id=?`, a.TenantID, a.IdentityID).Scan(&role); e != nil {
		return out, e
	}
	out.CostVisible = role == "owner" || role == "manager" || role == "stock"
	if in.Pending == "cost" && !out.CostVisible {
		return out, identity.ErrDenied
	}
	rows, e := tx.QueryContext(ctx, `SELECT p.id,p.sku,p.barcode,p.name,p.unit,p.price_cents,p.cost_cents,EXISTS(SELECT 1 FROM restock_policies r WHERE r.tenant_id=p.tenant_id AND r.product_id=p.id AND r.store_id=?) FROM products p WHERE p.tenant_id=? ORDER BY p.sku,p.id`, a.StoreID, a.TenantID)
	if e != nil {
		return out, e
	}
	for rows.Next() {
		var p SearchProduct
		var barcode sql.NullString
		var cost int64
		if e = rows.Scan(&p.ID, &p.SKU, &barcode, &p.Name, &p.Unit, &p.PriceCents, &cost, &p.MinimumConfigured); e != nil {
			rows.Close()
			return out, e
		}
		p.Barcode = barcode.String
		if out.CostVisible {
			v := cost
			p.CostCents = &v
		}
		if p.PriceCents < 0 || p.PriceCents > 9007199254740991 || cost < 0 || out.CostVisible && cost > 9007199254740991 {
			rows.Close()
			return out, ErrInvalidCatalog
		}
		if in.Unit != "" && p.Unit != in.Unit || in.Pending == "barcode" && p.Barcode != "" || in.Pending == "cost" && cost != 0 || in.Pending == "minimum" && p.MinimumConfigured {
			continue
		}
		matched, approx := searchMatch(p.Product, query)
		if !matched {
			continue
		}
		p.Approximate = approx
		if out.Total >= in.Offset && len(out.Items) < 50 {
			out.Items = append(out.Items, p)
		}
		out.Total++
	}
	e = rows.Err()
	closeErr := rows.Close()
	if e != nil {
		return out, e
	}
	if closeErr != nil {
		return out, closeErr
	}
	return out, tx.Commit()
}
