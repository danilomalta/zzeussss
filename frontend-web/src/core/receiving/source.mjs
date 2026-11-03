import {LocalAPIError} from '../local/localClient.mjs';
import {contextOf,contextKey} from '../context74/model.mjs';
import {receivingAccess,receivingID,receivingCommand,receivingTrace,receivingHistory,receivingReceipt,receivingResult} from './model.mjs';

export function createReceiving({token,session,capabilities,storage,locks,isCurrent,fetcher=globalThis.fetch}){
 const ctx=contextOf(session,capabilities),scope=contextKey(ctx),key='titan-receiving:v1:'+scope;
 if(typeof token!=='string'||!token||/[\r\n]/.test(token)||typeof isCurrent!=='function')throw Error('receivingInvalid');
 function authorize(kind){let valid=false;try{valid=isCurrent()&&contextKey(contextOf(session,capabilities))===scope;}catch{/* Expired or changed original context. */}if(!valid)throw new LocalAPIError(401,'receiving');const a=receivingAccess(capabilities);if(!(kind==='authorize'?a.authorize:kind?a.write:a.read))throw new LocalAPIError(403,'receiving');}
 const id=v=>{if(!receivingID(v))throw Error('receivingInvalid');return encodeURIComponent(v);};
 authorize();
 async function request(path,method='GET',body,signal,kind){
  authorize(method==='POST'?kind:undefined);const c=new AbortController(),abort=()=>c.abort();if(signal?.aborted)c.abort();signal?.addEventListener('abort',abort,{once:true});const timer=setTimeout(abort,15000);
  try{c.signal.throwIfAborted();const r=await fetcher('/local/v1/'+path,{method,credentials:'omit',cache:'no-store',signal:c.signal,headers:{Authorization:'Bearer '+token,...(body?{'Content-Type':'application/json'}:{})},...(body?{body:JSON.stringify(body)}:{})});authorize();if(!r.ok)throw new LocalAPIError(r.status,'receiving');const text=await r.text();authorize();c.signal.throwIfAborted();if(text.length>2000000)throw Error('receivingInvalid');return JSON.parse(text);}finally{clearTimeout(timer);signal?.removeEventListener('abort',abort);}
 }
 function pending(){authorize();try{const raw=storage.getItem(key);if(raw===null)return null;if(raw.length>8192)throw Error();return receivingCommand(JSON.parse(raw));}catch{throw Error('receivingStorage');}}
 function save(c){authorize();receivingCommand(c);try{const raw=JSON.stringify(c);storage.setItem(key,raw);if(storage.getItem(key)!==raw)throw Error();}catch{throw Error('receivingStorage');}}
 function clear(){authorize();storage.removeItem(key);if(storage.getItem(key)!==null)throw Error('receivingStorage');}
 async function locked(fn){if(!locks?.request)throw Error('receivingLocks');return locks.request(key,{ifAvailable:true},async lock=>{authorize();if(!lock)throw Error('receivingLocked');return fn();});}
 const receipt=async c=>receivingReceipt(await request('purchase-receiving/operations/'+c.kind+'/'+id(c.body.operation_id)+'/receipt'),c,ctx.identity_id,ctx.device_id);
 async function resolve(send,discard){
  const c=pending();if(!c)return null;authorize(send||discard?c.kind:undefined);let r;
  try{r=await receipt(c);}catch(e){if(!(e instanceof LocalAPIError)||e.status!==404)throw e;}
  if(r){clear();return {state:'confirmed',receipt:r};}if(discard){if(c.uncertain)throw Error('receivingUncertain');clear();return {state:'discarded'};}if(!send)return {state:'missing'};
  authorize(c.kind);save({...c,uncertain:true});
  const {order_id,receipt_id,...rest}=c.body,body=c.kind==='void'?{operation_id:rest.operation_id,reason:rest.reason}:rest;
  const path='purchase-orders/'+id(order_id)+'/'+({authorize:'authorize-receiving',receipt:'receipts',reject:'rejections',void:'receipts/'+(c.kind==='void'?id(receipt_id):'')+'/void'}[c.kind]);
  try{receivingResult(await request(path,'POST',body,undefined,c.kind),c,ctx.identity_id,ctx.device_id);}catch(e){if(!c.uncertain&&e instanceof LocalAPIError&&[400,403,404,409,413,422].includes(e.status))save({...c,uncertain:false});throw e;}
  r=await receipt(c);clear();return {state:'confirmed',receipt:r};
 }
 return {scope,access:receivingAccess(capabilities),canLock:!!locks?.request,pending,
  async history(order,offset=0,signal){authorize();if(!receivingAccess(capabilities).history)throw new LocalAPIError(403,'receiving');if(!Number.isSafeInteger(offset)||offset<0)throw Error('receivingInvalid');const v=receivingTrace(await request('purchase-orders/'+id(order)+'/receiving-trace?offset=0','GET',undefined,signal),order),h=receivingHistory(await request('purchase-orders/'+id(order)+'/receiving-history?offset='+offset,'GET',undefined,signal),order,offset);if(v.product_id!==h.product_id||v.unit!==h.unit||v.commercial_status!==h.commercial_status||v.receiving_status!==h.receiving_status||v.planned_milli!==h.totals.planned_milli||v.remaining_milli!==h.totals.remaining_milli||v.accepted_milli!==h.totals.effective_accepted_milli||v.delivered_milli!==h.totals.effective_delivered_milli||v.voided_accepted_milli_exact!==h.totals.voided_accepted_milli_exact)throw new LocalAPIError(409,'receiving');return {trace:v,history:h};},
  async locations(signal){const v=await request('locations','GET',undefined,signal),rows=v?.items===null?[]:v?.items;if(!Array.isArray(rows)||rows.length>1000||!rows.every(l=>receivingID(l?.ID)&&typeof l.name==='string'&&l.name.trim()&&['shelf','backroom','receiving','production'].includes(l.kind)))throw Error('receivingInvalid');return rows.map(l=>({id:l.ID,name:l.name,kind:l.kind}));},
  async submit(command){authorize(command.kind);const c=JSON.parse(JSON.stringify(receivingCommand(command)));if(c.uncertain)throw Error('receivingInvalid');return locked(async()=>{authorize(c.kind);if(pending())throw Error('receivingPending');save(c);return resolve(true,false);});},
  async recover(send=false,discard=false){return locked(()=>resolve(send,discard));}
 };
}
