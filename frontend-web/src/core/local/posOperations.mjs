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
  function current(token){client.assertCurrent?.(token);}
  function save(value,token) {
    current(token);
    try { const raw=JSON.stringify(value);storage.setItem(key,raw);if(storage.getItem(key)!==raw)throw Error(); }
    catch { throw new Error('Não foi possível preservar a operação neste navegador. Confira seu resultado antes de iniciar outra.'); }
  }
  function clear(value,token) {
    current(token);
    if(JSON.stringify(pending())!==JSON.stringify(value))throw new Error('A pendência mudou. Preserve os dados e confira a operação original.');
    try { storage.removeItem(key);if(storage.getItem(key)!==null)throw Error(); }
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
  async function execute(token,value,retry){
    current(token);lastRejection=null;
    const wasUncertain=value.uncertain||retry;let submitted=false,pendingPost=false;
    function beforePost(){
      current(token);
      if(JSON.stringify(pending())!==JSON.stringify(value))throw new Error('A pendência mudou antes do envio. Preserve a operação.');
      value.uncertain=true;save(value,token);submitted=true;pendingPost=true;
    }
    try{
      let result;
      if(value.kind==='sale'){
        result=retry?await readExisting(token,value):null;
        if(!result){beforePost();await client.completeSale(token,value.input);pendingPost=false;result=await readExisting(token,value);if(!result)throw new Error('Venda enviada, mas ainda não foi possível consultar o comprovante. Verifique a mesma operação.');}
      }else if(value.kind==='open'){beforePost();result=await client.openCash(token,value.input);}
      else{beforePost();result=await client.closeCash(token,value.input);}
      pendingPost=false;clear(value,token);return {kind:value.kind,result};
    }catch(error){
      // Only the first actual HTTP refusal can prove no write. A local guard
      // error or a refusal after prior uncertainty never permits deletion.
      if(submitted&&pendingPost&&!wasUncertain&&error?.serverRefusal===true&&rejects.includes(error?.status)){
        current(token);value.uncertain=false;save(value,token);lastRejection=JSON.stringify(value);
      }
      throw error;
    }
  }

  return {
    pending,
    canDiscard: () => lastRejection !== null && lastRejection === JSON.stringify(pending()),
    start(token, kind, input) {
      return exclusive(async () => {
        current(token);
        if (pending()) throw new Error('Resolva a operação pendente antes de iniciar outra.');
        const value = { version: 1, kind, input: JSON.parse(JSON.stringify(input)), uncertain: false };
        if (!validPending(value)) throw new Error('Operação inválida.');
        save(value,token);
        return execute(token, value, false);
      });
    },
    retry(token) {
      return exclusive(async () => {
        current(token);
        const value = pending();
        if (!value) throw new Error('Nenhuma operação pendente.');
        return execute(token, value, true);
      });
    },
    discardRejected(token) {
      return exclusive(async () => {
        current(token);
        const value = pending();
        if (!value || value.uncertain || lastRejection !== JSON.stringify(value)) throw new Error('Resultado incerto: consulte ou repita a mesma operação.');
        // A previous receipt wins over a later refusal (e.g. license expired).
        if (value.kind === 'sale' && await readExisting(token, value)) throw new Error('Esta venda já tem comprovante. Use Verificar operação.');
        clear(value,token); lastRejection = null;
      });
    },
  };
}
