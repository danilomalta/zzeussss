import {createLocalClient} from './localClient.mjs';
import {createCheckoutFiscalClient,checkoutFiscalScope} from './checkoutFiscal.mjs';

const validID=x=>typeof x==='string'&&/^[A-Za-z0-9][A-Za-z0-9_-]{0,127}$/.test(x);
const decimal=(n,places)=>{if(!Number.isSafeInteger(n)||n<0)throw Error('invalid');const s=String(n).padStart(places+1,'0');return s.slice(0,-places)+'.'+s.slice(-places);};
// Tax policies are explicitly selected by the customer. No browser tax table,
// fiscal request, CPF, token or issuer address is persisted by this adapter.
export function createNFCeCheckoutAdapter({getCurrent,getSetup,fetcher=globalThis.fetch,uuid=()=>crypto.randomUUID(),now=()=>new Date()}){
 const quotes=new Map(),locks=new Set(),local=createLocalClient(fetcher),fiscal=createCheckoutFiscalClient({fetcher,getCurrent});
 function bind(token){const session=getCurrent().session,scope=checkoutFiscalScope(session);const current=()=>{const s=getCurrent();if(!token||s.token!==token||s.session!==session||checkoutFiscalScope(s.session)!==scope)throw Error('context');};return {session,scope,current};}
 async function request(auth,path,body,signal){auth.current();if(signal?.aborted)throw Error('requestUncertain');const r=await fetcher('/local/v1'+path,{method:body===undefined?'GET':'POST',credentials:'omit',cache:'no-store',signal,headers:{Authorization:'Bearer '+getCurrent().token,...(body===undefined?{}:{'Content-Type':'application/json'})},...(body===undefined?{}:{body:JSON.stringify(body)})});auth.current();if(signal?.aborted)throw Error('requestUncertain');if(!r.ok)throw Error(({400:'invalid',401:'permission',403:'permission',409:'conflict'})[r.status]||'unavailable');const out=await r.json();auth.current();if(signal?.aborted)throw Error('requestUncertain');return out;}
 async function exclusive(auth,id,work){const k=JSON.stringify([auth.scope,id]);auth.current();if(locks.has(k))throw Error('busy');locks.add(k);try{return await work();}finally{locks.delete(k);}}
 async function result(auth,token,id,signal,view){
  view??=await fiscal.bind({token,session:auth.session}).read(id,signal);auth.current();
  const p=view.plan;if(!p?.completion)return {checkout:id,state:!p||['prepared','cancelled'].includes(p.state)?'rejected':'pending'};
  if(p.completion.sale_id!==id||p.completion.sale_status!=='committed')throw Error('invalidReply');
  const receipt=await local.receipt(token,id);auth.current();if(signal?.aborted)throw Error('requestUncertain');
  if(receipt.operation_id!==p.completion.operation_id||receipt.cash_session_id!==p.cash_session_id||receipt.total_cents!==p.total_cents)throw Error('invalidReply');
  return {checkout:id,state:'committed',receipt:{...receipt,fiscal_authorized:view.state==='authorized',fiscal_state:view.state}};
 }
 const unavailable=async()=>{throw Error('unavailable');};
 return {
  async quote(token,input,signal){
   const auth=bind(token);return exclusive(auth,input.checkout,async()=>{
    const setup=structuredClone(getSetup());
    if(input.fiscal!=='nfce'||input.discount!==0||input.points!==0||input.customer.club!=='skip'||!validID(input.checkout)||!setup.nature?.trim()||!['1','4'].includes(setup.presence)||!Array.isArray(input.items)||!input.items.length||input.items.length>500)throw Error('invalid');
    const refs=[],qid=uuid();let profile=null,approved=true,external=false;
    for(let index=0;index<input.items.length;index++){
     const item={...input.items[index],unit_price_cents:setup.unitPrices?.[index]},policyID=setup.policies?.[index];if(!validID(policyID))throw Error('checkoutNFCePolicyMissing');
     if(!Number.isSafeInteger(item.unit_price_cents)||item.unit_price_cents<0||!Number.isSafeInteger(item.quantity_milli)||item.quantity_milli<=0)throw Error('invalid');
     item.total_cents=Number((BigInt(item.quantity_milli)*BigInt(item.unit_price_cents)+500n)/1000n);
     const policy=await request(auth,'/fiscal/tax-policies/'+encodeURIComponent(policyID),undefined,signal),p=policy.input;
     if(!p?.active||p.rule?.product_id!==item.product_id||p.rule.model!=='65'||p.rule.origin_uf!==p.rule.destination_uf||p.case?.operation!=='sale'||p.case.final_consumer!=='yes'||!Number.isSafeInteger(policy.revision))throw Error('checkoutNFCePolicyMissing');
     if(!profile){profile=await request(auth,'/fiscal/profiles/'+encodeURIComponent(p.profile_id),undefined,signal);const v=profile.input;if(v?.environment!=='homologation'||v.model!=='65'||!v.timezone||v.require_tax_pricing!==true)throw Error('checkoutNFCeProfileUnavailable');
      const diagnostic=await request(auth,'/fiscal/onboarding/'+encodeURIComponent(p.profile_id),undefined,signal);
      external=diagnostic.configuration_valid===true&&diagnostic.profile_revision===profile.revision&&diagnostic.provider_configuration?.provider==='external'&&diagnostic.provider_configuration.environment==='homologation'&&diagnostic.provider_configuration.certificate_type==='A1';
     }
     if(p.profile_id!==profile.input.profile_id||p.profile_revision!==profile.revision||p.rule.crt!==profile.input.crt||p.rule.origin_uf!==profile.input.state)throw Error('checkoutNFCeProfileUnavailable');
     const date=new Intl.DateTimeFormat('en-CA',{timeZone:profile.input.timezone,year:'numeric',month:'2-digit',day:'2-digit'}).format(now());
     const op=qid+'-'+index;
     const quoted=await request(auth,'/fiscal/tax-quotes',{operation_id:op,policy_id:policyID,policy_revision:policy.revision,context:{origin_uf:p.rule.origin_uf,destination_uf:p.rule.destination_uf,crt:p.rule.crt,model:'65',date},case:p.case,item:{product_id:item.product_id,quantity:decimal(item.quantity_milli,3),unit_price:decimal(item.unit_price_cents,2),discount:'0',freight:'0',insurance:'0',other:'0'}},signal);
     // The first UI contract is cash at the displayed catalogue total. A tax
     // addition requires a new authoritative commercial quote, never a guess.
     if(quoted.input?.operation_id!==op||quoted.result?.total!==decimal(item.total_cents,2)||!quoted.result.goods_xml)throw Error('invalidReply');
     refs.push(op);approved&&=p.review?.accountant_approved===true;
    }
    const base=await local.quoteSale(token,{cash_session_id:input.cash,items:input.items,tax_quote_operations:refs});auth.current();if(signal?.aborted)throw Error('requestUncertain');
    if(base.total_cents!==input.gross)throw Error('checkoutNFCePriceChanged');
    const issuer=setup.issuerAddress;
    if(!external&&(!issuer||!['Street','Number','District','City'].every(k=>typeof issuer[k]==='string'&&issuer[k].trim()&&issuer[k].length<=100)))throw Error('checkoutNFCeIssuerIncomplete');
    // Focus uses the issuer's existing registration. Do not ask its address
    // again or transmit a browser override for that registration.
    const invoice={Environment:2,Nature:setup.nature.trim(),Presence:setup.presence,...(!external?{Issuer:{Address:issuer}}:{}),...(input.customer.cpf?{Recipient:{CPF:input.customer.cpf,IEDestination:'9'}}:{})};
    quotes.set(qid,{scope:auth.scope,token,session:auth.session,input:structuredClone(input),refs,invoice});
    return {id:qid,gross:base.total_cents,discount:0,net:base.total_cents,expires:new Date(now().getTime()+5*60000).toISOString(),accountantApproved:approved};
   });
  },
  async finalize(token,{plan,items},signal){
   const auth=bind(token);return exclusive(auth,plan.id,async()=>{
    const q=quotes.get(plan.quote?.id);
    if(!q||q.scope!==auth.scope||q.token!==token||q.session!==auth.session||q.input.checkout!==plan.id||q.input.cash!==plan.cash||JSON.stringify(q.input.items)!==JSON.stringify(items)||plan.fiscal!=='nfce'||plan.total!==q.input.gross||!Number.isSafeInteger(plan.received)||plan.received<plan.total||!plan.parts.length||plan.parts.some(p=>p.method!=='cash'||p.state!=='approved'||!Number.isSafeInteger(p.amount)||p.amount<=0)||plan.parts.reduce((n,p)=>n+p.amount,0)!==plan.total)throw Error('invalid');
    if(!Number.isFinite(Date.parse(plan.quote.expires))||Date.parse(plan.quote.expires)<=now().getTime())throw Error('quoteExpired96');
    await request(auth,'/checkout/plans',{operation_id:plan.id+'-plan',checkout_id:plan.id,cash_session_id:plan.cash,expected_total_cents:plan.total,items,tax_quote_operations:q.refs,parts:plan.parts.map(p=>({part_id:p.id,method:'cash',amount_cents:p.amount}))},signal);
    const view=await fiscal.bind({token,session:auth.session}).finalize(plan.id,{operation_id:plan.id,received_cents:plan.received,fiscal_invoice:q.invoice},signal);
    return result(auth,token,plan.id,signal,view);
   });
  },
  async status(token,id,signal){const auth=bind(token);return result(auth,token,id,signal);},
  async cancelPlan(token,id,reason,signal){
   const auth=bind(token);return exclusive(auth,id,async()=>{
    if(!validID(id)||typeof reason!=='string'||!reason.trim()||reason.length>512)throw Error('invalid');
    const view=await fiscal.bind({token,session:auth.session}).read(id,signal);auth.current();
    if(!view.plan)return;
    if(view.plan.completion)throw Error('conflict');
    const cancelled=await request(auth,'/checkout/plans/'+encodeURIComponent(id)+'/cancel',{operation_id:id+'-edit',reason},signal);
    if(cancelled.checkout_id!==id||cancelled.state!=='cancelled'||cancelled.completion)throw Error('invalidReply');
   });
  },
  charge:unavailable,chargeStatus:unavailable,cancelCharge:unavailable,
 };
}
