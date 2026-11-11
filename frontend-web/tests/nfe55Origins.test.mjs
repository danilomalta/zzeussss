import test from 'node:test';
import assert from 'node:assert/strict';
import {readFile} from 'node:fs/promises';
import {createNFe55Client} from '../src/core/fiscalCustomer/nfe55.mjs';

const origin={operation_id:'op-origin',kind:'inbound_receipt',source_id:'receipt-1',referenced_access_key:'35260112345678000195550010000000011234567890',freight_mode:'9',confirmed:true,recipient:{cnpj:'12345678000195',name:'Fornecedor',uf:'SP'},items:[{product_id:'p',quantity_milli:1000,unit_price_cents:100,total_cents:100,tax:{quote_operation_id:'quote-1'}}]};

test('origem usa rotas frozen e não converte venda',async()=>{
 const calls=[];
 const api=createNFe55Client({token:'synthetic-session',isCurrent:()=>true,fetcher:async(url,options)=>{
  calls.push({url,body:JSON.parse(options.body)});
  if(url.endsWith('/origins'))return new Response(JSON.stringify({origin_id:'orig-1',operation_id:'op-origin',kind:'inbound_receipt',repeated:false}),{status:201});
  return new Response(JSON.stringify({state:'authorized_homologation'}));
 }});
 const bound=await api.bindOrigin(origin);
 assert.equal(bound.origin_id,'orig-1');
 assert.equal(calls[0].url,'/local/v1/fiscal/nfe55/origins');
 assert.equal(calls[0].body.kind,'inbound_receipt');
 assert.equal(calls[0].body.confirmed,true);
 await api.emitOrigin('orig-1',{nature:'ENTRADA',purpose:'1',freight_mode:'9',confirmed:true,recipient:{cnpj:'12345678000195',name:'Fornecedor',address:{uf:'SP'}}});
 assert.equal(calls[1].url,'/local/v1/fiscal/nfe55/origins/orig-1/emit');
 assert.ok(!calls.some(c=>c.url.includes('/sales/')));
});

test('preview 55 permanece na venda registrada e tabela fiscal não é inventada',async()=>{
 const calls=[];
 const api=createNFe55Client({token:'synthetic',isCurrent:()=>true,fetcher:async(url,options)=>{
  calls.push(url);
  if(String(url).includes('/preview'))return new Response(JSON.stringify({sale_id:'sale-one',total_cents:100,items:[],mapping_supported:true}));
  return new Response(JSON.stringify({state:'not_requested'}));
 }});
 await api.preview('sale-one');
 assert.equal(calls[0],'/local/v1/fiscal/nfe55/sales/sale-one/preview');
 const tax=await readFile(new URL('../src/core/fiscalCustomer/taxTable.mjs',import.meta.url),'utf8');
 assert.match(tax,/\/local\/v1\/fiscal\/'\+path/);
 assert.match(tax,/tax-tables\/import/);
 const screen=await readFile(new URL('../src/modules/tenant/pages/enterprise89/FiscalNFe55.tsx',import.meta.url),'utf8');
 assert.match(screen,/FiscalNFeOrigin/);
 assert.doesNotMatch(screen,/GatewayContext95/);
 const originScreen=await readFile(new URL('../src/modules/tenant/pages/enterprise89/FiscalNFeOrigin.tsx',import.meta.url),'utf8');
 assert.doesNotMatch(originScreen,/localStorage|sessionStorage|console\.|GatewayContext95/);
});

test('origem recusa payload incompleto sem POST e sessão morta descarta resposta',async()=>{
 let calls=0,current=false;
 const api=createNFe55Client({token:'synthetic',isCurrent:()=>current,fetcher:async()=>{calls++;return new Response('{}');}});
 assert.throws(()=>api.bindOrigin({confirmed:true,kind:'sale'}),e=>e.code==='input_missing');
 assert.equal(calls,0);
 current=true;
 const live=createNFe55Client({token:'synthetic',isCurrent:()=>current,fetcher:async()=>{calls++;current=false;return new Response(JSON.stringify({origin_id:'x',operation_id:'o',kind:'inbound_receipt',repeated:false}));}});
 await assert.rejects(live.bindOrigin(origin),e=>e.code==='session');
 assert.equal(calls,1);
});

test('rótulos de origem existem nos cinco idiomas',async()=>{
 const source=await readFile(new URL('../src/modules/tenant/pages/enterprise89/FiscalNFeOrigin.tsx',import.meta.url),'utf8');
 const keys=[...source.matchAll(/\bfw\.([A-Za-z0-9_]+)/g)].map(m=>m[1]);
 for(const lang of ['pt-BR','en','es','fr','zh-CN']){
  const words=JSON.parse(await readFile(new URL(`../src/i18n/fiscalCustomer/${lang}.json`,import.meta.url)));
  for(const key of keys)assert.ok(typeof words[key]==='string'&&words[key].trim(),`${lang}:${key}`);
 }
});
