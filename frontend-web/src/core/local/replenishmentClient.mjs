import {LocalAPIError} from './localClient.mjs';
const id=v=>typeof v==='string'&&v.trim()===v&&v.length>0&&v.length<=128;
const n=v=>Number.isSafeInteger(v)&&v>=0;
const bad=()=>{throw new LocalAPIError(0,'Dados de reposição inválidos. A operação permanece pendente.');};
const fields={policy:['operation_id','product_id','minimum_milli','target_milli'],suggest:['operation_id','product_id'],review:['operation_id','suggestion_id','decision','reason']};
export function validRestockInput(kind,v){if(!fields[kind]||!v||Object.keys(v).length!==fields[kind].length||!fields[kind].every(k=>Object.hasOwn(v,k))||!id(v.operation_id))return false;
 if(kind==='review')return id(v.suggestion_id)&&['approved','rejected'].includes(v.decision)&&typeof v.reason==='string'&&v.reason.trim()===v.reason&&v.reason.length>0&&new TextEncoder().encode(v.reason).length<=255;
 return id(v.product_id)&&(kind==='suggest'||n(v.minimum_milli)&&n(v.target_milli)&&v.target_milli>v.minimum_milli);
}
export function parseThreshold(value,unit){const s=value.trim().replace(',','.');if(s.length>32||!/^\d+(?:\.\d{1,3})?$/.test(s))bad();const [whole,fraction='']=s.split('.');const v=BigInt(whole)*1000n+BigInt(fraction.padEnd(3,'0'));if(v>BigInt(Number.MAX_SAFE_INTEGER)||unit==='unit'&&v%1000n!==0n)bad();return Number(v);}
function result(kind,v,input){if(!v||v.operation_id!==input.operation_id||typeof v.repeated!=='boolean')bad();if(kind==='policy'&&(!n(v.revision)||v.revision<1))bad();if(kind==='suggest'&&(!id(v.suggestion_id)||!n(v.observed_milli)||!n(v.recommended_milli)||!n(v.policy_revision)||v.policy_revision<1||typeof v.needed!=='boolean'||v.needed!==(v.recommended_milli>0)))bad();if(kind==='review'&&(v.suggestion_id!==input.suggestion_id||v.decision!==input.decision))bad();return v;}
export function createReplenishmentClient(fetcher=globalThis.fetch){
 async function request(path,token,input){if(!token)bad();const controller=new AbortController(),timer=setTimeout(()=>controller.abort(),10000);try{const r=await fetcher('/local/v1/replenishment'+path,{method:input===undefined?'GET':'POST',signal:controller.signal,credentials:'omit',cache:'no-store',headers:{Authorization:`Bearer ${token}`,...(input===undefined?{}:{'Content-Type':'application/json'})},...(input===undefined?{}:{body:JSON.stringify(input)})});if(!r.ok){let code='';try{code=(await r.json()).code;}catch{};const messages={stale_suggestion:'Saldo ou política mudou. Rejeite esta sugestão e calcule outra.',pending_approval:'Este produto já tem uma aprovação pendente de recebimento.',missing_policy:'Cadastre mínimo e alvo antes de calcular.',operation_conflict:'Operação em conflito. Consulte o resultado original.'};throw new LocalAPIError(r.status,messages[code]||({401:'Sessão inválida.',403:'Permissão ou contrato de pedidos indisponível.',404:'Operação não encontrada.',503:'Verificador indisponível.'})[r.status]||'Reposição recusada. Confira os dados.');}return await r.json();}catch(e){if(e instanceof LocalAPIError)throw e;throw new LocalAPIError(0,'Resposta não confirmada. Consulte a operação pendente.');}finally{clearTimeout(timer);}}
 const offset=v=>{if(!n(v))bad();return v;};
 return {
 async products(token,o=0){const v=await request('/products?offset='+offset(o),token);if(!Array.isArray(v?.items)||v.items.length>50||!v.items.every(p=>id(p?.id)&&typeof p.name==='string'&&p.name.length>0&&['unit','kg','g','liter','ml','meter'].includes(p.unit)&&['balance_milli','minimum_milli','target_milli','revision'].every(k=>n(p[k]))&&(p.revision===0?p.minimum_milli===0&&p.target_milli===0:p.target_milli>p.minimum_milli)))bad();return v.items;},
 async suggestions(token,o=0){const v=await request('/suggestions?offset='+offset(o),token);if(!Array.isArray(v?.items)||v.items.length>50||!v.items.every(s=>id(s?.id)&&id(s.product_id)&&typeof s.name==='string'&&['unit','kg','g','liter','ml','meter'].includes(s.unit)&&['observed_milli','recommended_milli','policy_revision'].every(k=>n(s[k]))&&s.policy_revision>0&&['suggested','approved','rejected','not_needed'].includes(s.status)&&typeof s.stale==='boolean'&&typeof s.reason==='string'&&typeof s.created_at==='string'&&typeof s.order_id==='string'&&(s.order_id===''||id(s.order_id))))bad();return v.items;},
 async operation(token,kind,input){if(!validRestockInput(kind,input))bad();const v=await request('/operations/'+kind+'/'+encodeURIComponent(input.operation_id),token);if(v?.kind!==kind||!validRestockInput(kind,v.input)||!fields[kind].every(k=>v.input[k]===input[k]))bad();return result(kind,v.result,input);},
 async write(token,kind,input){if(!validRestockInput(kind,input))bad();return result(kind,await request('/'+({policy:'policies',suggest:'suggestions',review:'reviews'})[kind],token,input),input);}
 };
}
export function restockKey(s){if(!s||!['tenant_id','store_id','device_id','identity_id'].every(k=>id(s[k])))bad();return 'titan-restock:'+JSON.stringify([s.tenant_id,s.store_id,s.device_id,s.identity_id]);}
export function readRestockPending(storage,key){const raw=storage.getItem(key);if(raw===null)return null;let v;try{v=JSON.parse(raw);}catch{bad();}if(!v||Object.keys(v).length!==3||typeof v.uncertain!=='boolean'||!validRestockInput(v.kind,v.input))bad();return v;}
function save(storage,key,v){const raw=JSON.stringify(v);storage.setItem(key,raw);if(storage.getItem(key)!==raw)bad();}
export function persistRestockPending(storage,key,kind,input){if(readRestockPending(storage,key))throw new LocalAPIError(0,'Resolva a reposição pendente antes de iniciar outra.');if(!validRestockInput(kind,input))bad();save(storage,key,{kind,input,uncertain:false});}
// A caller must hold the browser lock. An unknown result is never discarded.
export async function resolveRestockPending(client,token,storage,key,send=false,discard=false){const p=readRestockPending(storage,key);if(!p)return null;let v;try{v=await client.operation(token,p.kind,p.input);}catch(e){if(!(e instanceof LocalAPIError)||e.status!==404)throw e;}
 if(v){storage.removeItem(key);return {state:'confirmed',result:v};}
 if(discard){if(p.uncertain)throw new LocalAPIError(0,'Resultado incerto: consulte ou repita a mesma operação.');storage.removeItem(key);return {state:'discarded'};}
 if(!send)return {state:'missing'};save(storage,key,{...p,uncertain:true});
 try{await client.write(token,p.kind,p.input);}catch(e){if(!p.uncertain&&e instanceof LocalAPIError&&[400,403,404,409,413].includes(e.status))save(storage,key,{...p,uncertain:false});throw e;}
 v=await client.operation(token,p.kind,p.input);storage.removeItem(key);return {state:'confirmed',result:v};
}
