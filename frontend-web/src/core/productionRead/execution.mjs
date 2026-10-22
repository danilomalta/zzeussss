const fail=()=>{throw Error('productionInvalid');};
const bytes=s=>new TextEncoder().encode(s).length;
const id=s=>typeof s==='string'&&s.length>0&&s.trim()===s&&bytes(s)<=128&&!/[\x00\r\n]/.test(s);
const int=(n,min=0,max=Number.MAX_SAFE_INTEGER)=>Number.isSafeInteger(n)&&n>=min&&n<=max;
const text=(s,max=255)=>typeof s==='string'&&s.length>0&&s.trim()===s&&bytes(s)<=max&&!/[\x00\r\n]/.test(s);
const exact=(o,keys)=>o&&typeof o==='object'&&!Array.isArray(o)&&Object.keys(o).length===keys.length&&keys.every(k=>Object.hasOwn(o,k));
const time=s=>typeof s==='string'&&Number.isFinite(Date.parse(s));
const units=['unit','kg','g','liter','ml','meter'];
export const executionPaths={reserve:'production/material-reservations',materials:'production/material-reservations/state',result:'production/results',stage_plan:'production/stage-plans',stage_state:'production/stages/state'};
export function executionInput(kind,b){
 if(!Object.hasOwn(executionPaths,kind))return null;
 if(!b||!id(b.operation_id)||!text(b.reason))fail();
 if(kind==='reserve'){
  if(!exact(b,['operation_id','reservation_id','order_id','reason'])||!id(b.reservation_id)||!id(b.order_id))fail();
 }else if(kind==='materials'){
  if(!exact(b,['operation_id','reservation_id','action','reason'])||!id(b.reservation_id)||!['release','consume'].includes(b.action))fail();
 }else if(kind==='result'){
  if(!exact(b,['operation_id','result_id','order_id','expected_revision','produced_milli','reason'])||!id(b.result_id)||!id(b.order_id)||!int(b.expected_revision,1,2147483646)||!int(b.produced_milli))fail();
 }else if(kind==='stage_plan'){
  if(!exact(b,['operation_id','order_id','stages','reason'])||!id(b.order_id)||!Array.isArray(b.stages)||b.stages.length<1||b.stages.length>20||!b.stages.every(s=>exact(s,['stage_id','name','responsible_id'])&&id(s.stage_id)&&id(s.responsible_id)&&text(s.name,120))||new Set(b.stages.map(s=>s.stage_id)).size!==b.stages.length)fail();
 }else if(!exact(b,['operation_id','order_id','stage_id','expected_revision','status','reason'])||!id(b.order_id)||!id(b.stage_id)||!['running','completed'].includes(b.status)||b.expected_revision!==(b.status==='running'?1:2))fail();
 return structuredClone(b);
}
export function executionResult(kind,b,r){
 if(!r||typeof r.repeated!=='boolean')fail();
 if(kind==='reserve'||kind==='materials'){
  if(!exact(r,['reservation_id','order_id','status','repeated'])||r.reservation_id!==b.reservation_id||!id(r.order_id)||(kind==='reserve'&&r.order_id!==b.order_id)||r.status!==(kind==='reserve'?'active':b.action==='release'?'released':'consumed'))fail();
 }else if(kind==='result'){
  if(!exact(r,['result_id','order_id','reservation_id','product_id','location_id','unit','planned_milli','produced_milli','shortfall_milli','revision','status','repeated'])||r.result_id!==b.result_id||r.order_id!==b.order_id||!['reservation_id','product_id','location_id'].every(k=>id(r[k]))||!units.includes(r.unit)||!int(r.planned_milli,1)||r.produced_milli!==b.produced_milli||r.produced_milli>r.planned_milli||r.shortfall_milli!==r.planned_milli-r.produced_milli||r.revision!==b.expected_revision+1||r.status!=='completed'||(r.unit==='unit'&&r.produced_milli%1000!==0))fail();
 }else if(kind==='stage_plan'){
  if(!exact(r,['order_id','revision','status','repeated'])||r.order_id!==b.order_id||r.revision!==1||r.status!=='configured')fail();
 }else if(kind==='stage_state'){
  if(!exact(r,['order_id','stage_id','revision','status','repeated'])||r.order_id!==b.order_id||r.stage_id!==b.stage_id||r.revision!==b.expected_revision+1||r.status!==b.status)fail();
 }else fail();
 return r;
}
export function productionExecutionTrace(v,order){
 if(!v||v.order.id!==order.id||!Array.isArray(v.ingredients)||v.ingredients.length!==order.recipe.ingredients.length||new Set(v.ingredients.map(i=>i?.product_id)).size!==v.ingredients.length||!v.materials||!['active_count','consumed_count','released_count'].every(k=>int(v.materials[k]))||v.materials.active_count+v.materials.consumed_count>1)fail();
 const current=v.materials.current;
 if(current===null){if(v.materials.active_count||v.materials.consumed_count)fail();}
 else{
  if(!current||!id(current.id)||current.order_id!==order.id||current.location_id!==order.location_id||!['active','consumed'].includes(current.status)||!id(current.created_by)||!time(current.created_at)||!time(current.updated_at)||v.materials.active_count!==(current.status==='active'?1:0)||v.materials.consumed_count!==(current.status==='consumed'?1:0)||!Array.isArray(current.items)||current.items.length!==v.ingredients.length||new Set(current.items.map(i=>i?.product_id)).size!==current.items.length)fail();
 }
 for(const i of v.ingredients){
  const line=order.recipe.ingredients.find(l=>l.product_id===i?.product_id);
  if(!line||i.unit!==line.unit||!int(i.planned_milli,1)||BigInt(i.planned_milli)!==BigInt(line.quantity_milli)*BigInt(order.planned_batches)||i.reserved_milli!==(current?.status==='active'?i.planned_milli:0)||i.consumed_milli!==(current?.status==='consumed'?i.planned_milli:0))fail();
  if(current){const m=current.items.find(m=>m.product_id===i.product_id);if(!m||m.unit!==i.unit||m.quantity_milli!==i.planned_milli)fail();}
 }
 if(v.stages!==null){const s=v.stages;if(!s||s.order_id!==order.id||!id(s.created_by)||!time(s.created_at)||!Array.isArray(s.items)||s.items.length<1||s.items.length>20||new Set(s.items.map(i=>i?.stage_id)).size!==s.items.length)fail();let incomplete=false;for(const [index,item] of s.items.entries()){if(!item||!id(item.stage_id)||!id(item.responsible_id)||!text(item.name,120)||item.position!==index+1||!['pending','running','completed'].includes(item.status)||item.revision!==({pending:1,running:2,completed:3}[item.status])||!time(item.updated_at)||(incomplete&&item.status!=='pending'))fail();if(item.status!=='completed')incomplete=true;}}
 if(v.result===null){if(order.status==='completed')fail();}
 else{
  const r=v.result;
  if(!r||order.status!=='completed'||current?.status!=='consumed'||r.reservation_id!==current.id||r.result_id!==order.completion_id||r.order_id!==order.id||r.product_id!==order.recipe.output_product_id||r.location_id!==order.location_id||r.unit!==order.recipe.output_unit||r.planned_milli!==order.planned_output_milli||!int(r.produced_milli)||r.produced_milli>r.planned_milli||r.shortfall_milli!==r.planned_milli-r.produced_milli||r.revision!==order.revision||r.status!=='completed'||r.repeated!==false||!['result_id','operation_id','device_id','actor_id'].every(k=>id(r[k]))||!text(r.reason)||!time(r.created_at)||(r.produced_milli>0?!id(r.movement_id):!!r.movement_id)||(r.unit==='unit'&&r.produced_milli%1000!==0)||v.stages?.items.some(s=>s.status!=='completed'))fail();
 }
 return {order,ingredients:v.ingredients,materials:v.materials,stages:v.stages,result:v.result};
}
