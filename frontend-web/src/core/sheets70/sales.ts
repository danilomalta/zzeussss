import { localClient } from '../local/useLocalSession';
import type { Adapter } from './contracts';
export const sales: Adapter = async (context, filters) => {
  const rows: (string | number | boolean)[][] = [];
  let partial = false;
  for (let offset = 0; offset < 500; offset += 20) {
    if (context.signal.aborted) throw Error('aborted');
    const page = await localClient.history(context.token, offset);
    for (const s of page) {
      const date = s.committed_at.slice(0, 10);
      if (
        (!filters.q ||
          (s.sale_id + ' ' + s.status).toLowerCase().includes(filters.q.toLowerCase())) &&
        (!filters.from || date >= filters.from) &&
        (!filters.to || date <= filters.to)
      )
        rows.push([s.sale_id, s.committed_at, s.status, s.total_cents / 100]);
    }
    if (page.length < 20) break;
    if (offset === 480) partial = true;
  }
  return {
    columns: ['sale', 'date', 'status', 'total'],
    rows,
    partial,
    mock: false,
    at: Date.now(),
  };
};
