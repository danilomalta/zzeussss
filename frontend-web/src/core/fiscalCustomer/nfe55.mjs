// Memory-only customer data. Fixed same-origin routes; never persist or log a request.
export class NFe55Error extends Error {constructor(code){super(code);this.code=code;}}
export function createNFe55Client({token,isCurrent,fetcher=fetch}){
 const current=()=>{if(!token||!isCurrent())throw new NFe55Error('session');};
 async function call(sale,action,input,download=false){
  current();if(!/^[A-Za-z0-9_-]{1,128}$/.test(sale))throw new NFe55Error('input_missing');
  const controller=new AbortController(),timer=setTimeout(()=>controller.abort(),20000);
  try{
   const response=await fetcher(`/local/v1/fiscal/nfe55/sales/${encodeURIComponent(sale)}/${action}`,{method:download?'GET':'POST',cache:'no-store',credentials:'omit',signal:controller.signal,headers:{Authorization:`Bearer ${token}`,...(!download?{'Content-Type':'application/json'}:{})},...(!download?{body:JSON.stringify(input??{})}:{})});current();
   if(!response.ok){let code='provider_unavailable';try{const error=await response.json();current();if(typeof error.error==='string')code=error.error;}catch(e){if(e instanceof NFe55Error)throw e;}throw new NFe55Error(code);}
   if(download){if(!response.headers.get('content-type')?.includes('application/pdf'))throw new NFe55Error('response');const blob=await response.blob();current();return blob;}
   const text=await response.text();current();if(text.length>524288)throw new NFe55Error('response');
   const value=JSON.parse(text);if(!value||typeof value!=='object'||Array.isArray(value))throw new NFe55Error('response');
   if(action==='preview'){if(value.sale_id!==sale||!Number.isSafeInteger(value.total_cents)||!Array.isArray(value.items)||typeof value.mapping_supported!=='boolean')throw new NFe55Error('response');}
   else if(typeof value.state!=='string')throw new NFe55Error('response');return value;
  }catch(error){if(error instanceof NFe55Error)throw error;throw new NFe55Error('network');}finally{clearTimeout(timer);}
 }
 return {preview:sale=>call(sale,'preview'),emit:(sale,input)=>call(sale,'emit',input),query:sale=>call(sale,'query'),cancel:(sale,input)=>call(sale,'cancel',input),danfe:sale=>call(sale,'danfe',null,true)};
}
