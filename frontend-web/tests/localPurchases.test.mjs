import test from 'node:test';
import assert from 'node:assert/strict';
import {createPurchaseClient,pendingPurchaseKey,readPurchasePending,persistPurchasePending,resolvePurchasePending} from '../src/core/local/purchaseClient.mjs';
import {LocalAPIError} from '../src/core/local/localClient.mjs';
import {areaForRoute,allowedAreas} from '../src/core/local/accessModel.mjs';
const input={operation_id:'op',order_id:'order',supplier_id:'supplier',suggestion_id:'suggestion'};
const order={id:'order',operation_id:'op',supplier_id:'supplier',supplier_name:'Padaria',suggestion_id:'suggestion',approved_by:'manager',approved_at:'now',created_at:'now',status:'local_not_sent',items:[{product_id:'product',sku:'A1',name:'Arroz',unit:'unit',quantity_milli:5000}]};
const response=v=>new Response(JSON.stringify(v),{status:200,headers:{'Content-Type':'application/json'}});
const storage=()=>{const m=new Map();return {getItem:k=>m.get(k)??null,setItem:(k,v)=>m.set(k,v),removeItem:k=>m.delete(k)};};
test('purchase client rejects unsafe input and forged sent status',async()=>{
 let calls=0;const c=createPurchaseClient(async()=>{calls++;return response({...order,status:'sent'});});
 await assert.rejects(()=>c.createOrder('token',{...input,tenant_id:'foreign'}));assert.equal(calls,0);
 await assert.rejects(()=>c.order('token','order'));assert.equal(calls,1);
 await assert.rejects(()=>createPurchaseClient(async()=>response({...order,items:[{...order.items[0],quantity_milli:Number.MAX_SAFE_INTEGER+1}]})).order('token','order'));
});
test('pending order remains after lost reply and reload consults instead of posting',async()=>{
 const s=storage();persistPurchasePending(s,'key',{kind:'order',input});let stored=false,posts=0;
 const c={order:async()=>{if(!stored)throw new LocalAPIError(404,'missing');return order;},createOrder:async()=>{posts++;stored=true;throw new LocalAPIError(0,'lost');}};
 await assert.rejects(()=>resolvePurchasePending(c,'token',s,'key',true));assert.ok(readPurchasePending(s,'key'));
 const result=await resolvePurchasePending(c,'token',s,'key',false);assert.equal(result.state,'order_confirmed');assert.equal(posts,1);assert.equal(s.getItem('key'),null);
});
test('query failure never triggers POST; explicit retry uses original IDs and payload',async()=>{
 const s=storage();persistPurchasePending(s,'key',{kind:'order',input});let posts=0;
 await assert.rejects(()=>resolvePurchasePending({order:async()=>{throw new LocalAPIError(0,'offline');},createOrder:async()=>{posts++;}},'token',s,'key',true));assert.equal(posts,0);
 let stored=false;const c={order:async()=>{if(!stored)throw new LocalAPIError(404,'missing');return order;},createOrder:async(t,v)=>{assert.deepEqual(v,input);posts++;stored=true;}};
 assert.equal((await resolvePurchasePending(c,'token',s,'key',false)).state,'order_missing');assert.equal(posts,0);
 await resolvePurchasePending(c,'token',s,'key',true);assert.equal(posts,1);
});
test('wrong receipt or failed confirmation preserves pending operation',async()=>{
 const s=storage();persistPurchasePending(s,'key',{kind:'order',input});
 await assert.rejects(()=>resolvePurchasePending({order:async()=>({...order,supplier_id:'other'})},'token',s,'key',false));assert.ok(readPurchasePending(s,'key'));
 let requests=0;await assert.rejects(()=>resolvePurchasePending({order:async()=>{requests++;throw new LocalAPIError(requests===1?404:0,'unknown');},createOrder:async()=>{}},'token',s,'key',true));assert.ok(readPurchasePending(s,'key'));
});
test('browser persistence failure and another pending operation block creation',()=>{
 assert.throws(()=>persistPurchasePending({getItem:()=>null,setItem:()=>{throw new Error('full');}},'key',{kind:'order',input}));
 const s=storage();persistPurchasePending(s,'key',{kind:'order',input});assert.throws(()=>persistPurchasePending(s,'key',{kind:'order',input:{...input,operation_id:'other'}}));
 s.setItem('key','broken');assert.throws(()=>readPurchasePending(s,'key'));
});
test('supplier retry preserves ID and never claims external account creation',async()=>{
 const s=storage(),v={operation_id:'op',id:'supplier',name:'Padaria',status:'active'};persistPurchasePending(s,'key',{kind:'supplier',input:v});let calls=0;
 const c={createSupplier:async(t,incoming)=>{assert.deepEqual(incoming,v);calls++;return {id:v.id,repeated:true};}};
 assert.equal((await resolvePurchasePending(c,'token',s,'key',false)).state,'supplier_uncertain');assert.equal(calls,0);
 await resolvePurchasePending(c,'token',s,'key',true);assert.equal(calls,1);assert.equal(s.getItem('key'),null);
});
test('purchase state and route are scoped to company store device and operator',()=>{
 const session={tenant_id:'tenant',store_id:'store',device_id:'device',identity_id:'owner'};const first=pendingPurchaseKey(session);
 for(const key of Object.keys(session))assert.notEqual(first,pendingPurchaseKey({...session,[key]:'other'}));
 assert.equal(areaForRoute('/local/orders'),'orders');assert.ok(!allowedAreas({role:'production',permissions:['view_orders'],license:{state:'active',modules:['core','production','inventory']}}).includes('orders'));
});
