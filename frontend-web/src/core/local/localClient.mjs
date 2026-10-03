import { isID, isCents, validSale } from './posModel.mjs';

export class LocalAPIError extends Error {
  constructor(status, message) {
    super(message);
    this.name = 'LocalAPIError';
    this.status = status;
  }
}

const validID = (value) => typeof value === 'string' && value.length > 0 && value.length <= 128;
const cents = (value) => Number.isSafeInteger(value) && value >= 0;
export function formatLocalCents(value) {
  if (!cents(value)) throw new LocalAPIError(0, 'Preço local fora do intervalo suportado.');
  const digits = String(value).padStart(3, '0');
  const whole = digits.slice(0, -2).replace(/\B(?=(\d{3})+(?!\d))/g, '.');
  return `R$ ${whole},${digits.slice(-2)}`;
}

export function createLocalClient(fetcher = globalThis.fetch) {
  async function request(path, token, body, method = 'GET') {
    const controller = new AbortController();
    const timeout = setTimeout(() => controller.abort(), 10000);
    try {
      const response = await fetcher(`/local/v1${path}`, {
        method, signal: controller.signal, credentials: 'omit', cache: 'no-store',
        headers: { ...(body === undefined ? {} : { 'Content-Type': 'application/json' }),
          ...(token ? { Authorization: `Bearer ${token}` } : {}) },
        ...(body === undefined ? {} : { body: JSON.stringify(body) }),
      });
      if (!response.ok) {
        const messages = { 401: 'Identidade ou senha inválida, sessão expirada ou acesso revogado.',
          403: 'Operação bloqueada: confira a permissão do operador e o módulo contratado.',
          429: 'Limite de tentativas excedido. Aguarde antes de tentar novamente.',
          400: 'Dados recusados. Confira os valores; o preço do catálogo pode ter mudado.',
          404: 'Registro não encontrado para esta sessão.',
          409: 'Operação em conflito: confira estoque, turno de caixa e valores.',
          503: 'Serviço local indisponível. Confira a configuração das chaves de licença.' };
        throw new LocalAPIError(response.status, messages[response.status] || 'A API local não concluiu a operação.');
      }
      if (response.status === 204) return undefined;
      try { return await response.json(); }
      catch { throw new LocalAPIError(0, 'Resposta da API local em formato inválido.'); }
    } catch (error) {
      if (error instanceof LocalAPIError) throw error;
      throw new LocalAPIError(0, 'Não foi possível acessar a API local. Confira se o servidor da instalação está em execução.');
    } finally { clearTimeout(timeout); }
  }
  const requireToken = (token) => { if (!token) throw new LocalAPIError(0, 'Entre na instalação local.'); };
  const invalid = () => { throw new LocalAPIError(0, 'A API local retornou dados incompatíveis.'); };
  async function mutation(path, token, input, valid, validateResponse) {
    requireToken(token);
    if (!valid) throw new LocalAPIError(0, 'Dados da operação local inválidos.');
    const result = await request(path, token, input, 'POST');
    if (!validateResponse(result)) invalid();
    return result;
  }
  return {
    async login(identityID, password) {
      const result = await request('/login', null, { identity_id: identityID, password }, 'POST');
      if (!result || typeof result.token !== 'string' || !result.token ||
          !Number.isSafeInteger(result.expires_unix) || result.expires_unix <= Date.now() / 1000) {
        throw new LocalAPIError(0, 'Resposta de autenticação local inválida.');
      }
      try {
        const session = await request('/me', result.token);
        if (!session || !['tenant_id', 'store_id', 'identity_id', 'device_id'].every((key) => validID(session[key])) ||
            session.identity_id !== identityID || session.expires_unix !== result.expires_unix) {
          throw new LocalAPIError(0, 'A API não confirmou o contexto da sessão local.');
        }
        return { token: result.token, session };
      } catch (error) {
        try { await request('/logout', result.token, undefined, 'POST'); } catch { /* Best effort; never log token/password. */ }
        throw error;
      }
    },
    async products(token, offset = 0) {
      if (!token || !Number.isSafeInteger(offset) || offset < 0) throw new LocalAPIError(0, 'Consulta local inválida.');
      const result = await request(`/products?limit=50&offset=${offset}`, token);
      // The Go list may encode a nil slice as null when no products exist.
      const items = result?.items === null ? [] : result?.items;
      if (!Array.isArray(items) || !items.every((item) => item && validID(item.id) &&
          typeof item.sku === 'string' && typeof item.name === 'string' && typeof item.unit === 'string' && cents(item.price_cents))) {
        throw new LocalAPIError(0, 'O catálogo retornou dados incompatíveis.');
      }
      return items;
    },
    async locations(token) {
      requireToken(token);
      const result = await request('/locations', token);
      if (result?.items === null) return [];
      if (!Array.isArray(result?.items) || !result.items.every((item) => item && isID(item.ID) && typeof item.name === 'string' && ['shelf', 'backroom', 'receiving', 'production'].includes(item.kind))) invalid();
      // Location.ID is currently exported by Go without a json tag.
      return result.items.map((item) => ({ id: item.ID, kind: item.kind, name: item.name }));
    },
    createProduct(token, input) {
      return mutation('/products', token, input, input && typeof input.sku === 'string' && input.sku.trim().length > 0 && input.sku.length <= 100 &&
        typeof input.name === 'string' && input.name.trim().length > 0 && input.name.length <= 255 &&
        ['unit', 'kg', 'g', 'liter', 'ml', 'meter'].includes(input.unit) && isCents(input.price_cents) && isCents(input.cost_cents), result => isID(result?.id));
    },
    createLocation(token, input) {
      return mutation('/locations', token, input, input && ['shelf', 'backroom', 'receiving', 'production'].includes(input.kind) &&
        typeof input.name === 'string' && input.name.trim().length > 0 && input.name.length <= 255, result => isID(result?.id));
    },
    stockEntry(token, input) {
      return mutation('/stock/operations', token, input, input && isID(input.operation_id) && input.kind === 'entry' && isID(input.product_id) && isID(input.to_location_id) &&
        Number.isSafeInteger(input.quantity_milli) && input.quantity_milli > 0 && typeof input.reason === 'string' && !!input.reason.trim(),
        result => result?.operation_id === input.operation_id && typeof result.repeated === 'boolean');
    },
    async history(token, offset = 0) {
      requireToken(token);
      if (!Number.isSafeInteger(offset) || offset < 0) invalid();
      const result = await request(`/sales?limit=20&offset=${offset}`, token);
      if (!Array.isArray(result?.items) || !result.items.every(item => isID(item?.sale_id) && isID(item.cash_session_id) &&
        ['committed', 'cancelled'].includes(item.status) && typeof item.committed_at === 'string' && isCents(item.total_cents))) invalid();
      return result.items;
    },
    async capabilities(token, session) {
      requireToken(token);
      const result = await request('/capabilities', token);
      const states = ['active','expired','not_yet_valid','unavailable','not_installed','invalid','clock_blocked'];
      const knownModules = ['core','inventory','pos','orders','logistics','finance','fiscal','accounting','staff','production'];
      const knownPermissions = ['view_catalog','sell','manage_stock','manage_staff','view_accounting','manage_production','view_orders','manage_replenishment','manage_cash','cancel_sale'];
      if (!result || !session || !['tenant_id','store_id','device_id','identity_id'].every(key => result[key] === session[key]) ||
          !['owner','manager','cashier','stock','production','employee','supplier','accountant'].includes(result.role) ||
          !Array.isArray(result.permissions) || !result.permissions.every(p => knownPermissions.includes(p)) ||
          !states.includes(result.license?.state) || !Array.isArray(result.license.modules) || !result.license.modules.every(m => knownModules.includes(m)) ||
          !Number.isSafeInteger(result.license.remaining_days) || result.license.remaining_days < 0 ||
          (result.license.expires_unix !== undefined && (!Number.isSafeInteger(result.license.expires_unix) || result.license.expires_unix <= 0))) invalid();
      return result;
    },
    createStaff(token, input) {
      return mutation('/staff',token,input,input && isID(input.operation_id) && isID(input.identity_id) && typeof input.name === 'string' && input.name.trim().length > 0 && input.name.length <= 255 &&
        ['manager','cashier','stock','production','employee','supplier','accountant'].includes(input.role) && typeof input.password === 'string' && new TextEncoder().encode(input.password).length >= 12 && new TextEncoder().encode(input.password).length <= 72,
        result => result?.identity_id === input.identity_id && typeof result.repeated === 'boolean');
    },
    async staff(token) {
      requireToken(token);
      const result=await request('/staff',token);
      if(!Array.isArray(result?.items) || !result.items.every(item => isID(item?.identity_id) && typeof item.name==='string' && ['owner','manager','cashier','stock','production','employee','supplier','accountant'].includes(item.role) && ['active','revoked'].includes(item.status))) invalid();
      return result.items;
    },
    async currentCash(token) {
      requireToken(token);
      const result = await request('/cash/current', token);
      if (!result || !Object.hasOwn(result, 'session')) invalid();
      if (result.session === null) return null;
      if (!isID(result.session?.session_id) || typeof result.session.opened_at !== 'string' || !result.session.opened_at) invalid();
      return result.session;
    },
    openCash(token, input) {
      return mutation('/cash/open', token, input, input && isID(input.session_id) && isCents(input.opening_cents),
        (result) => result?.session_id === input.session_id && typeof result.repeated === 'boolean');
    },
    closeCash(token, input) {
      return mutation('/cash/close', token, input, input && isID(input.session_id) && isID(input.operation_id) && isCents(input.declared_cents),
        (result) => result?.session_id === input.session_id && typeof result.repeated === 'boolean' && isCents(result.expected_cents) &&
          Number.isSafeInteger(result.difference_cents) && result.difference_cents === input.declared_cents - result.expected_cents);
    },
    completeSale(token, input) {
      return mutation('/sales', token, input, validSale(input),
        (result) => result?.sale_id === input.sale_id && result.total_cents === input.payments[0].amount_cents && typeof result.repeated === 'boolean');
    },
    async receipt(token, saleID) {
      requireToken(token);
      if (!isID(saleID)) invalid();
      const result = await request(`/sales/${encodeURIComponent(saleID)}`, token);
      if (!result || result.sale_id !== saleID || !isID(result.operation_id) || !isID(result.cash_session_id) ||
          !['committed', 'cancelled'].includes(result.status) || !isCents(result.total_cents) || result.fiscal_authorized !== false ||
          !Array.isArray(result.items) || !result.items.length || !result.items.every((item) => item && isID(item.item_id) && isID(item.product_id) && isID(item.location_id) &&
            Number.isSafeInteger(item.quantity_milli) && item.quantity_milli > 0 && isCents(item.unit_price_cents) && isCents(item.total_cents)) ||
          !Array.isArray(result.payments) || !result.payments.length || !result.payments.every((item) => item && isID(item.payment_id) && item.method === 'cash' && item.status === 'confirmed' && isCents(item.amount_cents))) invalid();
      const total = BigInt(result.total_cents);
      if (result.items.reduce((sum, item) => sum + BigInt(item.total_cents), 0n) !== total ||
          result.payments.reduce((sum, item) => sum + BigInt(item.amount_cents), 0n) !== total) invalid();
      return result;
    },
    async logout(token) { await request('/logout', token, undefined, 'POST'); },
  };
}
