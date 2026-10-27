import test from 'node:test';
import assert from 'node:assert/strict';
import {createDraftRegistry,leavesDrafts} from '../src/core/productionRead/formGuard.mjs';
const here={pathname:'/local/recipes',search:'',hash:''};
test('overlapping forms retain navigation protection until every dirty form is cleared',()=>{const events=[],r=createDraftRegistry(v=>events.push(v));r.set('recipe',true);r.set('lot',true);r.set('recipe',false);assert.equal(r.has(),true);assert.deepEqual(events,[true]);r.set('lot',false);assert.equal(r.has(),false);assert.deepEqual(events,[true,false]);});
test('repeated registration, clean forms and unmount cleanup are idempotent',()=>{const events=[],r=createDraftRegistry(v=>events.push(v));r.set('missing',false);r.set('quality',true);r.set('quality',true);r.set('quality',false);r.set('quality',false);assert.deepEqual(events,[true,false]);});
for(const [name,next] of [['route',{...here,pathname:'/local/home'}],['query',{...here,search:'?area=delivery'}],['anchor',{...here,hash:'#other'}]])test(`dirty forms block ${name} changes without performing any operation`,()=>{assert.equal(leavesDrafts(true,here,next),true);assert.equal(leavesDrafts(false,here,next),false);});
test('identical location never blocks refresh or same-link navigation',()=>assert.equal(leavesDrafts(true,here,{...here}),false));
test('registry is isolated by provider/context and never owns operation storage',()=>{const a=createDraftRegistry(()=>{}),b=createDraftRegistry(()=>{});a.set('same-react-id',true);assert.equal(a.has(),true);assert.equal(b.has(),false);b.set('same-react-id',false);assert.equal(a.has(),true);assert.equal(b.hasSelection(),false);});

test('combination survives recipe selection but still blocks leaving the route',()=>{const r=createDraftRegistry(()=>{});r.set('combination',true,false);assert.equal(r.has(),true);assert.equal(r.hasSelection(),false);r.set('scenario',true);assert.equal(r.hasSelection(),true);r.set('scenario',false);assert.equal(r.hasSelection(),false);assert.equal(r.has(),true);r.set('combination',false);assert.equal(r.has(),false);});
