const fail=()=>{throw Error('receivingInvalid');};
export const receivingID=v=>typeof v==='string'&&v===v.trim()&&v.length>0&&new TextEncoder().encode(v).length<=128&&!/[\x00-\x1f\x7f/\\?#]/.test(v);
const reason=v=>typeof v==='string'&&v===v.trim()&&v.length>0&&new TextEncoder().encode(v).length<=255&&!/[\x00-\x1f\x7f]/.test(v);
const count=(v,min=0)=>Number.isSafeInteger(v)&&v>=min;
const units=['unit','kg','g','liter','ml','meter'];
const stamp=v=>typeof v==='string'&&/^\d{4}-\d{2}-\d{2}T/.test(v)&&Number.isFinite(Date.parse(v));
const exact=(v,keys)=>v&&Object.keys(v).length===keys.length&&keys.every(k=>Object.hasOwn(v,k));
export function quantityInput(v){if(typeof v!=='string'||!/^\d{1,13}([.,]\d{1,3})?$/.test(v))fail();const [a,b='']=v.replace(',','.').split('.'),n=BigInt(a)*1000n+BigInt(b.padEnd(3,'0'));if(n<1n||n>BigInt(Number.MAX_SAFE_INTEGER))fail();return Number(n);}
export function quantityText(v,locale){const b=BigInt(v),separator=new Intl.NumberFormat(locale).formatToParts(1.1).find(p=>p.type==='decimal').value;return new Intl.NumberFormat(locale).format(b/1000n)+separator+String(b%1000n).padStart(3,'0');}
export function receivingAccess(cap){const p=new Set(cap?.permissions||[]),m=new Set(cap?.license?.modules||[]),active=cap?.license?.state==='active'&&m.has('core')&&m.has('orders'),read=p.has('view_orders');return {read,history:read&&p.has('manage_stock'),authorize:read&&p.has('manage_replenishment')&&active,write:read&&p.has('manage_replenishment')&&p.has('manage_stock')&&active&&m.has('inventory')};}
export function receivingCommand(c){
 if(!exact(c,['version','kind','body','uncertain'])||c.version!==1||typeof c.uncertain!=='boolean')fail();
 const keys={authorize:['operation_id','order_id','reference','reason'],receipt:['operation_id','order_id','delivery_reference','location_id','unit','delivered_milli','accepted_milli','reason'],reject:['operation_id','order_id','delivery_reference','unit','delivered_milli','reason'],void:['operation_id','order_id','receipt_id','reason']}[c.kind];
 if(!keys||!exact(c.body,keys)||!keys.filter(k=>!['reason','unit','delivered_milli','accepted_milli'].includes(k)).every(k=>receivingID(c.body[k]))||!reason(c.body.reason))fail();
 if(['receipt','reject'].includes(c.kind)&&(!units.includes(c.body.unit)||!count(c.body.delivered_milli,1)))fail();
 if(c.kind==='receipt'&&(!count(c.body.accepted_milli,1)||c.body.accepted_milli>c.body.delivered_milli))fail();return c;
}
function event(v,order){if(!v||!['operation_id','device_id','actor_id'].every(k=>receivingID(v[k]))||!reason(v.reason)||!stamp(v.created_at)||v.order_id!==undefined&&v.order_id!==order)fail();return v;}
function record(v,kind,order){event(v,order);if(v.order_id!==order||!units.includes(v.unit)||!receivingID(v.product_id))fail();if(kind==='void'){if(!receivingID(v.receipt_id)||!count(v.quantity_milli,1))fail();}else if(!receivingID(v.id)||!receivingID(v.delivery_reference)||!count(v.delivered_milli,1))fail();if(kind==='receipt'&&(!count(v.accepted_milli,1)||v.accepted_milli>v.delivered_milli))fail();if(kind!=='reject'&&!['location_id','stock_operation_id','movement_id'].every(k=>receivingID(v[k])))fail();if(kind==='receipt'&&v.void){record(v.void,'void',order);if(v.void.receipt_id!==v.id||v.void.quantity_milli!==v.accepted_milli||['product_id','unit','location_id'].some(k=>v.void[k]!==v[k]))fail();}return v;}
function basePage(v,order,offset){if(!v||v.order_id!==order||!['local_not_sent','cancelled'].includes(v.commercial_status)||!['not_authorized','authorized','partially_received','received'].includes(v.receiving_status)||!receivingID(v.product_id)||!units.includes(v.unit)||!count(v.total_count)||v.limit!==50||v.offset!==offset||!Array.isArray(v.items)||v.items.length!==Math.min(50,Math.max(0,v.total_count-offset))||v.has_more!==(offset+v.items.length<v.total_count))fail();}
export function receivingTrace(v,order){
 basePage(v,order,0);if(!['planned_milli','delivered_milli','accepted_milli','rejected_milli','remaining_milli','voided_count'].every(k=>count(v[k]))||v.planned_milli<1||BigInt(v.accepted_milli)+BigInt(v.remaining_milli)!==BigInt(v.planned_milli)||BigInt(v.accepted_milli)+BigInt(v.rejected_milli)!==BigInt(v.delivered_milli)||typeof v.voided_accepted_milli_exact!=='string'||!/^(0|[1-9][0-9]*)$/.test(v.voided_accepted_milli_exact))fail();
 if(v.authorization!==null){event(v.authorization,order);if(!receivingID(v.authorization.reference))fail();}else if(v.receiving_status!=='not_authorized')fail();
 const seen=new Set();for(const r of v.items){record(r,'receipt',order);if(r.product_id!==v.product_id||r.unit!==v.unit||seen.has(r.id))fail();seen.add(r.id);}return v;
}
export function receivingHistory(v,order,offset=0){
 basePage(v,order,offset);const t=v.totals;if(!t||!['planned_milli','effective_accepted_milli','effective_delivered_milli','partial_rejected_milli','remaining_milli'].every(k=>count(t[k]))||t.planned_milli<1||BigInt(t.effective_accepted_milli)+BigInt(t.remaining_milli)!==BigInt(t.planned_milli)||BigInt(t.effective_accepted_milli)+BigInt(t.partial_rejected_milli)!==BigInt(t.effective_delivered_milli))fail();
 if(!['fully_rejected_milli_exact','voided_accepted_milli_exact','recorded_accepted_milli_exact'].every(k=>typeof t[k]==='string'&&/^(0|[1-9][0-9]*)$/.test(t[k]))||BigInt(t.recorded_accepted_milli_exact)-BigInt(t.voided_accepted_milli_exact)!==BigInt(t.effective_accepted_milli))fail();
 const seen=new Set();for(const e of v.items){const kind={received:'receipt',rejected:'reject',voided:'void'}[e.kind],r=e[{received:'receipt',rejected:'rejection',voided:'void'}[e.kind]];if(!kind||!receivingID(e.id)||!stamp(e.created_at)||!r)fail();record(r,kind,order);if(e.created_at!==r.created_at||e.id!==(kind==='void'?r.receipt_id:r.id)||r.product_id!==v.product_id||r.unit!==v.unit||seen.has(e.kind+':'+e.id))fail();seen.add(e.kind+':'+e.id);}return v;
}
export function receivingResult(v,c,actor,device){
 receivingCommand(c);if(!v||typeof v.repeated!=='boolean')fail();const b=c.body,r=v[{authorize:'authorization',receipt:'receipt',reject:'rejection',void:'void'}[c.kind]];
 if(c.kind==='authorize'){event(r,b.order_id);if(v.order_id!==b.order_id||r.operation_id!==b.operation_id||r.reference!==b.reference||r.reason!==b.reason)fail();}else {record(r,c.kind,b.order_id);if(Object.keys(b).some(k=>r[k]!==b[k]))fail();}
 if(r.actor_id!==actor||r.device_id!==device)fail();return v;
}
export function receivingReceipt(v,c,actor,device){if(!v||v.kind!==c.kind||!v.input||Object.keys(v.input).length!==Object.keys(c.body).length||Object.keys(c.body).some(k=>v.input[k]!==c.body[k]))fail();receivingResult(v.result,c,actor,device);return v;}
