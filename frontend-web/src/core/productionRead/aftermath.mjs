const fail=()=>{throw Error('productionInvalid');};
const bytes=s=>new TextEncoder().encode(s).length;
const id=s=>typeof s==='string'&&s.length>0&&s.trim()===s&&bytes(s)<=128&&!/[\x00\r\n]/.test(s);
const int=(v,min=0,max=Number.MAX_SAFE_INTEGER)=>Number.isSafeInteger(v)&&v>=min&&v<=max;
const text=(s,max=255)=>typeof s==='string'&&s.length>0&&s.trim()===s&&bytes(s)<=max&&!/[\x00\r\n]/.test(s);
const code=s=>text(s,64)&&!/[\x00-\x1f\x7f]/.test(s);
const time=s=>typeof s==='string'&&Number.isFinite(Date.parse(s));
const exact=(o,keys)=>o&&typeof o==='object'&&!Array.isArray(o)&&Object.keys(o).length===keys.length&&keys.every(k=>Object.hasOwn(o,k));
const units=['unit','kg','g','liter','ml','meter'];
export const aftermathPaths={loss:'production/losses',loss_void:'production/losses/void',lot:'production/lots',lot_void:'production/lots/void',quality:'production/quality-reviews'};
export function productionCalendarDate(s){if(typeof s!=='string'||!/^\d{4}-\d{2}-\d{2}$/.test(s)||s<'0001-01-01')return false;const d=new Date(s+'T00:00:00Z');return Number.isFinite(d.getTime())&&d.toISOString().slice(0,10)===s;}
const dates=b=>productionCalendarDate(b.manufactured_on)&&typeof b.expires_on==='string'&&(b.expires_on===''||productionCalendarDate(b.expires_on)&&b.expires_on>=b.manufactured_on);
export function aftermathInput(kind,b){
 if(!Object.hasOwn(aftermathPaths,kind))return null;
 if(!b||!id(b.operation_id)||!text(b.reason))fail();
 if(kind==='loss'){
  if(!exact(b,['operation_id','loss_id','result_id','quantity_milli','reason'])||!id(b.loss_id)||!id(b.result_id)||!int(b.quantity_milli,1))fail();
 }else if(kind==='lot'){
  if(!exact(b,['operation_id','lot_id','result_id','quantity_milli','code','manufactured_on','expires_on','reason'])||!id(b.lot_id)||!id(b.result_id)||!int(b.quantity_milli,1)||!code(b.code)||!dates(b))fail();
 }else if(kind==='quality'){
  if(!exact(b,['operation_id','lot_id','expected_revision','status','criterion','reason'])||!id(b.lot_id)||!int(b.expected_revision,0,2147483646)||!['passed','failed'].includes(b.status)||!text(b.criterion))fail();
 }else{const field=kind==='lot_void'?'lot_id':'loss_id';if(!exact(b,['operation_id',field,'expected_revision','reason'])||!id(b[field])||b.expected_revision!==1)fail();}
 return {...b};
}
export function aftermathResult(kind,b,r){
 if(!r||typeof r.repeated!=='boolean')fail();
 if(kind==='quality'){
  if(!exact(r,['lot_id','revision','status','repeated'])||r.lot_id!==b.lot_id||r.revision!==b.expected_revision+1||r.status!==b.status)fail();
 }else{const field=kind.startsWith('lot')?'lot_id':'loss_id',voided=kind.endsWith('_void');if(!exact(r,[field,'result_id','status','revision','repeated'])||r[field]!==b[field]||!id(r.result_id)||(!voided&&r.result_id!==b.result_id)||r.status!==(voided?'voided':'recorded')||r.revision!==(voided?2:1))fail();}
 return r;
}
export function productionAftermathRow(v,kind,reference){
 if(!v||!['id','result_id','product_id','created_by'].every(k=>id(v[k]))||v.result_id!==reference.result_id||v.product_id!==reference.product_id||v.unit!==reference.unit||!units.includes(v.unit)||!int(v.quantity_milli,1)||(v.unit==='unit'&&v.quantity_milli%1000!==0)||!text(v.reason)||!['recorded','voided'].includes(v.status)||v.revision!==(v.status==='recorded'?1:2)||!time(v.created_at)||!time(v.updated_at))fail();
 if(kind==='lot'&&(!code(v.code)||!dates(v)))fail();
 return v;
}
export function productionAftermathSummary(v,kind,reference){
 if(!reference||!id(reference.result_id)||!id(reference.product_id)||!units.includes(reference.unit)||!int(reference.produced_milli)||!int(reference.planned_milli,1)||reference.produced_milli>reference.planned_milli||reference.shortfall_milli!==reference.planned_milli-reference.produced_milli)fail();
 if(!v||v.result_id!==reference.result_id||v.product_id!==reference.product_id||v.unit!==reference.unit||v.produced_milli!==reference.produced_milli||!Array.isArray(v.items)||v.items.length>50||new Set(v.items.map(i=>i?.id)).size!==v.items.length)fail();
 const recorded=kind==='lot'?v.assigned_milli:v.recorded_loss_milli,remaining=kind==='lot'?v.unassigned_milli:v.unclassified_shortfall_milli,total=kind==='lot'?reference.produced_milli:reference.shortfall_milli;
 if(!int(recorded)||!int(remaining)||BigInt(recorded)+BigInt(remaining)!==BigInt(total)||(kind==='loss'&&(v.planned_milli!==reference.planned_milli||v.shortfall_milli!==reference.shortfall_milli))||(v.unit==='unit'&&(recorded%1000!==0||remaining%1000!==0)))fail();
 let page=0n;for(const row of v.items){productionAftermathRow(row,kind,reference);if(row.status==='recorded')page+=BigInt(row.quantity_milli);if(row.quantity_milli>total)fail();}if(page>BigInt(recorded))fail();
 return v;
}
export function productionDeclarationHistory(v){if(!v||!Array.isArray(v.items)||v.items.length<1||v.items.length>2||!v.items.every((i,index)=>i&&['operation_id','device_id','actor_id'].every(k=>id(i[k]))&&i.revision===index+1&&i.kind===(index===0?'recorded':'voided')&&text(i.reason)&&time(i.created_at)))fail();return v.items;}
function review(v,lot){
 if(!v||v.lot_id!==lot.id||!int(v.revision,1,2147483647)||!['operation_id','device_id','actor_id'].every(k=>id(v[k]))||!['passed','failed'].includes(v.status)||!text(v.criterion)||!text(v.reason)||!time(v.created_at))fail();
 const snapshot=productionAftermathRow(v.lot_snapshot,'lot',lot);
 if(snapshot.id!==lot.id||snapshot.status!=='recorded'||snapshot.revision!==1||!['code','quantity_milli','manufactured_on','expires_on','reason','created_by','created_at'].every(k=>snapshot[k]===lot[k]))fail();
 return v;
}
export function productionQuality(v,lot){
 if(!v||v.lot_id!==lot.id||v.lot_status!==lot.status||!int(v.revision,0,2147483647))fail();
 if(v.revision===0){if(v.status!=='not_assessed'||v.latest!==null)fail();}
 else{const last=review(v.latest,lot);if(last.revision!==v.revision||last.status!==v.status)fail();}
 return v;
}
export function productionQualityHistory(v,lot,offset){if(!v||!Array.isArray(v.items)||v.items.length>50)fail();for(const [i,row] of v.items.entries()){review(row,lot);if(row.revision!==offset+i+1)fail();}return v.items;}
