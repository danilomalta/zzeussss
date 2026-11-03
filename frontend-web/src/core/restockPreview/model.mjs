import {salesInstant as parseInstant} from '../salesAnalysis/model.mjs';
import {reportID} from '../stockReports/model.mjs';
const fail=()=>{throw Error('restockPreviewInvalid');};
const instant=value=>{try{return parseInstant(value);}catch{fail();}};
const exact=(v,keys)=>v&&typeof v==='object'&&!Array.isArray(v)&&Object.keys(v).length===keys.length&&keys.every(k=>Object.hasOwn(v,k));
const int=(v,min=0,max=Number.MAX_SAFE_INTEGER)=>Number.isSafeInteger(v)&&v>=min&&v<=max;
const sameRat=(n,d,expectedN,expectedD)=>{let a=expectedN,b=expectedD;while(b){const next=a%b;a=b;b=next;}if(typeof n!=='string'||typeof d!=='string'||n!==String(expectedN/a)||d!==String(expectedD/a))fail();};
const ceil=(n,d,unit)=>{const step=unit==='unit'?1000n:1n,result=(n+d*step-1n)/(d*step)*step;if(result>BigInt(Number.MAX_SAFE_INTEGER))fail();return result;};
export function restockPreviewInput(value,product){
 if(!exact(value,['product_id','from','to','lead_days','coverage_days','safety_milli'])||!reportID(value.product_id)||!int(value.lead_days,0,365)||!int(value.coverage_days,1,365)||!int(value.safety_milli))fail();const duration=instant(value.to)-instant(value.from);if(duration<3600000000000n||duration>31622400000000000n)fail();if(product&&(!reportID(product.id)||product.id!==value.product_id||!['unit','kg','g','liter','ml','meter'].includes(product.unit)||product.unit==='unit'&&value.safety_milli%1000!==0))fail();return {...value};
}
export function restockPreviewReport(value,input,product){
 const request=restockPreviewInput(input,product),duration=instant(request.to)-instant(request.from),keys=['product_id','unit','from','to','measured_at','basis','demand_basis','sold_milli','daily_demand_milli_numerator','daily_demand_milli_denominator','physical_milli','reserved_milli','free_milli','policy_revision','policy_minimum_milli','policy_target_milli','suggested_minimum_milli','suggested_target_milli','suggested_order_milli','review_required','order_created'];
 if(!exact(value,keys)||value.product_id!==request.product_id||value.unit!==product?.unit||instant(value.from)!==instant(request.from)||instant(value.to)!==instant(request.to)||!instant(value.measured_at)||value.basis!=='same_store_demand_and_all_local_locations'||value.review_required!==true||value.order_created!==false||!['sold_milli','physical_milli','reserved_milli','free_milli','policy_minimum_milli','policy_target_milli','suggested_minimum_milli','suggested_target_milli','suggested_order_milli'].every(k=>int(value[k]))||!int(value.policy_revision,1)||value.policy_target_milli<value.policy_minimum_milli||BigInt(value.physical_milli)!==BigInt(value.reserved_milli)+BigInt(value.free_milli)||value.demand_basis!==(value.sold_milli===0?'no_sales_in_selected_period_policy_only':'observed_committed_sales_not_future_guarantee'))fail();
 if(value.unit==='unit'&&['sold_milli','policy_minimum_milli','policy_target_milli','suggested_minimum_milli','suggested_target_milli','suggested_order_milli'].some(k=>value[k]%1000!==0))fail();
 const numerator=BigInt(value.sold_milli)*86400000000000n; sameRat(value.daily_demand_milli_numerator,value.daily_demand_milli_denominator,numerator,duration);
 const safety=BigInt(request.safety_milli)*duration,min=ceil(numerator*BigInt(request.lead_days)+safety,duration,value.unit),target=ceil(numerator*BigInt(request.lead_days+request.coverage_days)+safety,duration,value.unit),minimum=min>BigInt(value.policy_minimum_milli)?min:BigInt(value.policy_minimum_milli),desired=target>BigInt(value.policy_target_milli)?target:BigInt(value.policy_target_milli),free=BigInt(value.free_milli),order=free<=minimum&&free<desired?ceil(desired-free,1n,value.unit):0n;
 if(BigInt(value.suggested_minimum_milli)!==minimum||BigInt(value.suggested_target_milli)!==desired||BigInt(value.suggested_order_milli)!==order)fail();return value;
}
export function restockDays(value,min,max){if(typeof value!=='string'||!/^\d{1,3}$/.test(value)||!int(Number(value),min,max))fail();return Number(value);}
