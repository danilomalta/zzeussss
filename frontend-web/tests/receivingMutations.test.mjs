import test from 'node:test';
import assert from 'node:assert/strict';
import {createReceivingMutations,validReceivingSnapshot,prepareReceivingAuthorization,prepareReceivingDelivery,prepareReceivingVoid,receivingPendingKey,readReceivingPending,persistReceivingPending,resolveReceivingPending} from '../src/core/local/receivingMutations.mjs';
const order={id:'order',supplier_id:'supplier',status:'local_not_sent',receiving_status:'not_authorized',items:[{product_id:'p',unit:'g',quantity_milli:5000}]};
const audit={actor_id:'actor',device_id:'device',created_at:'2026-10-11T00:00:00Z'};
const auth={operation_id:'auth',reference:'document',reason:'Decisão',...audit};
const snapshot=()=>({order_id:'order',product_id:'p',unit:'g',commercial_status:'local_not_sent',receiving_status:'authorized',authorization:auth,planned_milli:5000,accepted_milli:0,delivered_milli:0,rejected_milli:0,remaining_milli:5000,total_count:0,offset:0,limit:50,has_more:false,voided_count:0,voided_accepted_milli_exact:'0',items:[]});
const values={reference:'delivery',location:'room',delivered:'2.5',accepted:'2',reason:'500 recusados'};
const entryInput=()=>prepareReceivingDelivery(snapshot(),'receipt',values,'receipt-op');
const receipt=()=>({...entryInput(),id:'rid',product_id:'p',stock_operation_id:'stock-id',movement_id:'movement',...audit});
const withReceipt=()=>({...snapshot(),receiving_status:'partially_received',delivered_milli:2500,accepted_milli:2000,rejected_milli:500,remaining_milli:3000,total_count:1,items:[receipt()]});
const inputs=()=>({authorize:prepareReceivingAuthorization(order,'document','Decisão','auth'),receipt:entryInput(),reject:prepareReceivingDelivery(snapshot(),'reject',values,'reject-op'),void:prepareReceivingVoid(withReceipt(),'rid','Correção','void-op')});
function envelope(kind,input){let result={repeated:false};if(kind==='authorize')result={...result,order_id:input.order_id,authorization:{operation_id:input.operation_id,reference:input.reference,reason:input.reason,...audit}};if(kind==='receipt')result.receipt={...input,id:'rid',product_id:'p',stock_operation_id:'stock-id',movement_id:'movement',...audit};if(kind==='reject')result.rejection={...input,id:'rejection',product_id:'p',...audit};if(kind==='void')result.void={...input,quantity_milli:2000,unit:'g',product_id:'p',location_id:'room',stock_operation_id:'void-stock',movement_id:'void-move',...audit};return {kind,input,result};}
const storage=()=>{const m=new Map();return {getItem:k=>m.get(k)??null,setItem:(k,v)=>m.set(k,v),removeItem:k=>m.delete(k)}};
test('receiving authorization freezes consulted local order, rejects cancelled or already authorized',()=>{
 assert.deepEqual(inputs().authorize,{operation_id:'auth',order_id:'order',reference:'document',reason:'Decisão'});
 for(const v of [{...order,status:'cancelled'},{...order,receiving_status:'authorized'}])assert.throws(()=>prepareReceivingAuthorization(v,'ref','reason','op'));
 assert.throws(()=>prepareReceivingAuthorization(order,'ref','é'.repeat(128),'op'));assert.throws(()=>prepareReceivingAuthorization(order,'é'.repeat(65),'reason','op'));
});
test('accepted and rejected deliveries use exact explicit units, remaining cannot be exceeded',()=>{
 const p=inputs().receipt;assert.equal(p.delivered_milli,2500);assert.equal(p.accepted_milli,2000);assert.equal(p.unit,'g');assert.equal(inputs().reject.location_id,undefined);
 for(const v of [{...values,accepted:'6'},{...values,accepted:'3'},{...values,accepted:'0'},{...values,delivered:'1e3'},{...values,delivered:'1.0001'},{...values,delivered:'9007199254740.992'}])assert.throws(()=>prepareReceivingDelivery(snapshot(),'receipt',v,'op'));
 const s={...snapshot(),unit:'ml',planned_milli:Number.MAX_SAFE_INTEGER,remaining_milli:Number.MAX_SAFE_INTEGER};assert.equal(prepareReceivingDelivery(s,'receipt',{...values,delivered:'9007199254740.991',accepted:'9007199254740.991'},'op').accepted_milli,Number.MAX_SAFE_INTEGER);
 assert.throws(()=>prepareReceivingDelivery({...snapshot(),authorization:null},'reject',values,'op'));
});
test('void preserves original receipt and refuses another order, absent page item or already voided',()=>{
 assert.equal(inputs().void.receipt_id,'rid');assert.throws(()=>prepareReceivingVoid(snapshot(),'rid','reason','op'));const s=withReceipt(),vi=inputs().void;
 const v=envelope('void',vi).result.void;s.items[0].void=v;s.voided_count=1;s.voided_accepted_milli_exact='2000';s.accepted_milli=0;s.delivered_milli=0;s.rejected_milli=0;s.remaining_milli=5000;s.receiving_status='authorized';assert.ok(validReceivingSnapshot(s,'order'));assert.throws(()=>prepareReceivingVoid(s,'rid','reason','op'));
 assert.throws(()=>prepareReceivingVoid({...withReceipt(),order_id:'other'},'rid','reason','op'));
});
test('all four POSTs bind order path and void receipt path without actor or scope in body',async()=>{
 for(const [kind,input] of Object.entries(inputs())){const e=envelope(kind,input);const c=createReceivingMutations(async(url,opt)=>{if(opt.method==='POST'){const suffix={authorize:'authorize-receiving',receipt:'receipts',reject:'rejections',void:'receipts/rid/void'}[kind];assert.equal(url,'/local/v1/purchase-orders/order/'+suffix);const {order_id,receipt_id,...body}=input;assert.deepEqual(JSON.parse(opt.body),body);assert.equal(opt.credentials,'omit');assert.equal(opt.headers.Authorization,'Bearer t');return new Response(JSON.stringify(e.result));}assert.equal(url,`/local/v1/purchase-receiving/operations/${kind}/${input.operation_id}/receipt`);return new Response(JSON.stringify(e));});await c.write('t',kind,input);assert.deepEqual(await c.operation('t',kind,input),e.result);await assert.rejects(c.operation('t',kind,{...input,reason:'Other'}));}
});
test('lost response resolves each original receipt without repost, including original after later void',async()=>{
 for(const [kind,input] of Object.entries(inputs())){const s=storage();persistReceivingPending(s,'key',{kind,input});let exists=false,posts=0;const e=envelope(kind,input);const c=createReceivingMutations(async(_,opt)=>{if(opt.method==='POST'){posts++;exists=true;throw Error('lost');}return exists?new Response(JSON.stringify(e)):new Response(null,{status:404});});await assert.rejects(resolveReceivingPending(c,'t',s,'key',true));assert.equal(readReceivingPending(s,'key').uncertain,true);assert.equal((await resolveReceivingPending(c,'t',s,'key')).state,'confirmed');assert.equal(posts,1);assert.equal(s.getItem('key'),null);}
});
test('uncertain not found cannot be discarded, first definite refusal can, prior uncertainty remains',async()=>{
 const s=storage(),input=inputs().receipt;persistReceivingPending(s,'key',{kind:'receipt',input});const c=createReceivingMutations(async(_,opt)=>new Response(null,{status:opt.method==='POST'?409:404}));await assert.rejects(resolveReceivingPending(c,'t',s,'key',true));assert.equal(readReceivingPending(s,'key').uncertain,false);assert.equal((await resolveReceivingPending(c,'t',s,'key',false,true)).state,'discarded');persistReceivingPending(s,'key',{kind:'receipt',input});s.setItem('key',JSON.stringify({...readReceivingPending(s,'key'),uncertain:true}));await assert.rejects(resolveReceivingPending(c,'t',s,'key',true));await assert.rejects(resolveReceivingPending(c,'t',s,'key',false,true));assert.equal(readReceivingPending(s,'key').uncertain,true);
});
test('pending storage isolates full context, unknown or corrupt kinds block writes before network',()=>{
 const s={tenant_id:'t',store_id:'s',device_id:'d',identity_id:'a'};for(const k of Object.keys(s))assert.notEqual(receivingPendingKey(s),receivingPendingKey({...s,[k]:'other'}));assert.throws(()=>readReceivingPending({getItem:()=>'{bad'},'key'));assert.throws(()=>persistReceivingPending({getItem:()=>null,setItem:()=>{throw Error()}},'key',{kind:'receipt',input:inputs().receipt}));assert.throws(()=>persistReceivingPending(storage(),'key',{kind:'future',input:inputs().receipt}));
});
test('receiving snapshot validates global totals, selected context, page counts and void linkage',async()=>{
 const good=withReceipt();assert.ok(validReceivingSnapshot(good,'order'));
 for(const v of [{...good,order_id:'other'},{...good,remaining_milli:3001},{...good,total_count:2},{...good,has_more:true},{...good,receiving_status:'received'},{...good,items:[{...receipt(),accepted_milli:1999}]},{...good,items:[{...receipt(),product_id:'other'}]}])assert.equal(!!validReceivingSnapshot(v,'order'),false);
 const c=createReceivingMutations(async(url,opt)=>{assert.equal(opt.method,'GET');assert.equal(url,'/local/v1/purchase-orders/order/receiving-trace?offset=0');return new Response(JSON.stringify(good));});assert.deepEqual(await c.snapshot('t','order'),good);
 for(const status of [401,403,404,409,503])await assert.rejects(createReceivingMutations(async()=>new Response(null,{status})).snapshot('t','order'),e=>e.status===status);
});
test('malformed confirmed POST or original result cannot clear pending',async()=>{
 const input=inputs().receipt,s=storage();persistReceivingPending(s,'key',{kind:'receipt',input});let sent=false;const e=envelope('receipt',input);e.result.receipt.accepted_milli++;
 const c=createReceivingMutations(async(_,opt)=>{if(opt.method==='POST'){sent=true;return new Response(JSON.stringify(envelope('receipt',input).result));}return sent?new Response(JSON.stringify(e)):new Response(null,{status:404});});await assert.rejects(resolveReceivingPending(c,'t',s,'key',true));assert.equal(readReceivingPending(s,'key').uncertain,true);
});
