import { createCatalogSearch } from '../local/catalogSearch.mjs';
import type { Adapter } from './contracts';
export const catalog: Adapter = async (context, filters) => {
  const rows: (string | number | boolean)[][] = [];
  let total = 0,
    cost = false;
  for (let offset = 0; offset < 500; offset += 50) {
    const page = await createCatalogSearch()(
      context.token,
      { q: filters.q, offset },
      context.signal
    );
    total = page.total;
    cost = page.cost_visible;
    rows.push(
      ...page.items.map((p) => [
        p.sku,
        p.name,
        p.unit,
        p.price_cents / 100,
        ...(cost ? [p.cost_cents === undefined ? '' : p.cost_cents / 100] : []),
      ])
    );
    if (rows.length >= total) break;
  }
  return {
    columns: ['sku', 'product', 'unit', 'price', ...(cost ? ['cost'] : [])],
    rows,
    partial: rows.length < total,
    mock: false,
    at: Date.now(),
  };
};
