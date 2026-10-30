import test from 'node:test';
import assert from 'node:assert/strict';
import {readFile} from 'node:fs/promises';
import {createFiscalCustomer} from '../src/core/fiscalCustomer/client.mjs';
const diagnostic=()=>({profile_revision:1,configuration_valid:false,production_blocked:true,transmission_performed:false,official_authorization:false,blockers:['onboarding_configuration_required','onboarding_official_verification_required','onboarding_production_blocked']});
const client=(fetcher,isCurrent=()=>true)=>createFiscalCustomer({token:'fixture',isCurrent,fetcher});
test('onboarding reads authenticated same-origin diagnostics without sending credentials',async()=>{
 const value=await client(async(path,options)=>{
  assert.equal(path,'/local/v1/fiscal/onboarding/profile-one');assert.equal(options.method,'GET');assert.equal(options.body,undefined);assert.equal(options.cache,'no-store');assert.equal(options.headers.Authorization,'Bearer fixture');
  return Response.json(diagnostic());
 }).onboarding('profile-one');assert.deepEqual(value,diagnostic());
});
test('onboarding rejects unsafe and malformed diagnostic responses',async()=>{
 for(const patch of [{production_blocked:false},{transmission_performed:true},{official_authorization:true},{profile_revision:0},{blockers:['raw-secret']},{token:'raw-secret'},{configuration_valid:'yes'}]) {
  await assert.rejects(client(async()=>Response.json({...diagnostic(),...patch})).onboarding('profile-one'),e=>e.code==='response');
 }
});
test('onboarding refuses invalid identifiers and expired sessions',async()=>{
 let count=0;const api=client(async()=>{count++;return Response.json(diagnostic());});
 await assert.rejects(api.onboarding('../other'),e=>e.code==='invalid');assert.equal(count,0);
 let active=true;const pending=client(async()=>{active=false;return Response.json(diagnostic());},()=>active).onboarding('profile-one');
 await assert.rejects(pending,e=>e.code==='session');
});
test('all five languages translate every onboarding label and blocker',async()=>{
 const keys=['onboardingTitle','onboardingIntro','onboardingCheck','onboardingConfiguration',...diagnostic().blockers];
 for(const lang of ['pt-BR','en','es','fr','zh-CN']) {
  const words=JSON.parse(await readFile(new URL(`../src/i18n/fiscalCustomer/${lang}.json`,import.meta.url)));
  for(const key of keys)assert.ok(typeof words[key]==='string'&&words[key].trim(),`${lang}: ${key}`);
 }
});
