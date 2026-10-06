import { LocalAPIError } from './localClient.mjs';

const invalid = () => { throw new LocalAPIError(0, 'A busca do catálogo retornou dados incompatíveis.'); };
const cents = value => Number.isSafeInteger(value) && value >= 0;
const id = value => typeof value === 'string' && value.length > 0 && value.length <= 128;
const units = ['', 'unit', 'kg', 'g', 'liter', 'ml', 'meter'];

export function createCatalogSearch(fetcher = globalThis.fetch) {
  return async (token, { q = '', unit = '', pending = '', offset = 0 } = {}, signal) => {
    if (!token || typeof q !== 'string' || new TextEncoder().encode(q).length > 240 ||
        !units.includes(unit) || !['', 'barcode', 'cost', 'minimum'].includes(pending) || !cents(offset)) invalid();
    const controller = new AbortController();
    const abort = () => controller.abort();
    signal?.addEventListener('abort', abort, { once: true });
    if (signal?.aborted) controller.abort();
    const timer = setTimeout(abort, 10000);
    try {
      const params = new URLSearchParams({ q, unit, pending, offset: String(offset) });
      const response = await fetcher('/local/v1/catalog/search?' + params, {
        signal: controller.signal, credentials: 'omit', cache: 'no-store',
        headers: { Authorization: `Bearer ${token}` },
      });
      if (!response.ok) throw new LocalAPIError(response.status, ({
        401: 'Sessão expirada ou revogada.',
        403: 'Esta consulta exige permissão para visualizar os dados.',
        400: 'Confira o texto e os filtros da busca.',
      })[response.status] || 'Não foi possível consultar o catálogo.');
      const value = await response.json();
      if (!value || !cents(value.total) || value.offset !== offset || value.limit !== 50 ||
          typeof value.cost_visible !== 'boolean' || !Array.isArray(value.items) || value.items.length > 50 ||
          value.items.length !== Math.min(50, Math.max(0, value.total - offset)) ||
          !value.items.every(product => id(product?.id) &&
            typeof product.sku === 'string' && product.sku.length > 0 &&
            typeof product.name === 'string' && product.name.length > 0 &&
            units.slice(1).includes(product.unit) && cents(product.price_cents) &&
            typeof product.minimum_configured === 'boolean' && typeof product.approximate === 'boolean' &&
            (product.barcode === undefined || typeof product.barcode === 'string') &&
            (value.cost_visible ? cents(product.cost_cents) : product.cost_cents === undefined))) invalid();
      return value;
    } catch (error) {
      if (signal?.aborted) throw new DOMException('Consulta cancelada', 'AbortError');
      if (error instanceof LocalAPIError) throw error;
      throw new LocalAPIError(0, 'API indisponível. A tabela não foi confirmada.');
    } finally {
      clearTimeout(timer);
      signal?.removeEventListener('abort', abort);
    }
  };
}
