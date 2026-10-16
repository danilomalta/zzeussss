import test from 'node:test';
import assert from 'node:assert/strict';
import {build} from 'esbuild';
import {can86} from '../src/core/sectors86/rules.mjs';
const compiled=await build({entryPoints:[new URL('../src/core/sectors86/profile87.ts',import.meta.url).pathname],bundle:true,write:false,format:'esm',platform:'node'});
const p=await import('data:text/javascript;base64,'+Buffer.from(compiled.outputFiles[0].contents).toString('base64'));
test('company and store profile is shared by authorized local identities, never by another company or store',()=>{
 const a=JSON.stringify(['company','shop','owner','pc']),b=JSON.stringify(['company','shop','staff','tablet']);
 assert.equal(p.profileKey87(a),p.profileKey87(b));
 assert.notEqual(p.profileKey87(a),p.profileKey87(JSON.stringify(['another','shop','owner','pc'])));
 assert.notEqual(p.profileKey87(a),p.profileKey87(JSON.stringify(['company','other-shop','owner','pc'])));
 assert.throws(()=>p.profileKey87('["company","shop"]'));
});
test('pharmacy and market do not automatically enable restaurant features; mixed market can enable bakery',()=>{
 for(const activity of ['market','pharmacy']){const v=p.recommended87(activity);assert.equal(v.kitchen,false);assert.equal(v.tables,false);assert.equal(v.production,false);assert.equal(p.validProfile87(v),true);}
 assert.equal(p.validProfile87({...p.recommended87('market'),production:true,sectors:['bakery','checkout']}),true);
 assert.equal(p.recommended87('industry').menu,false);assert.equal(p.recommended87('industry').production,true);
});
test('profile rejects arbitrary activities, duplicates, wrong flags and kitchen without menu',()=>{
 const v=p.recommended87('pharmacy');for(const patch of [{activity:'unknown'},{sectors:['warehouse','warehouse']},{sectors:['anything']},{tables:'true'},{kitchen:true,menu:false},{configured:false}])assert.equal(p.validProfile87({...v,...patch}),false);
});
test('configuration is available to licensed company managers without granting production or staff access',()=>{
 const cap={role:'owner',license:{state:'active',modules:['staff']},permissions:[]};
 assert.equal(can86(cap,'sectors'),true);assert.equal(can86(cap,'recipes'),false);assert.equal(can86(cap,'food'),false);
 assert.equal(can86({...cap,role:'employee'},'sectors'),false);assert.equal(can86({...cap,license:{...cap.license,state:'expired'}},'sectors'),false);
});
