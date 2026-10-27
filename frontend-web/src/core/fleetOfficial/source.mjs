import {LocalAPIError} from '../local/localClient.mjs';
import {contextOf,contextKey} from '../context74/model.mjs';
import {fleetResources,financialResources,fleetID,fleetFail,fleetPeriod,fleetPage,fleetRecord,fleetHistory,fleetReport,fleetPreventive,fleetNotices} from './model.mjs';
// Deliberately GET-only: no draft is sent, no approval or notice ACK is inferred.
export function createFleetRead({token,session,capabilities,isCurrent,fetcher=globalThis.fetch}){
 const scope=contextKey(contextOf(session,capabilities)),running=new Map();let disposed=false;
 function authorize(financial=false){
  let current=false;try{current=!disposed&&typeof token==='string'&&!!token&&!/[\r\n]/.test(token)&&isCurrent()&&contextKey(contextOf(session,capabilities))===scope;}catch{}
  if(!current)throw new LocalAPIError(401,'fleetRead');
  if(!capabilities.permissions?.includes('view_orders')||financial&&!capabilities.permissions?.includes('view_accounting'))throw new LocalAPIError(403,'fleetRead');
 }
 authorize();
 function nameCheck(name){if(!Object.hasOwn(fleetResources,name))fleetFail();authorize(financialResources.includes(name));}
 function query(filters={},offset=0,ws=false){
  if(!Number.isSafeInteger(offset)||offset<0||offset>1000000||!filters||typeof filters!=='object'||Array.isArray(filters))fleetFail();
  const q=new URLSearchParams({limit:'50',offset:String(offset)}),allowed=ws?['request_id','state']:['asset_id','state','assigned_to'];
  for(const [k,v] of Object.entries(filters)){if(!allowed.includes(k)||typeof v!=='string')fleetFail();if(v){if(k==='state'?v.trim()!==v||new TextEncoder().encode(v).length>40||/[\u0000-\u001f\u007f]/.test(v):!fleetID(v))fleetFail();q.set(k,v);}}
  return q;
 }
 async function get(path,channel,financial,signal){
  authorize(financial);running.get(channel)?.abort();const ctrl=new AbortController();running.set(channel,ctrl);
  const abort=()=>ctrl.abort();if(signal?.aborted)abort();signal?.addEventListener('abort',abort,{once:true});const timer=setTimeout(abort,15000);
  function current(){authorize(financial);ctrl.signal.throwIfAborted();if(running.get(channel)!==ctrl)throw Error('fleetSuperseded');}
  try{
   current();const response=await fetcher('/local/v1/fleet/'+path,{method:'GET',credentials:'omit',cache:'no-store',headers:{Authorization:'Bearer '+token},signal:ctrl.signal});current();
   const text=await response.text();current();if(text.length>2000000)fleetFail();if(!response.ok)throw new LocalAPIError(response.status,'fleetRead');
   try{return JSON.parse(text);}catch{fleetFail();}
  }finally{clearTimeout(timer);signal?.removeEventListener('abort',abort);if(running.get(channel)===ctrl)running.delete(channel);}
 }
 return {scope,assertCurrent:()=>authorize(),dispose(){disposed=true;for(const c of running.values())c.abort();running.clear();},
  async notices(after=0,signal){if(!Number.isSafeInteger(after)||after<0)fleetFail();return fleetNotices(await get('notices?'+new URLSearchParams({after:String(after),limit:'50'}),'notices',false,signal),after);},
  async preventive(id,offset=0,signal){if(!fleetID(id))fleetFail();return fleetPreventive(id,await get('assets/'+encodeURIComponent(id)+'/preventive?'+query({},offset),'preventive',false,signal),offset);},
  async list(name,filters={},offset=0,signal){nameCheck(name);const q=query(filters,offset,name.startsWith('workshop/'));const result=fleetPage(name,await get(name+'?'+q,'list',financialResources.includes(name),signal),offset);for(const row of result.items)for(const [k,v] of Object.entries(filters))if(v&&row[k]!==v)fleetFail();return result;},
  async record(name,id,signal){nameCheck(name);if(!fleetID(id))fleetFail();const v=fleetRecord(name,await get(name+'/'+encodeURIComponent(id),'record',financialResources.includes(name),signal));if(v.id!==id)fleetFail();return v;},
  async history(name,id,offset=0,signal){nameCheck(name);if(!fleetID(id))fleetFail();return fleetHistory(name,id,await get(name+'/'+encodeURIComponent(id)+'/history?'+query({},offset),'history',financialResources.includes(name),signal),offset);},
  async report(kind,id,period,signal){authorize(kind==='costs');if(!fleetID(id)||!['availability','costs','workshop'].includes(kind))fleetFail();const p=kind==='costs'?fleetPeriod(period):null;
   const path=(kind==='workshop'?'requests/':'assets/')+encodeURIComponent(id)+'/'+kind+(p?'?'+new URLSearchParams(p):'');
   return fleetReport(kind,id,await get(path,'report:'+kind,kind==='costs',signal),p);
  }
 };
}
