import {keys,pathID,text,bagFail} from './model.mjs';
const reason=v=>typeof v==='string'&&!!v.trim()&&[...v].length<=500&&!v.includes('\0');
const safe=n=>Number.isSafeInteger(n)&&n>=0;
export function reviewRecord(v,id){
 if(!keys(v,['bag_id','revision','state','original_counted_cents','declared_cents','counted_cents','difference_cents','history'])||v.bag_id!==id||!pathID(id)||!safe(v.revision)||!['pending','investigating','resolved'].includes(v.state)||![v.original_counted_cents,v.declared_cents,v.counted_cents].every(safe)||!Number.isSafeInteger(v.difference_cents)||v.difference_cents!==v.counted_cents-v.declared_cents||!Array.isArray(v.history)||v.history.length!==v.revision)bagFail();
 let counted=v.original_counted_cents,state='pending';const operations=new Set();
 for(const [i,e] of v.history.entries()){
  if(!keys(e,['operation_id','revision','actor_id','action','reason','counted_cents','delta_cents','created_at'])||!pathID(e.operation_id)||operations.has(e.operation_id)||!text(e.actor_id)||e.revision!==i+1||!['investigate','recount','resolve'].includes(e.action)||!reason(e.reason)||!safe(e.counted_cents)||!Number.isSafeInteger(e.delta_cents)||e.delta_cents!==e.counted_cents-counted||!text(e.created_at)||!Number.isFinite(Date.parse(e.created_at))||state==='resolved'||e.action!=='recount'&&e.delta_cents!==0||e.action==='resolve'&&state!=='investigating')bagFail();
  operations.add(e.operation_id);counted=e.counted_cents;state=e.action==='resolve'?'resolved':'investigating';
 }
 if(counted!==v.counted_cents||state!==v.state)bagFail();return v;
}
export function reviewBody(v){if(!keys(v,['operation_id','expected_revision','action','counted_cents','reason'])||!pathID(v.operation_id)||!safe(v.expected_revision)||!safe(v.counted_cents)||!['investigate','recount','resolve'].includes(v.action)||!reason(v.reason))bagFail();return v;}
