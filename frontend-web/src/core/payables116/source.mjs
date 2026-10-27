import {LocalAPIError} from '../local/localClient.mjs';import {createPayables113} from '../payables113/client.mjs';import {access113,key113} from '../payables113/model.mjs';import {contextOf,contextKey} from '../context74/model.mjs';import {balances116,candidates116,command116,receipt116,queueKey116,read116,save116} from './model.mjs';
export function createFinance116({token,session,capabilities,storage,locks,fetcher=globalThis.fetch,isCurrent=()=>true}){
 const ctx=contextOf(session,capabilities),scope=contextKey(ctx),key=queueKey116(scope);if(!token||!access113(capabilities).read)throw Error('permission116');
 // The live container binds this callback to the exact token and session object.
 // A source kept by an async callback cannot act for a subsequent login.
 const authorize=write=>{if(!isCurrent())throw new LocalAPIError(401,'finance116');if(contextKey(contextOf(session,capabilities))!==scope)throw new LocalAPIError(401,'finance116');const a=access113(capabilities);if(!(write?a.write:a.read))throw Error('permission116');};
 const currentFetch=async(url,options)=>{authorize(options?.method==='POST');const response=await fetcher(url,options);authorize(false);return response;};
 const client=createPayables113(token,currentFetch);
 // Check again after body decoding, not only after the response headers arrive.
 const legacy=Object.fromEntries(Object.entries(client).map(([name,fn])=>[name,async(...args)=>{authorize(name==='submit');const result=await fn(...args);authorize(false);return result;}]));

 async function get(path,signal){authorize(false);const c=new AbortController(),abort=()=>c.abort();if(signal?.aborted)c.abort();signal?.addEventListener('abort',abort,{once:true});const timer=setTimeout(abort,15000);try{const r=await currentFetch('/local/v1/finance/'+path,{method:'GET',credentials:'omit',cache:'no-store',headers:{Authorization:'Bearer '+token},signal:c.signal});if(!r.ok)throw new LocalAPIError(r.status,'finance116');const text=await r.text();authorize(false);if(text.length>2000000)throw Error('invalid116');return JSON.parse(text);}finally{clearTimeout(timer);signal?.removeEventListener('abort',abort);}}
 const id=v=>{if(!key113(v))throw Error('invalid116');return encodeURIComponent(v);};
 async function lock(fn){if(!locks?.request)throw Error('locks116');return locks.request(key,{ifAvailable:true},async l=>{if(!l)throw Error('locked116');return fn();});}
 async function resolve(send,discard=false){const p=read116(storage,key);if(!p)return null;authorize(send||discard);let receipt;try{receipt=receipt116(await legacy.receipt(p.document,p.kind,p.body,p.paymentID||undefined),p);}catch(e){if(!(e instanceof LocalAPIError)||e.status!==404)throw e;}
  if(receipt){storage.removeItem(key);return {state:'confirmed',receipt};}if(discard){if(p.uncertain)throw Error('uncertain116');storage.removeItem(key);return {state:'discarded'};}if(!send)return {state:'missing'};
  authorize(true);save116(storage,key,{...p,uncertain:true});try{receipt116(await legacy.submit(p.document,p.kind,p.body,p.paymentID||undefined),p);}catch(e){if(!p.uncertain&&e instanceof LocalAPIError&&[400,403,404,409,413,422].includes(e.status))save116(storage,key,{...p,uncertain:false});throw e;}
  receipt=receipt116(await legacy.receipt(p.document,p.kind,p.body,p.paymentID||undefined),p);storage.removeItem(key);return {state:'confirmed',receipt};
 }
 return {legacy,scope,canLock:!!locks?.request,pending:()=>read116(storage,key),
  async balances(document,signal){return balances116(await get('payables/'+id(document.id)+'/installment-balances',signal),document);},
  async sources(document,kind,part,payment,offset=0,signal){if(!Number.isSafeInteger(offset)||offset<0||offset>1000000000||!['pay','return'].includes(kind)||!document.installments.some(p=>p.id===part)||kind==='return'&&(!payment||payment.payable_id!==document.id||payment.installment_id!==part))throw Error('invalid116');const target=kind==='pay'?document.id:payment.id,path=kind==='pay'?'payables/'+id(target)+'/cash-sources?installment_id='+id(part)+'&limit=20&offset='+offset:'payable-cash/'+id(target)+'/return-sources?limit=20&offset='+offset;return candidates116(await get(path,signal),{kind,target,part,actor:ctx.identity_id,offset,payment});},
  async submit(command){authorize(true);command116(command);if(command.uncertain)throw Error('invalid116');return lock(async()=>{authorize(true);if(read116(storage,key))throw Error('pending116');save116(storage,key,command);return resolve(true);});},
  async recover(send=false,discard=false){authorize(send||discard);return lock(()=>resolve(send,discard));}
 };
}
