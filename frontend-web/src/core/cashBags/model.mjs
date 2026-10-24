export const states=['prepared','in_transit','received','rejected','cancelled'];
export const bagFail=()=>{throw Error('cashBagInvalid');};
export const equal=(a,b)=>JSON.stringify(a)===JSON.stringify(b);
export const keys=(v,names)=>!!v&&typeof v==='object'&&!Array.isArray(v)&&Object.keys(v).length===names.length&&names.every(k=>Object.hasOwn(v,k));
export const text=(v,n=128,empty=false)=>typeof v==='string'&&(empty||v.length>0)&&v===v.trim()&&new TextEncoder().encode(v).length<=n&&!/[\u0000-\u001f\u007f]/.test(v);
export const pathID=v=>text(v)&&/^[A-Za-z0-9][A-Za-z0-9._:-]*$/.test(v);
const reason=v=>typeof v==='string'&&!!v.trim()&&[...v].length<=500&&!v.includes('\0');
const integer=(v,min=0)=>Number.isSafeInteger(v)&&v>=min;
const time=v=>typeof v==='string'&&/^\d{4}-\d\d-\d\dT.*(?:Z|[+-]\d\d:\d\d)$/.test(v)&&Number.isFinite(Date.parse(v));
export function bagRecord(v,actor){
 if(!keys(v,['id','session_id','withdrawal_operation_id','movement_id','sender_id','operator_id','seal_id','created_at','updated_at','state','declared_cents','counted_cents','difference_cents','receiver_id','received_at','history'])||!pathID(v.id)||!['session_id','withdrawal_operation_id','movement_id','sender_id','operator_id','seal_id'].every(k=>text(v[k]))||!states.includes(v.state)||!time(v.created_at)||!time(v.updated_at)||!Array.isArray(v.history)||v.history.length<1||v.history.length>3)bagFail();
 if(v.state==='received'){
  if(!integer(v.declared_cents,1)||!integer(v.counted_cents)||!Number.isSafeInteger(v.difference_cents)||BigInt(v.counted_cents)-BigInt(v.declared_cents)!==BigInt(v.difference_cents)||!text(v.receiver_id)||v.receiver_id===v.sender_id||v.receiver_id===v.operator_id||!time(v.received_at))bagFail();
 }else if(v.counted_cents!==null||v.difference_cents!==null||v.receiver_id!==null||v.received_at!==null)bagFail();
 if(v.state!=='received'&&((actor===v.sender_id||actor===v.operator_id)?!integer(v.declared_cents,1):v.declared_cents!==null))bagFail();
 let state='',previous=v.created_at;const seen=new Set();
 for(let i=0;i<v.history.length;i++){
  const e=v.history[i];
  if(!keys(e,['operation_id','actor_id','from_state','to_state','reason','created_at','action'])||!pathID(e.operation_id)||seen.has(e.operation_id)||!text(e.actor_id)||!time(e.created_at)||Date.parse(e.created_at)<Date.parse(previous)||e.from_state!==state||typeof e.reason!=='string'||e.reason!==''&&!reason(e.reason))bagFail();
  const sender=e.actor_id===v.sender_id,independent=!sender&&e.actor_id!==v.operator_id;
  const valid=e.action==='create'?i===0&&state===''&&e.to_state==='prepared'&&sender&&e.created_at===v.created_at&&e.reason==='':e.action==='dispatch'?state==='prepared'&&e.to_state==='in_transit'&&sender&&e.reason==='':e.action==='cancel'?state==='prepared'&&e.to_state==='cancelled'&&sender&&!!e.reason:e.action==='reject'?state==='in_transit'&&e.to_state==='rejected'&&independent&&!!e.reason:e.action==='receive'?state==='in_transit'&&e.to_state==='received'&&independent&&v.receiver_id===e.actor_id&&v.received_at===e.created_at&&e.reason==='':false;
  if(!valid)bagFail();seen.add(e.operation_id);state=e.to_state;previous=e.created_at;
 }
 if(state!==v.state||previous!==v.updated_at)bagFail();return v;
}
export function bagPage(v,actor,filters){
 if(!keys(v,['items','limit','offset','total_count','has_more'])||!Array.isArray(v.items)||v.limit!==50||v.offset!==filters.offset||!integer(v.total_count)||v.items.length!==Math.min(50,Math.max(0,v.total_count-v.offset))||v.has_more!==(v.offset+v.items.length<v.total_count))bagFail();
 const seen=new Set();for(const b of v.items){bagRecord(b,actor);if(seen.has(b.id)||filters.state&&b.state!==filters.state||filters.session_id&&b.session_id!==filters.session_id)bagFail();seen.add(b.id);}return v;
}
export function bagCoverage(v,cash){
 const names=['session_id','scope','complete','withdrawal_count','withdrawn_cents','unassigned_cents','prepared_cents','in_transit_cents','received_declared_cents','received_counted_cents','divergent_count','cancelled_bags','rejected_bags','difference_cents','supplier_paid_cents','supplier_returned_cents'];
 if(!keys(v,names)||v.session_id!==cash||v.scope!=='audited_local_withdrawals'||v.complete!==true||!names.slice(3).every(k=>integer(v[k],k==='difference_cents'?-Number.MAX_SAFE_INTEGER:0)))bagFail();
 if(['unassigned_cents','prepared_cents','in_transit_cents','received_declared_cents','supplier_paid_cents'].reduce((sum,k)=>sum+BigInt(v[k]),0n)!==BigInt(v.withdrawn_cents)||BigInt(v.received_counted_cents)-BigInt(v.received_declared_cents)!==BigInt(v.difference_cents)||v.supplier_returned_cents>v.supplier_paid_cents||v.divergent_count>v.withdrawal_count)bagFail();return v;
}
export function bagBody(action,b){
 const fields=action==='create'?['operation_id','bag_id','withdrawal_operation_id','seal_id']:action==='receive'?['operation_id','seal_id','counted_cents']:['cancel','reject'].includes(action)?['operation_id','reason']:action==='dispatch'?['operation_id']:null;
 if(!fields||!keys(b,fields)||!pathID(b.operation_id))bagFail();
 if(action==='create'&&(!pathID(b.bag_id)||!text(b.withdrawal_operation_id)||!text(b.seal_id))||action==='receive'&&(!text(b.seal_id)||!integer(b.counted_cents))||['cancel','reject'].includes(action)&&!reason(b.reason))bagFail();return b;
}
export function allowedBagActions(b,actor){bagRecord(b,actor);return b.state==='prepared'&&b.sender_id===actor?['dispatch','cancel']:b.state==='in_transit'&&actor!==b.sender_id&&actor!==b.operator_id?['receive','reject']:[];}
