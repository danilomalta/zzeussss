import {LocalAPIError} from './localClient.mjs';
import {id,integer,readAPI} from './operationsRead.mjs';
const fail=message=>{throw new LocalAPIError(0,message||'Dados do fornecedor incompatíveis. A operação pendente foi preservada.');};
const keys=['operation_id','supplier_id','expected_revision','name','status','reason'];
const exactFields=(v,keys)=>v&&typeof v==='object'&&!Array.isArray(v)&&Object.keys(v).length===keys.length&&keys.every(k=>Object.hasOwn(v,k));
const limited=(v,n)=>typeof v==='string'&&v.length>0&&v.trim()===v&&new TextEncoder().encode(v).length<=n&&!/[\x00\r\n]/.test(v);
const ident=v=>id(v)&&new TextEncoder().encode(v).length<=128;
const status=v=>['active','inactive'].includes(v);
const revision=v=>integer(v)&&v<=2147483647;
const date=v=>typeof v==='string'&&Number.isFinite(Date.parse(v));
export function validSupplierState(v,supplierID){return v&&ident(supplierID)&&v.id===supplierID&&limited(v.name,255)&&status(v.status)&&revision(v.revision)&&v.repeated===false;}
export function validSupplierInput(kind,v){return kind==='edit'&&exactFields(v,keys)&&ident(v.operation_id)&&ident(v.supplier_id)&&revision(v.expected_revision)&&v.expected_revision<2147483647&&limited(v.name,255)&&status(v.status)&&limited(v.reason,255);}
const canonical=v=>JSON.stringify(Object.fromEntries(keys.map(k=>[k,v[k]])));
const validResult=(v,input)=>v&&v.id===input.supplier_id&&v.name===input.name&&v.status===input.status&&v.revision===input.expected_revision+1&&typeof v.repeated==='boolean';
export function prepareSupplierEdit(snapshot,name,reason,operationID){if(!validSupplierState(snapshot,snapshot?.id)||typeof name!=='string'||name.trim()===snapshot.name)fail('Consulte o fornecedor e informe um nome diferente.');return prepared(snapshot,name.trim(),snapshot.status,reason,operationID);}
export function prepareSupplierStatus(snapshot,next,reason,operationID){if(!validSupplierState(snapshot,snapshot?.id)||!status(next)||snapshot.status===next)fail('Consulte o estado atual antes de decidir.');return prepared(snapshot,snapshot.name,next,reason,operationID);}
function prepared(snapshot,name,next,reason,operationID){if(typeof reason!=='string')fail();const input={operation_id:operationID,supplier_id:snapshot.id,expected_revision:snapshot.revision,name,status:next,reason:reason.trim()};if(!validSupplierInput('edit',input))fail();return input;}
export function validSupplierHistory(v,supplierID,offset){
 if(!v||!validSupplierState(v.current,supplierID)||!v.origin||v.origin.id!==supplierID||!limited(v.origin.name,255)||!status(v.origin.status)||!v.origin.creation||v.origin.creation.kind!=='supplier.created'||!ident(v.origin.creation.operation_id)||!ident(v.origin.creation.actor_id)||!ident(v.origin.creation.device_id)||!date(v.origin.creation.created_at)||v.offset!==offset||v.limit!==50||!integer(v.total_count)||v.total_count!==v.current.revision||!Array.isArray(v.items)||v.items.length!==Math.min(50,Math.max(0,v.total_count-offset))||v.has_more!==(offset+v.items.length<v.total_count))return false;
 const ids=new Set();
 for(let n=0;n<v.items.length;n++){
  const x=v.items[n];if(!x||x.revision!==offset+n+1||!ident(x.operation_id)||ids.has(x.operation_id)||!ident(x.actor_id)||!ident(x.device_id)||!date(x.created_at)||!limited(x.before_name,255)||!limited(x.name,255)||!limited(x.reason,255)||!status(x.before_status)||!status(x.status)||(x.name===x.before_name&&x.status===x.before_status))return false;
  ids.add(x.operation_id);const prior=n?v.items[n-1]:offset===0?v.origin:null;
  if(prior&&(x.before_name!==prior.name||x.before_status!==prior.status))return false;
 }
 if(v.total_count===0&&(v.current.name!==v.origin.name||v.current.status!==v.origin.status))return false;
 const last=v.items.at(-1);if(last&&last.revision===v.total_count&&(last.name!==v.current.name||last.status!==v.current.status))return false;
 return true;
}
export function createSupplierMutations(fetcher=globalThis.fetch){const get=readAPI(fetcher);return {
 async state(token,supplierID){if(!ident(supplierID))fail();return get(`/purchase-suppliers/${encodeURIComponent(supplierID)}/edit-state`,token,v=>validSupplierState(v,supplierID));},
 async history(token,supplierID,offset=0){if(!ident(supplierID)||!integer(offset))fail();return get(`/purchase-suppliers/${encodeURIComponent(supplierID)}/history?offset=${offset}`,token,v=>validSupplierHistory(v,supplierID,offset));},
 async operation(token,kind,input){if(!validSupplierInput(kind,input))fail();const v=await get(`/purchase-supplier-operations/${encodeURIComponent(input.operation_id)}/receipt`,token,v=>v&&validSupplierInput('edit',v.input)&&canonical(v.input)===canonical(input)&&validResult(v.result,input)&&v.result.repeated===false&&limited(v.before_name,255)&&status(v.before_status)&&ident(v.actor_id)&&ident(v.device_id)&&date(v.created_at)&&(v.before_name!==input.name||v.before_status!==input.status));return v.result;},
 async write(token,kind,input){if(!token||!validSupplierInput(kind,input))fail();const controller=new AbortController(),timer=setTimeout(()=>controller.abort(),30000);try{
  const {supplier_id,...body}=input;const response=await fetcher(`/local/v1/purchase-suppliers/${encodeURIComponent(supplier_id)}/update`,{method:'POST',signal:controller.signal,credentials:'omit',cache:'no-store',headers:{Authorization:`Bearer ${token}`,'Content-Type':'application/json'},body:JSON.stringify(body)});
  if(!response.ok)throw new LocalAPIError(response.status,({401:'Sessão expirada.',403:'Permissão ou contrato indisponível.',409:'Revisão em conflito. Consulte o fornecedor antes de corrigir.'})[response.status]||'Alteração do fornecedor recusada.');const v=await response.json();if(!validResult(v,input))fail();return v;
 }catch(e){if(e instanceof LocalAPIError)throw e;fail('Resposta não confirmada. Preserve a operação pendente.');}finally{clearTimeout(timer);}}
};}
export function supplierPendingKey(s){if(!s||!['tenant_id','store_id','device_id','identity_id'].every(k=>id(s[k])))fail();return 'titan-supplier-edit:'+JSON.stringify([s.tenant_id,s.store_id,s.device_id,s.identity_id]);}
export function readSupplierPending(storage,key){
 let raw;try{raw=storage.getItem(key);}catch{fail('O navegador não permitiu ler a operação pendente. Gravações bloqueadas.');}
 if(raw===null)return null;let v;try{v=JSON.parse(raw);}catch{fail('Operação pendente corrompida. Preserve os dados do navegador.');}
 if(!exactFields(v,['kind','input','uncertain'])||typeof v.uncertain!=='boolean'||!validSupplierInput(v.kind,v.input))fail();return v;
}
function save(storage,key,v){try{const raw=JSON.stringify(v);storage.setItem(key,raw);if(storage.getItem(key)!==raw)fail();}catch{fail('Não foi possível preservar a operação no navegador. Nenhuma nova gravação será enviada.');}}
function remove(storage,key,p){try{if(storage.getItem(key)!==JSON.stringify(p))fail();storage.removeItem(key);if(storage.getItem(key)!==null)fail();}catch{fail('Não foi possível limpar a operação confirmada. Consulte novamente; não crie outra operação.');}}
export function persistSupplierPending(storage,key,operation){if(readSupplierPending(storage,key))fail('Resolva a alteração de fornecedor pendente antes de iniciar outra operação.');if(!validSupplierInput(operation.kind,operation.input))fail();save(storage,key,{kind:operation.kind,input:operation.input,uncertain:false});}
// Caller holds a Web Lock shared by all supplier forms and browser tabs.
export async function resolveSupplierPending(client,token,storage,key,send=false,discard=false){
 const p=readSupplierPending(storage,key);if(!p)return null;let result;
 try{result=await client.operation(token,p.kind,p.input);}catch(e){if(!(e instanceof LocalAPIError)||e.status!==404)throw e;}
 if(result){remove(storage,key,p);return {state:'confirmed',result};}
 if(discard){if(p.uncertain)fail('Resultado incerto: não descarte. Consulte ou repita a mesma operação.');remove(storage,key,p);return {state:'discarded'};}
 if(!send)return {state:'missing'};
 const uncertain={...p,uncertain:true};save(storage,key,uncertain);
 try{await client.write(token,p.kind,p.input);}catch(e){if(!p.uncertain && e instanceof LocalAPIError && [400,403,404,409,413].includes(e.status))save(storage,key,p);throw e;}
 // Successful POST alone is insufficient; confirm the immutable receipt.
 result=await client.operation(token,p.kind,p.input);remove(storage,key,uncertain);return {state:'confirmed',result};
}
