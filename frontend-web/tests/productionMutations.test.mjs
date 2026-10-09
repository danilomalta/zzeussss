import test from 'node:test';
import assert from 'node:assert/strict';
import {LocalAPIError} from '../src/core/local/localClient.mjs';
import {validProductionInput,parseProductionQuantity,parseProductionInteger,plannedPreview,createProductionMutations,productionPendingKey,readProductionPending,persistProductionPending,resolveProductionPending} from '../src/core/local/productionMutations.mjs';
const input={operation_id:'op',recipe_id:'r',version_id:'v',expected_revision:0,name:'Pão',output_product_id:'bread',output_unit:'unit',yield_milli:10000,ingredients:[{product_id:'oil',unit:'ml',quantity_milli:100001},{product_id:'flour',unit:'g',quantity_milli:500000}]};
const result={recipe_id:'r',version_id:'v',revision:1,repeated:false};
const missing=()=>{throw new LocalAPIError(404,'missing');};
const storage=()=>{const values=new Map();return {getItem:k=>values.get(k)??null,setItem:(k,v)=>values.set(k,v),removeItem:k=>values.delete(k)};};
test('exact decimal parsing rejects rounded exponent negative and overflow inputs',()=>{
 assert.equal(parseProductionQuantity('0,001'),1);assert.equal(parseProductionQuantity('9007199254740.991'),Number.MAX_SAFE_INTEGER);
 for(const s of ['0','-1','1e3','1,2345','1.000,5','9007199254740.992'])assert.throws(()=>parseProductionQuantity(s));
 assert.equal(parseProductionInteger('0',true),0);assert.throws(()=>parseProductionInteger('1.0'));assert.throws(()=>parseProductionInteger('9007199254740992'));
});
test('recipe validates ingredients references units duplicate fields and UTF8 limits',()=>{
 assert.ok(validProductionInput('recipe',input));
 for(const delta of [{yield_milli:1.5},{expected_revision:2147483647},{name:'á'.repeat(128)},{tenant_id:'foreign'},{ingredients:[input.ingredients[0],input.ingredients[0]]},{ingredients:[{...input.ingredients[0],product_id:'bread'}]},{ingredients:[{...input.ingredients[0],unit:'box'}]}])assert.ok(!validProductionInput('recipe',{...input,...delta}));
});
test('lookup compares all original fields despite canonical ingredient sorting',async()=>{
 let calls=0;const c=createProductionMutations(async(url,opt)=>{calls++;assert.equal(url,'/local/v1/production/operations/recipe/op');assert.equal(opt.method,'GET');return new Response(JSON.stringify({kind:'recipe',input:{...input,ingredients:[...input.ingredients].reverse()},result}));});
 assert.deepEqual(await c.operation('t','recipe',input),result);
 await assert.rejects(c.operation('t','recipe',{...input,name:'Outro'}));assert.equal(calls,2);
});
test('lost POST reply stays uncertain; reload only consults and explicit retry preserves bytes',async()=>{
 const s=storage();persistProductionPending(s,'key',{kind:'recipe',input});let posts=0,found=false;const bodies=[];
 const client={operation:async()=>found?result:missing(),write:async(t,k,v)=>{posts++;bodies.push(JSON.stringify(v));if(posts===1)throw new LocalAPIError(0,'lost');found=true;return result;}};
 await assert.rejects(resolveProductionPending(client,'t',s,'key',true));assert.equal(readProductionPending(s,'key').uncertain,true);
 assert.equal((await resolveProductionPending(client,'t',s,'key')).state,'missing');assert.equal(posts,1);
 await assert.rejects(resolveProductionPending(client,'t',s,'key',false,true));assert.equal(posts,1);
 assert.equal((await resolveProductionPending(client,'t',s,'key',true)).state,'confirmed');assert.equal(bodies[0],bodies[1]);assert.equal(readProductionPending(s,'key'),null);
});
test('confirmed query never resends and unknown query failures never post',async()=>{
 for(const status of [403,409,0]){const s=storage();persistProductionPending(s,'k',{kind:'recipe',input});let writes=0;await assert.rejects(resolveProductionPending({operation:async()=>{throw new LocalAPIError(status,'bad');},write:async()=>{writes++;}},'t',s,'k',true));assert.equal(writes,0);assert.ok(readProductionPending(s,'k'));}
 const s=storage();persistProductionPending(s,'k',{kind:'recipe',input});const client={operation:async()=>result,write:async()=>assert.fail('no POST')};assert.equal((await resolveProductionPending(client,'t',s,'k',true)).state,'confirmed');
});
test('first definite refusal allows correction only after absence; later refusal keeps uncertainty',async()=>{
 for(const priorUncertain of [false,true]){
  const s=storage();persistProductionPending(s,'k',{kind:'recipe',input});if(priorUncertain)s.setItem('k',JSON.stringify({...readProductionPending(s,'k'),uncertain:true}));
  const client={operation:async()=>missing(),write:async()=>{throw new LocalAPIError(409,'conflict');}};
  await assert.rejects(resolveProductionPending(client,'t',s,'k',true));assert.equal(readProductionPending(s,'k').uncertain,priorUncertain);
  if(priorUncertain)await assert.rejects(resolveProductionPending(client,'t',s,'k',false,true));else assert.equal((await resolveProductionPending(client,'t',s,'k',false,true)).state,'discarded');
 }
});
test('POST success with failed confirmation remains pending; persistence fails before POST',async()=>{
 const s=storage();persistProductionPending(s,'k',{kind:'recipe',input});let queries=0;
 await assert.rejects(resolveProductionPending({operation:async()=>{queries++;if(queries===1)return missing();throw new LocalAPIError(0,'offline');},write:async()=>result},'t',s,'k',true));assert.equal(readProductionPending(s,'k').uncertain,true);
 assert.throws(()=>persistProductionPending(s,'k',{kind:'recipe',input}));
 const broken={getItem:()=>null,setItem:()=>{throw new Error('quota');}};assert.throws(()=>persistProductionPending(broken,'k',{kind:'recipe',input}));
});
test('context includes company store device and operator; corrupt pending is blocked',()=>{
 const scope={tenant_id:'t',store_id:'s',device_id:'d',identity_id:'i'};const keys=new Set([productionPendingKey(scope),...Object.keys(scope).map(k=>productionPendingKey({...scope,[k]:'other'}))]);assert.equal(keys.size,5);
 const s=storage();s.setItem('k','broken');assert.throws(()=>readProductionPending(s,'k'));
});
test('actual POST validates receipt identifiers, uses no cookies, no extra fields',async()=>{
 let calls=0;const c=createProductionMutations(async(url,opt)=>{calls++;assert.equal(url,'/local/v1/production/recipe-versions');assert.equal(opt.credentials,'omit');assert.deepEqual(JSON.parse(opt.body),input);return new Response(JSON.stringify({...result,version_id:'foreign'}));});
 await assert.rejects(c.write('t','recipe',input));await assert.rejects(c.write('t','recipe',{...input,tenant_id:'other'}));assert.equal(calls,1);
});
test('planning preview rejects product and ingredient overflow without rounding',()=>{
 const r={...input,revision:1};const p=plannedPreview(r,3);assert.equal(p.output_milli,30000);assert.equal(p.ingredients[0].planned_milli,300003);
 assert.throws(()=>plannedPreview(r,Number.MAX_SAFE_INTEGER));
});
