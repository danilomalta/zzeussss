import test from 'node:test';
import assert from 'node:assert/strict';
import {LocalAPIError} from '../src/core/local/localClient.mjs';
import {createPurchaseClient,resolvePurchasePending,persistPurchasePending as oldPersist} from '../src/core/local/purchaseClient.mjs';
import {createPurchaseCancelClient,validOrderTrace,preparePurchaseCancel,purchaseCancelPendingKey,persistPurchaseCancelPending,readPurchaseCancelPending,resolvePurchaseCancelPending} from '../src/core/local/purchaseCancel.mjs';
const now='2026-10-11T01:00:00Z';
const order={id:'order',operation_id:'created',supplier_id:'supplier',supplier_name:'Padaria original',suggestion_id:'suggestion',status:'local_not_sent',receiving_status:'not_authorized',created_at:now,approved_by:'reviewer',approved_at:now,items:[{product_id:'product',sku:'P1',name:'Produto original',unit:'kg',quantity_milli:1200}]};
const trace={order,creation:{kind:'purchase.created',operation_id:'created',device_id:'device',actor_id:'creator',created_at:now},approval:{suggestion_id:'suggestion',operation_id:'approval',device_id:'device',reviewer_id:'reviewer',decision:'approved',reason:'Repor gôndola',decided_at:now}};
const input=preparePurchaseCancel(trace,'Pedido desistido','cancel-op');
const op={kind:'cancel',input};
const cancellation={operation_id:'cancel-op',device_id:'device',actor_id:'operator',reason:input.reason,created_at:now};
const result={order_id:'order',status:'cancelled',repeated:false,cancellation};
const receipt={input,result};
const session={tenant_id:'tenant',store_id:'store',device_id:'device',identity_id:'operator'};
const response=(v,status=200)=>({ok:status===200,status,json:async()=>structuredClone(v)});
const error=status=>new LocalAPIError(status,'refused');
function storage(){const m=new Map();return {getItem:k=>m.get(k)??null,setItem:(k,v)=>m.set(k,v),removeItem:k=>m.delete(k)};}
test('order trace ties preserved items to approval and creation; rejects foreign or malformed evidence',()=>{
 assert.equal(validOrderTrace(trace,'order'),true);
 const changes=[{order:{...order,id:'foreign'}},{approval:{...trace.approval,reviewer_id:'other'}},{approval:{...trace.approval,suggestion_id:'other'}},{approval:{...trace.approval,decision:'rejected'}},{creation:{...trace.creation,operation_id:'other'}},{order:{...order,items:[{...order.items[0],quantity_milli:Number.MAX_SAFE_INTEGER+1}]}},{order:{...order,items:[]}},{order:{...order,status:'sent'}},{order:{...order,receiving_status:'authorized'}}];
 for(const x of changes)assert.equal(validOrderTrace({...trace,...x},'order'),false);
 assert.equal(validOrderTrace({...trace,order:{...order,status:'cancelled'},cancellation},'order'),true);
});
test('cancel preview freezes expected commercial status, reason and exact order ID',()=>{
 assert.deepEqual(input,{operation_id:'cancel-op',order_id:'order',expected_status:'local_not_sent',reason:'Pedido desistido'});
 for(const why of ['', 'á'.repeat(128),'bad\nreason'])assert.throws(()=>preparePurchaseCancel(trace,why,'op'));
 assert.throws(()=>preparePurchaseCancel(trace,'why','á'.repeat(65)));
 assert.throws(()=>preparePurchaseCancel({...trace,order:{...order,status:'cancelled'},cancellation},'why','op'));
 const auth={operation_id:'auth',actor_id:'operator',device_id:'device',reference:'reference',reason:'Receber',created_at:now};
 const authorized={...trace,order:{...order,receiving_status:'authorized'},receiving_authorization:auth};assert.equal(validOrderTrace(authorized,'order'),true);assert.throws(()=>preparePurchaseCancel(authorized,'why','op'));
});
test('cancel client uses exact URL/body, no scope fields and no automatic writes during queries',async()=>{
 const calls=[];const c=createPurchaseCancelClient(async(url,o)=>{calls.push({url,o});return response(url.endsWith('/trace')?trace:o.method==='POST'?result:receipt);});
 assert.deepEqual(await c.trace('opaque','order'),trace);assert.deepEqual(await c.receipt('opaque','cancel-op'),receipt);assert.equal(calls.filter(x=>x.o.method==='POST').length,0);
 await c.write('opaque','cancel',input);const last=calls.at(-1);assert.equal(last.url,'/local/v1/purchase-orders/order/cancel');assert.equal(last.o.credentials,'omit');assert.equal(last.o.headers.Authorization,'Bearer opaque');const {order_id,...body}=input;assert.deepEqual(JSON.parse(last.o.body),body);
 await assert.rejects(()=>c.write('opaque','cancel',{...input,tenant_id:'foreign'}));assert.equal(calls.length,3);
});
test('malformed or mismatched original cancellation never confirms success',async()=>{
 for(const change of [{input:{...input,reason:'different'}},{input:{...input,order_id:'foreign'}},{result:{...result,order_id:'foreign'}},{result:{...result,repeated:true}},{result:{...result,cancellation:{...cancellation,operation_id:'foreign'}}}]){
 const c=createPurchaseCancelClient(async()=>response({...receipt,...change}));await assert.rejects(()=>c.operation('token','cancel',input));
 }
});
test('lost cancel reply resolves by original GET after reload with only one POST',async()=>{
 const s=storage(),key=purchaseCancelPendingKey(session);persistPurchaseCancelPending(s,key,op);let stored=false,posts=0;
 const c=createPurchaseCancelClient(async(url,o)=>{if(o.method==='POST'){stored=true;posts++;throw Error('lost');}return stored?response(receipt):response(null,404);});
 await assert.rejects(()=>resolvePurchaseCancelPending(c,'token',s,key,true));assert.equal(readPurchaseCancelPending(s,key).uncertain,true);
 assert.equal((await resolvePurchaseCancelPending(c,'token',s,key)).state,'confirmed');assert.equal(posts,1);assert.equal(readPurchaseCancelPending(s,key),null);
});
test('unknown GET refusal cannot send POST; explicit retry keeps same original fields',async()=>{
 const s=storage(),key=purchaseCancelPendingKey(session);persistPurchaseCancelPending(s,key,op);let posts=0;let saved=false;
 const c={operation:async()=>{throw error(403);},write:async()=>{posts++;}};await assert.rejects(()=>resolvePurchaseCancelPending(c,'token',s,key,true));assert.equal(posts,0);
 c.operation=async()=>{if(saved)return result;throw error(404);};c.write=async(t,kind,v)=>{assert.equal(kind,'cancel');assert.deepEqual(v,input);posts++;saved=true;};
 assert.equal((await resolvePurchaseCancelPending(c,'token',s,key)).state,'missing');assert.equal(posts,0);
 assert.equal((await resolvePurchaseCancelPending(c,'token',s,key,true)).state,'confirmed');assert.equal(posts,1);
});
test('first definite conflict permits deliberate discard after absent GET; uncertainty survives later conflict',async()=>{
 const s=storage(),key=purchaseCancelPendingKey(session);persistPurchaseCancelPending(s,key,op);
 const c={operation:async()=>{throw error(404);},write:async()=>{throw error(409);}};
 await assert.rejects(()=>resolvePurchaseCancelPending(c,'token',s,key,true));assert.equal(readPurchaseCancelPending(s,key).uncertain,false);
 assert.equal((await resolvePurchaseCancelPending(c,'token',s,key,false,true)).state,'discarded');
 persistPurchaseCancelPending(s,key,op);c.write=async()=>{throw Error('lost');};await assert.rejects(()=>resolvePurchaseCancelPending(c,'token',s,key,true));c.write=async()=>{throw error(409);};await assert.rejects(()=>resolvePurchaseCancelPending(c,'token',s,key,true));
 assert.equal(readPurchaseCancelPending(s,key).uncertain,true);await assert.rejects(()=>resolvePurchaseCancelPending(c,'token',s,key,false,true));assert.ok(readPurchaseCancelPending(s,key));
});
test('successful cancellation POST with invalid confirmation preserves pending and blocks a new decision',async()=>{
 const s=storage(),key=purchaseCancelPendingKey(session);persistPurchaseCancelPending(s,key,op);let posted=false;
 const c=createPurchaseCancelClient(async(url,o)=>{if(o.method==='POST'){posted=true;return response(result);}return posted?response({...receipt,result:{...result,status:'local_not_sent'}}):response(null,404);});
 await assert.rejects(()=>resolvePurchaseCancelPending(c,'token',s,key,true));assert.equal(readPurchaseCancelPending(s,key).uncertain,true);assert.throws(()=>persistPurchaseCancelPending(s,key,op));
});
test('cancel queues isolate all identity dimensions and durable-storage failure blocks POST',()=>{
 const key=purchaseCancelPendingKey(session);for(const field of Object.keys(session))assert.notEqual(key,purchaseCancelPendingKey({...session,[field]:'other'}));
 const s=storage();s.setItem(key,'broken');assert.throws(()=>readPurchaseCancelPending(s,key));
 assert.throws(()=>persistPurchaseCancelPending({getItem:()=>null,setItem:()=>{throw Error('full');}},key,op));
});
test('empty token and invalid IDs fail before network and encoded slash stays one parameter',async()=>{
 const urls=[];const c=createPurchaseCancelClient(async url=>{urls.push(url);return response({...trace,order:{...order,id:'a/b'}});});
 await assert.rejects(()=>c.trace('', 'order'));await assert.rejects(()=>c.trace('token','á'.repeat(65)));assert.equal(urls.length,0);
 await c.trace('token','a/b');assert.match(urls[0],/a%2Fb\/trace$/);
});
test('legacy purchase list/detail accepts cancelled but still refuses invented sent state',async()=>{
 const cancelled={...order,status:'cancelled'};
 const c=createPurchaseClient(async url=>response(url.includes('?')?{items:[cancelled]}:cancelled));assert.deepEqual(await c.orders('token'),[cancelled]);assert.deepEqual(await c.order('token','order'),cancelled);
 const bad=createPurchaseClient(async()=>response({...order,status:'sent'}));await assert.rejects(()=>bad.order('token','order'));
});
test('old creation queue resolves preserved original IDs even if the order was later cancelled',async()=>{
 const s=storage(),original={operation_id:'created',order_id:'order',supplier_id:'supplier',suggestion_id:'suggestion'};oldPersist(s,'old',{kind:'order',input:original});let posts=0;
 const c=createPurchaseClient(async(url,o)=>{if(o.method==='POST')posts++;return response({...order,status:'cancelled'});});
 assert.equal((await resolvePurchasePending(c,'token',s,'old')).state,'order_confirmed');assert.equal(posts,0);assert.equal(s.getItem('old'),null);
});
