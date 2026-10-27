import {receivingID,quantityInput,quantityText} from '../receiving/model.mjs';
export {quantityInput,quantityText};
const fail=()=>{throw Error('stockInvalid');},n=v=>Number.isSafeInteger(v)&&v>=0;
const units=['unit','kg','g','liter','ml','meter'];
export function stockInput(v,legacy=false){
 const required=['operation_id','kind','product_id','quantity_milli','reason'],allowed=[...required,'from_location_id','to_location_id','expected_unit'];
 if(!v||!required.every(k=>Object.hasOwn(v,k))||Object.keys(v).some(k=>!allowed.includes(k))||!receivingID(v.operation_id)||!receivingID(v.product_id)||!n(v.quantity_milli)||!v.quantity_milli||typeof v.reason!=='string'||v.reason!==v.reason.trim()||!v.reason||new TextEncoder().encode(v.reason).length>255||/[\x00-\x1f\x7f]/.test(v.reason)||(!legacy||v.expected_unit!==undefined)&&!units.includes(v.expected_unit))fail();
 const from=v.from_location_id||'',to=v.to_location_id||'';
 if(from&&!receivingID(from)||to&&!receivingID(to)||!(v.kind==='entry'&&!from&&to||v.kind==='loss'&&from&&!to||v.kind==='transfer'&&from&&to&&from!==to))fail();return v;
}
export function stockCommand(c){if(!c||Object.keys(c).length!==4||c.version!==1||typeof c.uncertain!=='boolean'||typeof c.legacy!=='boolean')fail();stockInput(c.body,c.legacy);if(c.legacy&&(c.body.kind!=='entry'||!c.uncertain))fail();return c;}
export function availability(v,product,location){
 if(!v||v.product_id!==product||v.location_id!==location||!units.includes(v.unit)||!['physical_milli','reserved_milli','free_milli'].every(k=>n(v[k]))||BigInt(v.physical_milli)!==BigInt(v.reserved_milli)+BigInt(v.free_milli))fail();return v;
}
export function stockReceipt(v,body,actor,device){
 if(!v||!receivingID(v.actor_id)||!receivingID(v.device_id)||typeof v.occurred_at!=='string'||!/^\d{4}-\d{2}-\d{2}T/.test(v.occurred_at)||!Number.isFinite(Date.parse(v.occurred_at))||!units.includes(v.unit))fail();stockInput(v.input);
 if(v.input.expected_unit!==v.unit)fail();
 if(body&&(Object.keys(body).some(k=>v.input[k]!==body[k])||(body.from_location_id||'')!==(v.input.from_location_id||'')||(body.to_location_id||'')!==(v.input.to_location_id||'')||v.actor_id!==actor||v.device_id!==device))fail();return v;
}
export function stockHistory(v,product,location,offset=0){
 if(!v||v.product_id!==product||v.location_id!==location||!n(v.total_count)||v.limit!==50||v.offset!==offset||!Array.isArray(v.items)||v.items.length!==Math.min(50,Math.max(0,v.total_count-offset))||v.has_more!==(offset+v.items.length<v.total_count))fail();
 const seen=new Set();for(const r of v.items){stockReceipt(r);if(r.input.product_id!==product||![r.input.from_location_id,r.input.to_location_id].includes(location)||seen.has(r.input.operation_id))fail();seen.add(r.input.operation_id);}return v;
}
