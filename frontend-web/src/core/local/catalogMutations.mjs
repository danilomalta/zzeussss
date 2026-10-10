import {LocalAPIError} from './localClient.mjs';
import {id,integer,unit,readAPI} from './operationsRead.mjs';
import {parseMoney} from './posModel.mjs';
const fail=message=>{throw new LocalAPIError(0,message||'Dados do catálogo incompatíveis. A operação pendente foi preservada.');};
const fields={edit:['operation_id','product_id','expected_revision','sku','name','unit','barcode','price_cents','cost_cents'],state:['operation_id','product_id','expected_revision','status','reason']};
const exactFields=(v,keys)=>v&&typeof v==='object'&&!Array.isArray(v)&&Object.keys(v).length===keys.length&&keys.every(k=>Object.hasOwn(v,k));
const limited=(v,n,empty=false)=>typeof v==='string'&&v.trim()===v&&(empty||v.length>0)&&new TextEncoder().encode(v).length<=n;
export function validEditingDetails(v,productID){return v&&v.product_id===productID&&id(productID)&&integer(v.revision)&&v.revision<=2147483647&&v.product&&limited(v.product.sku,100)&&limited(v.product.name,255)&&limited(v.product.barcode,128,true)&&unit(v.product.unit)&&integer(v.product.price_cents)&&integer(v.product.cost_cents);}
export function validProductState(v,productID){return v&&v.product_id===productID&&id(productID)&&integer(v.revision)&&v.revision<=2147483647&&['active','inactive'].includes(v.status)&&typeof v.repeated==='boolean'&&(v.revision!==0||v.status==='active');}
export function validCatalogInput(kind,v){if(!fields[kind]||!exactFields(v,fields[kind])||!id(v.operation_id)||!id(v.product_id)||!integer(v.expected_revision)||v.expected_revision>=2147483647)return false;if(kind==='state')return ['active','inactive'].includes(v.status)&&limited(v.reason,255)&&!/[\x00\r\n]/.test(v.reason);return limited(v.sku,100)&&limited(v.name,255)&&unit(v.unit)&&limited(v.barcode,128,true)&&integer(v.price_cents)&&integer(v.cost_cents);}
function canonical(kind,input){return JSON.stringify(Object.fromEntries(fields[kind].map(k=>[k,input[k]])));}
function validResult(kind,v,input){return v&&v.product_id===input.product_id&&v.revision===input.expected_revision+1&&typeof v.repeated==='boolean'&&(kind!=='state'||v.status===input.status);}
export function editMoneyValue(cents){if(!integer(cents))fail();const n=BigInt(cents);return `${n/100n}.${String(n%100n).padStart(2,'0')}`;}
export function prepareCatalogEdit(snapshot,values,operationID){if(!validEditingDetails(snapshot,snapshot?.product_id))fail();const input={operation_id:operationID,product_id:snapshot.product_id,expected_revision:snapshot.revision,sku:values.sku.trim(),name:values.name.trim(),unit:values.unit,barcode:values.barcode.trim(),price_cents:parseMoney(values.price),cost_cents:parseMoney(values.cost)};if(!validCatalogInput('edit',input))fail();return input;}
export function prepareProductState(state,status,reason,operationID){if(!validProductState(state,state?.product_id)||state.status===status)fail();const input={operation_id:operationID,product_id:state.product_id,expected_revision:state.revision,status,reason:reason.trim()};if(!validCatalogInput('state',input))fail();return input;}
export function createCatalogMutations(fetcher=globalThis.fetch){const get=readAPI(fetcher);return {
 async details(token,productID){if(!id(productID))fail();return get(`/catalog/products/${encodeURIComponent(productID)}/edit-details`,token,v=>validEditingDetails(v,productID));},
 async state(token,productID){if(!id(productID))fail();return get(`/catalog/products/${encodeURIComponent(productID)}/state`,token,v=>validProductState(v,productID));},
 async operation(token,kind,input){if(!validCatalogInput(kind,input))fail();const v=await get(`/catalog/operations/${kind}/${encodeURIComponent(input.operation_id)}`,token,v=>v&&v.kind===kind&&validCatalogInput(kind,v.input)&&canonical(kind,v.input)===canonical(kind,input)&&validResult(kind,v.result,input));return v.result;},
 async write(token,kind,input){if(!token||!validCatalogInput(kind,input))fail();const controller=new AbortController(),timer=setTimeout(()=>controller.abort(),30000);try{const {product_id,...body}=input;const response=await fetcher(`/local/v1/catalog/products/${encodeURIComponent(product_id)}/${kind==='edit'?'update':'state'}`,{method:'POST',signal:controller.signal,credentials:'omit',cache:'no-store',headers:{Authorization:`Bearer ${token}`,'Content-Type':'application/json'},body:JSON.stringify(body)});if(!response.ok)throw new LocalAPIError(response.status,({401:'Sessão expirada.',403:'Permissão ou contrato indisponível.',409:'Revisão ou dados em conflito. Consulte antes de corrigir.'})[response.status]||'Alteração do catálogo recusada.');const v=await response.json();if(!validResult(kind,v,input))fail();return v;}catch(e){if(e instanceof LocalAPIError)throw e;fail('Resposta não confirmada. Preserve a operação pendente.');}finally{clearTimeout(timer);}}
};}
export function catalogPendingKey(s){if(!s||!['tenant_id','store_id','device_id','identity_id'].every(k=>id(s[k])))fail();return 'titan-catalog:'+JSON.stringify([s.tenant_id,s.store_id,s.device_id,s.identity_id]);}
export function readCatalogPending(storage,key){
 let raw;try{raw=storage.getItem(key);}catch{fail('O navegador não permitiu ler a operação pendente. Gravações bloqueadas.');}
 if(raw===null)return null;let v;try{v=JSON.parse(raw);}catch{fail('Operação pendente corrompida. Preserve os dados do navegador.');}
 if(!exactFields(v,['kind','input','uncertain'])||typeof v.uncertain!=='boolean'||!validCatalogInput(v.kind,v.input))fail();return v;
}
function save(storage,key,v){try{const raw=JSON.stringify(v);storage.setItem(key,raw);if(storage.getItem(key)!==raw)fail();}catch{fail('Não foi possível preservar a operação no navegador. Nenhuma nova gravação será enviada.');}}
function remove(storage,key,p){try{if(storage.getItem(key)!==JSON.stringify(p))fail();storage.removeItem(key);if(storage.getItem(key)!==null)fail();}catch{fail('Não foi possível limpar a operação confirmada. Consulte novamente; não crie outra operação.');}}
export function persistCatalogPending(storage,key,operation){if(readCatalogPending(storage,key))fail('Resolva a produção pendente antes de iniciar outra operação.');if(!validCatalogInput(operation.kind,operation.input))fail();save(storage,key,{kind:operation.kind,input:operation.input,uncertain:false});}
// Caller holds a Web Lock shared by all catalog forms and browser tabs.
export async function resolveCatalogPending(client,token,storage,key,send=false,discard=false){
 const p=readCatalogPending(storage,key);if(!p)return null;let result;
 try{result=await client.operation(token,p.kind,p.input);}catch(e){if(!(e instanceof LocalAPIError)||e.status!==404)throw e;}
 if(result){remove(storage,key,p);return {state:'confirmed',result};}
 if(discard){if(p.uncertain)fail('Resultado incerto: não descarte. Consulte ou repita a mesma operação.');remove(storage,key,p);return {state:'discarded'};}
 if(!send)return {state:'missing'};
 const uncertain={...p,uncertain:true};save(storage,key,uncertain);
 try{await client.write(token,p.kind,p.input);}catch(e){if(!p.uncertain && e instanceof LocalAPIError && [400,403,404,409,413].includes(e.status))save(storage,key,p);throw e;}
 // Successful POST alone is insufficient; confirm the immutable receipt.
 result=await client.operation(token,p.kind,p.input);remove(storage,key,uncertain);return {state:'confirmed',result};
}
