import {contextOf,contextKey} from '../context74/model.mjs';
import {LocalAPIError} from '../local/localClient.mjs';
import {bagRecord} from '../cashBags/model.mjs';
import {fail,keys,pathID,equal,command,receipt,vaultRecord,countRecord,paged,statement,report} from './model.mjs';
import {vaultCSV} from './export.mjs';
export function createCashVaults({token,session,capabilities,isCurrent,storage,locks,fetcher=globalThis.fetch}){
 const scope=contextKey(contextOf(session,capabilities)),key='titan-cash-vaults:v1:'+scope,actor=session.identity_id,controllers=new Set(),epochs=new Map(),listeners=new Set();let disposed=false;
 function current(write=false){let valid=false;try{valid=!disposed&&typeof token==='string'&&!!token&&!/[\r\n]/.test(token)&&isCurrent()&&contextKey(contextOf(session,capabilities))===scope;}catch{}if(!valid)throw new LocalAPIError(401,'cash-vaults');if(!capabilities.permissions?.includes('manage_cash')||write&&(capabilities.license?.state!=='active'||!['core','inventory','pos'].every(m=>capabilities.license.modules.includes(m))))throw new LocalAPIError(403,'cash-vaults');}
 current();
 function notify(){for(const fn of listeners){try{fn();}catch{}}}
 function pending(){current();try{const raw=storage.getItem(key);if(raw===null)return null;if(raw.length>50000)throw Error();return command(JSON.parse(raw),scope,actor);}catch{throw Error('cashVaultStorage');}}
 function save(c){current();command(c,scope,actor);try{const raw=JSON.stringify(c);storage.setItem(key,raw);if(storage.getItem(key)!==raw)throw Error();}catch{throw Error('cashVaultStorage');}notify();}
 function clear(c){current();if(!equal(pending(),c))throw Error('cashVaultPending');try{storage.removeItem(key);if(storage.getItem(key)!==null)throw Error();}catch{throw Error('cashVaultStorage');}notify();}
 async function locked(fn){current();if(!locks?.request)throw Error('cashVaultLocks');return locks.request(key,{mode:'exclusive',ifAvailable:true},async lock=>{if(!lock)throw Error('cashVaultLocked');current();return fn();});}
 async function request(path,body,csv=false){current(!!body);const ctrl=new AbortController();controllers.add(ctrl);const timer=setTimeout(()=>ctrl.abort(),15000);try{const r=await fetcher('/local/v1/cash/'+path,{method:body?'POST':'GET',headers:{Authorization:'Bearer '+token,...(body?{'Content-Type':'application/json'}:{})},credentials:'omit',cache:'no-store',signal:ctrl.signal,...(body?{body:JSON.stringify(body)}:{})});current();ctrl.signal.throwIfAborted();if(!r.ok){const e=new LocalAPIError(r.status,'cash-vaults');e.serverRefusal=true;throw e;}if(csv){if(!/^text\/csv\s*;\s*charset=utf-8$/i.test(r.headers.get('Content-Type')||'')||r.headers.get('Content-Disposition')!=='attachment; filename="titan-cofres.csv"'||r.headers.get('X-Content-Type-Options')!=='nosniff'||!r.headers.get('Cache-Control')?.includes('no-store'))fail();const data=new Uint8Array(await r.arrayBuffer());current();ctrl.signal.throwIfAborted();vaultCSV(data);return {data,name:'titan-cofres.csv'};}const raw=await r.text();current();ctrl.signal.throwIfAborted();if(raw.length>8000000)fail();try{return JSON.parse(raw);}catch{fail();}}finally{clearTimeout(timer);controllers.delete(ctrl);}}
 async function query(channel,path,validate,csv=false){current();const epoch=(epochs.get(channel)||0)+1;epochs.set(channel,epoch);const value=validate(await request(path,undefined,csv));current();if(epochs.get(channel)!==epoch)throw Error('cashVaultSuperseded');return value;}
 async function fresh(c){
  if(c.resource==='vault'&&c.action!=='create'){
   if(!equal(vaultRecord(await request('vaults/'+c.id)),c.before.source))throw Error('cashVaultChanged');
   if(c.action==='transfer'&&!equal(vaultRecord(await request('vaults/'+c.body.destination_id)),c.before.destination))throw Error('cashVaultChanged');
   if(c.action==='deposit'&&!equal(bagRecord(await request('bags/'+c.body.bag_id),actor),c.before.bag))throw Error('cashVaultChanged');
  }else if(c.resource==='count'){
   const value=c.action==='create'?vaultRecord(await request('vaults/'+c.body.vault_id)):countRecord(await request('vault-counts/'+c.id));if(!equal(value,c.before))throw Error('cashVaultChanged');
  }
 }
 async function resolve(send=false,discard=false){let c=pending();if(!c)return null;const group=c.resource==='count'?'vault-counts':'vaults';let result;try{result=receipt(await request(group+'/operations/'+c.body.operation_id),c,actor);}catch(e){if(!(e instanceof LocalAPIError&&e.status===404))throw e;}if(result){clear(c);return {state:'confirmed',result};}if(discard){if(c.uncertain)throw Error('cashVaultUncertain');clear(c);return {state:'discarded'};}if(!send)return {state:'missing'};
  current(true);if(!c.uncertain)await fresh(c);const wasUncertain=c.uncertain;c={...c,uncertain:true};save(c);
  try{receipt(await request(group+(c.action==='create'?'':'/'+c.id+'/'+c.action),c.body),c,actor);}catch(e){if(!wasUncertain&&e instanceof LocalAPIError&&e.serverRefusal&&[400,403,404,409,413,422].includes(e.status))save({...c,uncertain:false});throw e;}
  result=receipt(await request(group+'/operations/'+c.body.operation_id),c,actor);clear(c);return {state:'confirmed',result};
 }
 return {scope,key,actor,assertCurrent:()=>current(),get canWrite(){try{current(true);return !!locks?.request;}catch{return false;}},subscribe(fn){listeners.add(fn);return()=>listeners.delete(fn);},dispose(){disposed=true;for(const c of controllers)c.abort();listeners.clear();},pending,
  prepare(resource,action,id,body,before=null){current(true);return command(structuredClone({version:1,scope,resource,action,id,body,before,uncertain:false}),scope,actor);},
  submit(c){c=command(structuredClone(c),scope,actor);current(true);if(c.uncertain)fail();return locked(async()=>{if(pending())throw Error('cashVaultPending');save(c);return resolve(true);});},
  recover(send=false,discard=false){if(typeof send!=='boolean'||typeof discard!=='boolean'||send&&discard)fail();return locked(()=>resolve(send,discard));},
  readBag(id){if(!pathID(id))fail();return query('bag','bags/'+id,v=>{bagRecord(v,actor);if(v.id!==id||v.state!=='received'||v.receiver_id!==actor)fail();return v;});},
  readVault(id){if(!pathID(id))fail();return query('vault','vaults/'+id,v=>{vaultRecord(v);if(v.id!==id)fail();return v;});},
  readCount(id){if(!pathID(id))fail();return query('count','vault-counts/'+id,v=>{countRecord(v);if(v.id!==id)fail();return v;});},
  list(resource,offset=0){if(!['vaults','counts','bags'].includes(resource)||!Number.isSafeInteger(offset)||offset<0||offset>1000000000)fail();const path={vaults:'vaults',counts:'vault-counts',bags:'vaults/pending-bags'}[resource];return query(resource,path+'?limit=50&offset='+offset,v=>paged(v,offset,item=>{if(resource==='vaults')vaultRecord(item);else if(resource==='counts')countRecord(item);else{bagRecord(item,actor);if(item.state!=='received'||item.receiver_id!==actor)fail();}}));},
  statement(id,offset=0){if(!pathID(id)||!Number.isSafeInteger(offset)||offset<0||offset>1000000000)fail();return query('statement','vaults/'+id+'/statement?limit=50&offset='+offset,v=>statement(v,id,offset));},
  report(){return query('report','vaults/report',report);},exportCSV(){return query('export','vaults/report.csv',v=>v,true);}
 };
}
