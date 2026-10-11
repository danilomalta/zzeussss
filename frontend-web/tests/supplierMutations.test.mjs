import test from 'node:test';
import assert from 'node:assert/strict';
import {LocalAPIError} from '../src/core/local/localClient.mjs';
import {createSupplierMutations,prepareSupplierEdit,prepareSupplierStatus,supplierPendingKey,persistSupplierPending,readSupplierPending,resolveSupplierPending,validSupplierHistory} from '../src/core/local/supplierMutations.mjs';
const state={id:'supplier',name:'Padaria',status:'active',revision:0,repeated:false};
const input=prepareSupplierEdit(state,'Outro nome','Nome revisado','edit-op');
const op={kind:'edit',input};
const result={id:'supplier',name:input.name,status:input.status,revision:1,repeated:false};
const receipt={input,result,before_name:state.name,before_status:state.status,actor_id:'actor',device_id:'device',created_at:'2026-10-11T01:00:00Z'};
const session={tenant_id:'tenant',store_id:'store',device_id:'device',identity_id:'actor'};
function storage(){const m=new Map();return {getItem:k=>m.get(k)??null,setItem:(k,v)=>m.set(k,v),removeItem:k=>m.delete(k)};}
const response=(v,status=200)=>({ok:status===200,status,json:async()=>structuredClone(v)});
const error=status=>new LocalAPIError(status,'refused');
test('supplier name edit freezes consulted revision and preserves status; UTF8 boundaries',()=>{
 assert.deepEqual(input,{operation_id:'edit-op',supplier_id:'supplier',expected_revision:0,name:'Outro nome',status:'active',reason:'Nome revisado'});
 assert.equal(prepareSupplierEdit({...state,status:'inactive'},'New','why','op').status,'inactive');
 for(const [name,reason,id] of [['Padaria','why','op'],['New','','op'],['á'.repeat(128),'why','op'],['New','á'.repeat(128),'op'],['New','why','á'.repeat(65)],['New\nBad','why','op']])assert.throws(()=>prepareSupplierEdit(state,name,reason,id));
 assert.throws(()=>prepareSupplierEdit({...state,revision:2147483647},'New','why','op'));
});
test('activation and inactivation preserve name and require an opposite explicit state',()=>{
 const off=prepareSupplierStatus(state,'inactive','Suspenso','off');assert.equal(off.name,state.name);assert.equal(off.expected_revision,0);
 const on=prepareSupplierStatus({...state,status:'inactive',revision:4},'active','Retomado','on');assert.equal(on.name,state.name);assert.equal(on.expected_revision,4);
 assert.throws(()=>prepareSupplierStatus(state,'active','why','op'));assert.throws(()=>prepareSupplierStatus(state,'deleted','why','op'));
});
test('supplier exact POST body URL and original GET; no scope sent in body',async()=>{
 const calls=[];const client=createSupplierMutations(async(url,options)=>{calls.push({url,options});return response(options.method==='POST'?result:receipt);});
 await client.write('opaque','edit',input);assert.equal(calls[0].url,'/local/v1/purchase-suppliers/supplier/update');assert.equal(calls[0].options.credentials,'omit');assert.equal(calls[0].options.headers.Authorization,'Bearer opaque');
 const {supplier_id,...body}=input;assert.deepEqual(JSON.parse(calls[0].options.body),body);
 assert.deepEqual(await client.operation('opaque','edit',input),result);assert.equal(calls[1].url,'/local/v1/purchase-supplier-operations/edit-op/receipt');assert.equal(calls[1].options.method,'GET');
 await assert.rejects(()=>client.write('opaque','edit',{...input,tenant_id:'foreign'}));assert.equal(calls.length,2);
});
test('missing token and unsafe supplier IDs fail before network, encoded slash stays one parameter',async()=>{
 const urls=[];const client=createSupplierMutations(async url=>{urls.push(url);return response({...state,id:'a/b'});});
 await assert.rejects(()=>client.state('',state.id));await assert.rejects(()=>client.state('token','á'.repeat(65)));assert.equal(urls.length,0);
 await client.state('token','a/b');assert.match(urls[0],/a%2Fb\/edit-state$/);
});
test('later supplier state is never accepted as original receipt',async()=>{
 for(const change of [{result:{...result,name:'later'}},{input:{...input,reason:'altered'}},{result:{...result,revision:2}},{result:{...result,repeated:true}},{before_name:input.name,before_status:input.status}]){
 const c=createSupplierMutations(async()=>response({...receipt,...change}));await assert.rejects(()=>c.operation('token','edit',input));
 }
});
test('lost supplier reply resolves on reload by GET with only one POST',async()=>{
 const s=storage(),key=supplierPendingKey(session);persistSupplierPending(s,key,op);let saved=false,posts=0;
 const c=createSupplierMutations(async(url,o)=>{if(o.method==='POST'){saved=true;posts++;throw Error('lost');}return saved?response(receipt):response(null,404);});
 await assert.rejects(()=>resolveSupplierPending(c,'token',s,key,true));assert.equal(readSupplierPending(s,key).uncertain,true);
 assert.equal((await resolveSupplierPending(c,'token',s,key)).state,'confirmed');assert.equal(posts,1);assert.equal(readSupplierPending(s,key),null);
});
test('explicit repeat preserves exact operation and payload without automatic correction',async()=>{
 const s=storage(),key=supplierPendingKey(session);persistSupplierPending(s,key,op);let posted=false;let body;
 const c=createSupplierMutations(async(url,o)=>{if(o.method==='POST'){posted=true;body=JSON.parse(o.body);return response(result);}return posted?response(receipt):response(null,404);});
 assert.equal((await resolveSupplierPending(c,'token',s,key)).state,'missing');assert.equal(posted,false);
 assert.equal((await resolveSupplierPending(c,'token',s,key,true)).state,'confirmed');assert.equal(body.operation_id,input.operation_id);assert.equal(body.expected_revision,0);
});
test('initial definite revision conflict permits discard only after another absent GET',async()=>{
 const s=storage(),key=supplierPendingKey(session);persistSupplierPending(s,key,op);let reads=0;
 const c={operation:async()=>{reads++;throw error(404);},write:async()=>{throw error(409);}};
 await assert.rejects(()=>resolveSupplierPending(c,'token',s,key,true));assert.equal(readSupplierPending(s,key).uncertain,false);
 assert.equal((await resolveSupplierPending(c,'token',s,key,false,true)).state,'discarded');assert.equal(reads,2);
});
test('unknown GET refusal never sends POST and prior uncertainty cannot be discarded',async()=>{
 const s=storage(),key=supplierPendingKey(session);persistSupplierPending(s,key,op);let writes=0;
 const c={operation:async()=>{throw error(403);},write:async()=>{writes++;}};
 await assert.rejects(()=>resolveSupplierPending(c,'token',s,key,true));assert.equal(writes,0);
 c.operation=async()=>{throw error(404);};c.write=async()=>{writes++;throw Error('lost');};await assert.rejects(()=>resolveSupplierPending(c,'token',s,key,true));
 c.write=async()=>{throw error(409);};await assert.rejects(()=>resolveSupplierPending(c,'token',s,key,true));assert.equal(readSupplierPending(s,key).uncertain,true);
 await assert.rejects(()=>resolveSupplierPending(c,'token',s,key,false,true));assert.ok(readSupplierPending(s,key));
});
test('successful POST with malformed receipt keeps pending and prevents a new operation',async()=>{
 const s=storage(),key=supplierPendingKey(session);persistSupplierPending(s,key,op);let posted=false;
 const c=createSupplierMutations(async(url,o)=>{if(o.method==='POST'){posted=true;return response(result);}return posted?response({...receipt,result:{...result,name:'wrong'}}):response(null,404);});
 await assert.rejects(()=>resolveSupplierPending(c,'token',s,key,true));assert.equal(readSupplierPending(s,key).uncertain,true);assert.throws(()=>persistSupplierPending(s,key,op));
});
test('supplier storage is scoped and failed durable storage stops before POST',async()=>{
 const key=supplierPendingKey(session);for(const field of Object.keys(session))assert.notEqual(key,supplierPendingKey({...session,[field]:'other'}));
 const s=storage();s.setItem(key,'{broken');assert.throws(()=>readSupplierPending(s,key));
 const bad={getItem:()=>null,setItem:()=>{throw Error('quota');}};assert.throws(()=>persistSupplierPending(bad,key,op));
});
const history={current:result,origin:{id:'supplier',name:'Padaria',status:'active',creation:{kind:'supplier.created',operation_id:'created',actor_id:'actor',device_id:'device',created_at:receipt.created_at}},offset:0,limit:50,total_count:1,has_more:false,items:[{revision:1,operation_id:'edit-op',actor_id:'actor',device_id:'device',before_name:'Padaria',before_status:'active',name:input.name,status:'active',reason:input.reason,created_at:receipt.created_at}]};
test('supplier history validates scope original chain totals revisions and page context',async()=>{
 assert.equal(validSupplierHistory(history,'supplier',0),true);
 for(const x of [{...history,total_count:2},{...history,has_more:true},{...history,current:{...result,name:'bad'}},{...history,items:[]},{...history,items:[{...history.items[0],before_name:'bad'}]}])assert.equal(validSupplierHistory(x,'supplier',0),false);
 assert.equal(validSupplierHistory(history,'other',0),false);assert.equal(validSupplierHistory(history,'supplier',50),false);
 let calls=0;const c=createSupplierMutations(async(url,o)=>{calls++;assert.equal(o.method,'GET');assert.match(url,/history\?offset=0$/);return response(history);});assert.deepEqual(await c.history('token','supplier'),history);await assert.rejects(()=>c.history('token','supplier',-1));assert.equal(calls,1);
});
test('history pages preserve global current revision and contiguous revisions on 51 changes',()=>{
 let items=[];for(let i=1;i<=51;i++)items.push({...history.items[0],revision:i,operation_id:`op-${i}`,before_name:i===1?'Padaria':`Name ${i-1}`,name:`Name ${i}`});
 const page={...history,current:{...result,name:'Name 51',revision:51},total_count:51,items:items.slice(0,50),has_more:true};assert.equal(validSupplierHistory(page,'supplier',0),true);
 const next={...page,offset:50,items:items.slice(50),has_more:false};assert.equal(validSupplierHistory(next,'supplier',50),true);
 assert.equal(validSupplierHistory({...next,items:[{...items[50],revision:50}]},'supplier',50),false);
});
