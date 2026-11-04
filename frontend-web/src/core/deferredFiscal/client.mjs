export function deferredFiscalClient({token,session,getCurrent,fetcher=globalThis.fetch,digest=async text=>new Uint8Array(await crypto.subtle.digest('SHA-256',new TextEncoder().encode(text)))}){
 const fields=['tenant_id','store_id','identity_id','device_id'],scope=JSON.stringify(fields.map(k=>session?.[k]));let dead=false,busy=false;
 const check=()=>{const now=getCurrent();if(dead||!token||now.token!==token||JSON.stringify(fields.map(k=>now.session?.[k]))!==scope)throw Error('context');};
 async function send(sale,action,body){check();if(!/^[A-Za-z0-9][A-Za-z0-9_-]{0,127}$/.test(sale)||busy)throw Error('invalid');busy=true;
  const controller=new AbortController(),timer=setTimeout(()=>controller.abort(),20000);
  try{if(action==='emit'){
   if(!['21','91'].includes(body.payment_code)||!['1','4'].includes(body.presence)||!body.confirmed||typeof body.nature!=='string'||!body.nature.trim()||body.nature.length>60)throw Error('invalid');
   const hash=await digest(JSON.stringify([scope,sale]));check();body={...body,operation_id:'deferred-'+Array.from(hash,b=>b.toString(16).padStart(2,'0')).join('')};
  }
  check();const response=await fetcher('/local/v1/fiscal/deferred/sales/'+encodeURIComponent(sale)+'/'+action,{method:'POST',cache:'no-store',credentials:'omit',signal:controller.signal,headers:{Authorization:'Bearer '+token,'Content-Type':'application/json'},body:JSON.stringify(body)});
  check();if(!response.ok)throw Error('refused');const value=await response.json();check();if(value?.sale_id!==sale||!['authorized_homologation','failed','pending','recovery_required','cancelled_homologation','cancellation_failed'].includes(value.state))throw Error('invalid');return value;
  }finally{busy=false;clearTimeout(timer);}
 }
 return {dispose(){dead=true;},emit(sale,body){return send(sale,'emit',{...body});},query(sale){return send(sale,'query',{});}};
}
