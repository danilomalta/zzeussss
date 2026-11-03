import {recipeVersion} from './source.mjs';
const fail=()=>{throw Error('productionInvalid');};
const int=(v,min=0,max=Number.MAX_SAFE_INTEGER)=>Number.isSafeInteger(v)&&v>=min&&v<=max;
const id=v=>typeof v==='string'&&v===v.trim()&&v.length>0&&new TextEncoder().encode(v).length<=128&&!/[\x00\r\n]/.test(v);
const exact=(o,keys)=>o&&typeof o==='object'&&!Array.isArray(o)&&Object.keys(o).length===keys.length&&keys.every(k=>Object.hasOwn(o,k));
const time=v=>typeof v==='string'&&Number.isFinite(Date.parse(v));
const add=(a,b)=>[a[0]*b[1]+b[0]*a[1],a[1]*b[1]];
const mul=(a,b)=>[a[0]*b[0],a[1]*b[1]];
const eq=(a,b)=>a[0]*b[1]===b[0]*a[1];
const positiveProduct=(a,b)=>{if(!int(a,1)||!int(b,1)||BigInt(a)*BigInt(b)>BigInt(Number.MAX_SAFE_INTEGER))fail();return Number(BigInt(a)*BigInt(b));};
export function exactRational(v,signed=false){
 if(!exact(v,['numerator','denominator'])||typeof v.numerator!=='string'||typeof v.denominator!=='string'||v.numerator.length>100||v.denominator.length>100||!(signed?/^-?(0|[1-9][0-9]*)$/:/^(0|[1-9][0-9]*)$/).test(v.numerator)||!/^[1-9][0-9]*$/.test(v.denominator))fail();
 return [BigInt(v.numerator),BigInt(v.denominator)];
}
export function formatProductionRational(value,locale){
 let [n,d]=exactRational(value,true);const original=n,absolute=n<0n?-n:n;let a=absolute,b=d;while(b){const next=a%b;a=b;b=next;}n/=a;d/=a;
 const nf=new Intl.NumberFormat(locale,{maximumFractionDigits:0});if(d===1n)return nf.format(n);
 let remaining=d,twos=0,fives=0;while(remaining%2n===0n){remaining/=2n;twos++;}while(remaining%5n===0n){remaining/=5n;fives++;}const digits=Math.max(twos,fives);
 if(remaining!==1n||digits>6)return nf.format(n)+' / '+nf.format(d);
 const scale=10n**BigInt(digits),scaled=(n<0n?-n:n)*scale/d,decimal=new Intl.NumberFormat(locale).formatToParts(1.1).find(p=>p.type==='decimal')?.value||'.',minus=new Intl.NumberFormat(locale).formatToParts(-1).find(p=>p.type==='minusSign')?.value||'-';
 return (original<0n?minus:'')+nf.format(scaled/scale)+decimal+(scaled%scale).toString().padStart(digits,'0').replace(/0+$/,'');
}
const same=(v,expected,signed=false)=>{if(!eq(exactRational(v,signed),expected))fail();};
const rounded=v=>(2n*v[0]+v[1])/(2n*v[1]);
export function productionInteger(value,min=1,max=Number.MAX_SAFE_INTEGER){if(typeof value!=='string'||!/^\d+$/.test(value)||value.length>20)fail();const v=Number(value);if(!int(v,min,max))fail();return v;}
export function productionCents(value){if(typeof value!=='string'||value.length>24||!/^\d+(?:[.,]\d{1,2})?$/.test(value))fail();const [whole,fraction='']=value.replace(',','.').split('.'),n=BigInt(whole)*100n+BigInt(fraction.padEnd(2,'0'));if(n>BigInt(Number.MAX_SAFE_INTEGER))fail();return Number(n);}
export function costScenarioInput(value){
 if(!value||typeof value!=='object'||Array.isArray(value)||!Object.keys(value).every(k=>['batches','labor','overhead_cents','sale_price_cents'].includes(k))||!int(value.batches,1))fail();
 for(const key of ['overhead_cents','sale_price_cents'])if(Object.hasOwn(value,key)&&!int(value[key]))fail();
 if(Object.hasOwn(value,'labor')&&(!exact(value.labor,['minutes','workers','hourly_cost_cents'])||!int(value.labor.minutes,0,525600)||!int(value.labor.workers,1,10000)||!int(value.labor.hourly_cost_cents)))fail();
 return structuredClone(value);
}
export function materialCostEstimate(value,version,batches,orderID){
 const recipe=recipeVersion(version),keys=['source','valued_at','version_id','batches','output_unit','output_quantity_milli','lines','material_cost_cents_exact','material_cost_cents','material_unit_cost_cents_exact','rounding','labor_cost_cents','overhead_cost_cents','is_complete'];
 if(orderID!==undefined){if(!id(orderID))fail();keys.push('order_id');}
 if(!int(batches,1)||!exact(value,keys)||value.version_id!==recipe.version_id||value.batches!==batches||value.output_unit!==recipe.output_unit||value.output_quantity_milli!==positiveProduct(recipe.yield_milli,batches)||value.source!=='current_catalog_material_estimate'||!time(value.valued_at)||value.rounding!=='half_up_on_aggregate'||value.labor_cost_cents!==null||value.overhead_cost_cents!==null||value.is_complete!==false||orderID!==undefined&&value.order_id!==orderID||!Array.isArray(value.lines)||value.lines.length!==recipe.ingredients.length||new Set(value.lines.map(v=>v?.product_id)).size!==value.lines.length)fail();
 let total=[0n,1n];
 for(const row of value.lines){const original=recipe.ingredients.find(i=>i.product_id===row?.product_id);if(!original||!exact(row,['product_id','unit','quantity_milli','catalog_cost_cents','cost_cents_exact','requires_cost_review'])||row.unit!==original.unit||row.quantity_milli!==positiveProduct(original.quantity_milli,batches)||!int(row.catalog_cost_cents)||row.requires_cost_review!==(row.catalog_cost_cents===0))fail();const cost=[BigInt(row.quantity_milli)*BigInt(row.catalog_cost_cents),1000n];same(row.cost_cents_exact,cost);total=add(total,cost);}
 same(value.material_cost_cents_exact,total);same(value.material_unit_cost_cents_exact,mul(total,[1000n,BigInt(value.output_quantity_milli)]));if(!int(value.material_cost_cents)||BigInt(value.material_cost_cents)!==rounded(total))fail();
 return value;
}
export function productionCostScenario(value,version,input){
 const b=costScenarioInput(input),m=materialCostEstimate(value?.materials,version,b.batches),keys=['materials','basis','labor_cents_exact','overhead_cents','known_cost_cents_exact','known_cost_cents','known_unit_cost_cents_exact','scenario_revenue_cents_exact','scenario_contribution_cents_exact','scenario_margin_basis_points','all_requested_components_provided','historical_cost','profit'];
 if(!exact(value,keys)||value.basis!=='current_materials_and_user_supplied_run_assumptions'||value.historical_cost!==false||value.profit!==false||value.all_requested_components_provided!==(!!b.labor&&Object.hasOwn(b,'overhead_cents'))||value.overhead_cents!==(b.overhead_cents??null))fail();
 let total=exactRational(m.material_cost_cents_exact);
 if(b.labor){const labor=[BigInt(b.labor.minutes)*BigInt(b.labor.workers)*BigInt(b.labor.hourly_cost_cents),60n];same(value.labor_cents_exact,labor);total=add(total,labor);}else if(value.labor_cents_exact!==null)fail();
 if(b.overhead_cents!==undefined)total=add(total,[BigInt(b.overhead_cents),1n]);
 same(value.known_cost_cents_exact,total);same(value.known_unit_cost_cents_exact,mul(total,[1000n,BigInt(m.output_quantity_milli)]));if(!int(value.known_cost_cents)||BigInt(value.known_cost_cents)!==rounded(total))fail();
 if(b.sale_price_cents===undefined){if(value.scenario_revenue_cents_exact!==null||value.scenario_contribution_cents_exact!==null||value.scenario_margin_basis_points!==null)fail();}
 else{const revenue=[BigInt(b.sale_price_cents)*BigInt(m.output_quantity_milli),1000n];same(value.scenario_revenue_cents_exact,revenue);if(!value.all_requested_components_provided){if(value.scenario_contribution_cents_exact!==null||value.scenario_margin_basis_points!==null)fail();}else{const contribution=add(revenue,[-total[0],total[1]]);same(value.scenario_contribution_cents_exact,contribution,true);if(revenue[0]===0n){if(value.scenario_margin_basis_points!==null)fail();}else{const bp=contribution[0]*revenue[1]*10000n/(contribution[1]*revenue[0]);if(bp>BigInt(Number.MAX_SAFE_INTEGER)||bp<BigInt(-Number.MAX_SAFE_INTEGER)){if(value.scenario_margin_basis_points!==null)fail();}else if(!Number.isSafeInteger(value.scenario_margin_basis_points)||BigInt(value.scenario_margin_basis_points)!==bp)fail();}}}
 return value;
}
export function simultaneousInput(locationID,items){
 if(!id(locationID)||!Array.isArray(items)||items.length<1||items.length>20||new Set(items.map(i=>i?.version?.version_id)).size!==items.length)fail();
 const recipes=items.map(i=>{const v=recipeVersion(i.version);positiveProduct(v.yield_milli,i.batches);for(const ingredient of v.ingredients)positiveProduct(ingredient.quantity_milli,i.batches);return {version_id:v.version_id,batches:i.batches};});
 return {location_id:locationID,recipes};
}
export function productionSimultaneous(value,locationID,items){
 const b=simultaneousInput(locationID,items);
 if(!exact(value,['location_id','measured_at','basis','feasible','reserved','outputs','materials'])||value.location_id!==locationID||!time(value.measured_at)||value.basis!=='shared_current_free_stock_not_reserved'||typeof value.feasible!=='boolean'||value.reserved!==false||!Array.isArray(value.outputs)||value.outputs.length!==items.length||new Set(value.outputs.map(o=>o?.version_id)).size!==items.length||!Array.isArray(value.materials)||new Set(value.materials.map(i=>i?.product_id)).size!==value.materials.length)fail();
 const requirements=new Map();
 for(const item of items){const v=item.version,o=value.outputs.find(o=>o?.version_id===v.version_id);if(!exact(o,['version_id','product_id','unit','batches','output_milli'])||o.product_id!==v.output_product_id||o.unit!==v.output_unit||o.batches!==item.batches||o.output_milli!==positiveProduct(v.yield_milli,item.batches))fail();for(const line of v.ingredients){const previous=requirements.get(line.product_id),q=BigInt(positiveProduct(line.quantity_milli,item.batches));if(previous&&previous.unit!==line.unit)fail();const quantity=(previous?.quantity||0n)+q;if(quantity>BigInt(Number.MAX_SAFE_INTEGER))fail();requirements.set(line.product_id,{unit:line.unit,quantity});}}
 if(value.materials.length!==requirements.size)fail();let feasible=true;
 for(const row of value.materials){const required=requirements.get(row?.product_id);if(!required||!exact(row,['product_id','unit','required_milli','physical_milli','reserved_milli','free_milli','shortfall_milli','remaining_milli'])||row.unit!==required.unit||!['required_milli','physical_milli','reserved_milli','free_milli','shortfall_milli','remaining_milli'].every(k=>int(row[k]))||BigInt(row.required_milli)!==required.quantity||row.reserved_milli>row.physical_milli||row.free_milli!==row.physical_milli-row.reserved_milli)fail();const shortfall=Math.max(0,row.required_milli-row.free_milli),remaining=Math.max(0,row.free_milli-row.required_milli);if(row.shortfall_milli!==shortfall||row.remaining_milli!==remaining)fail();if(shortfall)feasible=false;}
 if(value.feasible!==feasible||b.recipes.length!==value.outputs.length)fail();return value;
}
