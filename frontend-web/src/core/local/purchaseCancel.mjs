import {LocalAPIError} from './localClient.mjs';
import {id,integer,unit,readAPI} from './operationsRead.mjs';
const fail=message=>{throw new LocalAPIError(0,message||'Dados do pedido incompatíveis. A operação pendente foi preservada.');};
const keys=['operation_id','order_id','expected_status','reason'];
const exactFields=(v,keys)=>v&&typeof v==='object'&&!Array.isArray(v)&&Object.keys(v).length===keys.length&&keys.every(k=>Object.hasOwn(v,k));
const ident=v=>id(v)&&new TextEncoder().encode(v).length<=128;
const readText=(v,n)=>typeof v==='string'&&v.length>0&&new TextEncoder().encode(v).length<=n;
const reason=v=>readText(v,255)&&v.trim()===v&&!/[\x00\r\n]/.test(v);
const date=v=>typeof v==='string'&&Number.isFinite(Date.parse(v));
const cancellation=v=>v&&ident(v.operation_id)&&ident(v.device_id)&&ident(v.actor_id)&&reason(v.reason)&&date(v.created_at);
export function validOrderTrace(v,orderID){
 if(!v||!ident(orderID)||!v.order||v.order.id!==orderID)return false;
 const o=v.order,c=v.creation,a=v.approval;
 if(!ident(o.operation_id)||!ident(o.supplier_id)||!readText(o.supplier_name,255)||!ident(o.suggestion_id)||!['local_not_sent','cancelled'].includes(o.status)||!['not_authorized','authorized','partially_received','received'].includes(o.receiving_status)||!date(o.created_at)||!ident(o.approved_by)||!date(o.approved_at)||!Array.isArray(o.items)||o.items.length!==1)return false;
 for(const i of o.items)if(!i||!ident(i.product_id)||!readText(i.sku,100)||!readText(i.name,255)||!unit(i.unit)||!integer(i.quantity_milli)||i.quantity_milli===0)return false;
 if(!c||c.kind!=='purchase.created'||c.operation_id!==o.operation_id||!ident(c.actor_id)||!ident(c.device_id)||c.created_at!==o.created_at)return false;
 if(!a||a.suggestion_id!==o.suggestion_id||!ident(a.operation_id)||!ident(a.device_id)||a.reviewer_id!==o.approved_by||a.decision!=='approved'||!readText(a.reason,255)||a.decided_at!==o.approved_at)return false;
 if(o.status==='cancelled'){if(!cancellation(v.cancellation)||v.receiving_authorization||o.receiving_status!=='not_authorized')return false;}else if(v.cancellation)return false;
 const auth=v.receiving_authorization;
 if(o.receiving_status==='not_authorized'){if(auth)return false;}else if(!auth||!ident(auth.operation_id)||!ident(auth.device_id)||!ident(auth.actor_id)||!ident(auth.reference)||!reason(auth.reason)||!date(auth.created_at))return false;
 return true;
}
export function validCancelInput(kind,v){return kind==='cancel'&&exactFields(v,keys)&&ident(v.operation_id)&&ident(v.order_id)&&v.expected_status==='local_not_sent'&&reason(v.reason);}
const canonical=v=>JSON.stringify(Object.fromEntries(keys.map(k=>[k,v[k]])));
function validResult(v,input){return v&&v.order_id===input.order_id&&v.status==='cancelled'&&typeof v.repeated==='boolean'&&cancellation(v.cancellation)&&v.cancellation.operation_id===input.operation_id&&v.cancellation.reason===input.reason;}
export function validCancelReceipt(v,opID){return v&&validCancelInput('cancel',v.input)&&v.input.operation_id===opID&&validResult(v.result,v.input)&&v.result.repeated===false;}
export function preparePurchaseCancel(trace,why,opID){if(!validOrderTrace(trace,trace?.order?.id)||trace.order.status!=='local_not_sent'||trace.order.receiving_status!=='not_authorized'||trace.receiving_authorization||trace.cancellation)fail('Somente um pedido local ainda não autorizado para recebimento pode ser cancelado. Consulte a situação atual.');if(typeof why!=='string')fail();const input={operation_id:opID,order_id:trace.order.id,expected_status:trace.order.status,reason:why.trim()};if(!validCancelInput('cancel',input))fail();return input;}
export function createPurchaseCancelClient(fetcher=globalThis.fetch){const get=readAPI(fetcher);const receipt=async(token,opID)=>{if(!ident(opID))fail();return get(`/purchase-cancellation-operations/${encodeURIComponent(opID)}/receipt`,token,v=>validCancelReceipt(v,opID));};return {
 async trace(token,orderID){if(!ident(orderID))fail();return get(`/purchase-orders/${encodeURIComponent(orderID)}/trace`,token,v=>validOrderTrace(v,orderID));},
 receipt,
 async operation(token,kind,input){if(!validCancelInput(kind,input))fail();const v=await receipt(token,input.operation_id);if(canonical(v.input)!==canonical(input))fail();return v.result;},
 async write(token,kind,input){if(!token||!validCancelInput(kind,input))fail();const controller=new AbortController(),timer=setTimeout(()=>controller.abort(),30000);try{const {order_id,...body}=input;const response=await fetcher(`/local/v1/purchase-orders/${encodeURIComponent(order_id)}/cancel`,{method:'POST',signal:controller.signal,credentials:'omit',cache:'no-store',headers:{Authorization:`Bearer ${token}`,'Content-Type':'application/json'},body:JSON.stringify(body)});if(!response.ok)throw new LocalAPIError(response.status,({401:'Sessão expirada.',403:'Permissão ou contrato indisponível.',409:'Pedido em conflito ou já autorizado. Consulte antes de decidir.'})[response.status]||'Cancelamento recusado.');const v=await response.json();if(!validResult(v,input))fail();return v;}catch(e){if(e instanceof LocalAPIError)throw e;fail('Resposta não confirmada. Preserve a operação pendente.');}finally{clearTimeout(timer);}}
};}
export function purchaseCancelPendingKey(s){if(!s||!['tenant_id','store_id','device_id','identity_id'].every(k=>id(s[k])))fail();return 'titan-purchase-cancel:'+JSON.stringify([s.tenant_id,s.store_id,s.device_id,s.identity_id]);}
export function readPurchaseCancelPending(storage,key){
 let raw;try{raw=storage.getItem(key);}catch{fail('O navegador não permitiu ler a operação pendente. Gravações bloqueadas.');}
 if(raw===null)return null;let v;try{v=JSON.parse(raw);}catch{fail('Operação pendente corrompida. Preserve os dados do navegador.');}
 if(!exactFields(v,['kind','input','uncertain'])||typeof v.uncertain!=='boolean'||!validCancelInput(v.kind,v.input))fail();return v;
}
function save(storage,key,v){try{const raw=JSON.stringify(v);storage.setItem(key,raw);if(storage.getItem(key)!==raw)fail();}catch{fail('Não foi possível preservar a operação no navegador. Nenhuma nova gravação será enviada.');}}
function remove(storage,key,p){try{if(storage.getItem(key)!==JSON.stringify(p))fail();storage.removeItem(key);if(storage.getItem(key)!==null)fail();}catch{fail('Não foi possível limpar a operação confirmada. Consulte novamente; não crie outra operação.');}}
export function persistPurchaseCancelPending(storage,key,operation){if(readPurchaseCancelPending(storage,key))fail('Resolva a cancelamento de pedido pendente antes de iniciar outra operação.');if(!validCancelInput(operation.kind,operation.input))fail();save(storage,key,{kind:operation.kind,input:operation.input,uncertain:false});}
// Caller holds a Web Lock shared by all cancellation forms and browser tabs.
export async function resolvePurchaseCancelPending(client,token,storage,key,send=false,discard=false){
 const p=readPurchaseCancelPending(storage,key);if(!p)return null;let result;
 try{result=await client.operation(token,p.kind,p.input);}catch(e){if(!(e instanceof LocalAPIError)||e.status!==404)throw e;}
 if(result){remove(storage,key,p);return {state:'confirmed',result};}
 if(discard){if(p.uncertain)fail('Resultado incerto: não descarte. Consulte ou repita a mesma operação.');remove(storage,key,p);return {state:'discarded'};}
 if(!send)return {state:'missing'};
 const uncertain={...p,uncertain:true};save(storage,key,uncertain);
 try{await client.write(token,p.kind,p.input);}catch(e){if(!p.uncertain && e instanceof LocalAPIError && [400,403,404,409,413].includes(e.status))save(storage,key,p);throw e;}
 // Successful POST alone is insufficient; confirm the immutable receipt.
 result=await client.operation(token,p.kind,p.input);remove(storage,key,uncertain);return {state:'confirmed',result};
}
