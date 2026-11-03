import {materialCostEstimate,productionCostScenario,costScenarioInput,simultaneousInput,productionSimultaneous} from './planning.mjs';
import {productionResponsibles} from './responsibles.mjs';
import {productionStageHistory} from './stageHistory.mjs';
import {recipeStateInput,productionRecipeState,recipeStateResult} from './recipeState.mjs';
import {aftermathInput,aftermathResult,aftermathPaths,productionAftermathRow,productionAftermathSummary,productionDeclarationHistory,productionQuality,productionQualityHistory} from './aftermath.mjs';
import {executionInput,executionResult,executionPaths,productionExecutionTrace} from './execution.mjs';
import {contextOf,contextKey} from '../context74/model.mjs';
import {LocalAPIError} from '../local/localClient.mjs';
import {recipeVersion} from './source.mjs';
const fail=()=>{throw Error('productionInvalid');};
const utf8=s=>new TextEncoder().encode(s).length;
const id=s=>typeof s==='string'&&s.length>0&&utf8(s)<=128&&s===s.trim()&&!/[\x00\r\n]/.test(s);
const int=(v,min=0,max=Number.MAX_SAFE_INTEGER)=>Number.isSafeInteger(v)&&v>=min&&v<=max;
const units=['unit','kg','g','liter','ml','meter'];
const exact=(o,keys)=>o&&typeof o==='object'&&!Array.isArray(o)&&Object.keys(o).length===keys.length&&keys.every(k=>Object.hasOwn(o,k));
const text=s=>typeof s==='string'&&s.trim()===s&&s.length>0&&utf8(s)<=255&&!/[\x00\r\n]/.test(s);
const canonical=v=>JSON.stringify(sort(v));
function sort(v){return Array.isArray(v)?v.map(sort):v&&typeof v==='object'?Object.fromEntries(Object.keys(v).sort().map(k=>[k,sort(v[k])])):v;}
export function productionInput(kind,b){
 if(kind==='recipe_state')return recipeStateInput(b);
 const aftermath=aftermathInput(kind,b);if(aftermath)return aftermath;
 const execution=executionInput(kind,b);if(execution)return execution;
 if(kind==='recipe'){
  if(!exact(b,['operation_id','recipe_id','version_id','expected_revision','name','output_product_id','output_unit','yield_milli','ingredients'])||!['operation_id','recipe_id','version_id','output_product_id'].every(k=>id(b[k]))||!int(b.expected_revision,0,2147483646)||!text(b.name)||!units.includes(b.output_unit)||!int(b.yield_milli,1)||!Array.isArray(b.ingredients)||b.ingredients.length<1||b.ingredients.length>100||!b.ingredients.every(i=>exact(i,['product_id','unit','quantity_milli'])&&id(i.product_id)&&i.product_id!==b.output_product_id&&units.includes(i.unit)&&int(i.quantity_milli,1))||new Set(b.ingredients.map(i=>i.product_id)).size!==b.ingredients.length)fail();
  return {...b,ingredients:b.ingredients.map(i=>({...i})).sort((a,b)=>a.product_id<b.product_id?-1:a.product_id>b.product_id?1:0)};
 }
 if(kind==='order'){
  if(!exact(b,['operation_id','order_id','version_id','location_id','responsible_id','planned_batches'])||!['operation_id','order_id','version_id','location_id','responsible_id'].every(k=>id(b[k]))||!int(b.planned_batches,1))fail();
 }else if(kind==='state'){
  if(!exact(b,['operation_id','order_id','expected_revision','status','reason'])||!id(b.operation_id)||!id(b.order_id)||!int(b.expected_revision,1,2147483646)||!['approved','cancelled'].includes(b.status)||!text(b.reason))fail();
 }else fail();
 return {...b};
}
export function productionReceipt(v,kind,input,original=true){
 const b=productionInput(kind,input),r=v?.result;
 if(!exact(v,['kind','input','result'])||v.kind!==kind||canonical(productionInput(kind,v.input))!==canonical(b)||!r||typeof r.repeated!=='boolean'||(original&&r.repeated!==false))fail();
 if(kind==='recipe_state'){recipeStateResult(r,b);return v;}
 if(Object.hasOwn(aftermathPaths,kind)){aftermathResult(kind,b,r);return v;}
 if(Object.hasOwn(executionPaths,kind)){executionResult(kind,b,r);return v;}
 if(kind==='recipe'){
  if(!exact(r,['recipe_id','version_id','revision','repeated'])||r.recipe_id!==b.recipe_id||r.version_id!==b.version_id||r.revision!==b.expected_revision+1)fail();
 }else if(!exact(r,['order_id','revision','status','repeated'])||r.order_id!==b.order_id||r.revision!==(kind==='order'?1:b.expected_revision+1)||r.status!==(kind==='order'?'planned':b.status))fail();
 return v;
}
export function productionOrder(v){
 if(!v||!['id','version_id','location_id','responsible_id','created_by'].every(k=>id(v[k]))||!int(v.planned_batches,1)||!int(v.planned_output_milli,1)||!int(v.revision,1,2147483647)||!['planned','approved','cancelled','completed'].includes(v.status)||!['created_at','updated_at'].every(k=>typeof v[k]==='string'&&Number.isFinite(Date.parse(v[k])))||(v.status==='completed'&&!id(v.completion_id)))fail();
 const recipe=recipeVersion(v.recipe);
 if(v.version_id!==recipe.version_id||BigInt(v.planned_output_milli)!==BigInt(v.planned_batches)*BigInt(recipe.yield_milli)||recipe.ingredients.some(i=>BigInt(i.quantity_milli)*BigInt(v.planned_batches)>BigInt(Number.MAX_SAFE_INTEGER)))fail();
 return v;
}
export function parseProductionMilli(value){
 if(typeof value!=='string'||!/^\d+(?:[.,]\d{1,3})?$/.test(value))fail();const [whole,fraction='']=value.replace(',','.').split('.'),n=BigInt(whole)*1000n+BigInt(fraction.padEnd(3,'0'));if(n<1n||n>BigInt(Number.MAX_SAFE_INTEGER))fail();return Number(n);
}
export function createProductionCommands({token,session,capabilities,isCurrent,storage,locks,fetcher=globalThis.fetch}){
 const scope=contextKey(contextOf(session,capabilities)),actor=session.identity_id,key='titan-production-commands:v1:'+scope;
 if(typeof token!=='string'||!token||/[\r\n]/.test(token)||typeof isCurrent!=='function')fail();
 const writable=()=>capabilities.permissions?.includes('manage_production')&&capabilities.license?.state==='active'&&['core','inventory','production'].every(m=>capabilities.license.modules.includes(m));
 const canListResponsibles=()=>capabilities.permissions?.includes('manage_staff')&&capabilities.license?.state==='active'&&['core','staff'].every(m=>capabilities.license.modules.includes(m));
 function authorizeStaff(){authorize();if(!canListResponsibles())throw new LocalAPIError(403,'production');}
 function authorize(write=false){let current=false;try{current=isCurrent()&&contextKey(contextOf(session,capabilities))===scope;}catch{}if(!current)throw new LocalAPIError(401,'production');if(!capabilities.permissions?.includes('manage_production')||(write&&!writable()))throw new LocalAPIError(403,'production');}
 authorize();
 function authorizeCosts(){authorize();if(!capabilities.permissions?.includes('view_accounting'))throw new LocalAPIError(403,'production');}
 async function request(path,method='GET',body,signal,readCalculation=false){
  authorize(method==='POST'&&!readCalculation);const c=new AbortController(),abort=()=>c.abort();if(signal?.aborted)abort();signal?.addEventListener('abort',abort,{once:true});const timer=setTimeout(abort,15000);
  try{c.signal.throwIfAborted();const r=await fetcher('/local/v1/'+path,{method,credentials:'omit',cache:'no-store',signal:c.signal,headers:{Authorization:'Bearer '+token,...(body?{'Content-Type':'application/json'}:{})},...(body?{body:JSON.stringify(body)}:{})});authorize();c.signal.throwIfAborted();if(!r.ok)throw new LocalAPIError(r.status,'production');const raw=await r.text();authorize();c.signal.throwIfAborted();if(raw.length>2000000)fail();try{return JSON.parse(raw);}catch{fail();}}finally{clearTimeout(timer);signal?.removeEventListener('abort',abort);}
 }
 function command(c){if(!exact(c,['version','scope','kind','body','uncertain'])||c.version!==1||c.scope!==scope||typeof c.uncertain!=='boolean')fail();return {...c,body:productionInput(c.kind,c.body)};}
 function pending(){authorize();try{const raw=storage.getItem(key);if(raw===null)return null;if(raw.length>80000)fail();return command(JSON.parse(raw));}catch{throw Error('productionStorage');}}
 function save(c){authorize();const raw=JSON.stringify(command(c));try{storage.setItem(key,raw);if(storage.getItem(key)!==raw)throw Error();}catch{throw Error('productionStorage');}}
 function clear(c){authorize();const current=pending();if(!current||canonical(current)!==canonical(c))throw Error('productionStorage');try{storage.removeItem(key);if(storage.getItem(key)!==null)throw Error();}catch{throw Error('productionStorage');}}
 async function locked(fn){if(!locks?.request)throw Error('productionLocks');return locks.request(key,{ifAvailable:true},async lock=>{authorize();if(!lock)throw Error('productionLocked');return fn();});}
 const receipt=c=>request('production/operations/'+c.kind+'/'+encodeURIComponent(c.body.operation_id)).then(v=>productionReceipt(v,c.kind,c.body));
 async function resolve(send,discard){
  let c=pending();if(!c)return null;authorize(send||discard);let r;
  try{r=await receipt(c);}catch(e){if(!(e instanceof LocalAPIError)||e.status!==404)throw e;}
  if(r){clear(c);return {state:'confirmed',receipt:r};}
  if(discard){if(c.uncertain)throw Error('productionUncertain');clear(c);return {state:'discarded'};}
  if(!send)return {state:'missing'};
  authorize(true);const prior=c.uncertain;c={...c,uncertain:true};save(c);
  try{const path={recipe_state:'production/recipe-state',recipe:'production/recipe-versions',order:'production/orders',state:'production/orders/state',...executionPaths,...aftermathPaths}[c.kind],result=await request(path,'POST',c.body);productionReceipt({kind:c.kind,input:c.body,result},c.kind,c.body,false);}catch(e){if(!prior&&e instanceof LocalAPIError&&[400,403,404,409,413,422].includes(e.status)){authorize();save({...c,uncertain:false});}throw e;}
  r=await receipt(c);clear(c);return {state:'confirmed',receipt:r};
 }
 return {scope,actor,get canViewCosts(){return capabilities.permissions?.includes('view_accounting')===true;},get canListResponsibles(){return !!canListResponsibles();},canLock:!!locks?.request,get canWrite(){return !!writable();},pending,
  prepare(kind,body){authorize(true);return command({version:1,scope,kind,body:productionInput(kind,body),uncertain:false});},
  async submit(c){authorize(true);const frozen=command(structuredClone(c));if(frozen.uncertain)fail();return locked(async()=>{authorize(true);if(pending())throw Error('productionPending');save(frozen);return resolve(true,false);});},
  async recover(send=false,discard=false){return locked(()=>resolve(send,discard));},
  async materialCost(version,batches,signal){authorizeCosts();const v=recipeVersion(structuredClone(version));if(!int(batches,1))fail();const result=await request('production/recipe-versions/'+encodeURIComponent(v.version_id)+'/material-cost?batches='+batches,'GET',undefined,signal);authorizeCosts();return materialCostEstimate(result,v,batches);},
  async orderMaterialCost(order,signal){authorizeCosts();const o=productionOrder(structuredClone(order)),result=await request('production/orders/'+encodeURIComponent(o.id)+'/material-cost','GET',undefined,signal);authorizeCosts();return materialCostEstimate(result,o.recipe,o.planned_batches,o.id);},
  async costScenario(version,input,signal){authorizeCosts();const v=recipeVersion(structuredClone(version)),b=costScenarioInput(input),result=await request('management/production/recipe-versions/'+encodeURIComponent(v.version_id)+'/cost-scenario','POST',b,signal,true);authorizeCosts();return productionCostScenario(result,v,b);},
  async simultaneous(locationID,items,signal){const frozen=structuredClone(items),body=simultaneousInput(locationID,frozen),result=await request('management/production/simultaneous-plan','POST',body,signal,true);return productionSimultaneous(result,locationID,frozen);},
  async responsibles(signal){authorizeStaff();const v=await request('staff','GET',undefined,signal);authorizeStaff();return productionResponsibles(v);},
  async recipeState(recipeID,signal){if(!id(recipeID))fail();return productionRecipeState(await request('production/recipes/'+encodeURIComponent(recipeID)+'/state','GET',undefined,signal),recipeID);},
  async products(query='',offset=0,signal){if(typeof query!=='string'||utf8(query)>240||!int(offset,0,2147483647))fail();const v=await request('catalog/search?'+new URLSearchParams({q:query,offset:String(offset)}),'GET',undefined,signal);if(!v||!Array.isArray(v.items)||v.limit!==50||v.offset!==offset||!int(v.total)||v.items.length!==Math.min(50,Math.max(0,v.total-offset))||!v.items.every(p=>p&&id(p.id)&&typeof p.name==='string'&&p.name.trim()&&typeof p.sku==='string'&&units.includes(p.unit))||new Set(v.items.map(p=>p.id)).size!==v.items.length)fail();return {items:v.items.map(p=>({id:p.id,name:p.name,sku:p.sku,unit:p.unit})),total:v.total};},
  async declarations(kind,reference,offset=0,signal){if(!['loss','lot'].includes(kind)||!id(reference?.result_id)||!int(offset,0,2147483647))fail();const v=await request('production/results/'+encodeURIComponent(reference.result_id)+'/'+(kind==='lot'?'lots':'losses')+'?offset='+offset,'GET',undefined,signal);productionAftermathSummary(v,kind,reference);return {items:v.items,recorded:kind==='lot'?v.assigned_milli:v.recorded_loss_milli,remaining:kind==='lot'?v.unassigned_milli:v.unclassified_shortfall_milli};},
  async declaration(kind,row,reference,signal){if(!['loss','lot'].includes(kind)||!id(row))fail();const v=productionAftermathRow(await request('production/'+(kind==='lot'?'lots':'losses')+'/'+encodeURIComponent(row),'GET',undefined,signal),kind,reference);if(v.id!==row)fail();return v;},
  async declarationHistory(kind,row,signal){if(!['loss','lot'].includes(kind)||!id(row))fail();return productionDeclarationHistory(await request('production/'+(kind==='lot'?'lots':'losses')+'/'+encodeURIComponent(row)+'/history','GET',undefined,signal));},
  async quality(lot,signal){if(!id(lot?.id))fail();return productionQuality(await request('production/lots/'+encodeURIComponent(lot.id)+'/quality','GET',undefined,signal),lot);},
  async qualityHistory(lot,offset=0,signal){if(!id(lot?.id)||!int(offset,0,2147483647))fail();return productionQualityHistory(await request('production/lots/'+encodeURIComponent(lot.id)+'/quality/history?offset='+offset,'GET',undefined,signal),lot,offset);},
  async stageHistory(orderID,offset=0,signal){if(!id(orderID)||!int(offset,0,2147483647))fail();return productionStageHistory(await request('production/orders/'+encodeURIComponent(orderID)+'/stages/history?offset='+offset,'GET',undefined,signal),orderID);},
  async trace(orderID,signal){if(!id(orderID))fail();const v=await request('production/orders/'+encodeURIComponent(orderID)+'/trace','GET',undefined,signal),order=productionOrder(v?.order);if(order.id!==orderID)fail();return productionExecutionTrace(v,order);},
  async orders(offset=0,signal){if(!int(offset,0,2147483647))fail();const v=await request('production/orders?offset='+offset,'GET',undefined,signal),items=v?.items===null?[]:v?.items;if(!Array.isArray(items)||items.length>50||new Set(items.map(i=>i?.id)).size!==items.length)fail();return items.map(productionOrder);},
  async order(orderID,signal){if(!id(orderID))fail();const v=productionOrder(await request('production/orders/'+encodeURIComponent(orderID),'GET',undefined,signal));if(v.id!==orderID)fail();return v;},
  async history(orderID,offset=0,signal){if(!id(orderID)||!int(offset,0,2147483647))fail();const v=await request('production/orders/'+encodeURIComponent(orderID)+'/history?offset='+offset,'GET',undefined,signal),items=v?.items===null?[]:v?.items;if(!Array.isArray(items)||items.length>50||new Set(items.map(i=>i?.operation_id)).size!==items.length||!items.every(i=>i&&id(i.operation_id)&&id(i.actor_id)&&['created','state','completed'].includes(i.kind)&&['','planned','approved','cancelled','completed'].includes(i.before_status)&&['planned','approved','cancelled','completed'].includes(i.after_status)&&int(i.revision,1,2147483647)&&typeof i.reason==='string'&&typeof i.created_at==='string'&&Number.isFinite(Date.parse(i.created_at))))fail();return items;}
 };
}
