import test from 'node:test';
import assert from 'node:assert/strict';
import {readFile} from 'node:fs/promises';
import {createFiscalCustomer} from '../src/core/fiscalCustomer/client.mjs';
const profile=id=>({input:{operation_id:'fixture',profile_id:id,expected_revision:0,cnpj:'11222333000181',legal_name:'Synthetic',state:'SP',municipality_ibge:'3550308',state_registration:'123',crt:'1',model:'55',environment:'homologation',series:1},revision:1,readiness:{blockers:['tax_rules_not_validated']}});
const reply=data=>new Response(JSON.stringify(data));
const api=(fetcher,isCurrent=()=>true)=>createFiscalCustomer({token:'fixture-only',isCurrent,fetcher});
test('resume a saved profile using authenticated pages without a remembered ID or storage',async()=>{
 let paths=[];const service=api(async(path,options)=>{paths.push(path);assert.equal(options.method,'GET');assert.equal(options.credentials,'omit');assert.equal(options.cache,'no-store');return path.includes('after=')?reply({rows:[profile('two')],has_more:false,next_after:''}):reply({rows:[profile('one')],has_more:true,next_after:'one'});});
 const first=await service.list('',1),second=await service.list(first.next_after,1);
 assert.deepEqual([...first.rows,...second.rows].map(v=>v.input.profile_id),['one','two']);
 assert.deepEqual(paths,['/local/v1/fiscal/profiles?limit=1','/local/v1/fiscal/profiles?limit=1&after=one']);
 const reopened=api(async()=>reply({rows:[profile('one')],has_more:false,next_after:''}));assert.equal((await reopened.list()).rows[0].input.profile_id,'one');
});
test('list refuses duplicate, reversed, overflowing or falsely complete pages',async()=>{
 for(const value of [
  {rows:[profile('two'),profile('one')],has_more:false,next_after:''},
  {rows:[profile('one'),profile('one')],has_more:false,next_after:''},
  {rows:[],has_more:true,next_after:'one'},
  {rows:[profile('one')],has_more:true,next_after:'wrong'},
  {rows:[],has_more:false,next_after:'one'},
  {rows:[],has_more:false,next_after:'',token:'unexpected'},
 ])await assert.rejects(api(async()=>reply(value)).list('',2),e=>e.code==='response');
 await assert.rejects(api(async()=>reply({rows:[profile('one')],has_more:false,next_after:''})).list('one'),e=>e.code==='response');
});
test('logout or tenant/store/device switch during the response cannot publish a profile list',async()=>{
 let current=true,resolve;const body=new Promise(r=>resolve=r);
 const pending=api(async()=>({ok:true,text:()=>body}),()=>current).list();
 await new Promise(r=>setImmediate(r));current=false;resolve(JSON.stringify({rows:[profile('one')],has_more:false,next_after:''}));
 await assert.rejects(pending,e=>e.code==='session');
 let calls=0;const stale=api(async()=>{calls++;},()=>false);await assert.rejects(stale.list(),e=>e.code==='session');assert.equal(calls,0);
});
test('invalid cursors and limits are rejected without HTTP',async()=>{
 let calls=0;const service=api(async()=>{calls++;});
 for(const after of ['../profile','x?tenant=other','/x'])await assert.rejects(service.list(after),e=>e.code==='invalid');
 for(const n of [0,51,1.5,NaN])await assert.rejects(service.list('',n),e=>e.code==='invalid');assert.equal(calls,0);
});
test('profile picker preserves manual lookup and has five-language labels without browser storage',async()=>{
 const picker=await readFile(new URL('../src/modules/tenant/pages/enterprise89/FiscalProfilePicker.tsx',import.meta.url),'utf8');
 assert.doesNotMatch(picker,/localStorage|sessionStorage|console\.|innerHTML/);
 for(const lang of ['pt-BR','en','es','fr','zh-CN']){
  const w=JSON.parse(await readFile(new URL('../src/i18n/fiscalCustomer/'+lang+'.json',import.meta.url),'utf8'));
  for(const k of ['profileList','profileListHint','profileListEmpty','profileListNext'])assert.ok(w[k]?.trim());
 }
});
