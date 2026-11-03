import {contextOf,contextKey} from '../context74/model.mjs';
import {LocalAPIError} from '../local/localClient.mjs';
const invalid=()=>{throw Error('productionInvalid');};
const id=v=>typeof v==='string'&&v.trim()===v&&v.length>0&&new TextEncoder().encode(v).length<=128&&!/[\x00\r\n]/.test(v);
const integer=(v,min=0,max=Number.MAX_SAFE_INTEGER)=>Number.isSafeInteger(v)&&v>=min&&v<=max;
const units=['unit','kg','g','liter','ml','meter'];
const unique=(rows,key)=>new Set(rows.map(r=>r[key])).size===rows.length;
export function recipeVersion(v){
 if(!v||!['operation_id','recipe_id','version_id','output_product_id','created_by'].every(k=>id(v[k]))||typeof v.name!=='string'||!v.name.trim()||new TextEncoder().encode(v.name).length>255||!integer(v.expected_revision,0,2147483646)||!integer(v.revision,1,2147483647)||v.revision!==v.expected_revision+1||!units.includes(v.output_unit)||!integer(v.yield_milli,1)||typeof v.created_at!=='string'||!Number.isFinite(Date.parse(v.created_at))||!Array.isArray(v.ingredients)||v.ingredients.length<1||v.ingredients.length>100||!unique(v.ingredients,'product_id')||!v.ingredients.every(i=>i&&id(i.product_id)&&units.includes(i.unit)&&integer(i.quantity_milli,1)))invalid();
 return v;
}
export function capacityResult(v,version,location){
 recipeVersion(version);
 if(!v||v.location_id!==location||typeof v.measured_at!=='string'||!Number.isFinite(Date.parse(v.measured_at))||v.basis!=='local_available_balance_after_reservations'||v.alternatives_independent!==true||v.simultaneous_total_available!==false||!Array.isArray(v.alternatives)||v.alternatives.length!==1)invalid();
 const a=v.alternatives[0];
 if(!a||a.recipe_id!==version.recipe_id||a.version_id!==version.version_id||a.revision!==version.revision||a.output_product_id!==version.output_product_id||a.output_unit!==version.output_unit||a.yield_per_batch_milli!==version.yield_milli||!integer(a.possible_batches)||!integer(a.possible_output_milli)||BigInt(a.possible_output_milli)!==BigInt(a.possible_batches)*BigInt(version.yield_milli)||!Array.isArray(a.materials)||a.materials.length!==version.ingredients.length||!unique(a.materials,'product_id')||!Array.isArray(a.limiting_product_ids))invalid();
 for(const m of a.materials){
  const line=version.ingredients.find(i=>i.product_id===m?.product_id);
  // Current backend refuses historic unit divergence. Never infer density or packaging.
  if(!line||m.recipe_unit!==line.unit||m.stock_unit!==line.unit||m.required_milli!==line.quantity_milli||!integer(m.stock_milli)||m.conversion_numerator!==1||m.conversion_denominator!==1||!integer(m.possible_batches)||BigInt(m.possible_batches)!==BigInt(m.stock_milli)/BigInt(line.quantity_milli)||typeof m.limiting!=='boolean'||m.limiting!==(m.possible_batches===a.possible_batches))invalid();
 }
 const minimum=Math.min(...a.materials.map(m=>m.possible_batches)),limiting=a.materials.filter(m=>m.limiting).map(m=>m.product_id);
 if(a.possible_batches!==minimum||new Set(a.limiting_product_ids).size!==a.limiting_product_ids.length||limiting.length!==a.limiting_product_ids.length||!limiting.every(p=>a.limiting_product_ids.includes(p)))invalid();
 return v;
}
export function createProductionRead({token,session,capabilities,isCurrent,fetcher=globalThis.fetch}){
 const scope=contextKey(contextOf(session,capabilities));
 if(typeof token!=='string'||!token||/[\r\n]/.test(token)||typeof isCurrent!=='function')invalid();
 function authorize(){let current=false;try{current=isCurrent()&&contextKey(contextOf(session,capabilities))===scope;}catch{}if(!current)throw new LocalAPIError(401,'production');if(!capabilities.permissions?.includes('manage_production'))throw new LocalAPIError(403,'production');}
 authorize();
 async function get(path,signal){
  authorize();const controller=new AbortController(),abort=()=>controller.abort();
  if(signal?.aborted)abort();signal?.addEventListener('abort',abort,{once:true});const timer=setTimeout(abort,15000);
  try{controller.signal.throwIfAborted();const r=await fetcher('/local/v1/'+path,{method:'GET',credentials:'omit',cache:'no-store',signal:controller.signal,headers:{Authorization:'Bearer '+token}});authorize();controller.signal.throwIfAborted();if(!r.ok)throw new LocalAPIError(r.status,'production');const text=await r.text();authorize();controller.signal.throwIfAborted();if(text.length>2000000)invalid();try{return JSON.parse(text);}catch{invalid();}}finally{clearTimeout(timer);signal?.removeEventListener('abort',abort);}
 }
 return {scope,
  async versions(recipe='',offset=0,signal){if((recipe!==''&&!id(recipe))||!integer(offset,0,2147483647))invalid();const q=new URLSearchParams({limit:'50',offset:String(offset)});if(recipe)q.set('recipe_id',recipe);const v=await get('production/recipe-versions?'+q,signal);const items=v?.items===null?[]:v?.items;if(!Array.isArray(items)||items.length>50||!unique(items,'version_id'))invalid();for(const row of items){recipeVersion(row);if(recipe&&row.recipe_id!==recipe)invalid();}return items;},
  async version(versionID,signal){if(!id(versionID))invalid();const v=recipeVersion(await get('production/recipe-versions/'+encodeURIComponent(versionID),signal));if(v.version_id!==versionID)invalid();return v;},
  async locations(signal){const v=await get('locations',signal),items=v?.items===null?[]:v?.items;if(!Array.isArray(items)||items.length>1000||!items.every(i=>i&&id(i.ID)&&typeof i.name==='string'&&i.name.trim())||!unique(items,'ID'))invalid();return items.map(i=>({id:i.ID,name:i.name}));},
  async capacity(version,location,signal){recipeVersion(version);if(!id(location))invalid();const q=new URLSearchParams({version_id:version.version_id,location_id:location});return capacityResult(await get('production/capacity?'+q,signal),version,location);}
 };
}
