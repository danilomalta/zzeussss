import {LocalAPIError} from './localClient.mjs';
const safe=n=>Number.isSafeInteger(n),positive=n=>safe(n)&&n>=0;
export function validateOfficial101(v,id){
 if(!v||v.session_id!==id||v.complete!==true||typeof v.operation_id!=='string'||!v.operation_id||!Number.isFinite(Date.parse(v.opened_at))||!Number.isFinite(Date.parse(v.closed_at))||Date.parse(v.closed_at)<Date.parse(v.opened_at)||v.bank_settlement_available!==false||v.financial_ledger_available!==false)throw Error('invalid101');
 const c=v.cash;if(!c||!['opening_cents','expected_cents','declared_cents','supply_cents','withdrawal_cents'].every(k=>positive(c[k]))||!safe(c.movement_net_cents)||!safe(c.difference_cents))throw Error('invalid101');
 if(BigInt(c.opening_cents)+BigInt(c.movement_net_cents)!==BigInt(c.expected_cents)||BigInt(c.declared_cents)-BigInt(c.expected_cents)!==BigInt(c.difference_cents))throw Error('invalid101');
 const hasReturns=Object.hasOwn(v,'returned_sales_cents'),hasRefund=Object.hasOwn(c,'customer_refund_cents');if(hasReturns!==hasRefund||hasReturns&&(!positive(v.returned_sales_cents)||!positive(c.customer_refund_cents)||v.returned_sales_cents!==c.customer_refund_cents))throw Error('invalid101');
 const returned=hasReturns?BigInt(v.returned_sales_cents):0n;
 if(!['sale_count','gross_sales_cents','cancelled_sales_cents','net_sales_cents'].every(k=>positive(v[k]))||BigInt(v.net_sales_cents)+BigInt(v.cancelled_sales_cents)+returned!==BigInt(v.gross_sales_cents))throw Error('invalid101');
 if(!Array.isArray(v.payments)||v.payments.length!==4)throw Error('invalid101');
 const seen=new Set();let recorded=0n,reversed=0n;
 for(const p of v.payments){if(!p||!['cash','card','pix','other'].includes(p.method)||seen.has(p.method)||!['parts','recorded_confirmed_cents','recorded_pending_cents','recorded_reversed_cents'].every(k=>positive(p[k])))throw Error('invalid101');seen.add(p.method);recorded+=BigInt(p.recorded_confirmed_cents)+BigInt(p.recorded_pending_cents);reversed+=BigInt(p.recorded_reversed_cents);}
 if(recorded!==BigInt(v.net_sales_cents)+returned||reversed!==BigInt(v.cancelled_sales_cents)||returned>BigInt(v.payments.find(p=>p.method==='cash').recorded_confirmed_cents))throw Error('invalid101');
 return v;
}
export function createOfficial101(fetcher=globalThis.fetch){return async(token,id,signal)=>{
 if(typeof token!=='string'||!token||typeof id!=='string'||!id||id.length>128||id.trim()!==id||/[\u0000\r\n]/.test(id))throw Error('invalid101');
 const controller=new AbortController(),abort=()=>controller.abort();if(signal?.aborted)controller.abort();signal?.addEventListener('abort',abort,{once:true});const timeout=setTimeout(abort,10000);
 try{const r=await fetcher('/local/v1/cash/sessions/'+encodeURIComponent(id)+'/report',{method:'GET',credentials:'omit',cache:'no-store',headers:{Authorization:'Bearer '+token},signal:controller.signal});if(!r.ok)throw new LocalAPIError(r.status,'report101');return validateOfficial101(await r.json(),id);}finally{clearTimeout(timeout);signal?.removeEventListener('abort',abort);}
};}
