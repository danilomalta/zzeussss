import {LocalAPIError} from '../local/localClient.mjs';
import {contextOf,contextKey} from '../context74/model.mjs';
import {catalogID,catalogEqual,catalogDetails,catalogState,catalogCommand,catalogResult,catalogReceipt,catalogHistory} from './model.mjs';
const invalid=()=>{throw Error('catalogInvalid');};
export function createCatalogManagement({token,session,capabilities,isCurrent,storage,locks,fetcher=globalThis.fetch}){
 const ctx=contextOf(session,capabilities),scope=contextKey(ctx),key='titan-catalog-management:v1:'+scope;
 if(typeof token!=='string'||!token||/[\r\n]/.test(token)||typeof isCurrent!=='function')invalid();
 const manager=()=>capabilities.permissions?.includes('manage_stock')===true;
 const writable=()=>capabilities.permissions?.includes('view_catalog')&&manager()&&capabilities.license?.state==='active'&&['core','inventory'].every(m=>capabilities.license.modules.includes(m));
 function authorize(manage=false,write=false){let current=false;try{current=isCurrent()&&contextKey(contextOf(session,capabilities))===scope;}catch{}if(!current)throw new LocalAPIError(401,'catalog');if(!capabilities.permissions?.includes('view_catalog')||manage&&!manager()||write&&!writable())throw new LocalAPIError(403,'catalog');}
 authorize();
 const id=value=>{if(!catalogID(value))invalid();return encodeURIComponent(value);};
 async function request(path,method='GET',body,signal,manage=true){authorize(manage,method==='POST');const c=new AbortController(),abort=()=>c.abort();if(signal?.aborted)abort();signal?.addEventListener('abort',abort,{once:true});const timer=setTimeout(abort,15000);try{c.signal.throwIfAborted();const response=await fetcher('/local/v1/catalog/'+path,{method,credentials:'omit',cache:'no-store',signal:c.signal,headers:{Authorization:'Bearer '+token,...(body?{'Content-Type':'application/json'}:{})},...(body?{body:JSON.stringify(body)}:{})});authorize(manage);c.signal.throwIfAborted();const raw=await response.text();authorize(manage);c.signal.throwIfAborted();if(raw.length>2000000)invalid();if(!response.ok)throw new LocalAPIError(response.status,'catalog');try{return JSON.parse(raw);}catch{invalid();}}finally{clearTimeout(timer);signal?.removeEventListener('abort',abort);}}
 function pending(){authorize(true);try{const raw=storage.getItem(key);if(raw===null)return null;if(raw.length>20000)invalid();return catalogCommand(JSON.parse(raw),scope);}catch{throw Error('catalogStorage');}}
 function save(command){authorize(true);const raw=JSON.stringify(catalogCommand(command,scope));try{storage.setItem(key,raw);if(storage.getItem(key)!==raw)throw Error();}catch{throw Error('catalogStorage');}}
 function clear(command){authorize(true);const current=pending();if(!current||!catalogEqual(current,command))throw Error('catalogStorage');try{storage.removeItem(key);if(storage.getItem(key)!==null)throw Error();}catch{throw Error('catalogStorage');}}
 async function locked(fn){if(!locks?.request)throw Error('catalogLocks');return locks.request(key,{ifAvailable:true},async lock=>{authorize(true);if(!lock)throw Error('catalogLocked');return fn();});}
 const details=async(product,signal)=>catalogDetails(await request('products/'+id(product)+'/edit-details','GET',undefined,signal),product);
 const state=async(product,signal)=>catalogState(await request('products/'+id(product)+'/state','GET',undefined,signal,false),product);
 const receipt=async command=>catalogReceipt(await request('operations/'+command.kind+'/'+id(command.body.operation_id)),command);
 async function preflight(command){authorize(true,true);const fresh=command.kind==='edit'?await details(command.product_id):await state(command.product_id);authorize(true,true);if(!catalogEqual(fresh,command.before))throw new LocalAPIError(409,'catalog');}
 async function resolve(send,discard){let command=pending();if(!command)return null;authorize(true,send||discard);let result;try{result=await receipt(command);}catch(error){if(!(error instanceof LocalAPIError)||error.status!==404)throw error;}
  if(result){clear(command);return {state:'confirmed',receipt:result,command};}
  if(discard){if(command.uncertain)throw Error('catalogUncertain');clear(command);return {state:'discarded',command};}
  if(!send)return {state:'missing',command};
  await preflight(command);authorize(true,true);const prior=command.uncertain;command={...command,uncertain:true};save(command);
  try{catalogResult(await request('products/'+id(command.product_id)+(command.kind==='edit'?'/update':'/state'),'POST',command.body),command);}catch(error){if(!prior&&error instanceof LocalAPIError&&[400,403,404,409,413,422].includes(error.status)){authorize(true);save({...command,uncertain:false});}throw error;}
  result=await receipt(command);clear(command);return {state:'confirmed',receipt:result,command};
 }
 return {scope,get canManage(){return manager();},get canWrite(){return !!writable();},canLock:!!locks?.request,assertCurrent(){authorize();},pending,details,state,
  async history(product,offset=0,signal){if(!Number.isSafeInteger(offset)||offset<0)invalid();return catalogHistory(await request('products/'+id(product)+'/state-history?'+new URLSearchParams({offset:String(offset)}),'GET',undefined,signal),product,offset);},
  prepare(kind,product,before,body){authorize(true,true);return catalogCommand(structuredClone({version:1,scope,kind,product_id:product,before,body,uncertain:false}),scope);},
  async submit(value){authorize(true,true);const command=catalogCommand(structuredClone(value),scope);if(command.uncertain)invalid();return locked(async()=>{authorize(true,true);if(pending())throw Error('catalogPending');save(command);return resolve(true,false);});},
  async recover(send=false,discard=false){if(typeof send!=='boolean'||typeof discard!=='boolean'||send&&discard)invalid();return locked(()=>resolve(send,discard));}
 };
}
