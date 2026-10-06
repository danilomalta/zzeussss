import { LocalAPIError } from './localClient.mjs';

const invalid = () => { throw new LocalAPIError(0, 'Dados do comparador incompatíveis.'); };
const text = (s, max) => typeof s === 'string' && s.length > 0 && new TextEncoder().encode(s).length <= max && s.trim() === s && !/[\u0000-\u001f\u007f]/u.test(s);
const integer = n => Number.isSafeInteger(n) && n >= 0;
export function validComparisonOrigin(origin) {
  try {
    const u = new URL(origin), host = u.hostname;
    return origin === 'https://' + host && u.protocol === 'https:' && !u.username && !u.password && !u.port &&
      host.length <= 253 && host.includes('.') && !host.endsWith('.') &&
      !/\.(local|internal|localhost|test|invalid|example|onion|arpa)$/u.test(host) &&
      /^[a-z0-9.-]+$/u.test(host) && host.split('.').every(p => p.length > 0 && p.length <= 63 && !p.startsWith('-') && !p.endsWith('-')) &&
      /[a-z]/u.test(host.split('.').at(-1));
  } catch { return false; }
}
export function validComparisonTemplate(origin, template) {
  if (!validComparisonOrigin(origin) || typeof template !== 'string') return false;
  if (template === '') return true;
  if (!text(template, 1600) || template.split('{query}').length !== 2) return false;
  const replaced = template.replace('{query}', 'titan-product');
  if (/[{}\\]/u.test(replaced)) return false;
  try { const u = new URL(replaced); return u.origin === origin && !u.username && !u.password && !u.hash; } catch { return false; }
}
export function comparisonQuery(name, variant) {
  if (typeof name !== 'string' || typeof variant !== 'string') invalid();
  const source = (name + ' ' + variant).replace(/[\u0000-\u001f\u007f]/gu, ' ').replace(/\s+/gu, ' ').trim();
  let query = '';
  for (const char of source) {
    if (new TextEncoder().encode(query + char).length > 240) break;
    query += char;
  }
  query = query.trim(); if (!text(query, 240)) invalid(); return query;
}
export function comparisonLink(site, query) {
  if (!validComparisonOrigin(site?.origin) || !validComparisonTemplate(site.origin, site.search_template) ||
      site.status !== 'active' || !text(query, 240)) invalid();
  if (!site.search_template) return site.origin;
  return site.search_template.replace('{query}', encodeURIComponent(query));
}
export function validSiteInput(v) {
  return v && Object.keys(v).length === 7 && ['operation_id', 'id'].every(k => text(v[k], 128)) &&
    text(v.name, 120) && integer(v.expected_revision) && v.expected_revision < Number.MAX_SAFE_INTEGER &&
    validComparisonOrigin(v.origin) && validComparisonTemplate(v.origin, v.search_template) && ['active', 'inactive'].includes(v.status);
}
const site = v => v && text(v.id, 128) && text(v.name, 120) && validComparisonOrigin(v.origin) &&
  validComparisonTemplate(v.origin, v.search_template) && ['active', 'inactive'].includes(v.status) &&
  integer(v.revision) && v.revision > 0 && typeof v.changed_at === 'string' && Number.isFinite(Date.parse(v.changed_at)) && v.mode === 'external_link';
