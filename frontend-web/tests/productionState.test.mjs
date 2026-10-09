import test from 'node:test';
import assert from 'node:assert/strict';
import {transitionChoices,prepareProductionState} from '../src/core/local/productionState.mjs';
import {createProductionMutations,validProductionInput,persistProductionPending,readProductionPending,resolveProductionPending} from '../src/core/local/productionMutations.mjs';
import {LocalAPIError} from '../src/core/local/localClient.mjs';
const recipe={recipe_id:'r',version_id:'v',revision:1,name:'Pão',output_product_id:'bread',output_unit:'unit',yield_milli:10000,ingredients:[{product_id:'flour',unit:'g',quantity_milli:500000}]};
const order={id:'o',version_id:'v',location_id:'l',responsible_id:'person',planned_batches:3,planned_output_milli:30000,revision:1,status:'planned',created_at:'2026-10-09',recipe};
test('UI decisions only planned->approved/cancelled and approved->cancelled, terminals closed',()=>{
 assert.deepEqual(transitionChoices(order),['approved','cancelled']);assert.deepEqual(transitionChoices({...order,status:'approved',revision:2}),['cancelled']);
 for(const status of ['completed','cancelled','unknown'])assert.deepEqual(transitionChoices({...order,status}),[]);
 assert.deepEqual(transitionChoices({...order,revision:2147483647}),[]);
});
test('state input takes expected revision from consulted order, UTF8 reason bounded',()=>{
 const input=prepareProductionState(order,'approved','  Plano conferido  ','approve-op');assert.equal(input.expected_revision,1);assert.equal(input.reason,'Plano conferido');
 assert.throws(()=>prepareProductionState(order,'completed','reason','op'));assert.throws(()=>prepareProductionState(order,'approved','á'.repeat(128),'op'));
 assert.ok(!validProductionInput('state',{...input,expected_revision:1.5}));assert.ok(!validProductionInput('state',{...input,release_reservations:true}));
});
test('approval posts expected state and reason only, original receipt survives later cancel',async()=>{
 const input=prepareProductionState(order,'approved','Plano conferido','op'),result={order_id:'o',revision:2,status:'approved',repeated:false};
 const c=createProductionMutations(async(url,opt)=>{if(opt.method==='POST'){assert.equal(url,'/local/v1/production/orders/state');assert.deepEqual(JSON.parse(opt.body),input);return new Response(JSON.stringify(result));}return new Response(JSON.stringify({kind:'state',input,result}));});
 assert.deepEqual(await c.write('t','state',input),result);assert.equal((await c.operation('t','state',input)).status,'approved');await assert.rejects(c.operation('t','state',{...input,expected_revision:2}));
});
test('conflicting stale state never automatically retries with a new revision',async()=>{
 const m=new Map(),s={getItem:k=>m.get(k)??null,setItem:(k,v)=>m.set(k,v),removeItem:k=>m.delete(k)},input=prepareProductionState(order,'cancelled','Cancelado pelo operador','op');
 persistProductionPending(s,'k',{kind:'state',input});let posts=0;const c={operation:async()=>{throw new LocalAPIError(404,'missing');},write:async(t,k,saved)=>{posts++;assert.equal(saved.expected_revision,1);throw new LocalAPIError(409,'stale');}};
 await assert.rejects(resolveProductionPending(c,'t',s,'k',true));await resolveProductionPending(c,'t',s,'k');assert.equal(posts,1);assert.equal(readProductionPending(s,'k').input.expected_revision,1);
});
