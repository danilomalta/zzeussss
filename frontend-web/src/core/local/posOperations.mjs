import { isID, isCents, validSale, receiptMatches } from './posModel.mjs';

const rejects = [400, 403, 409, 413];
function validPending(value) {
  if (!value || value.version !== 1 || typeof value.uncertain !== 'boolean') return false;
  const input = value.input;
  if (value.kind === 'sale') return validSale(input);
  if (value.kind === 'open') return input && isID(input.session_id) && isCents(input.opening_cents);
  if (value.kind === 'close') return input && isID(input.session_id) && isID(input.operation_id) && isCents(input.declared_cents);
  return false;
}

// Only operation payloads are saved: no password, bearer token or product names.
// Scope comes from /me. It is isolation of browser state, not authorization.
export function createPOSOperations(client, storage, scope, lock) {
  const fields = ['tenant_id', 'store_id', 'device_id', 'identity_id'];
  if (!scope || !fields.every((key) => isID(scope[key]))) throw new Error('Contexto local inválido.');
  const key = `titansystem-pos-v1:${JSON.stringify(fields.map((field) => scope[field]))}`;
  let busy = false;
  let lastRejection = null;
  function pending() {
    let raw;
    try { raw = storage.getItem(key); }
    catch { throw new Error('Não foi possível ler a pendência deste navegador. As operações estão bloqueadas.'); }
    if (raw === null) return null;
    try {
      if (raw.length > 140000) throw new Error();
      const parsed = JSON.parse(raw);
      if (!validPending(parsed)) throw new Error();
      return parsed;
    } catch { throw new Error('Pendência local inválida. Preserve os dados e solicite conferência antes de vender novamente.'); }
  }
  function save(value) {
    try { storage.setItem(key, JSON.stringify(value)); }
    catch { throw new Error('Não foi possível preservar a operação neste navegador. Confira seu resultado antes de iniciar outra.'); }
  }
  function clear() {
    try { storage.removeItem(key); }
    catch { throw new Error('Operação confirmada, mas a pendência não pôde ser removida. Consulte-a novamente antes de iniciar outra.'); }
  }
  async function exclusive(action) {
    if (busy) throw new Error('Aguarde a operação em andamento.');
    if (!lock) throw new Error('Este navegador não oferece proteção entre abas. Abra o sistema em localhost usando um navegador atualizado.');
    busy = true;
    try { return await lock(key, action); }
    finally { busy = false; }
  }
  async function readExisting(token, value) {
    try {
      const receipt = await client.receipt(token, value.input.sale_id);
      if (!receiptMatches(receipt, value.input)) throw new Error('O comprovante não corresponde à operação pendente. Preserve os dados para conferência.');
      return receipt;
    } catch (error) { if (error?.status === 404) return null; throw error; }
  }
  async function execute(token, value, retry) {
    lastRejection = null;
    try {
      let result;
      if (value.kind === 'sale') {
        result = retry ? await readExisting(token, value) : null;
        if (!result) {
          await client.completeSale(token, value.input);
          value.uncertain = true;
          save(value);
          result = await readExisting(token, value);
          if (!result) throw new Error('Venda enviada, mas ainda não foi possível consultar o comprovante. Verifique a mesma operação.');
        }
      } else if (value.kind === 'open') {
        // The same ID is retried: never create a new turn because a reply was lost.
        result = await client.openCash(token, value.input);
      } else { result = await client.closeCash(token, value.input); }
      clear();
      return { kind: value.kind, result };
    } catch (error) {
      lastRejection = !value.uncertain && rejects.includes(error?.status) ? JSON.stringify(value) : null;
      if (!lastRejection) { value.uncertain = true; save(value); }
      throw error;
    }
  }
  return {
    pending,
    canDiscard: () => lastRejection !== null && lastRejection === JSON.stringify(pending()),
    start(token, kind, input) {
      return exclusive(async () => {
        if (pending()) throw new Error('Resolva a operação pendente antes de iniciar outra.');
        const value = { version: 1, kind, input: JSON.parse(JSON.stringify(input)), uncertain: false };
        if (!validPending(value)) throw new Error('Operação inválida.');
        save(value);
        return execute(token, value, false);
      });
    },
    retry(token) {
      return exclusive(async () => {
        const value = pending();
        if (!value) throw new Error('Nenhuma operação pendente.');
        return execute(token, value, true);
      });
    },
    discardRejected(token) {
      return exclusive(async () => {
        const value = pending();
        if (!value || value.uncertain || lastRejection !== JSON.stringify(value)) throw new Error('Resultado incerto: consulte ou repita a mesma operação.');
        // A previous receipt wins over a later refusal (e.g. license expired).
        if (value.kind === 'sale' && await readExisting(token, value)) throw new Error('Esta venda já tem comprovante. Use Verificar operação.');
        clear(); lastRejection = null;
      });
    },
  };
}
