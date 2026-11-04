import test from 'node:test';
import assert from 'node:assert/strict';
import {readFile} from 'node:fs/promises';
import {createNFe55Client} from '../src/core/fiscalCustomer/nfe55.mjs';

test('CC-e query uses only the original sale ID, never the correction text or another POST to issue',async()=>{
 const calls=[];
 const api=createNFe55Client({token:'synthetic-session',isCurrent:()=>true,fetcher:async(url,options)=>{calls.push({url,options});return new Response(JSON.stringify({state:'correction_reconciliation_required'}));}});
 await api.queryCorrection('sale-one');
 assert.equal(calls.length,1);assert.equal(calls[0].url,'/local/v1/fiscal/nfe55/sales/sale-one/correction/query');
 assert.deepEqual(JSON.parse(calls[0].options.body),{});assert.equal(calls[0].options.cache,'no-store');
});

test('uncertain CC-e query is never retried or converted to a new correction',async()=>{
 let calls=0;
 const api=createNFe55Client({token:'synthetic',isCurrent:()=>true,fetcher:async()=>{calls++;throw Error('private sentinel');}});
 await assert.rejects(api.queryCorrection('sale-one'),e=>e.code==='network'&&!e.message.includes('private sentinel'));
 assert.equal(calls,1);
});

test('CC-e query rejects logged-out and changed contexts without exposing an old response',async()=>{
 let current=false,calls=0;
 const api=createNFe55Client({token:'synthetic',isCurrent:()=>current,fetcher:async()=>{calls++;current=false;return new Response(JSON.stringify({state:'correction_authorized_homologation'}));}});
 await assert.rejects(api.queryCorrection('sale-one'),e=>e.code==='session');assert.equal(calls,0);
 current=true;await assert.rejects(api.queryCorrection('sale-one'),e=>e.code==='session');assert.equal(calls,1);
});

test('CC-e recovery never touches browser persistence or telemetry',async()=>{
 const descriptors=new Map(['localStorage','sessionStorage','indexedDB','navigator'].map(key=>[key,Object.getOwnPropertyDescriptor(globalThis,key)]));
 const touched=[];
 try{
  for(const key of ['localStorage','sessionStorage','indexedDB'])Object.defineProperty(globalThis,key,{configurable:true,get(){touched.push(key);throw Error('persistence forbidden');}});
  Object.defineProperty(globalThis,'navigator',{configurable:true,value:{sendBeacon(){touched.push('telemetry');}}});
  const api=createNFe55Client({token:'synthetic',isCurrent:()=>true,fetcher:async()=>new Response(JSON.stringify({state:'correction_reconciliation_required'}))});
  await api.queryCorrection('sale-one');assert.deepEqual(touched,[]);
 }finally{for(const [key,value]of descriptors){if(value)Object.defineProperty(globalThis,key,value);else delete globalThis[key];}}
});

test('CC-e screen has an explicit query gate and five translated states without persistent request storage',async()=>{
 const source=await readFile(new URL('../src/modules/tenant/pages/enterprise89/FiscalCorrection.tsx',import.meta.url),'utf8');
 assert.doesNotMatch(source,/localStorage|sessionStorage|indexedDB|sendBeacon|console\./);
 assert.match(source,/correction_not_requested/);assert.match(source,/!canSend/);assert.match(source,/inFlight\.current/);assert.match(source,/setText\(''\)/);
 const keys=[...source.matchAll(/\bfw\.([A-Za-z0-9_]+)/g)].map(m=>m[1]);
 for(const lang of ['pt-BR','en','es','fr','zh-CN']){
  const words=JSON.parse(await readFile(new URL(`../src/i18n/fiscalCustomer/${lang}.json`,import.meta.url)));
  for(const key of [...keys,'correction_not_requested','correction_authorized_homologation','correction_failed','correction_reconciliation_required'])assert.ok(words[key]?.trim(),`${lang}:${key}`);
 }
});
