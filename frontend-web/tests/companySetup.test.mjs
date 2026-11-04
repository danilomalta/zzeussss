import test from 'node:test';
import assert from 'node:assert/strict';
import {readFile} from 'node:fs/promises';
import {companySetupStatus,setupLocale} from '../src/core/companyRegistration/journey.mjs';
import {createRegistrationClient} from '../src/core/companyRegistration/client.mjs';
const input={operation_id:'minimum',expected_revision:0,stage:'pre_registration',company_name:'Synthetic',responsible:'Owner',cnpj:'',legal_name:'',uf:'',state_registration:'',municipality_ibge:'',crt:'',provider:'',environment:''};
test('only persisted API revisions advance registration; reopening preserves progress',async()=>{
 let saved={input:{...input,operation_id:''},revision:0,created_at:'',repeated:false};
 const fetcher=async(_,options)=>{if(options.method==='POST'){const next=JSON.parse(options.body);saved={input:next,revision:next.expected_revision+1,created_at:'2026-11-02T00:00:00Z',repeated:false};}return new Response(JSON.stringify(saved));};
 const make=()=>createRegistrationClient({token:'fixture-only',isCurrent:()=>true,fetcher});
 assert.deepEqual(companySetupStatus(await make().read()),{preSaved:false,fullSaved:false});
 const pre=await make().save(input);assert.deepEqual(companySetupStatus(pre),{preSaved:true,fullSaved:false});
 const unsavedDraft={...input,stage:'company_profile'};assert.equal(unsavedDraft.stage,'company_profile');assert.equal(companySetupStatus(pre).fullSaved,false);
 await make().save({...input,operation_id:'complete',expected_revision:1,stage:'company_profile',cnpj:'11222333000181',legal_name:'Synthetic',uf:'SP',state_registration:'12345',municipality_ibge:'3550308',crt:'1',provider:'external',environment:'homologation'});
 assert.deepEqual(companySetupStatus(await make().read()),{preSaved:true,fullSaved:true});
});
test('missing, malformed or unsaved records never imply completion or official authorization',()=>{
 for(const record of [null,{}, {revision:0,input:{stage:'company_profile'}},{revision:-1,input:{stage:'company_profile'}},{revision:1,input:{stage:'unknown'}}])assert.deepEqual(companySetupStatus(record),{preSaved:false,fullSaved:false});
 assert.deepEqual(Object.keys(companySetupStatus(null)),['preSaved','fullSaved']);
});
test('guide locale is limited to the five existing CLI languages',()=>{
 for(const [locale,expected]of [['pt','pt-BR'],['en-US','en'],['es-ES','es'],['fr-FR','fr'],['zh-Hans','zh-CN'],['x; echo secret','pt-BR']])assert.equal(setupLocale(locale),expected);
});
test('guide uses saved data, correct staff route and private existing CLI; never reads secrets or transmits',async()=>{
 const component=await readFile(new URL('../src/modules/tenant/pages/CompanySetupSteps.tsx',import.meta.url),'utf8');
 const page=await readFile(new URL('../src/modules/tenant/pages/LocalCompanyRegistration.tsx',import.meta.url),'utf8');
 assert.match(page,/CompanySetupSteps record=\{saved\}/);assert.match(page,/to="\/local\/staff"/);assert.doesNotMatch(page,/area=access/);
 assert.doesNotMatch(component,/localStorage|sessionStorage|fetch\(|console\.|innerHTML|window\.titan|type="password"/);
 for(const command of ['vault-init','vault-export-key','vault-rotate'])assert.ok(component.includes(command));
 assert.match(component,/to="\/local\/staff"/);assert.match(component,/to="\/local\/enterprise\?area=fiscal"/);
 for(const locale of ['pt-BR','en','es','fr','zh-CN']){
  const words=JSON.parse(await readFile(new URL('../src/i18n/companyRegistration/'+locale+'.json',import.meta.url),'utf8'));
  for(const m of component.matchAll(/\bcw\.([A-Za-z0-9_]+)/g))assert.ok(words[m[1]]?.trim(),locale+':'+m[1]);
 }
});
