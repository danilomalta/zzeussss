import test from 'node:test';
import assert from 'node:assert/strict';
import {readFile} from 'node:fs/promises';
import {createTaxTableClient,TaxTableError} from '../src/core/fiscalCustomer/taxTable.mjs';

const hashBytes=new Uint8Array(32).fill(1);
const fileHash=Array.from(hashBytes,b=>b.toString(16).padStart(2,'0')).join('');
const profile={profile_id:'issuer-one',revision:1,model:'65',crt:'1',environment:'homologation'};
const policy=(id='policy-one',operation='tax-policy-one')=>({
 operation_id:operation,expected_revision:0,profile_id:'issuer-one',profile_revision:1,active:true,
 rule:{id,product_id:'bread',cfop:'5102',ncm:'19059090',model:'65',crt:'1',valid_from:'2026-01-01',valid_until:'2026-12-31'},
 sources:['https://www.planalto.gov.br/ccivil_03/leis/lcp/lcp87.htm']
});
const table=(policies=[policy()])=>({version:1,revision:'table-one',regime:'simples_nacional',policies});
const bytes=value=>new TextEncoder().encode(JSON.stringify(value));
const digest=async()=>hashBytes;
function client(fetcher,isCurrent=()=>true){
 return createTaxTableClient({token:'fixture-only',isCurrent,profile,fetcher,digest});
}
function saved(id='policy-one',operation='tax-policy-one'){
 return {revision:1,input:{operation_id:operation,profile_id:'issuer-one',profile_revision:1,rule:{id},review:{file_sha256:fileHash,accountant_approved:false}}};
}

test('prepare keeps the original JSON and never invents accountant approval',async()=>{
 const view=await client(async()=>{throw Error('no network');}).prepare(bytes(table()));
 assert.equal(view.hash,fileHash);assert.equal(view.revision,'table-one');assert.equal(view.rows[0].id,'policy-one');assert.equal(view.rows[0].active,true);
});
test('production profile and accountant-approved files are refused before HTTP',async()=>{
 let calls=0;const api=createTaxTableClient({token:'x',isCurrent:()=>true,profile:{...profile,environment:'production'},fetcher:async()=>{calls++;},digest});
 await assert.rejects(api.prepare(bytes(table())),e=>e instanceof TaxTableError&&e.code==='profile');
 const approved=table();approved.policies[0].review={accountant_approved:true,accountant_reference:'x'};
 await assert.rejects(client(async()=>{calls++;}).prepare(bytes(approved)),e=>e.code==='approval');
 assert.equal(calls,0);
});
test('wrong profile, CRT, model or empty sources never reach the server',async()=>{
 let calls=0;const api=client(async()=>{calls++;});
 for(const mutate of [
  t=>{t.policies[0].profile_id='other';},
  t=>{t.policies[0].rule.crt='3';},
  t=>{t.policies[0].rule.model='55';},
  t=>{t.policies[0].sources=[];},
  t=>{t.version=2;},
  t=>{t.policies=[];}
 ]){
  const value=table();mutate(value);
  await assert.rejects(api.prepare(bytes(value)),e=>e.code==='file'||e.code==='profile');
 }
 assert.equal(calls,0);
});
test('import posts the original file and confirms hash, revision and unapproved review',async()=>{
 const file=table(),api=client(async(path,options)=>{
  assert.equal(path,'/local/v1/fiscal/tax-tables/import');assert.equal(options.method,'POST');
  assert.equal(options.credentials,'omit');assert.equal(options.cache,'no-store');
  assert.equal(options.headers.Authorization,'Bearer fixture-only');
  assert.equal(options.body,JSON.stringify(file));
  return new Response(JSON.stringify([saved()]),{status:201});
 });
 const view=await api.prepare(bytes(file));
 assert.deepEqual(await api.importTable(view),{state:'confirmed',count:1});
 await assert.rejects(api.importTable(view),e=>e.code==='query_required');
});
test('409 conflict can be retried; 500 leaves the attempt uncertain',async()=>{
 const file=table();
 let status=409,calls=0;
 const api=client(async()=>{calls++;return new Response('{}',{status});});
 const view=await api.prepare(bytes(file));
 await assert.rejects(api.importTable(view),e=>e.code==='conflict'&&!e.uncertain);
 status=500;await assert.rejects(api.importTable(view),e=>e.code==='refused'&&e.uncertain);
 await assert.rejects(api.importTable(view),e=>e.code==='query_required');
 assert.equal(calls,2);
});
test('reconcile uses GET of the expected revision and does not POST',async()=>{
 const file=table([policy(),policy('policy-two','tax-policy-two')]);
 const api=client(async(path,options)=>{
  assert.equal(options.method,'GET');assert.equal(options.body,undefined);
  if(path.includes('policy-two'))return new Response(JSON.stringify(saved('policy-two','tax-policy-two')));
  assert.equal(path,'/local/v1/fiscal/tax-policies/policy-one?revision=1');
  return new Response(JSON.stringify(saved()));
 });
 const view=await api.prepare(bytes(file));
 assert.deepEqual(await api.reconcile(view),{state:'confirmed',count:2});
});
test('logout during the import body cannot publish confirmation',async()=>{
 let current=true;let resolve;const body=new Promise(r=>resolve=r);
 const api=client(async()=>({ok:true,status:201,text:()=>body}),()=>current);
 const view=await api.prepare(bytes(table()));
 const pending=api.importTable(view);
 await new Promise(r=>setImmediate(r));current=false;resolve(JSON.stringify([saved()]));
 await assert.rejects(pending,e=>e.code==='session');
});
test('screen stays on homologation, never stores the file, and five languages cover tax table errors',async()=>{
 const screen=await readFile(new URL('../src/modules/tenant/pages/enterprise89/FiscalCustomer.tsx',import.meta.url),'utf8');
 assert.match(screen,/environment==='homologation'&&<FiscalTaxTable/);
 const ui=await readFile(new URL('../src/modules/tenant/pages/enterprise89/FiscalTaxTable.tsx',import.meta.url),'utf8');
 assert.doesNotMatch(ui,/localStorage|sessionStorage|innerHTML/);
 const clientSource=await readFile(new URL('../src/core/fiscalCustomer/taxTable.mjs',import.meta.url),'utf8');
 assert.match(clientSource,/accountant_approved===false/);
 let keys;
 for(const lang of ['pt-BR','en','es','fr','zh-CN']){
  const words=JSON.parse(await readFile(new URL('../src/i18n/fiscalCustomer/'+lang+'.json',import.meta.url),'utf8'));
  keys??=Object.keys(words).filter(k=>k.startsWith('taxTable')).sort();
  assert.deepEqual(Object.keys(words).filter(k=>k.startsWith('taxTable')).sort(),keys);
  for(const k of keys)assert.ok(words[k].trim());
 }
});
