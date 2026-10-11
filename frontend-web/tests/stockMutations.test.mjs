import test from 'node:test';
import assert from 'node:assert/strict';
import {createStockMutations,prepareStockOperation,validStockInput,stockPendingKey,readStockPending,persistStockPending,resolveStockPending} from '../src/core/local/stockMutations.mjs';
const product={id:'p',unit:'g'};
const make=(kind='entry')=>prepareStockOperation(product,{kind,from:'a',to:'b',quantity:'1.001',reason:' Conferência '},'op');
const receipt=input=>({input,actor_id:'actor',device_id:'device',occurred_at:'2026-10-10T12:00:00Z',unit:'g'});
const storage=()=>{const map=new Map();return {getItem:k=>map.get(k)??null,setItem:(k,v)=>map.set(k,v),removeItem:k=>map.delete(k)}};
test('stock entry transfer loss use exact scale and compatible explicit units',()=>{
 for(const kind of ['entry','transfer','loss']){const input=make(kind);assert.equal(input.quantity_milli,1001);assert.ok(validStockInput(kind,input));assert.equal(input.reason,'Conferência');}
 assert.equal(prepareStockOperation(product,{kind:'entry',from:'',to:'b',quantity:'9007199254740.991',reason:'X'},'op').quantity_milli,Number.MAX_SAFE_INTEGER);
 for(const quantity of ['0','-1','1e3','1.0001','9007199254740.992'])assert.throws(()=>prepareStockOperation(product,{kind:'entry',from:'',to:'b',quantity,reason:'X'},'op'));
 assert.throws(()=>prepareStockOperation({id:'p',unit:'unit'},{kind:'entry',from:'',to:'b',quantity:'1.001',reason:'X'},'op'));
 assert.throws(()=>prepareStockOperation(product,{kind:'transfer',from:'a',to:'a',quantity:'1',reason:'X'},'op'));
 assert.throws(()=>prepareStockOperation(product,{kind:'loss',from:'a',to:'',quantity:'1',reason:'é'.repeat(128)},'op'));
});
test('stock POST binds exact original payload and GET refuses mismatched receipt',async()=>{
 for(const kind of ['entry','transfer','loss']){const input=make(kind);let methods=[];const c=createStockMutations(async(url,opt)=>{methods.push(opt.method);assert.equal(opt.credentials,'omit');assert.equal(opt.headers.Authorization,'Bearer t');assert.ok(!Object.hasOwn(JSON.parse(opt.body||'{}'),'actor_id'));if(opt.method==='POST'){assert.equal(url,'/local/v1/stock/operations');assert.deepEqual(JSON.parse(opt.body),input);return new Response(JSON.stringify({operation_id:'op',repeated:false}));}return new Response(JSON.stringify(receipt(input)));});await c.write('t',kind,input);await c.operation('t',kind,input);assert.deepEqual(methods,['POST','GET']);await assert.rejects(c.operation('t',kind,{...input,quantity_milli:1002}));}
});
test('lost stock reply resolves by immutable GET after reload, only one POST',async()=>{
 for(const kind of ['entry','transfer','loss']){const input=make(kind),s=storage();persistStockPending(s,'key',{kind,input});let exists=false,posts=0;const c=createStockMutations(async(_,opt)=>{if(opt.method==='POST'){posts++;exists=true;throw Error('lost');}return exists?new Response(JSON.stringify(receipt(input))):new Response(null,{status:404});});await assert.rejects(resolveStockPending(c,'t',s,'key',true));assert.equal(readStockPending(s,'key').uncertain,true);assert.equal((await resolveStockPending(c,'t',s,'key')).state,'confirmed');assert.equal(posts,1);assert.equal(s.getItem('key'),null);}
});
test('legacy entry pending preserves same operation and cannot be discarded when uncertain',async()=>{
 const input=make(),s=storage(),{from_location_id,expected_unit,...legacy}=input;s.setItem('key',JSON.stringify(legacy));assert.deepEqual(readStockPending(s,'key'),{kind:'entry',input:{...input,expected_unit:''},uncertain:true});let calls=0;const c=createStockMutations(async(_,opt)=>{calls++;assert.equal(opt.method,'GET');return new Response(JSON.stringify(receipt(input)));});await resolveStockPending(c,'t',s,'key');assert.equal(calls,1);assert.equal(s.getItem('key'),null);
 s.setItem('key',JSON.stringify(legacy));const missing=createStockMutations(async()=>new Response(null,{status:404}));await assert.rejects(resolveStockPending(missing,'t',s,'key',false,true));assert.ok(s.getItem('key'));
});
test('definite refusal permits deliberate discard, later refusal preserves previous uncertainty',async()=>{
 const input=make(),s=storage();persistStockPending(s,'key',{kind:'entry',input});const c=createStockMutations(async(_,opt)=>new Response(null,{status:opt.method==='POST'?409:404}));await assert.rejects(resolveStockPending(c,'t',s,'key',true));assert.equal(readStockPending(s,'key').uncertain,false);assert.equal((await resolveStockPending(c,'t',s,'key',false,true)).state,'discarded');persistStockPending(s,'key',{kind:'entry',input});const p=readStockPending(s,'key');s.setItem('key',JSON.stringify({...p,uncertain:true}));await assert.rejects(resolveStockPending(c,'t',s,'key',true));assert.equal(readStockPending(s,'key').uncertain,true);
});
test('malformed pending storage and failed durability block POST; contexts use legacy-compatible keys',async()=>{
 const ctx={tenant_id:'t',store_id:'s',device_id:'d',identity_id:'i'};assert.equal(stockPendingKey(ctx),'titan-stock-entry:t:s:d:i');for(const k of Object.keys(ctx))assert.notEqual(stockPendingKey(ctx),stockPendingKey({...ctx,[k]:'other'}));assert.throws(()=>readStockPending({getItem:()=>'{bad'},'key'));assert.throws(()=>persistStockPending({getItem:()=>null,setItem:()=>{throw Error();}},'key',{kind:'entry',input:make()}));
 const s=storage();persistStockPending(s,'key',{kind:'entry',input:make()});s.setItem=()=>{throw Error();};let calls=0;await assert.rejects(resolveStockPending({operation:async()=>{calls++},write:async()=>{calls++}},'t',s,'key',true));assert.equal(calls,0);
});
test('manual history validates pagination context unique operations, every request read only',async()=>{
 const input=make(),item=receipt(input),v={product_id:'p',location_id:'b',items:[item],total_count:1,offset:0,limit:50,has_more:false};const c=createStockMutations(async(url,opt)=>{assert.equal(opt.method,'GET');assert.equal(url,'/local/v1/stock/operations/history?product_id=p&location_id=b&offset=0');return new Response(JSON.stringify(v));});assert.deepEqual(await c.history('t','p','b'),v);
 for(const bad of [{...v,product_id:'other'},{...v,has_more:true},{...v,total_count:2},{...v,items:[item,item],total_count:2},{...v,items:[{...item,input:{...input,to_location_id:'x'}}]}])await assert.rejects(createStockMutations(async()=>new Response(JSON.stringify(bad))).history('t','p','b'));
 for(const status of [401,403,404,409,503])await assert.rejects(createStockMutations(async()=>new Response(null,{status})).receipt('t','op'),e=>e.status===status);
});
test('new stock writes freeze the unit and malformed confirmation never clears uncertainty',async()=>{
 const input=make();assert.equal(input.expected_unit,'g');const s=storage();persistStockPending(s,'key',{kind:'entry',input});let posted=false;
 const c=createStockMutations(async(_,opt)=>{if(opt.method==='POST'){assert.equal(JSON.parse(opt.body).expected_unit,'g');posted=true;return new Response(JSON.stringify({operation_id:'op',repeated:false}));}return posted?new Response(JSON.stringify(receipt({...input,quantity_milli:1002}))):new Response(null,{status:404});});
 await assert.rejects(resolveStockPending(c,'t',s,'key',true));assert.equal(readStockPending(s,'key').uncertain,true);
});
