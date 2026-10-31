import test from 'node:test';import assert from 'node:assert/strict';import {readFile} from 'node:fs/promises';
import {createNFe55Client} from '../src/core/fiscalCustomer/nfe55.mjs';
test('NF-e uses fixed same-origin route and never sends price/tax overrides',async()=>{let request;const api=createNFe55Client({token:'synthetic-session',isCurrent:()=>true,fetcher:async(url,options)=>{request={url,options};return new Response(JSON.stringify({state:'recovery_required'}),{status:200});}});const input={nature:'VENDA',purpose:'1',freight_mode:'9',recipient:{name:'Private fixture'}};await api.emit('sale-one',input);assert.equal(request.url,'/local/v1/fiscal/nfe55/sales/sale-one/emit');assert.equal(request.options.cache,'no-store');assert.equal(request.options.credentials,'omit');assert.deepEqual(JSON.parse(request.options.body),input);assert.ok(!request.url.includes('Private'));});
test('logout prevents POST and context change discards an old response',async()=>{let current=false,calls=0;const api=createNFe55Client({token:'synthetic-session',isCurrent:()=>current,fetcher:async()=>{calls++;current=false;return new Response(JSON.stringify({state:'authorized_homologation'}));}});await assert.rejects(api.query('sale-one'),e=>e.code==='session');assert.equal(calls,0);current=true;await assert.rejects(api.emit('sale-one',{}),e=>e.code==='session');assert.equal(calls,1);});
test('uncertain request is not automatically resent',async()=>{let calls=0;const api=createNFe55Client({token:'synthetic',isCurrent:()=>true,fetcher:async()=>{calls++;throw Error('synthetic timeout');}});await assert.rejects(api.emit('sale-one',{}),e=>e.code==='network');assert.equal(calls,1);});
test('five languages visibly identify homologation and all fiscal states',async()=>{for(const lang of ['pt-BR','en','es','fr','zh-CN']){const words=JSON.parse(await readFile(new URL(`../src/i18n/fiscalCustomer/${lang}.json`,import.meta.url)));for(const key of ['nfeBanner','nfeAuthorized','nfeRejected','nfeCancelled','nfeCancelError','nfePending','nfePrivacy','nfeError_tax_mapping_unsupported'])assert.ok(words[key]);}const pt=JSON.parse(await readFile(new URL('../src/i18n/fiscalCustomer/pt-BR.json',import.meta.url)));assert.equal(pt.nfeBanner,'HOMOLOGAÇÃO - SEM VALOR FISCAL');});

test('recipient remains only in the explicit API body: no browser storage, logs or telemetry',async()=>{
 const personal='Private recipient and address sentinel';
 const descriptors=new Map(['localStorage','sessionStorage','navigator'].map(k=>[k,Object.getOwnPropertyDescriptor(globalThis,k)]));
 const priorConsole=new Map(['log','warn','error','debug','info'].map(k=>[k,console[k]]));
 const sideEffects=[];
 try {
  for(const key of ['localStorage','sessionStorage'])Object.defineProperty(globalThis,key,{configurable:true,get(){sideEffects.push(key);throw Error('unexpected storage');}});
  Object.defineProperty(globalThis,'navigator',{configurable:true,value:{sendBeacon(){sideEffects.push('telemetry');return true;}}});
  for(const key of priorConsole.keys())console[key]=()=>sideEffects.push('console');
  const calls=[];
  const api=createNFe55Client({token:'disposable-session',isCurrent:()=>true,fetcher:async(url,options)=>{
   calls.push({url,body:JSON.parse(options.body)});
   return new Response(JSON.stringify({state:'recovery_required'}));
  }});
  await api.emit('sale-private',{recipient:{name:personal,address:{street:personal}},nature:'VENDA',purpose:'1',freight_mode:'9'});
  await api.query('sale-private');
  assert.equal(calls.length,2);assert.equal(calls[0].body.recipient.name,personal);
  assert.deepEqual(calls[1].body,{});assert.ok(calls.every(c=>!c.url.includes(personal)));
  const failing=createNFe55Client({token:'disposable-session',isCurrent:()=>true,fetcher:async()=>{throw Error(personal);}});
  await assert.rejects(failing.emit('sale-private',{}),e=>e.code==='network'&&!e.message.includes(personal));
  assert.deepEqual(sideEffects,[]);
 } finally {
  for(const [key,descriptor] of descriptors){if(descriptor)Object.defineProperty(globalThis,key,descriptor);else delete globalThis[key];}
  for(const [key,value] of priorConsole)console[key]=value;
 }
 const source=await readFile(new URL('../src/modules/tenant/pages/enterprise89/FiscalNFe55.tsx',import.meta.url),'utf8');
 assert.doesNotMatch(source,/localStorage|sessionStorage|indexedDB|sendBeacon|console\.|telemetry/i);
 assert.match(source,/setInput\(empty\(\)\)/);
});

test('every literal NF-e screen label resolves in the fiscal dictionary in five languages',async()=>{
 const source=await readFile(new URL('../src/modules/tenant/pages/enterprise89/FiscalNFe55.tsx',import.meta.url),'utf8');
 const keys=[...source.matchAll(/\bfw\.([A-Za-z0-9_]+)/g)].map(match=>match[1]);
 keys.push(...['name','cnpj','cpf','ie','street','number','district','city_code','city','uf','postal_code'].map(key=>'nfe_'+key));
 const states=['nfeAuthorized','nfeRejected','nfeCancelled','nfeCancelError','nfePending','nfeNotRequested'];
 for(const lang of ['pt-BR','en','es','fr','zh-CN']){
  const words=JSON.parse(await readFile(new URL(`../src/i18n/fiscalCustomer/${lang}.json`,import.meta.url)));
  for(const key of [...keys,...states])assert.ok(typeof words[key]==='string'&&words[key].trim(),`${lang}:${key}`);
 }
});
