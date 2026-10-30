import test from 'node:test';
import assert from 'node:assert/strict';
import {readFile} from 'node:fs/promises';
import {createRequire} from 'node:module';
import {createFiscalCustomer} from '../src/core/fiscalCustomer/client.mjs';
const input=()=>({operation_id:'original-operation',expected_revision:0,profile_revision:1,token:'SyntheticFocusToken123',csc:'SyntheticCSC123'});
const status=()=>({revision:1,operation_id:'original-operation',profile_id:'profile',profile_revision:1,environment:'homologation',token_present:true,csc_present:true,production_blocked:true});
const client=(fetcher,isCurrent=()=>true)=>createFiscalCustomer({token:'session-fixture',isCurrent,fetcher});
test('owner credential request uses same-origin authenticated API and reads only presence after a lost response',async()=>{
 const calls=[];let persisted=false;
 const api=client(async(path,options)=>{
  calls.push({path,...options});assert.equal(path,'/local/v1/fiscal/onboarding/profile/provider');assert.equal(options.cache,'no-store');assert.equal(options.headers.Authorization,'Bearer session-fixture');
  if(options.method==='POST'){persisted=true;assert.deepEqual(JSON.parse(options.body),input());throw new TypeError('synthetic lost response');}
  assert.ok(persisted);assert.equal(options.body,undefined);return Response.json(status());
 });
 await assert.rejects(api.saveProviderCredentials('profile',input()),e=>e.code==='network');
 assert.deepEqual(await api.providerStatus('profile'),status());assert.equal(calls.filter(c=>c.method==='POST').length,1);
});
test('credential responses reject secret fields and malformed or production-enabled claims',async()=>{
 for(const patch of [{token:'private'},{csc:'private'},{certificate:'private'},{production_blocked:false},{token_present:'yes'},{revision:-1},{profile_id:'foreign'},{environment:'untrusted'}]){
  await assert.rejects(client(async()=>Response.json({...status(),...patch})).providerStatus('profile'),e=>e.code==='response');
 }
 for(const patch of [{operation_id:'different'},{revision:2},{profile_revision:2}]){
  await assert.rejects(client(async()=>Response.json({...status(),...patch})).saveProviderCredentials('profile',input()),e=>e.code==='response');
 }
});
test('credential validation rejects tenant, URL and vault injection before any transport',async()=>{
 let calls=0;const api=client(async()=>{calls++;return Response.json(status());});
 for(const patch of [{tenant_id:'foreign'},{vault_path:'/tmp/other'},{focus_base_url:'https://production.example'},{certificate_type:'A3'},{token:'bad:token'},{csc:'with space'},{expected_revision:-1},{profile_revision:0}]){
  await assert.rejects(api.saveProviderCredentials('profile',{...input(),...patch}),e=>e.code==='invalid');
 }
 await assert.rejects(api.providerStatus('../foreign'),e=>e.code==='invalid');assert.equal(calls,0);
});
test('logout and context changes reject old credential requests and responses',async()=>{
 let active=false,calls=0;
 const api=client(async()=>{calls++;return Response.json(status());},()=>active);
 await assert.rejects(api.saveProviderCredentials('profile',input()),e=>e.code==='session');assert.equal(calls,0);
 active=true;const stale=client(async()=>{active=false;return Response.json(status());},()=>active);
 await assert.rejects(stale.saveProviderCredentials('profile',input()),e=>e.code==='session');
});
test('production can be saved as configuration without advertising emission',async()=>{
 const value={...status(),environment:'production'};
 assert.deepEqual(await client(async()=>Response.json(value)).saveProviderCredentials('profile',input()),value);
});
test('credential screen labels are translated in all five languages',async()=>{
 const keys=['providerCredentialsTitle','providerCredentialsIntro','providerCredentialsRead','providerToken','providerCSC','providerReplaceWarning','providerCredentialsSave'];
 for(const lang of ['pt-BR','en','es','fr','zh-CN']){
  const words=JSON.parse(await readFile(new URL(`../src/i18n/fiscalCustomer/${lang}.json`,import.meta.url)));
  for(const key of keys)assert.ok(typeof words[key]==='string'&&words[key].trim(),`${lang}: ${key}`);
 }
});

test('credential form renders concealed fields and disabled save until an authenticated read, in five languages',async()=>{
 const require=createRequire(import.meta.url),ts=require('typescript'),React=require('react'),{renderToStaticMarkup}=require('react-dom/server');
 const source=await readFile(new URL('../src/modules/tenant/pages/enterprise89/FiscalProviderCredentials.tsx',import.meta.url),'utf8');
 const js=ts.transpileModule(source,{compilerOptions:{module:ts.ModuleKind.CommonJS,jsx:ts.JsxEmit.ReactJSX,target:ts.ScriptTarget.ES2022}}).outputText;
 const module={exports:{}};const fiscal=await import('../src/core/fiscalCustomer/client.mjs');
 Function('require','exports','module',js)(name=>name.startsWith('react')?require(name):fiscal,module.exports,module);
 for(const lang of ['pt-BR','en','es','fr','zh-CN']){
  const words=JSON.parse(await readFile(new URL(`../src/i18n/fiscalCustomer/${lang}.json`,import.meta.url)));
  const html=renderToStaticMarkup(React.createElement(module.exports.FiscalProviderCredentials,{api:client(()=>{throw Error('unexpected request');}),profile:{input:{profile_id:'profile'},revision:1},isCurrent:()=>true,words}));
  assert.equal((html.match(/type="password"/g)||[]).length,2);assert.match(html,/<fieldset disabled=""/);assert.match(html,/autoComplete="off"/);
  assert.ok(html.includes(words.providerCredentialsTitle));assert.ok(html.includes(words.onboarding_production_blocked));assert.ok(!html.includes('SyntheticFocusToken123'));
 }
});
