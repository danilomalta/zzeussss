import {id,requireValid,readAPI} from './operationsRead.mjs';
import {validRecipeVersion} from './productionRead.mjs';
const bytes=v=>new TextEncoder().encode(v).length;
const validID=v=>id(v)&&bytes(v)<=128;
export function validComparableVersion(v){return !!(validRecipeVersion(v)&&[v.recipe_id,v.version_id,v.output_product_id].every(validID)&&v.revision<=2147483647&&v.name.trim()===v.name&&v.name.length>0&&bytes(v.name)<=255&&v.ingredients.length<=100&&new Set(v.ingredients.map(i=>i.product_id)).size===v.ingredients.length&&v.ingredients.every(i=>validID(i.product_id)&&i.product_id!==v.output_product_id));}
const ratio=(from,to)=>from===to?[1n,1n]:from==='kg'&&to==='g'||from==='liter'&&to==='ml'?[1000n,1n]:from==='g'&&to==='kg'||from==='ml'&&to==='liter'?[1n,1000n]:null;
// Magnitudes are exact rational thousandths of the origin unit. Never use floating point.
export function quantityDifference(before,after){
 requireValid(before&&after&&Number.isSafeInteger(before.quantity_milli)&&before.quantity_milli>0&&Number.isSafeInteger(after.quantity_milli)&&after.quantity_milli>0&&['unit','kg','g','liter','ml','meter'].includes(before.unit)&&['unit','kg','g','liter','ml','meter'].includes(after.unit));
 const r=ratio(after.unit,before.unit);if(!r)return {direction:'incompatible',numerator:null,denominator:null,unit:before.unit};
 const d=BigInt(after.quantity_milli)*r[0]-BigInt(before.quantity_milli)*r[1];let n=d<0n?-d:d,den=r[1];let a=n,b=den;while(b){[a,b]=[b,a%b];}n/=a;den/=a;
 return {direction:d>0n?'increase':d<0n?'decrease':'same',numerator:n.toString(),denominator:den.toString(),unit:before.unit};
}
export function compareRecipeVersions(before,after){
 requireValid(validComparableVersion(before)&&validComparableVersion(after)&&before.recipe_id===after.recipe_id&&before.version_id!==after.version_id&&before.revision!==after.revision);
 const previous=new Map(before.ingredients.map(i=>[i.product_id,i])),next=new Map(after.ingredients.map(i=>[i.product_id,i]));
 const added=after.ingredients.filter(i=>!previous.has(i.product_id)).map(i=>({...i})),removed=before.ingredients.filter(i=>!next.has(i.product_id)).map(i=>({...i}));
 const changed=[],unchanged=[];
 for(const b of before.ingredients){const a=next.get(b.product_id);if(!a)continue;if(a.unit===b.unit&&a.quantity_milli===b.quantity_milli)unchanged.push(b.product_id);else changed.push({product_id:b.product_id,before:{...b},after:{...a},difference:quantityDifference(b,a)});}
 const sameProduct=before.output_product_id===after.output_product_id;
 return {before:structuredClone(before),after:structuredClone(after),same_product:sameProduct,yield_difference:sameProduct?quantityDifference({quantity_milli:before.yield_milli,unit:before.output_unit},{quantity_milli:after.yield_milli,unit:after.output_unit}):null,added,removed,changed,unchanged};
}
export function createRecipeComparison(fetcher){const get=readAPI(fetcher);return {async compare(token,from,to){requireValid(validID(from)&&validID(to)&&from!==to);const results=await Promise.allSettled([from,to].map(versionID=>get('/production/recipe-versions/'+encodeURIComponent(versionID),token,v=>validComparableVersion(v)&&v.version_id===versionID)));const failures=results.filter(r=>r.status==='rejected');if(failures.length)throw (failures.find(r=>r.reason?.status===401)||failures[0]).reason;return compareRecipeVersions(results[0].value,results[1].value);}};}
