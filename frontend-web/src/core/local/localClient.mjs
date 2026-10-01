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
          403: 'A sessão não tem permissão para esta operação.',
          429: 'Limite de tentativas excedido. Aguarde antes de tentar novamente.',
          503: 'Serviço local indisponível.' };
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
    async logout(token) { await request('/logout', token, undefined, 'POST'); },
  };
}
