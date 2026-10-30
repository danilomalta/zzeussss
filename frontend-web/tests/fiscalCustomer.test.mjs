import test from 'node:test';
import assert from 'node:assert/strict';
import {readFile} from 'node:fs/promises';
import {createFiscalCustomer,FiscalCustomerError,profileMatches,states} from '../src/core/fiscalCustomer/client.mjs';

const input={operation_id:'op-fixture',profile_id:'profile-fixture',expected_revision:0,cnpj:'11222333000181',legal_name:'Disposable fixture',state:'SP',municipality_ibge:'3550308',state_registration:'12345',crt:'3',model:'55',environment:'homologation',series:1};
const profile=()=>({input:{...input},revision:1,created_at:'2026-10-28T00:00:00Z',readiness:{blockers:['tax_rules_not_validated']}});
const reply=value=>new Response(JSON.stringify(value));
const deferred=()=>{let resolve;const promise=new Promise(r=>{resolve=r;});return {promise,resolve};};
function client(fetcher,isCurrent=()=>true){return createFiscalCustomer({token:'fixture-only',isCurrent,fetcher});}

test('profile write uses only existing authenticated same-origin API and exact revision',async()=>{
 let count=0;
 const value=await client(async(path,options)=>{count++;assert.equal(path,'/local/v1/fiscal/profiles');assert.equal(options.method,'POST');assert.equal(options.credentials,'omit');assert.equal(options.cache,'no-store');assert.equal(options.headers.Authorization,'Bearer fixture-only');assert.deepEqual(JSON.parse(options.body),input);return reply(profile());}).save(input);
 assert.equal(count,1);assert.equal(value.revision,1);assert.equal(value.readiness.blockers[0],'tax_rules_not_validated');
});
for(const change of ['logout','company','store','user','device'])test(`old ${change} context cannot start a POST`,async()=>{
 let count=0;await assert.rejects(client(async()=>{count++;},()=>false).save(input),error=>error.code==='session');assert.equal(count,0);
});
test('session revoked during response body cannot publish old result',async()=>{
 let active=true;const body=deferred(),api=client(async()=>({ok:true,text:()=>body.promise}),()=>active),pending=api.read(input.profile_id);
 await new Promise(resolve=>setImmediate(resolve));active=false;body.resolve(JSON.stringify(profile()));
 await assert.rejects(pending,error=>error.code==='session');
});
test('session revoked during request rejects old response',async()=>{
 let active=true;const network=deferred(),pending=client(()=>network.promise,()=>active).save(input);
 active=false;network.resolve(reply(profile()));await assert.rejects(pending,error=>error.code==='session');
});
test('uncertain writes never retry or change operation identifiers',async()=>{
 const snapshot=structuredClone(input);let count=0;
 await assert.rejects(client(async()=>{count++;throw Error('lost response');}).save(input),error=>error.code==='network');
 assert.equal(count,1);assert.deepEqual(input,snapshot);
});
test('recovery reads exact original revision and never writes',async()=>{
 const api=client(async(path,options)=>{assert.equal(path,'/local/v1/fiscal/profiles/profile-fixture?revision=1');assert.equal(options.method,'GET');assert.equal(options.body,undefined);return reply(profile());});
 assert.equal(profileMatches(await api.read(input.profile_id,1),input),true);
});
for(const field of Object.keys(input))test(`recovery detects changed ${field}`,()=>{
 const other=profile();other.input[field]=typeof input[field]==='number'?input[field]+1:input[field]+'x';assert.equal(profileMatches(other,input),false);
});
for(const status of [400,401,403,404,409,503])test(`HTTP ${status} stays an error, never a fake profile`,async()=>{
 let count=0;await assert.rejects(client(async()=>{count++;return new Response('{}',{status});}).save(input),error=>error instanceof FiscalCustomerError&&error.status===status);assert.equal(count,1);
});
for(const value of [null,[],{},'<html>not JSON</html>'])test(`reject malformed response ${JSON.stringify(value)}`,async()=>{
 await assert.rejects(client(async()=>typeof value==='string'?new Response(value):reply(value)).read(input.profile_id),error=>error.code==='response');
});
test('reject oversized response',async()=>{await assert.rejects(client(async()=>new Response(' '.repeat(262145))).read(input.profile_id),error=>error.code==='response');});
test('reject wrong profile/revision from lookup and write',async()=>{
 const wrong=profile();wrong.input.profile_id='other';const api=client(async()=>reply(wrong));await assert.rejects(api.read(input.profile_id),error=>error.code==='response');await assert.rejects(api.save(input),error=>error.code==='response');
 const revision=profile();revision.revision=2;await assert.rejects(client(async()=>reply(revision)).read(input.profile_id,1),error=>error.code==='response');
});
test('reject secret/tenant injection and path traversal before transport',async()=>{
 let count=0;const api=client(async()=>{count++;});
 for(const extra of ['certificate','password','tenant_id','store_id','role','endpoint'])await assert.rejects(api.save({...input,[extra]:'forbidden'}),error=>error.code==='invalid');
 for(const id of ['../profile','x?revision=1','/profile',''])await assert.rejects(api.read(id),error=>error.code==='invalid');
 for(const revision of [0,-1,1.5,Infinity])await assert.rejects(api.read('profile',revision),error=>error.code==='invalid');
 assert.equal(count,0);
});
test('all 27 states use their real requirements endpoint with dated references',async()=>{
 assert.equal(new Set(states).size,27);
 for(const uf of states){const out=await client(async(path,options)=>{assert.equal(path,'/local/v1/fiscal/requirements/'+uf);assert.equal(options.method,'GET');return reply({uf,reference_date:'2026-10-26',issuer_requirements:'fixture',developer_requirements:'fixture',limitations:'not current tax rates',sources:[]});}).requirements(uf);assert.equal(out.uf,uf);}
 await assert.rejects(client(async()=>{}).requirements('XX'),error=>error.code==='invalid');
});
test('capabilities distinguish unavailable from installed transport',async()=>{
 for(const transmission_available of [false,true]){const value={transmission_available,nfse_available:false,sped_generation_available:false,production_cancellation_available:false};assert.deepEqual(await client(async()=>reply(value)).capabilities(),value);}
 await assert.rejects(client(async()=>reply({transmission_available:'yes'})).capabilities(),error=>error.code==='response');
});
test('malformed state references cannot crash source-link rendering',async()=>{
 for(const sources of [[null],[{}],[{title:3,url:'https://example.gov.br'}]]){
  await assert.rejects(client(async()=>reply({uf:'SP',reference_date:'2026-10-26',issuer_requirements:'fixture',developer_requirements:'fixture',limitations:'fixture',sources})).requirements('SP'),error=>error.code==='response');
 }
});
test('five languages have complete nonempty matching keys, including backend blockers',async()=>{
 const languages=['pt-BR','en','es','fr','zh-CN'];let keys;
 for(const lang of languages){const words=JSON.parse(await readFile(new URL('../src/i18n/fiscalCustomer/'+lang+'.json',import.meta.url),'utf8'));keys??=Object.keys(words).sort();assert.deepEqual(Object.keys(words).sort(),keys);assert.ok(Object.values(words).every(v=>typeof v==='string'&&v.trim()));for(const key of ['uncertain','draft','private','discard','tax_rules_not_validated'])assert.ok(words[key]);}
});
test('fiscal screen literal labels belong to its own complete translation namespace',async()=>{
 const words=JSON.parse(await readFile(new URL('../src/i18n/fiscalCustomer/pt-BR.json',import.meta.url),'utf8'));
 for(const file of ['FiscalCustomer.tsx','FinanceFiscal.tsx']){
  const source=await readFile(new URL('../src/modules/tenant/pages/enterprise89/'+file,import.meta.url),'utf8');
  for(const match of source.matchAll(/\bfw\.([A-Za-z0-9_]+)/g))assert.ok(words[match[1]],file+':'+match[1]);
 }
});
