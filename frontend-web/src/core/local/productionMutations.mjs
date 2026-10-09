import {LocalAPIError} from './localClient.mjs';
import {id,integer,positive,unit,readAPI} from './operationsRead.mjs';
import {validRecipeVersion,validProductionOrder} from './productionRead.mjs';
const fields={recipe:['operation_id','recipe_id','version_id','expected_revision','name','output_product_id','output_unit','yield_milli','ingredients'],order:['operation_id','order_id','version_id','location_id','responsible_id','planned_batches'],state:['operation_id','order_id','expected_revision','status','reason'],recipe_state:['operation_id','recipe_id','expected_revision','status','reason'],reserve:['operation_id','reservation_id','order_id','reason'],materials:['operation_id','reservation_id','action','reason'],result:['operation_id','result_id','order_id','expected_revision','produced_milli','reason'],stage_plan:['operation_id','order_id','stages','reason'],stage_state:['operation_id','order_id','stage_id','expected_revision','status','reason'],loss:['operation_id','loss_id','result_id','quantity_milli','reason'],loss_void:['operation_id','loss_id','expected_revision','reason']};
const fail=message=>{throw new LocalAPIError(0,message || 'Dados de produção inválidos. A operação pendente foi preservada.');};
const exactFields=(v,keys)=>v && typeof v==='object' && !Array.isArray(v) && Object.keys(v).length===keys.length && keys.every(k=>Object.hasOwn(v,k));
const shortText=v=>typeof v==='string' && v.trim()===v && v.length>0 && new TextEncoder().encode(v).length<=255;
export function validProductionInput(kind,v){
 if(!fields[kind] || !exactFields(v,fields[kind]) || !id(v.operation_id))return false;
 if(kind==='order')return ['order_id','version_id','location_id','responsible_id'].every(k=>id(v[k])) && positive(v.planned_batches);
 if(kind==='loss')return id(v.loss_id)&&id(v.result_id)&&positive(v.quantity_milli)&&shortText(v.reason);
 if(kind==='loss_void')return id(v.loss_id)&&v.expected_revision===1&&shortText(v.reason);
 if(kind==='stage_plan')return id(v.order_id)&&shortText(v.reason)&&Array.isArray(v.stages)&&v.stages.length>0&&v.stages.length<=20&&v.stages.every(s=>exactFields(s,['stage_id','name','responsible_id'])&&id(s.stage_id)&&id(s.responsible_id)&&shortText(s.name)&&new TextEncoder().encode(s.name).length<=120)&&new Set(v.stages.map(s=>s.stage_id)).size===v.stages.length;
 if(kind==='reserve')return id(v.reservation_id)&&id(v.order_id)&&shortText(v.reason);
 if(kind==='materials')return id(v.reservation_id)&&['release','consume'].includes(v.action)&&shortText(v.reason);
 if(!integer(v.expected_revision) || v.expected_revision>=2147483647)return false;
 if(kind==='state')return id(v.order_id) && v.expected_revision>0 && ['approved','cancelled'].includes(v.status) && shortText(v.reason);
 if(kind==='stage_state')return id(v.order_id)&&id(v.stage_id)&&shortText(v.reason)&&((v.status==='running'&&v.expected_revision===1)||(v.status==='completed'&&v.expected_revision===2));
 if(kind==='recipe_state')return id(v.recipe_id)&&['active','inactive'].includes(v.status)&&shortText(v.reason);
 if(kind==='result')return id(v.result_id)&&id(v.order_id)&&v.expected_revision>0&&integer(v.produced_milli)&&shortText(v.reason);
 return id(v.recipe_id) && id(v.version_id) && id(v.output_product_id) && unit(v.output_unit) && shortText(v.name) && positive(v.yield_milli) && Array.isArray(v.ingredients) && v.ingredients.length>0 && v.ingredients.length<=100 && v.ingredients.every(i=>exactFields(i,['product_id','unit','quantity_milli']) && id(i.product_id) && i.product_id!==v.output_product_id && unit(i.unit) && positive(i.quantity_milli)) && new Set(v.ingredients.map(i=>i.product_id)).size===v.ingredients.length;
}
function canonical(kind,input){
 const v=Object.fromEntries(fields[kind].map(k=>[k,input[k]]));
 if(kind==='recipe')v.ingredients=input.ingredients.map(i=>({product_id:i.product_id,unit:i.unit,quantity_milli:i.quantity_milli})).sort((a,b)=>a.product_id<b.product_id?-1:a.product_id>b.product_id?1:0);
 if(kind==='stage_plan')v.stages=input.stages.map(s=>({stage_id:s.stage_id,name:s.name,responsible_id:s.responsible_id}));
 return JSON.stringify(v);
}
function validResult(kind,r,input){
 if(!r || typeof r.repeated!=='boolean')return false;
 if(kind==='reserve'||kind==='materials')return r.reservation_id===input.reservation_id&&id(r.order_id)&&(kind!=='reserve'||r.order_id===input.order_id)&&r.status===(kind==='reserve'?'active':input.action==='release'?'released':'consumed');
 if(!positive(r.revision))return false;
 if(kind==='loss'||kind==='loss_void')return r.loss_id===input.loss_id&&id(r.result_id)&&(kind!=='loss'||r.result_id===input.result_id)&&r.revision===(kind==='loss'?1:2)&&r.status===(kind==='loss'?'recorded':'voided');
 if(kind==='stage_plan')return r.order_id===input.order_id&&r.status==='configured'&&r.revision===1&&!r.stage_id;
 if(kind==='stage_state')return r.order_id===input.order_id&&r.stage_id===input.stage_id&&r.status===input.status&&r.revision===input.expected_revision+1;
 if(kind==='recipe_state')return r.recipe_id===input.recipe_id&&r.status===input.status&&r.revision===input.expected_revision+1;
 if(kind==='result')return r.result_id===input.result_id&&r.order_id===input.order_id&&r.revision===input.expected_revision+1&&r.status==='completed'&&['reservation_id','product_id','location_id'].every(k=>id(r[k]))&&unit(r.unit)&&positive(r.planned_milli)&&integer(r.produced_milli)&&integer(r.shortfall_milli)&&r.produced_milli===input.produced_milli&&BigInt(r.produced_milli)+BigInt(r.shortfall_milli)===BigInt(r.planned_milli)&&(r.unit!=='unit'||r.produced_milli%1000===0);
 if(kind==='recipe')return r.recipe_id===input.recipe_id && r.version_id===input.version_id && r.revision===input.expected_revision+1;
 return r.order_id===input.order_id && r.revision===(kind==='order'?1:input.expected_revision+1) && r.status===(kind==='order'?'planned':input.status);
}
export function parseProductionQuantity(value,allowZero=false){
 const s=value.trim();if(s.length>32 || !/^\d+(?:[.,]\d{1,3})?$/.test(s))fail('Use quantidade positiva com até três casas decimais, sem separador de milhar.');
 const [whole,fraction='']=s.replace(',','.').split('.'),v=BigInt(whole)*1000n+BigInt(fraction.padEnd(3,'0'));
 if(v<(allowZero?0n:1n) || v>BigInt(Number.MAX_SAFE_INTEGER))fail('Quantidade fora do intervalo exato suportado.');return Number(v);
}
export function parseProductionInteger(value,allowZero=false){
 if(!/^\d{1,16}$/.test(value.trim()))fail('Informe um número inteiro, sem casas decimais.');
 const v=BigInt(value.trim());if(v<(allowZero?0n:1n)||v>BigInt(Number.MAX_SAFE_INTEGER))fail('Número inteiro fora do intervalo suportado.');return Number(v);
}
export function plannedPreview(recipe,batches){
 if(!validRecipeVersion(recipe)||!positive(batches))fail();
 const output=BigInt(recipe.yield_milli)*BigInt(batches),ingredients=recipe.ingredients.map(i=>({...i,planned_milli:BigInt(i.quantity_milli)*BigInt(batches)}));
 if(output>BigInt(Number.MAX_SAFE_INTEGER)||ingredients.some(i=>i.planned_milli>BigInt(Number.MAX_SAFE_INTEGER)))fail('O planejamento ultrapassa o limite exato de quantidade.');
 return {output_milli:Number(output),ingredients:ingredients.map(i=>({...i,planned_milli:Number(i.planned_milli)}))};
}
export function createProductionMutations(fetcher=globalThis.fetch){
 const get=readAPI(fetcher);
 return {
  async recipeState(token,recipeID){if(!id(recipeID))fail();return get(`/production/recipes/${encodeURIComponent(recipeID)}/state`,token,v=>v&&v.recipe_id===recipeID&&['active','inactive'].includes(v.status)&&integer(v.revision)&&v.revision<=2147483647&&typeof v.repeated==='boolean');},
  async version(token,versionID){if(!id(versionID))fail();return get('/production/recipe-versions/'+encodeURIComponent(versionID),token,v=>validRecipeVersion(v)&&v.version_id===versionID);},
  async order(token,orderID){if(!id(orderID))fail();return get('/production/orders/'+encodeURIComponent(orderID),token,v=>validProductionOrder(v)&&v.id===orderID);},
  async operation(token,kind,input){
   if(!validProductionInput(kind,input))fail();
   const v=await get(`/production/operations/${kind}/${encodeURIComponent(input.operation_id)}`,token,v=>v && v.kind===kind && validProductionInput(kind,v.input) && canonical(kind,v.input)===canonical(kind,input) && validResult(kind,v.result,input));return v.result;
  },
  async write(token,kind,input){
   if(!token||!validProductionInput(kind,input))fail();
   const controller=new AbortController(),timer=setTimeout(()=>controller.abort(),30000);
   try {
    const r=await fetcher('/local/v1/production/'+({recipe:'recipe-versions',order:'orders',state:'orders/state',recipe_state:'recipe-state',reserve:'material-reservations',materials:'material-reservations/state',result:'results',stage_plan:'stage-plans',stage_state:'stages/state',loss:'losses',loss_void:'losses/void'})[kind],{method:'POST',signal:controller.signal,credentials:'omit',cache:'no-store',headers:{Authorization:`Bearer ${token}`,'Content-Type':'application/json'},body:JSON.stringify(input)});
    if(!r.ok)throw new LocalAPIError(r.status,({400:'Confira quantidades, unidades e referências da produção.',401:'Sessão expirada ou revogada.',403:'Permissão ou contrato de produção indisponível.',404:'Referência não encontrada nesta loja.',409:'Revisão, estado, referência ou operação em conflito. Consulte antes de corrigir.',503:'Verificação do contrato indisponível.'})[r.status] || 'Gravação de produção recusada.');
    const v=await r.json();if(!validResult(kind,v,input))fail('Resposta de gravação incompatível. Consulte a operação pendente.');return v;
   }catch(e){if(e instanceof LocalAPIError)throw e;fail('Resposta não confirmada. Consulte ou repita a mesma operação pendente.');}finally{clearTimeout(timer);}
  },
 };
}
export function productionPendingKey(s){if(!s||!['tenant_id','store_id','device_id','identity_id'].every(k=>id(s[k])))fail();return 'titan-production:'+JSON.stringify([s.tenant_id,s.store_id,s.device_id,s.identity_id]);}
export function readProductionPending(storage,key){
 let raw;try{raw=storage.getItem(key);}catch{fail('O navegador não permitiu ler a operação pendente. Gravações bloqueadas.');}
 if(raw===null)return null;let v;try{v=JSON.parse(raw);}catch{fail('Operação pendente corrompida. Preserve os dados do navegador.');}
 if(!exactFields(v,['kind','input','uncertain'])||typeof v.uncertain!=='boolean'||!validProductionInput(v.kind,v.input))fail();return v;
}
function save(storage,key,v){try{const raw=JSON.stringify(v);storage.setItem(key,raw);if(storage.getItem(key)!==raw)fail();}catch{fail('Não foi possível preservar a operação no navegador. Nenhuma nova gravação será enviada.');}}
function remove(storage,key,p){try{if(storage.getItem(key)!==JSON.stringify(p))fail();storage.removeItem(key);if(storage.getItem(key)!==null)fail();}catch{fail('Não foi possível limpar a operação confirmada. Consulte novamente; não crie outra operação.');}}
export function persistProductionPending(storage,key,operation){if(readProductionPending(storage,key))fail('Resolva a produção pendente antes de iniciar outra operação.');if(!validProductionInput(operation.kind,operation.input))fail();save(storage,key,{kind:operation.kind,input:operation.input,uncertain:false});}
// Caller holds a Web Lock shared by all production forms and browser tabs.
export async function resolveProductionPending(client,token,storage,key,send=false,discard=false){
 const p=readProductionPending(storage,key);if(!p)return null;let result;
 try{result=await client.operation(token,p.kind,p.input);}catch(e){if(!(e instanceof LocalAPIError)||e.status!==404)throw e;}
 if(result){remove(storage,key,p);return {state:'confirmed',result};}
 if(discard){if(p.uncertain)fail('Resultado incerto: não descarte. Consulte ou repita a mesma operação.');remove(storage,key,p);return {state:'discarded'};}
 if(!send)return {state:'missing'};
 const uncertain={...p,uncertain:true};save(storage,key,uncertain);
 try{await client.write(token,p.kind,p.input);}catch(e){if(!p.uncertain && e instanceof LocalAPIError && [400,403,404,409,413].includes(e.status))save(storage,key,p);throw e;}
 // Successful POST alone is insufficient; confirm the immutable receipt.
 result=await client.operation(token,p.kind,p.input);remove(storage,key,uncertain);return {state:'confirmed',result};
}
