const limit=1024*1024,id=/^[A-Za-z0-9][A-Za-z0-9_-]{0,127}$/;
const validID=value=>typeof value==='string'&&id.test(value);
export class TaxTableError extends Error {constructor(code,uncertain=false){super(code);this.code=code;this.uncertain=uncertain;}}
// Only manually selected UTF-8 bytes; no source URL, storage, telemetry or defaults.
export function createTaxTableClient({token,isCurrent,profile,fetcher=fetch,digest=async bytes=>new Uint8Array(await crypto.subtle.digest('SHA-256',bytes))}){
 const binding={id:profile.profile_id,revision:profile.revision,model:profile.model,crt:profile.crt,environment:profile.environment};
 const plans=new WeakMap();let dead=false,busy=false;
 const current=()=>{if(dead||!token||!isCurrent())throw new TaxTableError('session');};
 const enter=()=>{current();if(busy)throw new TaxTableError('busy');busy=true;};
 async function request(path,body){
  current();const controller=new AbortController(),timer=setTimeout(()=>controller.abort(),20000);
  try{
   const response=await fetcher('/local/v1/fiscal/'+path,{method:body===undefined?'GET':'POST',cache:'no-store',credentials:'omit',signal:controller.signal,headers:{Authorization:'Bearer '+token,...(body===undefined?{}:{'Content-Type':'application/json'})},...(body===undefined?{}:{body})});current();
   if(!response.ok)throw new TaxTableError(response.status===404?'not_found':response.status===409?'conflict':response.status===401||response.status===403?'denied':'refused',response.status>=500);
   const raw=await response.text();current();if(raw.length>2*limit)throw new TaxTableError('response',true);
   try{return JSON.parse(raw);}catch{throw new TaxTableError('response',true);}
  }catch(error){if(error instanceof TaxTableError)throw error;throw new TaxTableError('network',true);}finally{clearTimeout(timer);}
 }
 function matches(record,expected,hash){
  const input=record?.input,review=input?.review;
  return record?.revision===expected.expected_revision+1&&input?.operation_id===expected.operation_id&&input?.rule?.id===expected.policy_id&&input?.profile_id===binding.id&&input?.profile_revision===binding.revision&&review?.file_sha256===hash&&review?.accountant_approved===false;
 }
 return {
  dispose(){dead=true;},
  async prepare(bytes){enter();try{
   if(binding.environment!=='homologation'||!validID(binding.id)||!Number.isSafeInteger(binding.revision)||binding.revision<1)throw new TaxTableError('profile');
   if(!(bytes instanceof Uint8Array)||bytes.byteLength<1||bytes.byteLength>limit)throw new TaxTableError('file');
   const original=new Uint8Array(bytes);let raw,table;
   try{raw=new TextDecoder('utf-8',{fatal:true,ignoreBOM:true}).decode(original);table=JSON.parse(raw);}catch{throw new TaxTableError('file');}
   if(!table||Array.isArray(table)||table.version!==1||!validID(table.revision)||!['simples_nacional','lucro_presumido','lucro_real'].includes(table.regime)||!Array.isArray(table.policies)||table.policies.length<1||table.policies.length>100)throw new TaxTableError('file');
   const operations=new Set(),ids=new Set(),expected=[];
   const rows=table.policies.map(p=>{
    if(!p||p.profile_id!==binding.id||p.profile_revision!==binding.revision||p.rule?.model!==binding.model||p.rule?.crt!==binding.crt)throw new TaxTableError('profile');
    if(!validID(p.operation_id)||!validID(p.rule.id)||operations.has(p.operation_id)||ids.has(p.rule.id)||!Number.isSafeInteger(p.expected_revision)||p.expected_revision<0||p.expected_revision>=1000000||typeof p.active!=='boolean'||!Array.isArray(p.sources)||!p.sources.length||!p.sources.every(s=>typeof s==='string'&&s.trim())||typeof p.rule.valid_from!=='string'||!p.rule.valid_from||typeof p.rule.valid_until!=='string')throw new TaxTableError('file');
    if(p.review&&(p.review.accountant_approved===true||p.review.accountant_reference))throw new TaxTableError('approval');
    operations.add(p.operation_id);ids.add(p.rule.id);
    expected.push({operation_id:p.operation_id,policy_id:p.rule.id,expected_revision:p.expected_revision});
    return Object.freeze({id:p.rule.id,product:String(p.rule.product_id??''),ncm:String(p.rule.ncm??''),cfop:String(p.rule.cfop??''),from:p.rule.valid_from,until:p.rule.valid_until,sources:Object.freeze([...p.sources]),active:p.active});
   });
   const hash=Array.from(await digest(original),b=>b.toString(16).padStart(2,'0')).join('');current();
   if(!/^[a-f0-9]{64}$/.test(hash))throw new TaxTableError('file');
   const view=Object.freeze({hash,revision:table.revision,regime:table.regime,rows:Object.freeze(rows)});
   plans.set(view,{raw,expected,attempted:false});return view;
  }finally{busy=false;}},
  async importTable(view){enter();try{
   const plan=plans.get(view);if(!plan||plan.attempted)throw new TaxTableError('query_required');
   plan.attempted=true;let result;
   try{result=await request('tax-tables/import',plan.raw);}catch(error){if(error instanceof TaxTableError&&!error.uncertain&&['refused','conflict','denied'].includes(error.code))plan.attempted=false;throw error;}
   if(!Array.isArray(result)||result.length!==plan.expected.length||!result.every((row,i)=>matches(row,plan.expected[i],view.hash)))throw new TaxTableError('response',true);
   return {state:'confirmed',count:result.length};
  }finally{busy=false;}},
  async reconcile(view){enter();try{
   const plan=plans.get(view);if(!plan)throw new TaxTableError('file');let applied=0;
   for(const expected of plan.expected){
    try{const row=await request('tax-policies/'+encodeURIComponent(expected.policy_id)+'?revision='+(expected.expected_revision+1));if(matches(row,expected,view.hash))applied++;}
    catch(error){if(!(error instanceof TaxTableError)||error.code!=='not_found')throw error;}
   }
   current();if(applied===plan.expected.length)plan.attempted=true;
   return {state:applied===plan.expected.length?'confirmed':'unresolved',count:applied};
  }finally{busy=false;}}
 };
}
