import test from 'node:test';
import assert from 'node:assert/strict';
import {createFinance116} from '../src/core/payables116/source.mjs';
import {contextKey} from '../src/core/context74/model.mjs';
import {queueKey116,save116,read116} from '../src/core/payables116/model.mjs';

const session={tenant_id:'tenant',store_id:'store',device_id:'device',identity_id:'owner',expires_unix:Math.floor(Date.now()/1000)+86400};
const capabilities={...session,permissions:['view_accounting','manage_cash'],license:{state:'active',modules:['core','finance']}};
const key=queueKey116(contextKey(session));
const command=()=>({version:1,document:'document',kind:'pay',installment:'part',amount_cents:3000,paymentID:null,body:{operation_id:'operation',payment_id:'payment',installment_id:'part',withdrawal_operation_id:'withdraw',recipient:'Recebedor',reference:'REF-1'},uncertain:false});
const receipt=()=>({operation_id:'operation',repeated:false,payment:{id:'payment',payable_id:'document',installment_id:'part',withdrawal_operation_id:'withdraw',amount_cents:3000,actor_id:'owner',recipient:'Recebedor',reference:'REF-1',created_at:'2026-10-20T10:00:00Z',operation_id:'operation',return:null}});
const memory=()=>{const data=new Map();return {getItem:k=>data.get(k)??null,setItem:(k,v)=>data.set(k,v),removeItem:k=>data.delete(k)};};
const locks={request:async(n,o,fn)=>fn({name:n})};
const json=(v,status=200)=>new Response(JSON.stringify(v),{status});
const make=(storage,fetcher,extra={})=>createFinance116({token:'fixture-only',session,capabilities,storage,locks,fetcher,...extra});

test('70 the submitted command is captured before waiting for its Web Lock',async()=>{
 const storage=memory(),input=command();let release;const waiting=new Promise(r=>release=r);const urls=[];
 const client=make(storage,async u=>{urls.push(u);return json(receipt());},{locks:{request:async(n,o,fn)=>{await waiting;return fn({name:n});}}});
 const result=client.submit(input);input.document='another';input.body.operation_id='another-operation';input.body.withdrawal_operation_id='another-withdrawal';input.body.reference='changed';release();
 assert.equal((await result).state,'confirmed');assert.deepEqual(urls,['/local/v1/finance/payable-cash/operations/operation']);assert.equal(client.pending(),null);
});

test('70 recovery preserves a replacement command arriving while the receipt is decoded',async()=>{
 const storage=memory(),original=command(),replacement={...command(),body:{...command().body,operation_id:'replacement-operation',payment_id:'replacement-payment'}};save116(storage,key,original);
 const client=make(storage,async()=>({ok:true,status:200,text:async()=>{save116(storage,key,replacement);return JSON.stringify(receipt());}}));
 await assert.rejects(()=>client.recover(),/storage116/);assert.deepEqual(client.pending(),replacement);
});

test('70 a failed storage removal remains recoverable through GET without another POST',async()=>{
 const storage=memory();save116(storage,key,{...command(),uncertain:true});const remove=storage.removeItem;storage.removeItem=()=>{};let posts=0,gets=0;
 const client=make(storage,async(_u,o)=>{if(o.method==='POST')posts++;else gets++;return json(receipt());});
 await assert.rejects(()=>client.recover(),/storage116/);assert.ok(client.pending());storage.removeItem=remove;
 assert.equal((await client.recover()).state,'confirmed');assert.equal(posts,0);assert.equal(gets,2);assert.equal(client.pending(),null);
});

test('70 equivalent JSON property order may be cleared after matching receipt',async()=>{
 const storage=memory();save116(storage,key,command());const client=make(storage,async()=>({ok:true,status:200,text:async()=>{const c=command();storage.setItem(key,JSON.stringify(Object.fromEntries(Object.entries(c).reverse())));return JSON.stringify(receipt());}}));
 assert.equal((await client.recover()).state,'confirmed');assert.equal(read116(storage,key),null);
});