function result(v) { if (!v || !text(v.operation_id, 128) || !site(v.site) || typeof v.repeated !== 'boolean') invalid(); return v; }
export function siteResultMatches(v, input) {
  return v?.operation_id === input.operation_id && v.site?.id === input.id && v.site.name === input.name &&
    v.site.origin === input.origin && v.site.search_template === input.search_template && v.site.status === input.status &&
    v.site.revision === input.expected_revision + 1;
}
export function createComparisonClient(fetcher = globalThis.fetch) {
  async function request(path, token, input, signal) {
    if (!token) invalid();
    const controller = new AbortController(), abort = () => controller.abort();
    signal?.addEventListener('abort', abort, { once: true });
    if (signal?.aborted) controller.abort();
    const timer = setTimeout(abort, 10000);
    try {
      const r = await fetcher('/local/v1' + path, { method: input ? 'POST' : 'GET', credentials: 'omit', cache: 'no-store',
        signal: controller.signal, headers: { Authorization: `Bearer ${token}`, ...(input ? { 'Content-Type': 'application/json' } : {}) },
        ...(input ? { body: JSON.stringify(input) } : {}) });
      if (!r.ok) throw new LocalAPIError(r.status, ({ 401: 'Sessão expirada ou revogada.', 403: 'Permissão ou licença insuficiente.',
        404: 'Operação ainda não encontrada.', 409: 'Revisão, endereço ou limite de sites em conflito.',
        400: 'Confira o nome e os endereços do site.', 503: 'Configuração de licença indisponível.' })[r.status] || 'Não foi possível consultar o comparador.');
      return await r.json();
    } catch (e) {
      if (signal?.aborted) throw new DOMException('Consulta cancelada', 'AbortError');
      if (e instanceof LocalAPIError) throw e;
      throw new LocalAPIError(0, 'Resposta não confirmada. Consulte a operação antes de repetir.');
    } finally { clearTimeout(timer); signal?.removeEventListener('abort', abort); }
  }
  return {
    async sites(token, signal) { const v = await request('/comparison-sites', token, undefined, signal);
      if (!v || v.limit !== 32 || v.automatic_state !== 'not_configured' || !Array.isArray(v.items) || v.items.length > 32 ||
          !v.items.every(site) || new Set(v.items.map(v => v.id)).size !== v.items.length) invalid(); return v; },
    async save(token, input) { if (!validSiteInput(input)) invalid(); const v = result(await request('/comparison-sites', token, input)); if (!siteResultMatches(v, input)) invalid(); return v; },
    async operation(token, id) { if (!text(id, 128)) invalid(); return result(await request('/comparison-site-operations/' + encodeURIComponent(id), token)); },
  };
}
export function sitePendingKey(session) {
  if (!session || !['tenant_id', 'store_id', 'device_id', 'identity_id'].every(k => text(session[k], 128))) invalid();
  return 'titan-comparison-site:' + JSON.stringify([session.tenant_id, session.store_id, session.device_id, session.identity_id]);
}
export function readSitePending(storage, key) {
  const raw = storage.getItem(key); if (raw === null) return null;
  let v; try { v = JSON.parse(raw); } catch { invalid(); }
  if (!v || Object.keys(v).length !== 2 || !validSiteInput(v.input) || !['prepared', 'uncertain', 'refused'].includes(v.state)) invalid(); return v;
}
export function persistSitePending(storage, key, input) {
  if (!validSiteInput(input) || storage.getItem(key) !== null) invalid();
  const raw = JSON.stringify({input, state: 'prepared'});
  storage.setItem(key, raw); if (storage.getItem(key) !== raw) invalid();
}
function updatePending(storage, key, pending) {
  const raw = JSON.stringify(pending); storage.setItem(key, raw); if (storage.getItem(key) !== raw) invalid();
}
// Query first. Only explicit retry after a confirmed 404 can send the original input.
export async function resolveSitePending(client, token, storage, key, send = false) {
  const pending = readSitePending(storage, key); if (!pending) return null;
  const input = pending.input; let v;
  try { v = await client.operation(token, input.operation_id); }
  catch (e) { if (!(e instanceof LocalAPIError) || e.status !== 404) throw e; }
  if (!v && send) {
    const firstKnownAttempt = pending.state !== 'uncertain';
    updatePending(storage, key, {...pending, state: 'uncertain'});
    try { await client.save(token, input); }
    catch (e) {
      // A later refusal cannot erase an earlier uncertain submission.
      if (firstKnownAttempt && e instanceof LocalAPIError && [400,403,409,413,503].includes(e.status))
        updatePending(storage, key, {...pending, state: 'refused'});
      throw e;
    }
    v = await client.operation(token, input.operation_id);
  }
  if (!v) return { state: 'not_found' };
  if (!siteResultMatches(v, input)) invalid();
  storage.removeItem(key); return { state: 'confirmed', result: v };
}
export async function releaseRefusedSite(client, token, storage, key) {
  const pending = readSitePending(storage, key);
  if (pending?.state !== 'refused') invalid();
  let v;
  try { v = await client.operation(token, pending.input.operation_id); }
  catch (e) {
    if (!(e instanceof LocalAPIError) || e.status !== 404) throw e;
    storage.removeItem(key); return {state: 'released'};
  }
  if (!siteResultMatches(v, pending.input)) invalid();
  storage.removeItem(key); return {state: 'confirmed', result: v};
}
// Manual observations are never API-confirmed prices. Unknown freight excludes total ranking.
export function manualComparison(price, freight, available, identical) {
  if (!integer(price) || price <= 0 || (freight !== null && !integer(freight)) ||
      !['available', 'unavailable', 'unknown'].includes(available) || typeof identical !== 'boolean') invalid();
  let total = null;
  if (freight !== null) { const sum = BigInt(price) + BigInt(freight); if (sum > BigInt(Number.MAX_SAFE_INTEGER)) invalid(); total = Number(sum); }
  return { price_cents: price, freight_cents: freight, total_cents: total, availability: available, identical,
    eligible: available === 'available' && identical && total !== null, source: 'manual' };
}
