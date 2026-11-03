import {keys,pathID,equal,bagRecord} from '../cashBags/model.mjs';
export {keys,pathID,equal};
export const fail=()=>{throw Error('cashVaultInvalid');};
export const integer=(v,min=0)=>Number.isSafeInteger(v)&&v>=min;
export const text=(v,n=128,empty=false)=>typeof v==='string'&&(empty||v.length>0)&&v===v.trim()&&[...v].length<=n&&!v.includes('\0');
export const time=v=>typeof v==='string'&&/^\d{4}-\d\d-\d\dT.*(?:Z|[+-]\d\d:\d\d)$/.test(v)&&Number.isFinite(Date.parse(v));
export function decimalCents(s){if(typeof s!=='string'||!/^\d{1,14}([.,]\d{1,2})?$/.test(s.trim()))fail();const [a,b='']=s.trim().replace(',','.').split('.'),v=BigInt(a)*100n+BigInt(b.padEnd(2,'0'));if(v>BigInt(Number.MAX_SAFE_INTEGER))fail();return Number(v);}
export function vaultRecord(v){
 if(!keys(v,['id','name','created_by','created_at','updated_at','creation_operation_id','last_operation_id','state','balance_cents'])||!pathID(v.id)||!text(v.name,100)||!text(v.created_by)||!time(v.created_at)||!time(v.updated_at)||!pathID(v.creation_operation_id)||!pathID(v.last_operation_id)||!['active','archived'].includes(v.state)||!integer(v.balance_cents)||v.state==='archived'&&v.balance_cents!==0)fail();return v;
}
export function vaultEntry(e){
 if(!keys(e,['id','vault_id','operation_id','actor_id','reason','created_at','kind','delta_cents','bag_id','cash_session_id','cash_movement_id'])||!text(e.id)||!pathID(e.vault_id)||!pathID(e.operation_id)||!text(e.actor_id)||!text(e.reason,500,true)||!time(e.created_at)||!Number.isSafeInteger(e.delta_cents))fail();
 if(e.kind==='deposit'){if(e.delta_cents<0||!pathID(e.bag_id)||e.cash_session_id!==null||e.cash_movement_id!==null||e.reason!=='')fail();}
 else if(['transfer_out','transfer_in'].includes(e.kind)){if(e.bag_id!==null||e.cash_session_id!==null||e.cash_movement_id!==null||!text(e.reason,500)||(e.kind==='transfer_in'?e.delta_cents<=0:e.delta_cents>=0))fail();}
 else if(e.kind==='cash_supply'){if(e.delta_cents>=0||e.bag_id!==null||!text(e.cash_session_id)||!text(e.cash_movement_id)||!text(e.reason,500))fail();}else fail();return e;
}
export function countRecord(c){
 if(!keys(c,['id','vault_id','creator_id','counter_id','reason','state','expected_cents','counted_cents','difference_cents','vault_operation_id','reviewer_id','created_at','updated_at','last_operation_id','history'])||!['id','vault_id','vault_operation_id','last_operation_id'].every(k=>pathID(c[k]))||!text(c.creator_id)||!text(c.counter_id)||c.creator_id===c.counter_id||!text(c.reason,500)||!time(c.created_at)||!time(c.updated_at)||!Array.isArray(c.history)||c.history.length<1||c.history.length>3)fail();
 if(['counting','cancelled'].includes(c.state)){if(c.expected_cents!==null||c.counted_cents!==null||c.difference_cents!==null||c.reviewer_id!==null)fail();}
 else if(['submitted','accepted','rejected'].includes(c.state)){if(!integer(c.expected_cents)||!integer(c.counted_cents)||!Number.isSafeInteger(c.difference_cents)||BigInt(c.counted_cents)-BigInt(c.expected_cents)!==BigInt(c.difference_cents)||(c.state==='submitted'?c.reviewer_id!==null:!text(c.reviewer_id)||c.reviewer_id===c.counter_id))fail();}else fail();
 let state='',at=c.created_at;const seen=new Set();for(let i=0;i<c.history.length;i++){const e=c.history[i];if(!keys(e,['operation_id','actor_id','action','from_state','to_state','reason','created_at'])||!pathID(e.operation_id)||seen.has(e.operation_id)||!text(e.actor_id)||!time(e.created_at)||Date.parse(e.created_at)<Date.parse(at)||e.from_state!==state)fail();
  const valid=e.action==='create'?i===0&&state===''&&e.to_state==='counting'&&e.actor_id===c.creator_id&&e.reason===c.reason&&e.created_at===c.created_at:e.action==='submit'?state==='counting'&&e.to_state==='submitted'&&e.actor_id===c.counter_id&&e.reason==='':e.action==='cancel'?state==='counting'&&e.to_state==='cancelled'&&e.actor_id===c.creator_id&&text(e.reason,500):['accept','reject'].includes(e.action)?state==='submitted'&&e.to_state===(e.action==='accept'?'accepted':'rejected')&&e.actor_id===c.reviewer_id&&e.actor_id!==c.counter_id&&text(e.reason,500):false;
  if(!valid)fail();seen.add(e.operation_id);state=e.to_state;at=e.created_at;
 }
 if(state!==c.state||at!==c.updated_at||c.last_operation_id!==c.history.at(-1).operation_id)fail();return c;
}
export function countActions(c,actor){countRecord(c);return c.state==='counting'?(c.counter_id===actor?['submit']:c.creator_id===actor?['cancel']:[]):c.state==='submitted'&&c.counter_id!==actor?['accept','reject']:[];}
export function paged(v,offset,validate,extra=[]){
 if(!keys(v,['items','total_count','limit','offset','has_more',...extra])||!integer(v.total_count)||v.limit!==50||v.offset!==offset||!Array.isArray(v.items)||v.items.length!==Math.min(50,Math.max(0,v.total_count-offset))||v.has_more!==(offset+v.items.length<v.total_count))fail();const seen=new Set();for(const item of v.items){validate(item);if(seen.has(item.id))fail();seen.add(item.id);}return v;
}
export function statement(v,id,offset){vaultRecord(v.vault);if(v.vault.id!==id)fail();paged(v,offset,e=>{vaultEntry(e);if(e.vault_id!==id)fail();},['vault']);if(offset===0&&!v.has_more&&v.items.reduce((sum,e)=>sum+BigInt(e.delta_cents),0n)!==BigInt(v.vault.balance_cents))fail();return v;}
export function report(v){
 const counters=['vault_count','active_vault_count','archived_vault_count','counting_count','awaiting_review_count','divergent_latest_count'];
 if(!keys(v,['scope','generated_at','total_balance_cents',...counters,'items'])||v.scope!=='local_physical_vaults'||!time(v.generated_at)||!integer(v.total_balance_cents)||!counters.every(k=>integer(v[k])&&v[k]<=1000)||!Array.isArray(v.items)||v.items.length!==v.vault_count)fail();
 const sums=Object.fromEntries(counters.map(k=>[k,0]));let balance=0n;const seen=new Set();for(const item of v.items){if(!keys(item,['vault','latest_count']))fail();const vault=vaultRecord(item.vault);if(seen.has(vault.id))fail();seen.add(vault.id);balance+=BigInt(vault.balance_cents);sums.vault_count++;sums[vault.state==='active'?'active_vault_count':'archived_vault_count']++;
  if(item.latest_count!==null){const c=countRecord(item.latest_count);if(c.vault_id!==vault.id)fail();if(c.state==='counting')sums.counting_count++;if(c.state==='submitted')sums.awaiting_review_count++;if(c.difference_cents!==null&&c.difference_cents!==0)sums.divergent_latest_count++;if(['counting','submitted'].includes(c.state)&&(vault.state!=='active'||vault.last_operation_id!==c.vault_operation_id||c.expected_cents!==null&&c.expected_cents!==vault.balance_cents))fail();}
 }
 if(balance!==BigInt(v.total_balance_cents)||!counters.every(k=>sums[k]===v[k]))fail();return v;
}
export function bodyFor(resource,action,b){
 const names=resource==='vault'?({create:['operation_id','vault_id','name'],deposit:['operation_id','bag_id'],transfer:['operation_id','destination_id','amount_cents','reason'],supply:['operation_id','cash_session_id','amount_cents','reason'],archive:['operation_id','reason']})[action]:resource==='count'?({create:['operation_id','count_id','vault_id','counter_id','reason'],submit:['operation_id','counted_cents'],accept:['operation_id','reason'],reject:['operation_id','reason'],cancel:['operation_id','reason']})[action]:null;
 if(!names||!keys(b,names)||!pathID(b.operation_id))fail();for(const k of ['vault_id','count_id','bag_id','destination_id'])if(Object.hasOwn(b,k)&&!pathID(b[k]))fail();for(const k of ['counter_id','cash_session_id'])if(Object.hasOwn(b,k)&&!text(b[k]))fail();if(Object.hasOwn(b,'name')&&!text(b.name,100)||Object.hasOwn(b,'reason')&&!text(b.reason,500)||Object.hasOwn(b,'amount_cents')&&!integer(b.amount_cents,1)||Object.hasOwn(b,'counted_cents')&&!integer(b.counted_cents))fail();return b;
}
export function command(c,scope,actor){
 if(!keys(c,['version','scope','resource','action','id','body','before','uncertain'])||c.version!==1||c.scope!==scope||!pathID(c.id)||typeof c.uncertain!=='boolean')fail();bodyFor(c.resource,c.action,c.body);const b=c.body;
 if(c.resource==='count'){
  if(c.action==='create'){const v=vaultRecord(c.before);if(c.id!==b.count_id||v.id!==b.vault_id||v.state!=='active'||b.counter_id===actor)fail();}
  else{const previous=countRecord(c.before);if(c.id!==previous.id||!countActions(previous,actor).includes(c.action))fail();}
 }else if(c.action==='create'){if(c.id!==b.vault_id||c.before!==null)fail();}
 else{if(!keys(c.before,['source','destination','bag']))fail();const v=vaultRecord(c.before.source);if(v.id!==c.id||v.state!=='active')fail();if(c.action==='transfer'){const to=vaultRecord(c.before.destination);if(to.id!==b.destination_id||to.id===c.id||to.state!=='active'||v.balance_cents<b.amount_cents||BigInt(to.balance_cents)+BigInt(b.amount_cents)>BigInt(Number.MAX_SAFE_INTEGER))fail();}else if(c.before.destination!==null)fail();
  if(c.action==='deposit'){const bag=bagRecord(c.before.bag,actor);if(bag.id!==b.bag_id||bag.state!=='received'||bag.receiver_id!==actor||BigInt(v.balance_cents)+BigInt(bag.counted_cents)>BigInt(Number.MAX_SAFE_INTEGER))fail();}else if(c.before.bag!==null)fail();
  if(c.action==='supply'&&v.balance_cents<b.amount_cents||c.action==='archive'&&v.balance_cents!==0)fail();
 }return c;
}
export function receipt(r,c,actor){
 if(c.resource==='count'){
  if(!keys(r,['operation_id','repeated','count'])||typeof r.repeated!=='boolean'||r.operation_id!==c.body.operation_id)fail();const v=countRecord(r.count),e=v.history.at(-1);if(v.id!==c.id||e.operation_id!==c.body.operation_id||e.action!==c.action||e.actor_id!==actor||e.reason!==(c.body.reason||''))fail();
  if(c.action==='create'){if(v.creator_id!==actor||v.vault_id!==c.body.vault_id||v.counter_id!==c.body.counter_id||v.reason!==c.body.reason||v.history.length!==1)fail();}
  else{if(!['id','vault_id','creator_id','counter_id','reason','vault_operation_id','created_at'].every(k=>v[k]===c.before[k])||v.history.length!==c.before.history.length+1||!equal(v.history.slice(0,-1),c.before.history))fail();if(c.action==='submit'&&v.counted_cents!==c.body.counted_cents||c.before.counted_cents!==null&&['expected_cents','counted_cents','difference_cents'].some(k=>v[k]!==c.before[k]))fail();}return r;
 }
 if(!keys(r,['operation_id','kind','reason','created_at','repeated','vaults','entries','bag_id','cash_movement_id'])||r.operation_id!==c.body.operation_id||r.kind!==c.action||r.reason!==(c.body.reason||'')||!time(r.created_at)||typeof r.repeated!=='boolean'||!Array.isArray(r.vaults)||r.vaults.length!==(c.action==='transfer'?2:1)||!Array.isArray(r.entries)||r.entries.length!==({create:0,archive:0,deposit:1,supply:1,transfer:2})[c.action])fail();
 for(const v of r.vaults){vaultRecord(v);if(v.last_operation_id!==c.body.operation_id||v.updated_at!==r.created_at)fail();}if(r.vaults[0].id!==c.id||c.action==='transfer'&&r.vaults[1].id!==c.body.destination_id)fail();
 for(const e of r.entries){vaultEntry(e);if(e.operation_id!==r.operation_id||e.actor_id!==actor||e.created_at!==r.created_at||e.reason!==r.reason)fail();}
 if(c.action==='create'){const v=r.vaults[0];if(v.name!==c.body.name||v.created_by!==actor||v.balance_cents!==0||v.state!=='active'||v.creation_operation_id!==r.operation_id||v.created_at!==r.created_at)fail();}
 else{for(const [i,v] of r.vaults.entries()){const before=i===0?c.before.source:c.before.destination;if(!['id','name','created_by','created_at','creation_operation_id'].every(k=>v[k]===before[k])||v.state!==(c.action==='archive'?'archived':'active'))fail();}
  if(c.action==='deposit'){const e=r.entries[0];if(e.kind!=='deposit'||e.vault_id!==c.id||e.bag_id!==c.body.bag_id||e.delta_cents!==c.before.bag.counted_cents||r.bag_id!==c.body.bag_id)fail();}
  if(c.action==='transfer'){const [a,b]=r.entries;if(a.kind!=='transfer_out'||b.kind!=='transfer_in'||a.id===b.id||a.vault_id!==c.id||b.vault_id!==c.body.destination_id||a.delta_cents!==-c.body.amount_cents||b.delta_cents!==c.body.amount_cents)fail();}
  if(c.action==='supply'){const e=r.entries[0];if(e.kind!=='cash_supply'||e.vault_id!==c.id||e.delta_cents!==-c.body.amount_cents||e.cash_session_id!==c.body.cash_session_id||!text(r.cash_movement_id)||e.cash_movement_id!==r.cash_movement_id)fail();}
 }
 if(c.action!=='deposit'&&r.bag_id!==null||c.action!=='supply'&&r.cash_movement_id!==null)fail();return r;
}
