import test from 'node:test';
import assert from 'node:assert/strict';
import {createProductionRead} from '../src/core/local/productionRead.mjs';
import {exactQuantity} from '../src/core/local/operationsRead.mjs';
import {allowedAreas,areaForRoute} from '../src/core/local/accessModel.mjs';
const recipe={recipe_id:'r',version_id:'v',revision:1,name:'Pão',output_product_id:'bread',output_unit:'unit',yield_milli:2000,ingredients:[{product_id:'flour',unit:'g',quantity_milli:500000}]};
const reply=v=>async()=>new Response(JSON.stringify(v));
test('exact quantity never rounds a safe maximum or a cumulative string',()=>{
 assert.equal(exactQuantity(1),'0,001');assert.equal(exactQuantity(1000),'1,000');
 assert.equal(exactQuantity(Number.MAX_SAFE_INTEGER),'9.007.199.254.740,991');
 assert.equal(exactQuantity('18014398509482029'),'18.014.398.509.482,029');
 for(const v of [-1,1.5,Number.MAX_SAFE_INTEGER+1,'1e3','01','1,0'])assert.throws(()=>exactQuantity(v));
});
test('GET-only client encodes IDs and does not persist or send credentials',async()=>{
 let call;const c=createProductionRead(async(url,options)=>{call={url,options};return new Response(JSON.stringify({items:[]}));});
 await c.versions('secret',50,'r/?&');assert.equal(call.url,'/local/v1/production/recipe-versions?limit=50&offset=50&recipe_id=r%2F%3F%26');
 assert.equal(call.options.method,'GET');assert.equal(call.options.credentials,'omit');assert.equal(call.options.cache,'no-store');assert.equal(call.options.body,undefined);assert.equal(call.options.headers.Authorization,'Bearer secret');
});
test('null recipes mean empty, unsafe ingredients and wrong filter are refused',async()=>{
 assert.deepEqual(await createProductionRead(reply({items:null})).versions('t'),[]);
 assert.deepEqual(await createProductionRead(reply({items:[recipe]})).versions('t'),[recipe]);
 await assert.rejects(createProductionRead(reply({items:[recipe]})).versions('t',0,'foreign'));
 await assert.rejects(createProductionRead(reply({items:[{...recipe,yield_milli:1.5}]})).versions('t'));
 let calls=0;await assert.rejects(createProductionRead(async()=>{calls++;}).versions('',0));assert.equal(calls,0);
});
test('orders retain recipe version and exact planned output',async()=>{
 const order={id:'o',version_id:'v',location_id:'l',responsible_id:'person',planned_batches:3,planned_output_milli:6000,revision:1,created_at:'2026-10-09',status:'completed',recipe};
 const page={total_count:1,limit:50,has_more:false,filters:{status:'completed',offset:0},items:[order]};
 assert.equal((await createProductionRead(reply(page)).orders('t','completed')).items[0].recipe.version_id,'v');
 for(const delta of [{version_id:'other'},{planned_output_milli:6001},{status:'approved'}])await assert.rejects(createProductionRead(reply({...page,items:[{...order,...delta}]})).orders('t','completed'));
});
test('capacity is independent, available after reservations, and dimension explicit',async()=>{
 const a={version_id:'v',revision:1,output_unit:'unit',yield_per_batch_milli:2000,possible_batches:2,possible_output_milli:4000,limiting_product_ids:['flour'],materials:[{product_id:'flour',recipe_unit:'g',stock_unit:'kg',required_milli:500000,stock_milli:1000,conversion_numerator:1000,conversion_denominator:1,possible_batches:2,limiting:true}]};
 const v={location_id:'l',measured_at:'2026-10-09',basis:'local_available_balance_after_reservations',alternatives_independent:true,simultaneous_total_available:false,alternatives:[a]};
 assert.equal((await createProductionRead(reply(v)).capacity('t','v','l')).alternatives[0].materials[0].stock_unit,'kg');
 for(const delta of [{location_id:'other'},{simultaneous_total_available:true},{basis:'physical_stock'},{alternatives:[{...a,possible_output_milli:4001}]}])await assert.rejects(createProductionRead(reply({...v,...delta})).capacity('t','v','l'));
});
test('production direct route is checked against the production area',()=>{
 assert.equal(areaForRoute('/local/production'),'production');
 assert.ok(!allowedAreas({permissions:['view_catalog'],license:{state:'active',modules:['inventory']}}).includes('production'));
});
test('401 and invalid JSON do not become empty successful pages',async()=>{
 await assert.rejects(createProductionRead(async()=>new Response('',{status:401})).versions('t'),e=>e.status===401);
 await assert.rejects(createProductionRead(async()=>new Response('broken')).versions('t'));
});
