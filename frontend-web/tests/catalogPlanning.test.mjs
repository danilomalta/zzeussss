import test from 'node:test';
import assert from 'node:assert/strict';
import {createCatalogSearch} from '../src/core/local/catalogSearch.mjs';
import {createCatalogPlanning,validProductUsage,capacitySelection,validAlternatives} from '../src/core/local/catalogPlanning.mjs';
const response=(v,status=200)=>({ok:status===200,status,json:async()=>structuredClone(v)});
const product={id:'product',name:'Farinha',unit:'kg'};
const link={kind:'versions',id:'version',recipe_id:'recipe',version_id:'version',role:'ingredient',unit:'g',quantity_milli:500000,quantity_basis:'per_batch',unit_matches_catalog:false,latest_version:true};
const usage={product,store_id:'store',kind:'versions',recipe_version_count:1,latest_recipe_version_count:1,order_count:1,offset:0,limit:50,has_more:false,items:[link]};
const material={product_id:'flour',recipe_unit:'g',stock_unit:'kg',required_milli:500000,stock_milli:2000,conversion_numerator:1000,conversion_denominator:1,possible_batches:4,limiting:true};
const alternative={recipe_id:'recipe',version_id:'v1',revision:1,output_product_id:'bread',output_unit:'unit',yield_per_batch_milli:2000,possible_batches:4,possible_output_milli:8000,limiting_product_ids:['flour'],materials:[material]};
const input={location_id:'room',version_ids:['v1','v2']};
const capacity={location_id:'room',measured_at:'2026-10-11T01:00:00Z',basis:'local_available_balance_after_reservations',alternatives_independent:true,simultaneous_total_available:false,alternatives:[alternative,{...alternative,recipe_id:'recipe2',version_id:'v2'}]};
test('catalog status filter is exact, encoded, and retains confidentiality of cost',async()=>{
 let path;const row={id:'p',sku:'P1',name:'Produto',unit:'kg',price_cents:100,status:'inactive',state_revision:1,minimum_configured:false,approximate:false};
 const page={status:'inactive',items:[row],total:1,offset:0,limit:50,cost_visible:false};
 const c=createCatalogSearch(async(u,o)=>{path=u;assert.equal(o.credentials,'omit');return response(page);});
 assert.deepEqual((await c('token',{status:'inactive'})).items,[row]);assert.equal(new URL(path,'http://localhost').searchParams.get('status'),'inactive');
 for(const x of [{...page,status:'active'},{...page,items:[{...row,status:'active'}]},{...page,items:[{...row,cost_cents:1}]},{...page,items:[{...row,state_revision:-1}]},{...page,items:[{...row,state_revision:2147483648}]}])await assert.rejects(()=>createCatalogSearch(async()=>response(x))('token',{status:'inactive'}));
});
test('catalog all/active filters and invalid state do not infer values',async()=>{
 let calls=0;const c=createCatalogSearch(async()=>{calls++;return response({status:'all',items:[],total:0,offset:0,limit:50,cost_visible:false});});
 await c('token',{status:'all'});await assert.rejects(()=>c('token',{status:'deleted'}));assert.equal(calls,1);await assert.rejects(()=>c('token',{status:'active'}));
});
test('version usage preserves historical unit, exact per-batch quantity and store context',async()=>{
 assert.equal(validProductUsage(usage,'product','store','versions',0),true);
 let url;const c=createCatalogPlanning(async(u,o)=>{url=u;assert.equal(o.method,'GET');return response(usage);});
 const out=await c.usage('token','product','store');assert.equal(out.items[0].unit,'g');assert.equal(out.items[0].quantity_milli,500000);assert.match(url,/production-usage\?kind=versions&offset=0$/);
 for(const x of [{...usage,store_id:'foreign'},{...usage,product:{...product,id:'other'}},{...usage,latest_recipe_version_count:2},{...usage,items:[{...link,unit_matches_catalog:true}]},{...usage,items:[{...link,quantity_basis:'planned_total'}]}])assert.equal(validProductUsage(x,'product','store','versions',0),false);
});
test('order usage validates immutable version, explicit planned-total basis and effective status',()=>{
 const orderLink={...link,kind:'orders',id:'order',quantity_basis:'planned_total',quantity_milli:1000000,order_status:'completed',location_id:'room'};delete orderLink.latest_version;
 const page={...usage,kind:'orders',items:[orderLink]};assert.equal(validProductUsage(page,'product','store','orders',0),true);
 for(const x of [{...orderLink,order_status:'sent'},{...orderLink,quantity_basis:'per_batch'},{...orderLink,quantity_milli:Number.MAX_SAFE_INTEGER+1}])assert.equal(validProductUsage({...page,items:[x]},'product','store','orders',0),false);
});
test('usage pagination keeps global counts, rejects duplicate IDs and never hides absent pages',()=>{
 const page={...usage,recipe_version_count:51,latest_recipe_version_count:1,offset:50,items:[{...link,latest_version:false}]};assert.equal(validProductUsage(page,'product','store','versions',50),true);
 assert.equal(validProductUsage({...page,has_more:true},'product','store','versions',50),false);
 assert.equal(validProductUsage({...usage,recipe_version_count:2,items:[link,link]},'product','store','versions',0),false);
 assert.equal(validProductUsage({...usage,recipe_version_count:0,latest_recipe_version_count:0,items:[]},'product','store','versions',0),true);
});
test('capacity selections reject duplicates, more than 20, invalid IDs and empty lists before network',async()=>{
 assert.deepEqual(capacitySelection('room',['v1','v2']),input);
 for(const v of [[],['v1','v1'],Array.from({length:21},(_,i)=>'v'+i),['á'.repeat(65)]])assert.throws(()=>capacitySelection('room',v));
 let calls=0;const c=createCatalogPlanning(async()=>{calls++;return response(capacity);});await assert.rejects(()=>c.alternatives('token','room',[]));await assert.rejects(()=>c.alternatives('','room',['v1']));assert.equal(calls,0);
});
test('two alternatives share a snapshot but are never summed as simultaneous outputs',async()=>{
 assert.equal(validAlternatives(capacity,input),true);
 let sent;const c=createCatalogPlanning(async(url,o)=>{sent={url,o};return response(capacity);});
 const out=await c.alternatives('opaque','room',['v1','v2']);assert.equal(sent.url,'/local/v1/production/capacity/alternatives');assert.deepEqual(JSON.parse(sent.o.body),input);assert.equal(sent.o.credentials,'omit');assert.equal(sent.o.headers.Authorization,'Bearer opaque');
 assert.equal(out.alternatives[0].possible_output_milli,8000);assert.equal(out.alternatives[1].possible_output_milli,8000);assert.equal(out.simultaneous_total_available,false);
 assert.equal(validAlternatives({...capacity,simultaneous_total_available:true},input),false);assert.equal(validAlternatives({...capacity,alternatives_independent:false},input),false);
});
test('capacity conversion distinguishes grams from integer scale and refuses volume to mass',()=>{
 for(const m of [{...material,conversion_numerator:1},{...material,stock_unit:'liter'},{...material,possible_batches:4000}])assert.equal(validAlternatives({...capacity,alternatives:[{...alternative,materials:[m]}]}, {location_id:'room',version_ids:['v1']}),false);
 const liquid={...material,product_id:'water',stock_unit:'ml',recipe_unit:'liter',stock_milli:2000000,required_milli:500,conversion_numerator:1,conversion_denominator:1000,possible_batches:4};
 assert.equal(validAlternatives({...capacity,alternatives:[{...alternative,materials:[liquid],limiting_product_ids:['water']}]},{location_id:'room',version_ids:['v1']}),true);
});
test('limiting ingredients must equal the exact minimum, including ties and zero balance',()=>{
 const oil={...material,product_id:'oil',recipe_unit:'liter',stock_unit:'liter',required_milli:1000,stock_milli:4000,conversion_numerator:1};
 const tie={...alternative,materials:[material,oil],limiting_product_ids:['flour','oil']};const one={location_id:'room',version_ids:['v1']};assert.equal(validAlternatives({...capacity,alternatives:[tie]},one),true);
 assert.equal(validAlternatives({...capacity,alternatives:[{...tie,limiting_product_ids:['flour']}]},one),false);
 const zero={...alternative,possible_batches:0,possible_output_milli:0,materials:[{...material,stock_milli:0,possible_batches:0}]};assert.equal(validAlternatives({...capacity,alternatives:[zero]},one),true);
});
test('unsafe output arithmetic and inconsistent shared stock are rejected, not rounded',()=>{
 const changed={...alternative,version_id:'v2',materials:[{...material,stock_milli:2001}]};assert.equal(validAlternatives({...capacity,alternatives:[alternative,changed]},input),false);
 const unsafe={...alternative,possible_output_milli:Number.MAX_SAFE_INTEGER+1};assert.equal(validAlternatives({...capacity,alternatives:[unsafe]},{location_id:'room',version_ids:['v1']}),false);
 assert.equal(validAlternatives({...capacity,alternatives:[alternative,alternative]},input),false);
});
test('read refusal stays visible and never triggers a mutation endpoint or fallback',async()=>{
 const paths=[];const c=createCatalogPlanning(async(path,o)=>{paths.push([path,o.method]);return response(null,403);});
 await assert.rejects(()=>c.usage('token','product','store'),e=>e.status===403);await assert.rejects(()=>c.alternatives('token','room',['v1']),e=>e.status===403);
 assert.deepEqual(paths.map(x=>x[1]),['GET','POST']);assert.ok(paths.every(x=>x[0].includes('production-usage')||x[0].endsWith('/capacity/alternatives')));
});
