import test from 'node:test';
import assert from 'node:assert/strict';
import {createProductionMutations,validProductionInput,plannedPreview,persistProductionPending,readProductionPending,resolveProductionPending} from '../src/core/local/productionMutations.mjs';
import {LocalAPIError} from '../src/core/local/localClient.mjs';
const recipe={recipe_id:'r',version_id:'v',revision:1,name:'Pão',output_product_id:'bread',output_unit:'unit',yield_milli:10000,ingredients:[{product_id:'flour',unit:'g',quantity_milli:500000}]};
const input={operation_id:'create-op',order_id:'o',version_id:'v',location_id:'l',responsible_id:'person',planned_batches:3};
const result={order_id:'o',revision:1,status:'planned',repeated:false};
test('planning uses the immutable recipe and exact ingredient/output multiplication',()=>{
 const plan=plannedPreview(recipe,3);assert.equal(plan.output_milli,30000);assert.equal(plan.ingredients[0].planned_milli,1500000);
 for(const delta of [{planned_batches:1.5},{planned_batches:0},{responsible_id:''},{location_id:' l'},{expected_revision:1}])assert.ok(!validProductionInput('order',{...input,...delta}));
 assert.throws(()=>plannedPreview({...recipe,yield_milli:Number.MAX_SAFE_INTEGER},2));
 assert.throws(()=>plannedPreview({...recipe,ingredients:[{...recipe.ingredients[0],quantity_milli:Number.MAX_SAFE_INTEGER}]},2));
});
test('original creation receipt stays planned even if current order was approved',async()=>{
 const c=createProductionMutations(async(url,opt)=>{assert.equal(url,'/local/v1/production/operations/order/create-op');assert.equal(opt.method,'GET');return new Response(JSON.stringify({kind:'order',input,result}));});
 assert.equal((await c.operation('t','order',input)).status,'planned');
 await assert.rejects(c.operation('t','order',{...input,responsible_id:'other'}));
});
test('order send contains planned data only, never reserve consume or result',async()=>{
 let calls=0;const c=createProductionMutations(async(url,opt)=>{calls++;assert.equal(url,'/local/v1/production/orders');assert.deepEqual(JSON.parse(opt.body),input);return new Response(JSON.stringify(result));});
 await c.write('t','order',input);assert.equal(calls,1);
 await assert.rejects(c.write('t','order',{...input,quantity_milli:30000}));assert.equal(calls,1);
});
test('created order after lost reply is confirmed without another POST or new order ID',async()=>{
 const values=new Map(),s={getItem:k=>values.get(k)??null,setItem:(k,v)=>values.set(k,v),removeItem:k=>values.delete(k)};
 persistProductionPending(s,'key',{kind:'order',input});let committed=false,posts=0;
 const c={operation:async()=>{if(committed)return result;throw new LocalAPIError(404,'missing');},write:async()=>{posts++;committed=true;throw new LocalAPIError(0,'lost');}};
 await assert.rejects(resolveProductionPending(c,'t',s,'key',true));assert.equal(readProductionPending(s,'key').input.order_id,'o');
 assert.equal((await resolveProductionPending(c,'t',s,'key')).result.order_id,'o');assert.equal(posts,1);assert.equal(readProductionPending(s,'key'),null);
});
