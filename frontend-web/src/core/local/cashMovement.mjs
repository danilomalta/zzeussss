import {LocalAPIError} from './localClient.mjs';
import {contextOf,contextKey} from '../context74/model.mjs';
const fail=()=>{throw Error('cashMovementInvalid');};
const id=v=>typeof v==='string'&&v.trim()===v&&v.length>0&&new TextEncoder().encode(v).length<=128&&!/[\x00-\x1f\x7f]/.test(v);
const equal=(a,b)=>JSON.stringify(a)===JSON.stringify(b);
function body(v){if(!v||Object.keys(v).length!==5||!id(v.session_id)||!id(v.operation_id)||!['withdrawal','supply'].includes(v.kind)||!Number.isSafeInteger(v.amount_cents)||v.amount_cents<=0||typeof v.reason!=='string'||!v.reason.trim()||v.reason.trim()!==v.reason||[...v.reason].length>500||/[\x00-\x1f\x7f]/.test(v.reason))fail();return v;}
function receipt(v,b){if(!v||Object.keys(v).length!==8||!Object.keys(b).every(k=>v[k]===b[k])||!id(v.movement_id)||v.repeated!==false||typeof v.created_at!=='string'||!Number.isFinite(Date.parse(v.created_at)))fail();return v;}
export function createCashMovement({token,session,capabilities,isCurrent,storage,locks,fetcher=globalThis.fetch}){
 const scope=contextKey(contextOf(session,capabilities)),key='titan-cash-movement:v1:'+scope,legacy='titansystem-pos-v1:'+JSON.stringify([session.tenant_id,session.store_id,session.device_id,session.identity_id]);let disposed=false;const controllers=new Set();
 function current(write=false){let ok=false;try{ok=!disposed&&typeof token==='string'&&!!token&&!/[\r\n]/.test(token)&&isCurrent()&&contextKey(contextOf(session,capabilities))===scope;}catch{}if(!ok)throw new LocalAPIError(401,'cash-movement');if(!capabilities.permissions?.includes('view_accounting')||!capabilities.permissions.includes('manage_cash')||write&&(capabilities.license?.state!=='active'||!['core','inventory','pos'].every(m=>capabilities.license.modules.includes(m))))throw new LocalAPIError(403,'cash-movement');}
 function command(c){if(!c||Object.keys(c).length!==4||c.version!==1||c.scope!==scope||typeof c.uncertain!=='boolean')fail();body(c.body);return c;}
 function pending(){current();try{const raw=storage.getItem(key);if(raw===null)return null;if(raw.length>16000)fail();return command(JSON.parse(raw));}catch{throw Error('cashMovementStorage');}}
 function save(c){current();command(c);try{const raw=JSON.stringify(c);storage.setItem(key,raw);if(storage.getItem(key)!==raw)throw Error();}catch{throw Error('cashMovementStorage');}}
 function clear(c){current();if(!equal(c,pending()))fail();storage.removeItem(key);if(storage.getItem(key)!==null)throw Error('cashMovementStorage');}
 async function request(path,b){current(!!b);const c=new AbortController();controllers.add(c);const timer=setTimeout(()=>c.abort(),15000);try{const r=await fetcher('/local/v1/cash/'+path,{method:b?'POST':'GET',headers:{Authorization:'Bearer '+token,...(b?{'Content-Type':'application/json'}:{})},credentials:'omit',cache:'no-store',signal:c.signal,...(b?{body:JSON.stringify(b)}:{})});current();c.signal.throwIfAborted();if(!r.ok){const e=new LocalAPIError(r.status,'cash-movement');e.serverRefusal=true;throw e;}const raw=await r.text();current();c.signal.throwIfAborted();if(raw.length>16000)fail();return JSON.parse(raw);}finally{clearTimeout(timer);controllers.delete(c);}}
 async function resolve(send,discard){let c=pending();if(!c)return null;try{const r=receipt(await request('movements/operations/'+encodeURIComponent(c.body.operation_id)),c.body);clear(c);return r;}catch(e){if(!(e instanceof LocalAPIError&&e.status===404))throw e;}
  if(discard){if(c.uncertain)throw Error('cashMovementUncertain');clear(c);return null;}if(!send)return null;current(true);
  // Shared POS lock and explicit check of other original pending operations.
  if(storage.getItem(legacy)!==null||storage.getItem('titan-pos-carts:v1:'+scope)!==null)throw Error('cashMovementPending');
  const wasUncertain=c.uncertain;c={...c,uncertain:true};save(c);current(true);
  try{await request('movements',c.body);}catch(e){if(!wasUncertain&&e instanceof LocalAPIError&&e.serverRefusal&&[400,403,404,409,413,422].includes(e.status))save({...c,uncertain:false});throw e;}
  const r=receipt(await request('movements/operations/'+encodeURIComponent(c.body.operation_id)),c.body);clear(c);return r;
 }
 async function locked(fn){current();if(!locks?.request)throw Error('cashMovementLocks');return locks.request(legacy,{mode:'exclusive',ifAvailable:true},async lock=>{current();if(!lock)throw Error('cashMovementLocked');return fn();});}
 current();return {key,pending,assertCurrent:()=>current(),get canWrite(){try{current(true);return !!locks?.request;}catch{return false;}},prepare(input){current(true);return command({version:1,scope,body:structuredClone(body(input)),uncertain:false});},submit(c){c=command(structuredClone(c));if(c.uncertain)fail();return locked(async()=>{current(true);if(pending())throw Error('cashMovementPending');save(c);return resolve(true,false);});},recover(send=false,discard=false){if(typeof send!=='boolean'||typeof discard!=='boolean'||send&&discard)fail();return locked(()=>resolve(send,discard));},dispose(){disposed=true;for(const c of controllers)c.abort();}};
}
