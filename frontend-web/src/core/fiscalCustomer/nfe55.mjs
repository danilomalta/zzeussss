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
   if(action==='preview'){if(value.sale_id!==sale||!Number.isSafeInteger(value.total_cents)||!Array.isArray(value.items)||typeof value.mapping_supported!=='boolean'||(value.deferred_payment_code!==undefined&&!['21','91'].includes(value.deferred_payment_code)))throw new NFe55Error('response');}
   else if(typeof value.state!=='string')throw new NFe55Error('response');return value;
  }catch(error){if(error instanceof NFe55Error)throw error;throw new NFe55Error('network');}finally{clearTimeout(timer);}
 }
 async function postJSON(url,input){
  current();
  const controller=new AbortController(),timer=setTimeout(()=>controller.abort(),20000);
  try{
   const response=await fetcher(url,{method:'POST',cache:'no-store',credentials:'omit',signal:controller.signal,headers:{Authorization:`Bearer ${token}`,'Content-Type':'application/json'},body:JSON.stringify(input)});current();
   if(!response.ok){let code='provider_unavailable';try{const error=await response.json();current();if(typeof error.error==='string')code=error.error;}catch(e){if(e instanceof NFe55Error)throw e;}throw new NFe55Error(code);}
   const text=await response.text();current();if(text.length>524288)throw new NFe55Error('response');
   const value=JSON.parse(text);if(!value||typeof value!=='object'||Array.isArray(value))throw new NFe55Error('response');
   return value;
  }catch(error){if(error instanceof NFe55Error)throw error;throw new NFe55Error('network');}finally{clearTimeout(timer);}
 }
 function requireOrigin(input){
  if(!input||input.confirmed!==true||!['inbound_receipt','customer_return'].includes(input.kind)||!/^[A-Za-z0-9_-]{1,128}$/.test(input.operation_id||'')||!/^[A-Za-z0-9_-]{1,128}$/.test(input.source_id||'')||!/^[0-9]{44}$/.test(input.referenced_access_key||'')||!['0','1','2','9'].includes(input.freight_mode)||!input.recipient?.name||!input.recipient?.uf||!Array.isArray(input.items)||!input.items.length)throw new NFe55Error('input_missing');
 }
 return {preview:(sale,input)=>{if(input!==undefined&&(!['21','91'].includes(input.payment_code)||input.confirmed!==true))throw new NFe55Error('input_missing');return call(sale,'preview',input);},emit:(sale,input)=>call(sale,'emit',input),query:sale=>call(sale,'query'),correction:(sale,input)=>call(sale,'correction',input),queryCorrection:sale=>call(sale,'correction/query'),cancel:(sale,input)=>call(sale,'cancel',input),danfe:sale=>call(sale,'danfe',null,true),
  bindOrigin(input){requireOrigin(input);return postJSON('/local/v1/fiscal/nfe55/origins',input).then(value=>{if(typeof value.origin_id!=='string'||typeof value.operation_id!=='string'||typeof value.kind!=='string')throw new NFe55Error('response');return value;});},
  emitOrigin(id,input){if(!/^[A-Za-z0-9_-]{1,128}$/.test(id||'')||!input||input.confirmed!==true||!input.nature||!input.purpose||!input.freight_mode||!input.recipient?.name)throw new NFe55Error('input_missing');return postJSON(`/local/v1/fiscal/nfe55/origins/${encodeURIComponent(id)}/emit`,input).then(value=>{if(typeof value.state!=='string')throw new NFe55Error('response');return value;});}};
}
