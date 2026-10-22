import {LocalAPIError} from '../local/localClient.mjs';
import {contextOf,contextKey} from '../context74/model.mjs';
import {access113,key113} from '../payables113/model.mjs';
import {availableActions,documentCommand,documentPage,payableDocument,documentReceipt} from './model.mjs';

// No token, supplier name or response is persisted. Only one original command
// per tenant/store/actor/device is retained until its result can be verified.
export function createPayableDocuments({token,session,capabilities,storage,locks,fetcher=globalThis.fetch,isCurrent}){
 const ctx=contextOf(session,capabilities),scope=contextKey(ctx),key='titan-payable-documents:v1:'+scope;
 if(typeof token!=='string'||!token||/[\r\n]/.test(token)||typeof isCurrent!=='function')throw Error('documentInvalid');
 function authorize(write=false){let valid=false;try{valid=isCurrent()&&contextKey(contextOf(session,capabilities))===scope;}catch{/* Invalid or expired original context. */}if(!valid)throw new LocalAPIError(401,'documents');if(!(write?access113(capabilities).write:access113(capabilities).read))throw new LocalAPIError(403,'documents');}
 authorize();
 const id=v=>{if(!key113(v))throw Error('documentInvalid');return encodeURIComponent(v);};
 async function request(path,method='GET',body,signal){
  authorize(method==='POST');const controller=new AbortController(),abort=()=>controller.abort();if(signal?.aborted)controller.abort();signal?.addEventListener('abort',abort,{once:true});const timer=setTimeout(abort,15000);
  try{controller.signal.throwIfAborted();const r=await fetcher('/local/v1/'+path,{method,credentials:'omit',cache:'no-store',signal:controller.signal,headers:{Authorization:'Bearer '+token,...(body?{'Content-Type':'application/json'}:{})},...(body?{body:JSON.stringify(body)}:{})});authorize();if(!r.ok)throw new LocalAPIError(r.status,'documents');const raw=await r.text();authorize();controller.signal.throwIfAborted();if(raw.length>2000000)throw Error('documentInvalid');return JSON.parse(raw);}finally{clearTimeout(timer);signal?.removeEventListener('abort',abort);}
 }
 function pending(){authorize();try{const raw=storage.getItem(key);if(raw===null)return null;if(raw.length>65536)throw Error();return documentCommand(JSON.parse(raw));}catch{throw Error('documentStorage');}}
 function save(c){authorize();documentCommand(c);try{const raw=JSON.stringify(c);storage.setItem(key,raw);if(storage.getItem(key)!==raw)throw Error();}catch{throw Error('documentStorage');}}
 function clear(){authorize();storage.removeItem(key);if(storage.getItem(key)!==null)throw Error('documentStorage');}
 async function locked(fn){if(!locks?.request)throw Error('documentLocks');return locks.request(key,{ifAvailable:true},async lock=>{authorize();if(!lock)throw Error('documentLocked');return fn();});}
 const read=async(document,signal)=>{const p=payableDocument(await request('finance/payables/'+id(document),'GET',undefined,signal));if(p.id!==document)throw Error('documentInvalid');return p;};
 async function resolve(send,discard=false){
  const c=pending();if(!c)return null;authorize(send||discard);let receipt;
  try{receipt=documentReceipt(await request('finance/payables/operations/'+id(c.body.operation_id)),c,ctx.identity_id);}catch(e){if(!(e instanceof LocalAPIError)||e.status!==404)throw e;}
  if(receipt){clear();return {state:'confirmed',receipt};}
  if(discard){if(c.uncertain)throw Error('documentUncertain');clear();return {state:'discarded'};}
  if(!send)return {state:'missing'};
  if(c.kind!=='create'){
   const p=await read(c.document);if(p.last_operation_id!==c.expected.last_operation_id||!availableActions(p,ctx.identity_id).includes(c.kind))throw new LocalAPIError(409,'documents');
  }
  authorize(true);save({...c,uncertain:true});
  try{documentReceipt(await request('finance/payables'+(c.kind==='create'?'':'/'+id(c.document)+'/'+c.kind),'POST',c.body),c,ctx.identity_id);}catch(e){if(!c.uncertain&&e instanceof LocalAPIError&&[400,403,404,409,413,422].includes(e.status))save({...c,uncertain:false});throw e;}
  receipt=documentReceipt(await request('finance/payables/operations/'+id(c.body.operation_id)),c,ctx.identity_id);clear();return {state:'confirmed',receipt};
 }
 return {scope,actor:ctx.identity_id,canLock:!!locks?.request,pending,read,
  async page(state='',offset=0,signal){if(!['','pending','approved','rejected','cancelled'].includes(state)||!Number.isInteger(offset)||offset<0||offset>1000000000)throw Error('documentInvalid');return documentPage(await request('finance/payables?limit=20&offset='+offset+(state?'&state='+state:''),'GET',undefined,signal),offset);},
  async suppliers(offset=0,signal){if(!Number.isInteger(offset)||offset<0||offset>1000000000)throw Error('documentInvalid');const v=await request('purchase-suppliers?offset='+offset,'GET',undefined,signal);if(!v||!Array.isArray(v.items)||v.items.length>50||!v.items.every(s=>s&&typeof s.id==='string'&&s.id.trim()&&s.id.length<=128&&typeof s.name==='string'&&s.name.trim()&&['active','inactive'].includes(s.status))||new Set(v.items.map(s=>s.id)).size!==v.items.length)throw Error('documentInvalid');return v.items;},
  async submit(command){authorize(true);const c=JSON.parse(JSON.stringify(documentCommand(command)));if(c.uncertain)throw Error('documentInvalid');return locked(async()=>{authorize(true);if(pending())throw Error('documentPending');save(c);return resolve(true);});},
  async recover(send=false,discard=false){authorize(send||discard);return locked(()=>resolve(send,discard));}
 };
}
