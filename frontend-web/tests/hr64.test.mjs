import test from 'node:test';
import assert from 'node:assert/strict';
import {readFileSync} from 'node:fs';
import {hrSections,hrEditableSections,validateHRDraft,createHRDraft} from '../src/core/local/hrDrafts.mjs';
import {allowedAreas,areaForRoute} from '../src/core/local/accessModel.mjs';
const valid={section:'recruitment',subject:'Vaga',person:'Maria',email:'maria@example.com',date:'2026-10-13',endDate:'2026-10-14',notes:'Revisar',stage:'planned',amount:'',startTime:'',endTime:''};
test('RH exige módulo e permissão de gestão sem liberar folha a funcionário comum',()=>{
 assert.equal(areaForRoute('/local/hr'),'hr');
 const base={role:'owner',permissions:['manage_staff'],license:{state:'active',modules:['staff']}};
 assert.ok(allowedAreas(base).includes('hr'));assert.ok(allowedAreas({...base,role:'manager'}).includes('hr'));
 assert.ok(!allowedAreas({...base,permissions:[]}).includes('hr'));
 assert.ok(!allowedAreas({...base,license:{state:'active',modules:['pos']}}).includes('hr'));
 assert.ok(!allowedAreas({...base,license:{state:'invalid',modules:['staff']}}).includes('hr'));
 const employee=allowedAreas({...base,role:'employee',permissions:[]});assert.ok(employee.includes('point'));assert.ok(!employee.includes('hr'));
});
test('rascunho contém somente campos autorizados e não transporta senha ou token',()=>{
 const draft=createHRDraft({...valid,subject:' Vaga ',password:'do-not-store',token:'do-not-store'},'draft-id');
 assert.equal(draft.subject,'Vaga');assert.equal(draft.id,'draft-id');assert.ok(!('password' in draft));assert.ok(!('token' in draft));assert.ok(!('status' in draft));
 assert.ok(validateHRDraft(valid));for(const section of hrEditableSections)assert.ok(validateHRDraft({...valid,section}));
 assert.throws(()=>createHRDraft({...valid,section:'security'},'x'));
});
test('datas reais e ordenadas, referências monetárias e horários são validados',()=>{
 for(const change of [{date:'2026-02-30'},{endDate:'2026-10-12'},{subject:' '},{email:'invalid'},{notes:'x'.repeat(2001)},{amount:'-1'},{amount:'1e4'},{amount:'1.234'},{startTime:'25:00'},{endTime:'12:60'},{stage:'sent'}])assert.ok(!validateHRDraft({...valid,...change}),JSON.stringify(change));
 assert.ok(validateHRDraft({...valid,date:'2028-02-29',endDate:'2028-03-01',amount:'1000,50',startTime:'23:00',endTime:'01:00'}));
});
test('todas as 13 áreas e explicações de rascunhos são traduzidas nos cinco idiomas',()=>{
 const locales=['pt-BR','en','es','fr','zh-CN'].map(lang=>JSON.parse(readFileSync(new URL(`../src/i18n/hr/${lang}.json`,import.meta.url))));
 for(const copy of locales){assert.deepEqual(Object.keys(copy),Object.keys(locales[0]));for(const section of hrSections){assert.ok(copy.sections[section]);assert.ok(copy.descriptions[section]);}assert.ok(copy.draftHelp&&copy.mode&&copy.integration&&copy.switchConfirm);}
});
