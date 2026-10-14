import { createPurchaseClient } from '../local/purchaseClient.mjs';
import type { Adapter } from './contracts';
export const purchases: Adapter = async (context, filters) => {
  const orders = await createPurchaseClient().orders(context.token);
  return {
    columns: ['order', 'supplier', 'date', 'status'],
    rows: orders
      .filter(
        (o) =>
          o.supplier_name.toLocaleLowerCase().includes(filters.q.toLocaleLowerCase()) &&
          (!filters.from || o.created_at.slice(0, 10) >= filters.from) &&
          (!filters.to || o.created_at.slice(0, 10) <= filters.to)
      )
      .map((o) => [o.id, o.supplier_name, o.created_at, o.status]),
    partial: orders.length >= 50,
    mock: false,
    at: Date.now(),
  };
};
