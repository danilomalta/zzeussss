import {LocalAPIError} from './localClient.mjs';
const id=v=>typeof v==='string' && v.length>0 && v.length<=128 && v.trim()===v;
const text=v=>typeof v==='string' && v.length>0;
export const validPurchaseInput=v=>v && ['operation_id','order_id','supplier_id','suggestion_id'].every(k=>id(v[k])) && Object.keys(v).length===4;
export const validSupplierInput=v=>v && id(v.operation_id) && id(v.id) && text(v.name) && v.name.trim()===v.name && new TextEncoder().encode(v.name).length<=255 && ['active','inactive'].includes(v.status) && Object.keys(v).length===4;
const summary=v=>v && ['id','operation_id','supplier_id','suggestion_id','approved_by'].every(k=>id(v[k])) && text(v.supplier_name) && text(v.created_at) && text(v.approved_at) && v.status==='local_not_sent';
const item=v=>v && id(v.product_id) && text(v.sku) && text(v.name) && text(v.unit) && Number.isSafeInteger(v.quantity_milli) && v.quantity_milli>0;
const invalid=()=>{throw new LocalAPIError(0,'Dados de pedido incompatíveis com a API local.');};
export function createPurchaseClient(fetcher=globalThis.fetch){
 async function request(path,token,input){
  if(!token)invalid();const controller=new AbortController(),timer=setTimeout(()=>controller.abort(),10000);
  try{const response=await fetcher(`/local/v1${path}`,{method:input===undefined?'GET':'POST',signal:controller.signal,credentials:'omit',cache:'no-store',headers:{Authorization:`Bearer ${token}`,...(input===undefined?{}:{'Content-Type':'application/json'})},...(input===undefined?{}:{body:JSON.stringify(input)})});
   if(!response.ok)throw new LocalAPIError(response.status,({401:'Sessão inválida ou revogada.',403:'Confira a permissão e o módulo de pedidos contratado.',404:'Pedido não encontrado nesta loja.',409:'Operação em conflito, aprovação indisponível ou fornecedor inativo.',503:'Verificador de contratos indisponível.'})[response.status] || 'A operação de pedido foi recusada.');
   return await response.json();
  }catch(e){if(e instanceof LocalAPIError)throw e;throw new LocalAPIError(0,'Resposta não confirmada. Consulte a operação pendente antes de continuar.');}finally{clearTimeout(timer);}
 }
 const offset=n=>{if(!Number.isSafeInteger(n)||n<0)invalid();return n;};
 return {
  async suppliers(token,n=0){const v=await request(`/purchase-suppliers?offset=${offset(n)}`,token);if(!Array.isArray(v?.items)||!v.items.every(s=>id(s?.id)&&text(s.name)&&['active','inactive'].includes(s.status)))invalid();return v.items;},
  async approvals(token,n=0){const v=await request(`/purchase-approvals?offset=${offset(n)}`,token);if(!Array.isArray(v?.items)||!v.items.every(s=>id(s?.suggestion_id)&&id(s.product_id)&&text(s.name)&&text(s.unit)&&Number.isSafeInteger(s.quantity_milli)&&s.quantity_milli>0))invalid();return v.items;},
  async orders(token,n=0){const v=await request(`/purchase-orders?offset=${offset(n)}`,token);if(!Array.isArray(v?.items)||!v.items.every(summary))invalid();return v.items;},
  async order(token,orderID){if(!id(orderID))invalid();const v=await request(`/purchase-orders/${encodeURIComponent(orderID)}`,token);if(!summary(v)||v.id!==orderID||!Array.isArray(v.items)||v.items.length!==1||!v.items.every(item))invalid();return v;},
  async createOrder(token,input){if(!validPurchaseInput(input))invalid();const v=await request('/purchase-orders',token,input);if(v?.id!==input.order_id||typeof v.repeated!=='boolean')invalid();return v;},
  async createSupplier(token,input){if(!validSupplierInput(input))invalid();const v=await request('/purchase-suppliers',token,input);if(v?.id!==input.id||typeof v.repeated!=='boolean')invalid();return v;},
 };
}
export function pendingPurchaseKey(session){if(!session||!['tenant_id','store_id','device_id','identity_id'].every(k=>id(session[k])))invalid();return 'titan-purchase:'+JSON.stringify([session.tenant_id,session.store_id,session.device_id,session.identity_id]);}
export function readPurchasePending(storage,key){const raw=storage.getItem(key);if(raw===null)return null;let v;try{v=JSON.parse(raw);}catch{invalid();}if(!v||Object.keys(v).length!==2||!((v.kind==='order'&&validPurchaseInput(v.input))||(v.kind==='supplier'&&validSupplierInput(v.input))))invalid();return v;}
export function persistPurchasePending(storage,key,v){if(readPurchasePending(storage,key))throw new LocalAPIError(0,'Resolva a operação pendente antes de criar outra.');if(!((v.kind==='order'&&validPurchaseInput(v.input))||(v.kind==='supplier'&&validSupplierInput(v.input))))invalid();storage.setItem(key,JSON.stringify(v));if(storage.getItem(key)!==JSON.stringify(v))invalid();}
function matches(v,incoming){return v.id===incoming.order_id&&v.operation_id===incoming.operation_id&&v.supplier_id===incoming.supplier_id&&v.suggestion_id===incoming.suggestion_id;}
// Caller holds the browser lock. Query failures never cause a POST. Explicit
// retry uses the original persisted IDs and payload; it never creates new IDs.
export async function resolvePurchasePending(client,token,storage,key,send=false){
 const pending=readPurchasePending(storage,key);if(!pending)return null;
 if(pending.kind==='supplier'){if(!send)return {state:'supplier_uncertain'};await client.createSupplier(token,pending.input);storage.removeItem(key);return {state:'supplier_confirmed'};}
 let order;
 try{order=await client.order(token,pending.input.order_id);}catch(e){if(!(e instanceof LocalAPIError)||e.status!==404)throw e;}
 if(order){if(!matches(order,pending.input))invalid();storage.removeItem(key);return {state:'order_confirmed',order};}
 if(!send)return {state:'order_missing'};
 await client.createOrder(token,pending.input);
 order=await client.order(token,pending.input.order_id);if(!matches(order,pending.input))invalid();storage.removeItem(key);return {state:'order_confirmed',order};
}
