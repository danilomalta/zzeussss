import test from 'node:test';import assert from 'node:assert/strict';import {readFileSync} from 'node:fs';
import {deferredFiscalClient} from '../src/core/deferredFiscal/client.mjs';import {dictionaries} from '../src/core/deferredFiscal/words.mjs';
const session={tenant_id:'tenant',store_id:'store',identity_id:'owner',device_id:'device'};
const input={payment_code:'21',presence:'1',nature:'Explicit test nature',confirmed:true};
function fixture(fetcher,digest){let now={token:'synthetic',session:{...session}};const calls=[];const bind=()=>deferredFiscalClient({...now,getCurrent:()=>now,...(digest?{digest}:{}),fetcher:async(p,o)=>{calls.push({path:p,...o});return fetcher?fetcher(p,o):new Response(JSON.stringify({sale_id:'sale-one',state:'recovery_required'}));}});return {bind,calls,change(v){now={...now,...v};}};}
test('deferred fiscal uses explicit method, stable operation and no fake cash',async()=>{
 const f=fixture();await f.bind().emit('sale-one',input);await f.bind().emit('sale-one',input);
 const one=JSON.parse(f.calls[0].body),two=JSON.parse(f.calls[1].body);assert.equal(one.operation_id,two.operation_id);assert.equal(one.payment_code,'21');assert.equal(one.confirmed,true);assert.equal(one.received_cents,undefined);
 for(const value of ['','01','90','99'])await assert.rejects(f.bind().emit('sale-one',{...input,payment_code:value}));assert.equal(f.calls.length,2);
});
test('recovery after recreating client sends only sale ID and never an emission',async()=>{
 const f=fixture();await f.bind().query('sale-one');assert.ok(f.calls[0].path.endsWith('/sale-one/query'));assert.deepEqual(JSON.parse(f.calls[0].body),{});
});
test('session change during operation hashing never sends old request',async()=>{
 let done;const f=fixture(null,()=>new Promise(r=>done=r));const call=f.bind().emit('sale-one',input);f.change({token:'new-session'});done(new Uint8Array([1]));await assert.rejects(call,/context/);assert.equal(f.calls.length,0);
});
test('foreign reply and late session replies are refused',async()=>{
 const f=fixture(()=>new Response(JSON.stringify({sale_id:'foreign-sale',state:'authorized_homologation'})));await assert.rejects(f.bind().query('sale-one'),/invalid/);
 let done;const g=fixture(()=>new Promise(r=>done=r));const call=g.bind().query('sale-one');g.change({session:{...session,store_id:'another-store'}});done(new Response(JSON.stringify({sale_id:'sale-one',state:'pending'})));await assert.rejects(call,/context/);
});
test('five languages, explicit homologation, no browser persistence or silent calls',()=>{
 const keys=Object.keys(dictionaries['pt-BR']).sort();for(const lang of ['pt-BR','en','es','fr','zh-CN']){assert.deepEqual(Object.keys(dictionaries[lang]).sort(),keys);assert.ok(Object.values(dictionaries[lang]).every(v=>v.trim()));}
 const client=readFileSync(new URL('../src/core/deferredFiscal/client.mjs',import.meta.url),'utf8'),ui=readFileSync(new URL('../src/modules/financial/components/DeferredNFCe.tsx',import.meta.url),'utf8');
 for(const text of [client,ui])assert.doesNotMatch(text,/localStorage|sessionStorage|indexedDB|console\./);assert.match(ui,/w\.banner/);assert.match(ui,/setCode\(e\.target\.value\)/);assert.match(ui,/client\.query\(saleId\)/);
});
