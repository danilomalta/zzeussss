// Front-end planning rules. No function in this module changes commercial records.
export const sections83=['closing','journal','returns','inventory','quality','supply','opportunities','performance','devices'];
export const cents83=value=>{const s=String(value).trim();if(!/^\d{1,10}([.,]\d{1,2})?$/.test(s))throw Error('invalid');const [a,b='']=s.replace(',','.').split('.');return Number(a)*100+Number(b.padEnd(2,'0'));};
export function quantity83(value,signed=false){const s=String(value).trim();if(!(signed?/^-?\d{1,9}([.,]\d{1,3})?$/:/^\d{1,9}([.,]\d{1,3})?$/).test(s))throw Error('invalid');const negative=s.startsWith('-'),[a,b='']=s.replace('-','').replace(',','.').split('.');return (Number(a)*1000+Number(b.padEnd(3,'0')))*(negative?-1:1);}
export function scope83(context){const keys=['tenant_id','store_id','identity_id','device_id'];if(keys.some(k=>!context?.[k]||typeof context[k]!=='string'))throw Error('context');return JSON.stringify(keys.map(k=>context[k]));}
export function can83(cap,area){
 if(!cap||cap.license?.state!=='active')return false;
 const m=new Set(cap.license.modules),p=new Set(cap.permissions);
 const has=(mod,permission)=>m.has(mod)&&p.has(permission);
 if(['closing','journal','returns'].includes(area))return has('pos','sell');
 if(area==='inventory')return has('inventory','manage_stock');
 if(area==='quality')return has('production','manage_production');
 if(['supply','opportunities'].includes(area))return has('orders','view_orders');
 if(area==='performance')return cap.role==='owner'&&(m.has('inventory')||m.has('production'));
 return area==='devices'&&cap.role==='owner';
}
export const approves83=cap=>['owner','manager'].includes(cap?.role);
export function reconciliation83(sales,evidence){
 const seen=new Set();
 return sales.map(sale=>{
  let confirmed=0,pending=0;const issues=[];
  for(const part of sale.payments){
   const candidates=evidence.filter(e=>e.paymentId===part.id&&e.method===part.method);
   if(!candidates.length){pending+=part.amount;continue;}
   const e=candidates[0],key=e.provider+'|'+e.terminal+'|'+e.reference;
   if(seen.has(key)){issues.push('duplicate');pending+=part.amount;continue;}seen.add(key);
   if(e.amount!==part.amount){issues.push('amount');pending+=part.amount;continue;}
   if(!e.confirmed){issues.push('pending');pending+=part.amount;continue;}
   confirmed+=e.amount;
  }
  return {id:sale.id,total:sale.total,confirmed,pending,balanced:confirmed===sale.total&&pending===0&&!issues.length,issues};
 });
}
export function countCash83(counts){return [20000,10000,5000,2000,1000,500,200,100,50,25,10,5].reduce((sum,n)=>{const q=Number(counts[n]||0);if(!Number.isSafeInteger(q)||q<0||q>100000)throw Error('invalid');return sum+n*q;},0);}
export function inventoryAdjustment83(expectedAtCount,counted,movements){if(![expectedAtCount,counted,movements].every(Number.isSafeInteger)||expectedAtCount<0||counted<0)throw Error('invalid');const difference=counted-expectedAtCount,proposed=counted+movements,expectedNow=expectedAtCount+movements;if(![difference,proposed,expectedNow].every(Number.isSafeInteger)||proposed<0||expectedNow<0)throw Error('invalid');return {difference,proposed,expectedNow};}
export function refundImpact83(refund,unrecoverableCost,recovered=0,fees=0){if([refund,unrecoverableCost,recovered,fees].some(x=>!Number.isSafeInteger(x)||x<0)||recovered>unrecoverableCost+fees)throw Error('invalid');return {refund,loss:unrecoverableCost+fees-recovered};}
export function production83(stock,perBatch,outputPerBatch,lossBasisPoints=0){if(![stock,perBatch,outputPerBatch,lossBasisPoints].every(Number.isSafeInteger)||stock<0||perBatch<=0||outputPerBatch<=0||lossBasisPoints<0||lossBasisPoints>10000)throw Error('invalid');const batches=Math.floor(stock/perBatch),gross=BigInt(batches)*BigInt(outputPerBatch),net=gross*BigInt(10000-lossBasisPoints)/10000n;if(gross>BigInt(Number.MAX_SAFE_INTEGER))throw Error('overflow');return {batches,gross:Number(gross),net:Number(net)};}
export function supplierAction83(rejections){return rejections>=3?'visit':'review';}
export function compareCohort83(a,b){return !!a&&!!b&&a.band===b.band&&a.region===b.region&&a.activity===b.activity&&a.period===b.period&&a.consent===true&&b.consent===true;}
export function transfer83(remaining,quantity,originLocked){if(!Number.isSafeInteger(remaining)||!Number.isSafeInteger(quantity)||quantity<=0||quantity>remaining)throw Error('invalid');return originLocked?{source:remaining-quantity,destination:quantity,state:'preview'}:{source:remaining,destination:0,state:'awaiting_origin'};}
export function append83(events,event){const previous=events.find(e=>e.id===event.id);if(previous){if(JSON.stringify(previous)!==JSON.stringify(event))throw Error('integrity');return events;}return [...events,Object.freeze({...event})];}
export function transition83(kind,state,action,manager=false){
 const map={bag:{draft:{seal:'sealed'},sealed:{deliver:'in_transit'},in_transit:{receive:'received'},received:{verify:'verified'}},returns:{requested:{approve:'approved',reject:'rejected'},approved:{requestRefund:'refund_requested'},refund_requested:{confirmRefund:'refunded'}},inventory:{draft:{submit:'submitted'},submitted:{approve:'approved',recount:'draft'}},quality:{quarantine:{release:'released',reject:'rejected'}},supply:{suggested:{accept:'accepted',reject:'rejected'},accepted:{supplierConfirm:'confirmed'},confirmed:{dispatch:'dispatched'},dispatched:{receive:'received'}}};
 if(['approve','release','reject','verify','confirmRefund'].includes(action)&&!manager)throw Error('permission');
 const next=map[kind]?.[state]?.[action];if(!next)throw Error('transition');return next;
}
