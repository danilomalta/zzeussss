import {contextOf,contextKey} from '../context74/model.mjs';
import {LocalAPIError} from '../local/localClient.mjs';
import {bagFail,keys,pathID,text,equal,bagRecord,bagPage,bagCoverage,bagBody,allowedBagActions,states} from './model.mjs';
export function createCashBags({token,session,capabilities,isCurrent,storage,locks,fetcher=globalThis.fetch}){
 const scope=contextKey(contextOf(session,capabilities)),key='titan-cash-bags:v1:'+scope,actor=session.identity_id,listeners=new Set(),controllers=new Set(),epochs=new Map();let disposed=false;
 function current(write=false){let valid=false;try{valid=!disposed&&typeof token==='string'&&!!token&&!/[\r\n]/.test(token)&&isCurrent()&&contextKey(contextOf(session,capabilities))===scope;}catch{}if(!valid)throw new LocalAPIError(401,'cash-bags');if(!capabilities.permissions?.includes('manage_cash')||write&&(capabilities.license?.state!=='active'||!['core','inventory','pos'].every(m=>capabilities.license.modules.includes(m))))throw new LocalAPIError(403,'cash-bags');}
 current();
 function notify(){for(const fn of listeners){try{fn();}catch{}}}
 function validate(c){
  if(!keys(c,['version','scope','action','id','body','before','uncertain'])||c.version!==1||c.scope!==scope||typeof c.uncertain!=='boolean'||!pathID(c.id))bagFail();bagBody(c.action,c.body);
  if(c.action==='create'){if(c.before!==null||c.id!==c.body.bag_id)bagFail();}
  else{bagRecord(c.before,actor);if(c.before.id!==c.id||!allowedBagActions(c.before,actor).includes(c.action))bagFail();}return c;
 }
 function pending(){current();let raw;try{raw=storage.getItem(key);if(raw===null)return null;if(raw.length>30000)throw Error();return validate(JSON.parse(raw));}catch{throw Error('cashBagStorage');}}
 function save(c){current();validate(c);try{const raw=JSON.stringify(c);storage.setItem(key,raw);if(storage.getItem(key)!==raw)throw Error();}catch{throw Error('cashBagStorage');}notify();}
 function clear(c){current();if(!equal(pending(),c))throw Error('cashBagPending');try{storage.removeItem(key);if(storage.getItem(key)!==null)throw Error();}catch{throw Error('cashBagStorage');}notify();}
 async function locked(fn){current();if(!locks?.request)throw Error('cashBagLocks');return locks.request(key,{mode:'exclusive',ifAvailable:true},async lock=>{if(!lock)throw Error('cashBagLocked');current();return fn();});}
 async function request(path,body){current(!!body);const ctrl=new AbortController();controllers.add(ctrl);const timer=setTimeout(()=>ctrl.abort(),15000);try{const r=await fetcher('/local/v1/'+path,{method:body?'POST':'GET',headers:{Authorization:'Bearer '+token,...(body?{'Content-Type':'application/json'}:{})},credentials:'omit',cache:'no-store',signal:ctrl.signal,...(body?{body:JSON.stringify(body)}:{})});current();ctrl.signal.throwIfAborted();if(!r.ok){const error=new LocalAPIError(r.status,'cash-bags');error.serverRefusal=true;throw error;}const raw=await r.text();current();ctrl.signal.throwIfAborted();if(raw.length>2000000)bagFail();let v;try{v=JSON.parse(raw);}catch{bagFail();}return v;}finally{clearTimeout(timer);controllers.delete(ctrl);}}
 function receipt(r,c){
  if(!keys(r,['operation_id','repeated','bag'])||r.operation_id!==c.body.operation_id||typeof r.repeated!=='boolean')bagFail();const b=bagRecord(r.bag,actor),event=b.history.at(-1);
  if(b.id!==c.id||event.operation_id!==c.body.operation_id||event.actor_id!==actor||event.action!==c.action||event.reason!==(c.body.reason||''))bagFail();
  if(c.action==='create'){if(b.withdrawal_operation_id!==c.body.withdrawal_operation_id||b.seal_id!==c.body.seal_id||b.sender_id!==actor||b.history.length!==1)bagFail();}
  else{const before=c.before;if(!['id','session_id','withdrawal_operation_id','movement_id','sender_id','operator_id','seal_id','created_at'].every(k=>b[k]===before[k])||b.history.length!==before.history.length+1||!equal(b.history.slice(0,-1),before.history))bagFail();if(c.action==='receive'&&(b.seal_id!==c.body.seal_id||b.counted_cents!==c.body.counted_cents))bagFail();}return r;
 }
 async function resolve(send=false,discard=false){let c=pending();if(!c)return null;let result;try{result=receipt(await request('cash/bags/operations/'+c.body.operation_id),c);}catch(e){if(!(e instanceof LocalAPIError&&e.status===404))throw e;}if(result){clear(c);return {state:'confirmed',result};}if(discard){if(c.uncertain)throw Error('cashBagUncertain');clear(c);return {state:'discarded'};}if(!send)return {state:'missing'};
  current(true);if(!c.uncertain&&c.action!=='create'){const fresh=bagRecord(await request('cash/bags/'+c.id),actor);if(!equal(fresh,c.before))throw Error('cashBagChanged');}
  const wasUncertain=c.uncertain;c={...c,uncertain:true};save(c);
  try{receipt(await request('cash/bags'+(c.action==='create'?'':'/'+c.id+'/'+c.action),c.body),c);}catch(e){if(!wasUncertain&&e instanceof LocalAPIError&&e.serverRefusal&&[400,403,404,409,413,422].includes(e.status))save({...c,uncertain:false});throw e;}
  result=receipt(await request('cash/bags/operations/'+c.body.operation_id),c);clear(c);return {state:'confirmed',result};
 }
 return {scope,key,actor,assertCurrent:()=>current(),get canWrite(){try{current(true);return !!locks?.request;}catch{return false;}},subscribe(fn){listeners.add(fn);return()=>listeners.delete(fn);},dispose(){disposed=true;for(const c of controllers)c.abort();listeners.clear();},pending,
  prepare(action,id,body,before=null){current(true);return validate(structuredClone({version:1,scope,action,id,body,before,uncertain:false}));},
  submit(c){c=validate(structuredClone(c));current(true);if(c.uncertain)bagFail();return locked(async()=>{if(pending())throw Error('cashBagPending');save(c);return resolve(true);});},
  recover(send=false,discard=false){if(typeof send!=='boolean'||typeof discard!=='boolean'||send&&discard)bagFail();return locked(()=>resolve(send,discard));},
  async read(id){current();if(!pathID(id))bagFail();const n=(epochs.get('bag')||0)+1;epochs.set('bag',n);const b=bagRecord(await request('cash/bags/'+id),actor);if(epochs.get('bag')!==n)throw Error('cashBagSuperseded');if(b.id!==id)bagFail();return b;},
  async list(filters={}){current();if(!keys(filters,['state','session_id','offset'])||filters.state!==''&&!states.includes(filters.state)||filters.session_id!==''&&!text(filters.session_id)||!Number.isSafeInteger(filters.offset)||filters.offset<0||filters.offset>1000000000)bagFail();const n=(epochs.get('list')||0)+1;epochs.set('list',n);const q=new URLSearchParams({limit:'50',offset:String(filters.offset)});for(const k of ['state','session_id'])if(filters[k])q.set(k,filters[k]);const v=bagPage(await request('cash/bags?'+q),actor,filters);if(epochs.get('list')!==n)throw Error('cashBagSuperseded');return v;},
  async coverage(id){current();if(!text(id))bagFail();const n=(epochs.get('coverage')||0)+1;epochs.set('coverage',n);const v=bagCoverage(await request('cash/sessions/'+encodeURIComponent(id)+'/custody'),id);if(epochs.get('coverage')!==n)throw Error('cashBagSuperseded');return v;}
 };
}
