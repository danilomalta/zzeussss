const id=v=>typeof v==='string'&&v.length>0&&v.length<=128&&!/[\u0000\r\n]/.test(v);
const cents=v=>Number.isSafeInteger(v)&&v>=0;
const fields=['tenant_id','store_id','identity_id','device_id'];
export function checkoutFiscalScope(session){if(!session||!fields.every(k=>id(session[k])))throw Error('context');return JSON.stringify(fields.map(k=>session[k]));}
export function checkoutFiscalView(plan,checkout){
 if(!plan||plan.checkout_id!==checkout)throw Error('invalidReply');
 if(!plan.fiscal)return {state:'none',plan};
 const f=plan.fiscal,c=plan.completion;
 if(!c||c.sale_id!==checkout||!id(c.operation_id)||!cents(c.received_cents)||!id(f.operation_id))throw Error('invalidReply');
 if(f.state==='authorized_homologation'){
  if(!id(f.intent_id)||!/^[0-9A-Z]{44}$/.test(f.key||''))throw Error('invalidReply');
  return {state:'authorized',plan};
 }
 if(f.state==='cancelled_homologation'||f.state==='cancellation_failed')return {state:f.state==='cancelled_homologation'?'cancelled':'cancellationFailed',message:typeof f.message==='string'?f.message:'',plan};
 if(f.state==='failed')return {state:'failed',plan};
 if(['pending','recovery_required','uncertain','reserved','sending','response_received','not_sent','recovery_pending','protocol_received_homologation'].includes(f.state))return {state:'pending',plan};
 throw Error('invalidReply');
}

// The server owns the original request. Recovery sends only the sale ID;
// no browser storage, guessed invoice or automatic POST participates in retry.
export function createCheckoutFiscalClient({fetcher=globalThis.fetch,getCurrent}){
 const locks=new Set();
 return {bind(context){
  const scope=checkoutFiscalScope(context.session),token=context.token;
  const assertCurrent=()=>{const now=getCurrent();if(!token||now.token!==token||now.session!==context.session||checkoutFiscalScope(now.session)!==scope)throw Error('context');};
  const key=checkout=>JSON.stringify([scope,checkout]);
  async function request(checkout,body,signal,reprocess=false){
   if(!id(checkout))throw Error('invalidReply');assertCurrent();
   const abort=new AbortController(),onAbort=()=>abort.abort();signal?.addEventListener('abort',onAbort,{once:true});if(signal?.aborted)abort.abort();
   const timer=setTimeout(()=>abort.abort(),15000);
   try{
    if(abort.signal.aborted)throw Error('aborted');
    const response=await fetcher(`/local/v1/checkout/plans/${encodeURIComponent(checkout)}${body?(reprocess?'/fiscal/reprocess':'/finalize-cash'):''}`,{method:body?'POST':'GET',credentials:'omit',cache:'no-store',signal:abort.signal,headers:{Authorization:`Bearer ${token}`,...(body?{'Content-Type':'application/json'}:{})},...(body?{body:JSON.stringify(body)}:{})});
    assertCurrent();if(abort.signal.aborted)throw Error('aborted');
    if(response.status===404&&!body)return {state:'none',plan:null,canRetry:false};
    if(!response.ok)throw Error('unavailable');
    const view=checkoutFiscalView(await response.json(),checkout);assertCurrent();if(abort.signal.aborted)throw Error('aborted');
    return {...view,canRetry:view.state==='failed'&&view.plan.completion.sale_status==='committed'&&view.plan.fiscal.can_reprocess===true};
   }finally{clearTimeout(timer);signal?.removeEventListener('abort',onAbort);}
  }
  async function read(checkout,signal){return request(checkout,undefined,signal);}
  async function locked(checkout,work){const k=key(checkout);assertCurrent();if(locks.has(k))throw Error('busy');locks.add(k);try{return await work();}finally{locks.delete(k);}}
  async function finalize(checkout,input,signal){return locked(checkout,async()=>{
   if(!id(input?.operation_id)||!cents(input.received_cents)||!input.received_cents||input.fiscal_invoice?.Environment!==2)throw Error('invalidReply');
   const body=JSON.parse(JSON.stringify({operation_id:input.operation_id,received_cents:input.received_cents,fiscal_invoice:input.fiscal_invoice}));
   return request(checkout,body,signal);
  });}
  async function retry(checkout,signal){return locked(checkout,async()=>{
   const view=await read(checkout,signal);assertCurrent();
   if(view.state!=='failed')return view;
   if(!view.canRetry)throw Error('missingRequest');
   return request(checkout,{},signal,true);
  });}
  return {read,finalize,retry};
 }};
}
