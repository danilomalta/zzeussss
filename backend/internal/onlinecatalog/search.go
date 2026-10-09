package onlinecatalog

import (
	"context"
	"strings"
)

func (s Store) Search(ctx context.Context, a Actor, q, active string, limit, offset int) ([]Product, error) {
	if !clean(q, 120, false) || (active != "all" && active != "active" && active != "inactive") || limit < 1 || limit > 100 || offset < 0 || offset > 1000000000 {
		return nil, ErrInput
	}
	if s.DB == nil {
		return nil, ErrUnavailable
	}
	pattern := ""
	if q != "" {
		pattern = "%" + strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(q) + "%"
	}
	rows, e := s.DB.QueryContext(ctx, `SELECT id,nome,COALESCE(descricao,''),sku,preco::text,ativo,catalog_version FROM products WHERE tenant_id=$1 AND deleted_at IS NULL AND ($2::text='all' OR ativo=($2='active')) AND ($3::text='' OR nome ILIKE $3 ESCAPE '\' OR sku ILIKE $3 ESCAPE '\') ORDER BY id DESC LIMIT $4 OFFSET $5`, a.Tenant, active, pattern, limit, offset)
	if e != nil {
		return nil, ErrUnavailable
	}
	defer rows.Close()
	items := make([]Product, 0)
	for rows.Next() {
		p, e := scanProduct(rows)
		if e != nil {
			return nil, e
		}
		items = append(items, p)
	}
	if rows.Err() != nil {
		return nil, ErrUnavailable
	}
	return items, nil
}
func (s Store) CreationOfProduct(ctx context.Context, a Actor, id int64) (Creation, error) {
	if _, e := s.Product(ctx, a, id); e != nil {
		return Creation{}, e
	}
	c, _, e := scanCreation(s.DB.QueryRowContext(ctx, `SELECT operation_id::text,actor_id::text,snapshot,created_at,payload_hash FROM online_catalog_creations WHERE tenant_id=$1 AND product_id=$2`, a.Tenant, id))
	return c, e
}
