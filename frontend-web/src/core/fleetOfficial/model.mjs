import fleet from './fleet-maintenance.schemas.mjs';
import workshop from './fleet-workshop.schemas.mjs';
const schemas={...fleet,...workshop};
export const fleetResources=Object.freeze({assets:['asset','Asset'],requests:['request','Request'],trips:['trip','Trip'],plans:['plan','Plan'],providers:['provider','Provider'],quotes:['quote','Quote'],expenses:['expense','Amount'],income:['income','Amount'],impacts:['impact','Amount'],profiles:['profile','Profile'],'workshop/templates':['template','CheckTemplate'],'workshop/requirements':['binding','CheckBinding'],'workshop/inspections':['inspection','Inspection'],'workshop/labor':['labor','Labor']});
export const financialResources=Object.freeze(['quotes','expenses','income','impacts','profiles']);
export function fleetFail(){throw Error('fleetInvalid');}
const object=v=>v!==null&&typeof v==='object'&&!Array.isArray(v);
const natural=v=>Number.isSafeInteger(v)&&v>=0;
export function fleetID(v){return typeof v==='string'&&v.length>0&&v.trim()===v&&!/[\u0000-\u001f\u007f/\uD800-\uDFFF]/u.test(v)&&new TextEncoder().encode(v).length<=128;}
// Date.parse validates the instant, while the calendar comparison rejects normalized dates.
export function fleetStamp(v){if(typeof v!=='string'||!/^\d{4}-\d\d-\d\dT\d\d:\d\d:\d\d(?:\.\d{1,9})?Z$/.test(v)||!Number.isFinite(Date.parse(v)))return false;return new Date(v).toISOString().slice(0,19)===v.slice(0,19);}
function matches(s,v,depth=0){
 if(!s||depth>25)return false;
 if(v===null)return s.nullable===true;
 if(s.$ref)return matches(schemas[s.$ref.split('/').pop()],v,depth+1);
 if(s.allOf&&!s.allOf.every(x=>matches(x,v,depth+1)))return false;
 if(s.oneOf&&s.oneOf.filter(x=>matches(x,v,depth+1)).length!==1)return false;
 if(s.enum&&!s.enum.includes(v))return false;
 switch(s.type){
 case 'object':return object(v)&&(s.required||[]).every(k=>Object.hasOwn(v,k))&&Object.keys(v).every(k=>Object.hasOwn(s.properties,k)&&matches(s.properties[k],v[k],depth+1));
 case 'array':return Array.isArray(v)&&v.length<=5000&&v.every(x=>matches(s.items,x,depth+1));
 case 'string':return typeof v==='string'&&v.length<=32768&&(!s.minLength||v.length>=s.minLength)&&(!s.maxLength||v.length<=s.maxLength);
 case 'integer':return Number.isSafeInteger(v)&&(s.minimum===undefined||v>=s.minimum)&&(s.maximum===undefined||v<=s.maximum);
 case 'boolean':return typeof v==='boolean';
 default:return !!(s.allOf||s.oneOf);
 }
}
export function fleetSchema(name,v){if(!schemas[name]||!matches(schemas[name],v))fleetFail();return v;}
function resource(name){if(!Object.hasOwn(fleetResources,name))fleetFail();return fleetResources[name];}
export function fleetRecord(name,v){
 const [kind,schema]=resource(name),ws=name.startsWith('workshop/');
 fleetSchema(ws?'WorkshopResult':'Record',v);
 if(v.kind!==kind||!fleetID(v.id)||!Number.isSafeInteger(v.revision)||v.revision<1)fleetFail();
 fleetSchema(schema,v.data);
 if(ws){if(!fleetID(v.actor_id)||!fleetID(v.device_id)||!fleetID(v.operation_id)||!fleetStamp(v.recorded_at))fleetFail();}
 else if(!fleetID(v.created_by)||!fleetStamp(v.created_at)||!fleetStamp(v.updated_at))fleetFail();
 return v;
}
export function fleetPage(name,v,offset=0){
 resource(name);fleetSchema(name.startsWith('workshop/')?'WorkshopPage':'Page',v);
 if(!natural(offset)||offset>1000000||v.offset!==offset||v.limit!==50||!natural(v.total)||v.items.length!==Math.min(50,Math.max(0,v.total-offset)))fleetFail();
 if(v.measured_at!==undefined&&!fleetStamp(v.measured_at))fleetFail();
 const ids=new Set();for(const r of v.items){fleetRecord(name,r);if(ids.has(r.id))fleetFail();ids.add(r.id);}
 return v;
}
export function fleetHistory(name,id,v,offset=0){
 resource(name);fleetSchema(name.startsWith('workshop/')?'WorkshopHistory':'History',v);
 if(!fleetID(id)||v.offset!==offset||v.limit!==50||!natural(v.total)||v.items.length!==Math.min(50,Math.max(0,v.total-offset)))fleetFail();
 const seen=new Set();for(let index=0;index<v.items.length;index++){const event=v.items[index],r=name.startsWith('workshop/')?event.result:event.result.record;fleetRecord(name,r);if(r.id!==id||r.revision!==offset+index+1||seen.has(event.result.operation_id))fleetFail();seen.add(event.result.operation_id);if(!name.startsWith('workshop/')&&(!fleetID(event.actor_id)||!fleetID(event.device_id)||!fleetID(event.result.operation_id)))fleetFail();}
 return v;
}
export function fleetPeriod(input){
 if(!object(input)||Object.keys(input).length!==2||!fleetStamp(input.from)||!fleetStamp(input.to)||input.from.includes('.')||input.to.includes('.'))fleetFail();
 const span=Date.parse(input.to)-Date.parse(input.from);if(span<=0||span>366*86400000)fleetFail();return {from:input.from,to:input.to};
}
const exact=(v,signed=false)=>typeof v==='string'&&(signed?/^(0|-?[1-9]\d*)$/:/^(0|[1-9]\d*)$/).test(v)&&v.length<=128;
export function fleetReport(kind,id,v,period){
 const schema={availability:'Availability',workshop:'WorkshopSummary',costs:'CostReport'}[kind];
 if(!schema||!fleetID(id))fleetFail();fleetSchema(schema,v);
 if((kind==='workshop'?v.request_id:v.asset_id)!==id||!fleetStamp(v.measured_at))fleetFail();
 if(kind==='availability'&&(!natural(v.blocking_requests)||v.live_position!==false||v.available_for_dispatch!==(v.manual_state==='available'&&v.blocking_requests===0&&v.active_trip_id==='')))fleetFail();
 if(kind==='workshop'&&(v.payroll_posted!==false||v.source_records>1000||!natural(v.source_records)||v.request_revision<1||!natural(v.binding_revision)))fleetFail();
 if(kind==='costs'){
  const p=fleetPeriod(period);if(v.from!==p.from||v.to_exclusive!==p.to||v.cash_confirmed!==false||v.net_profit!==false||v.source_records>5000||!natural(v.source_records)||!natural(v.voided_records)||!natural(v.returned_trips))fleetFail();
  if(!['km','hour','none'].includes(v.meter_basis)||v.profile_present!==(v.book_value_cents_exact!==null))fleetFail();
 }
 for(const [k,n] of Object.entries(v))if(k.endsWith('_exact')||k==='cost_per_meter_numerator_cents'||k==='cost_per_meter_denominator_milli'){if(n===null&&k==='book_value_cents_exact')continue;if(!exact(n,k==='partial_contribution_cents_exact'))fleetFail();}
 return v;
}
export function fleetDecimal(value,scale=100,locale='pt-BR'){
 if(!exact(value,true)||![1,100,1000].includes(scale))fleetFail();const n=BigInt(value),negative=n<0n,a=negative?-n:n,den=BigInt(scale),digits=String(scale).length-1;
 const whole=new Intl.NumberFormat(locale,{maximumFractionDigits:0}).format(a/den),separator=new Intl.NumberFormat(locale).formatToParts(1.1).find(p=>p.type==='decimal')?.value||'.';
 return (negative?'-':'')+whole+(digits?separator+String(a%den).padStart(digits,'0'):'');
}

export function fleetPreventive(id,v,offset=0){
 fleetSchema('PreventivePage',v);if(!fleetID(id)||!natural(offset)||v.offset!==offset||v.limit!==50||!natural(v.total)||v.items.length!==Math.min(50,Math.max(0,v.total-offset))||v.automatic_request!==false||!fleetStamp(v.measured_at))fleetFail();
 const ids=new Set();for(const item of v.items){fleetRecord('plans',item.plan);if(item.plan.data.asset_id!==id||ids.has(item.plan.id))fleetFail();ids.add(item.plan.id);}return v;
}
export function fleetNotices(v,after=0){
 fleetSchema('NoticePage',v);if(!natural(after)||v.limit!==50||v.items.length>50||v.push_delivered!==false||!natural(v.next_cursor))fleetFail();
 let previous=after;for(const item of v.items){if(!natural(item.id)||item.id<=previous||!fleetID(item.request_id)||item.revision<1||!fleetStamp(item.created_at))fleetFail();previous=item.id;}if(v.next_cursor!==previous)fleetFail();return v;
}
