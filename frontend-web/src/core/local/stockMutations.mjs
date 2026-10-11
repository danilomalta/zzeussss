import {LocalAPIError} from './localClient.mjs';
import {id,positive,integer,unit,readAPI,query} from './operationsRead.mjs';
import {parseQuantity} from './posModel.mjs';
const fail=message=>{throw new LocalAPIError(0,message||'Movimentação incompatível. Preserve a operação pendente.');};
const fields=['operation_id','kind','expected_unit','product_id','from_location_id','to_location_id','quantity_milli','reason'];
const exactFields=(v,keys)=>v&&typeof v==='object'&&!Array.isArray(v)&&Object.keys(v).length===keys.length&&keys.every(k=>Object.hasOwn(v,k));
export function validStockInput(kind,v){if(!exactFields(v,fields)||v.kind!==kind||!['entry','transfer','loss'].includes(kind)||!(v.expected_unit===''||unit(v.expected_unit))||!id(v.operation_id)||!id(v.product_id)||!positive(v.quantity_milli)||typeof v.reason!=='string'||!v.reason||v.reason.trim()!==v.reason||new TextEncoder().encode(v.reason).length>255||/[\x00\r\n]/.test(v.reason))return false;const f=v.from_location_id,t=v.to_location_id;return kind==='entry'?f===''&&id(t):kind==='loss'?id(f)&&t==='':id(f)&&id(t)&&f!==t;}
const canonical=v=>JSON.stringify(fields.map(k=>v[k]));
function validReceipt(v){return v&&validStockInput(v.input?.kind,v.input)&&id(v.actor_id)&&id(v.device_id)&&unit(v.unit)&&typeof v.occurred_at==='string'&&/^\d{4}-\d{2}-\d{2}T/.test(v.occurred_at)&&Number.isFinite(Date.parse(v.occurred_at));}
export function prepareStockOperation(product,values,operationID){if(!product||!id(product.id)||!unit(product.unit))fail();const input={operation_id:operationID,kind:values.kind,expected_unit:product.unit,product_id:product.id,from_location_id:values.kind==='entry'?'':values.from.trim(),to_location_id:values.kind==='loss'?'':values.to.trim(),quantity_milli:parseQuantity(values.quantity,product.unit),reason:values.reason.trim()};if(!validStockInput(input.kind,input))fail();return input;}
export function createStockMutations(fetcher=globalThis.fetch){const get=readAPI(fetcher);return {
 async receipt(token,operationID){if(!id(operationID))fail();return get(`/stock/operations/${encodeURIComponent(operationID)}/receipt`,token,v=>validReceipt(v)&&v.input.operation_id===operationID);},
 async operation(token,kind,input){if(!validStockInput(kind,input))fail();return get(`/stock/operations/${encodeURIComponent(input.operation_id)}/receipt`,token,v=>validReceipt(v)&&canonical(v.input)===canonical({...input,expected_unit:input.expected_unit||v.unit}));},
 async history(token,product,location,offset=0){if(!id(product)||!id(location)||!integer(offset))fail();return get(`/stock/operations/history?${query({product_id:product,location_id:location,offset})}`,token,v=>v&&v.product_id===product&&v.location_id===location&&v.offset===offset&&v.limit===50&&integer(v.total_count)&&Array.isArray(v.items)&&v.items.length===Math.min(50,Math.max(0,v.total_count-offset))&&v.has_more===(offset<v.total_count&&v.items.length<v.total_count-offset)&&v.items.every(i=>validReceipt(i)&&i.input.product_id===product&&(i.input.from_location_id===location||i.input.to_location_id===location))&&new Set(v.items.map(i=>i.input.operation_id)).size===v.items.length);},
 async write(token,kind,input){if(!token||!validStockInput(kind,input))fail();const controller=new AbortController(),timer=setTimeout(()=>controller.abort(),30000);try{const response=await fetcher('/local/v1/stock/operations',{method:'POST',signal:controller.signal,credentials:'omit',cache:'no-store',headers:{Authorization:`Bearer ${token}`,'Content-Type':'application/json'},body:JSON.stringify(input)});if(!response.ok)throw new LocalAPIError(response.status,({401:'Sessão expirada.',403:'Permissão ou contrato indisponível.',409:'Saldo livre, reserva ou identificador em conflito.'})[response.status]||'Movimentação recusada.');const v=await response.json();if(v?.operation_id!==input.operation_id||typeof v.repeated!=='boolean')fail();return v;}catch(e){if(e instanceof LocalAPIError)throw e;fail('Resposta não confirmada. Consulte ou repita a mesma operação.');}finally{clearTimeout(timer);}}
};}
export function stockPendingKey(s){if(!s||!['tenant_id','store_id','device_id','identity_id'].every(k=>id(s[k])))fail();return `titan-stock-entry:${s.tenant_id}:${s.store_id}:${s.device_id}:${s.identity_id}`;}
export function readStockPending(storage,key){
 let raw;try{raw=storage.getItem(key);}catch{fail('O navegador não permitiu ler a operação pendente. Gravações bloqueadas.');}
 if(raw===null)return null;let v;try{v=JSON.parse(raw);}catch{fail('Operação pendente corrompida. Preserve os dados do navegador.');}
 if(v?.kind==='entry'&&!Object.hasOwn(v,'input')){const input={from_location_id:'',expected_unit:'',...v};if(!validStockInput('entry',input))fail();return {kind:'entry',input,uncertain:true};}
 if(!exactFields(v,['kind','input','uncertain'])||typeof v.uncertain!=='boolean'||!validStockInput(v.kind,v.input))fail();return v;
}
function save(storage,key,v){try{const raw=JSON.stringify(v);storage.setItem(key,raw);if(storage.getItem(key)!==raw)fail();}catch{fail('Não foi possível preservar a operação no navegador. Nenhuma nova gravação será enviada.');}}
function remove(storage,key,p){try{if(storage.getItem(key)!==JSON.stringify(p))fail();storage.removeItem(key);if(storage.getItem(key)!==null)fail();}catch{fail('Não foi possível limpar a operação confirmada. Consulte novamente; não crie outra operação.');}}
export function persistStockPending(storage,key,operation){if(readStockPending(storage,key))fail('Resolva a movimentação pendente antes de iniciar outra operação.');if(!validStockInput(operation.kind,operation.input))fail();save(storage,key,{kind:operation.kind,input:operation.input,uncertain:false});}
// Caller holds a Web Lock shared by all stock forms and browser tabs.
export async function resolveStockPending(client,token,storage,key,send=false,discard=false){
 const p=readStockPending(storage,key);if(!p)return null;save(storage,key,p);let result;
 try{result=await client.operation(token,p.kind,p.input);}catch(e){if(!(e instanceof LocalAPIError)||e.status!==404)throw e;}
 if(result){remove(storage,key,p);return {state:'confirmed',result};}
 if(discard){if(p.uncertain)fail('Resultado incerto: não descarte. Consulte ou repita a mesma operação.');remove(storage,key,p);return {state:'discarded'};}
 if(!send)return {state:'missing'};
 const uncertain={...p,uncertain:true};save(storage,key,uncertain);
 try{await client.write(token,p.kind,p.input);}catch(e){if(!p.uncertain && e instanceof LocalAPIError && [400,403,404,409,413].includes(e.status))save(storage,key,p);throw e;}
 // Successful POST alone is insufficient; confirm the immutable receipt.
 result=await client.operation(token,p.kind,p.input);remove(storage,key,uncertain);return {state:'confirmed',result};
}
