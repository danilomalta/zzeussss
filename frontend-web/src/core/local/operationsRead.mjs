import {LocalAPIError} from './localClient.mjs';

export const id = v => typeof v === 'string' && v.length > 0 && v.length <= 128 && v.trim() === v && !/[\x00\r\n]/.test(v);
export const text = v => typeof v === 'string';
export const integer = v => Number.isSafeInteger(v) && v >= 0;
export const positive = v => integer(v) && v > 0;
export const unit = v => ['unit','kg','g','liter','ml','meter'].includes(v);
export const array = v => Array.isArray(v);
export function requireValid(ok) { if (!ok) throw new LocalAPIError(0, 'Resposta local incompatível. Atualize a consulta; nenhum valor foi presumido.'); }
export function offsetValue(v) { requireValid(integer(v)); return v; }
export function query(values) {
  const params = new URLSearchParams();
  for (const [key,value] of Object.entries(values)) if (value !== '' && value !== undefined) params.set(key,String(value));
  return params.toString();
}
// Thousandths of the explicit unit; strings can exceed the safe JS integer range.
export function exactQuantity(value) {
  const digits = typeof value === 'number' ? (requireValid(integer(value)), String(value)) : value;
  requireValid(typeof digits === 'string' && /^(0|[1-9][0-9]*)$/.test(digits));
  const padded=digits.padStart(4,'0');
  return `${padded.slice(0,-3).replace(/\B(?=(\d{3})+(?!\d))/g,'.')},${padded.slice(-3)}`;
}
export function readAPI(fetcher = globalThis.fetch) {
  return async (path,token,valid) => {
    requireValid(typeof token === 'string' && token.length > 0);
    const controller = new AbortController();
    const timer = setTimeout(()=>controller.abort(),30000);
    try {
      const response=await fetcher(`/local/v1${path}`,{method:'GET',signal:controller.signal,cache:'no-store',credentials:'omit',headers:{Authorization:`Bearer ${token}`}});
      if (!response.ok) {
        const messages={401:'Sessão expirada ou revogada. Entre novamente.',403:'Acesso recusado para este operador.',404:'Registro não encontrado nesta loja.',409:'Dados locais inconsistentes ou em conflito. Confira a instalação.',503:'Serviço local indisponível.'};
        throw new LocalAPIError(response.status,messages[response.status] || 'Não foi possível concluir a consulta local.');
      }
      const result=await response.json(); requireValid(valid(result)); return result;
    } catch (e) {
      if (e instanceof LocalAPIError) throw e;
      throw new LocalAPIError(0,'A consulta não foi concluída. Confira o servidor local e tente novamente.');
    } finally {clearTimeout(timer);}
  };
}
