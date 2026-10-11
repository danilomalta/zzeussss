import {id,requireValid} from './operationsRead.mjs';
import {validAlternatives} from './catalogPlanning.mjs';
const maximum=BigInt(Number.MAX_SAFE_INTEGER);
export function plannedBatches(value){requireValid(typeof value==='string'&&/^[1-9][0-9]*$/.test(value));const n=BigInt(value);requireValid(n<=maximum);return Number(n);}
export function planAlternative(snapshot,versionID,value){
 requireValid(snapshot&&Array.isArray(snapshot.alternatives)&&id(versionID));
 const input={location_id:snapshot.location_id,version_ids:snapshot.alternatives.map(c=>c?.version_id)};
 requireValid(validAlternatives(snapshot,input));const c=snapshot.alternatives.find(c=>c.version_id===versionID);requireValid(c);
 const batches=plannedBatches(value),count=BigInt(batches),output=count*BigInt(c.yield_per_batch_milli);requireValid(output<=maximum&&(c.output_unit!=='unit'||c.yield_per_batch_milli%1000===0));
 const materials=c.materials.map(m=>{const demand=count*BigInt(m.required_milli);requireValid(demand<=maximum&&(m.recipe_unit!=='unit'||m.required_milli%1000===0)&&(m.stock_unit!=='unit'||m.stock_milli%1000===0));const numerator=BigInt(m.conversion_numerator),denominator=BigInt(m.conversion_denominator),available=BigInt(m.stock_milli)*numerator,needed=demand*denominator;const sufficient=available>=needed;return {product_id:m.product_id,recipe_unit:m.recipe_unit,stock_unit:m.stock_unit,required_milli:demand.toString(),stock_free_milli:m.stock_milli,sufficient,limiting:m.limiting,missing_recipe_numerator:(sufficient?0n:needed-available).toString(),missing_recipe_denominator:denominator.toString(),remaining_stock_numerator:(sufficient?available-needed:0n).toString(),remaining_stock_denominator:numerator.toString()};});
 return {version_id:c.version_id,location_id:snapshot.location_id,measured_at:snapshot.measured_at,batches,possible_batches:c.possible_batches,output_product_id:c.output_product_id,output_unit:c.output_unit,planned_output_milli:output.toString(),sufficient:materials.every(m=>m.sufficient),materials};
}
