import {LocalAPIError} from '../local/localClient.mjs';
import {contextOf,contextKey} from '../context74/model.mjs';
import {createStockWorkspace} from '../stockWorkspace/source.mjs';
import {reviewedID,reviewedInput,reviewedCommand,reviewedResult,reviewedDetail,reviewedCanAction,reviewedList,reviewedHistory,reviewedOriginal} from './model.mjs';
const invalid=()=>{throw Error('reviewedInvalid');};
const canonical=v=>JSON.stringify(Array.isArray(v)?v.map(o=>JSON.parse(canonical(o))):v&&typeof v==='object'?Object.fromEntries(Object.keys(v).sort().map(k=>[k,JSON.parse(canonical(v[k]))])):v);
const knownCodes=['inventory_invalid','inventory_not_found','inventory_recount_required','inventory_below_reserved','inventory_conflict'];
export function createReviewedInventory({token,session,capabilities,isCurrent,storage,locks,fetcher=globalThis.fetch,directory}){
 const ctx=contextOf(session,capabilities),scope=contextKey(ctx),key='titan-reviewed-inventory:v1:'+scope;
 if(typeof token!=='string'||!token||/[\r\n]/.test(token)||typeof isCurrent!=='function')invalid();
 const writable=()=>capabilities.permissions?.includes('manage_stock')&&capabilities.license?.state==='active'&&['core','inventory'].every(m=>capabilities.license.modules.includes(m));
 const reviewer=()=>['owner','manager'].includes(capabilities.role);
 function authorize(write=false){let current=false;try{current=isCurrent()&&contextKey(contextOf(session,capabilities))===scope;}catch{}if(!current)throw new LocalAPIError(401,'reviewed');if(!capabilities.permissions?.includes('manage_stock')||write&&!writable())throw new LocalAPIError(403,'reviewed');}
 authorize();const catalog=directory||createStockWorkspace({token,session,capabilities,isCurrent,storage,locks,fetcher});
 const id=v=>{if(!reviewedID(v))invalid();return encodeURIComponent(v);};
 async function request(path,method='GET',body,signal,csv=false){
  authorize(method==='POST');const controller=new AbortController(),abort=()=>controller.abort();if(signal?.aborted)abort();signal?.addEventListener('abort',abort,{once:true});const timer=setTimeout(abort,15000);
  try{controller.signal.throwIfAborted();const response=await fetcher('/local/v1/'+path,{method,credentials:'omit',cache:'no-store',signal:controller.signal,headers:{Authorization:'Bearer '+token,...(body?{'Content-Type':'application/json'}:{})},...(body?{body:JSON.stringify(body)}:{})});authorize();controller.signal.throwIfAborted();const raw=await response.text();authorize();controller.signal.throwIfAborted();if(raw.length>2000000)invalid();if(!response.ok){let code='reviewed';try{const v=JSON.parse(raw);if(knownCodes.includes(v?.code))code=v.code;}catch{}throw new LocalAPIError(response.status,code);}if(csv){if(!response.headers?.get('content-type')?.toLowerCase().startsWith('text/csv')||raw.replace(/^\uFEFF/,'').split(/\r?\n/,1)[0]!=='inventory_id,status,revision,location_id,product_id,unit,counted_milli,previous_milli,difference_milli,counted_by,counted_at,movement_id')invalid();return raw;}try{return JSON.parse(raw);}catch{invalid();}}finally{clearTimeout(timer);signal?.removeEventListener('abort',abort);}
 }
 function pending(){authorize();try{const raw=storage.getItem(key);if(raw===null)return null;if(raw.length>100000)invalid();return reviewedCommand(JSON.parse(raw),scope);}catch{throw Error('reviewedStorage');}}
 function save(command){authorize();const raw=JSON.stringify(reviewedCommand(command,scope));try{storage.setItem(key,raw);if(storage.getItem(key)!==raw)throw Error();}catch{throw Error('reviewedStorage');}}
 function clear(command){authorize();const current=pending();if(!current||canonical(current)!==canonical(command))throw Error('reviewedStorage');try{storage.removeItem(key);if(storage.getItem(key)!==null)throw Error();}catch{throw Error('reviewedStorage');}}
 async function locked(fn){if(!locks?.request)throw Error('reviewedLocks');return locks.request(key,{ifAvailable:true},async lock=>{authorize();if(!lock)throw Error('reviewedLocked');return fn();});}
 const detail=async(inventoryID,signal)=>reviewedDetail(await request('stock/inventories/'+id(inventoryID),'GET',undefined,signal),inventoryID);
 async function history(inventoryID,offset=0,signal,limit=50){id(inventoryID);if(!Number.isSafeInteger(offset)||offset<0||offset>1000000||!Number.isInteger(limit)||limit<1||limit>100)invalid();return reviewedHistory(await request('stock/inventories/'+id(inventoryID)+'/history?'+new URLSearchParams({limit:String(limit),offset:String(offset)}),'GET',undefined,signal),inventoryID,offset,limit);}
 async function receipt(command){const r=reviewedResult(await request('stock/inventory-operations/'+id(command.body.operation_id)),command);if(r.revision-1>1000000)throw Error('reviewedHistoryLimit');let h;try{h=await history(command.inventory_id,r.revision-1,undefined,1);}catch(error){if(error instanceof LocalAPIError&&error.status===404)throw Error('reviewedHistoryUnavailable');throw error;}if(h.items.length!==1)invalid();return reviewedOriginal(r,h.items[0],command,ctx.identity_id,ctx.device_id);}
 async function preflight(command){authorize(true);if(command.kind==='open')return;const d=await detail(command.inventory_id);authorize(true);const action=command.kind==='count'?'count':command.body.action;if(d.inventory.revision!==command.body.expected_revision||!reviewedCanAction(d,ctx.identity_id,reviewer(),action))throw new LocalAPIError(409,'inventory_conflict');if(command.kind==='count')for(const row of command.body.items){const line=d.lines.find(l=>l.product_id===row.product_id);if(!line||line.unit==='unit'&&row.counted_milli%1000!==0)invalid();}}
 async function resolve(send,discard){
  let command=pending();if(!command)return null;authorize(send||discard);let result;
  try{result=await receipt(command);}catch(error){if(!(error instanceof LocalAPIError)||error.status!==404)throw error;}
  if(result){clear(command);return {state:'confirmed',receipt:result};}
  if(discard){if(command.uncertain)throw Error('reviewedUncertain');clear(command);return {state:'discarded'};}
  if(!send)return {state:'missing'};
  await preflight(command);authorize(true);const prior=command.uncertain;command={...command,uncertain:true};save(command);
  try{const path=command.kind==='open'?'stock/inventories':'stock/inventories/'+id(command.inventory_id)+(command.kind==='count'?'/counts':'/state');reviewedResult(await request(path,'POST',command.body),command,false);}catch(error){if(!prior&&error instanceof LocalAPIError&&[400,403,404,409,413,422].includes(error.status)){authorize();save({...command,uncertain:false});}throw error;}
  result=await receipt(command);clear(command);return {state:'confirmed',receipt:result};
 }
 return {scope,actor:ctx.identity_id,device:ctx.device_id,canLock:!!locks?.request,get canWrite(){return !!writable();},get canReview(){return reviewer();},assertCurrent(){authorize();},pending,detail,history,
  prepare(kind,inventoryID,body){authorize(true);const command=reviewedCommand({version:1,scope,kind,inventory_id:inventoryID,body:reviewedInput(kind,inventoryID,body),uncertain:false},scope);if(kind==='state'&&['approve','reject'].includes(command.body.action)&&!reviewer())throw new LocalAPIError(403,'reviewed');return command;},
  async submit(value){authorize(true);const command=reviewedCommand(structuredClone(value),scope);if(command.uncertain)invalid();return locked(async()=>{authorize(true);if(pending())throw Error('reviewedPending');save(command);return resolve(true,false);});},
  async recover(send=false,discard=false){if(typeof send!=='boolean'||typeof discard!=='boolean'||send&&discard)invalid();return locked(()=>resolve(send,discard));},
  async list(status='',offset=0,signal){if(!Number.isSafeInteger(offset)||offset<0||offset>1000000)invalid();if(status!==''&&!['open','submitted','approved','rejected','cancelled'].includes(status))invalid();return reviewedList(await request('stock/inventories?'+new URLSearchParams({limit:'50',offset:String(offset),...(status?{status}:{})}),'GET',undefined,signal),offset,status);},
  async products(query='',offset=0,signal){authorize();const page=await catalog.products(query,offset,signal);authorize();if(!page.items.every(p=>reviewedID(p.id)))invalid();return page;},
  async locations(signal){authorize();const locations=await catalog.locations(signal);authorize();if(!locations.every(p=>reviewedID(p.id)))invalid();return locations;},
  async exportCSV(inventoryID,signal){return request('stock/inventories/'+id(inventoryID)+'/report.csv','GET',undefined,signal,true);}
 };
}
