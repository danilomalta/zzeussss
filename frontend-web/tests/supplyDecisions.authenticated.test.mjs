import test from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs';
import {cycleDecision,cyclePage,decisionCommand,decisionResult} from '../src/core/supplyDecisions/model.mjs';
import {schemas as decisions} from '../src/core/supplyDecisions/schemas.mjs';
import {schemas as deliveries} from '../src/core/deliveryCycle/schemas.mjs';
import {decisionWords} from '../src/core/supplyDecisions/words.mjs';

const at='2026-11-14T12:00:00Z';
const decision=()=>({operation_id:'OP',expected_revision:0,decision:'confirmed',accepted_milli:3000,promised_at:at,contact:'supplier-actor',evidence_reference:'sha256:'+'a'.repeat(64),observed_at:at,reason:'Explicit authenticated partial decision',order_id:'ORDER',revision:1,evidence_mode:'authenticated_company',external_verified:true,recorded_by:'BUYER',recorded_at:at});
const order={id:'ORDER',supplier_id:'SUPPLIER',product_id:'PRODUCT',unit:'kg',planned_milli:5000};
const page=()=>({order_id:'ORDER',decision:decision(),planned_milli:5000,effective_accepted_milli:0,remaining_order_milli:5000,allocated_milli:0,deliveries:[],total:0,limit:50,offset:0,has_more:false});
test('authenticated decision is readable by receiving without changing quantities or identity',()=>{
 const p=page();assert.equal(cycleDecision(p.decision,'ORDER'),p.decision);
 assert.equal(cyclePage(p,order),p);assert.equal(p.decision.accepted_milli,3000);
 assert.equal(p.decision.evidence_mode,'authenticated_company');
});
test('authenticated/manual evidence cannot be relabelled, malformed or revised locally',()=>{
 for(const patch of [{external_verified:false},{evidence_mode:'manual_external'},{evidence_reference:'manual call'},{evidence_reference:'sha256:'+'z'.repeat(64)},{expected_revision:1,revision:2},{order_id:'OTHER'}]){
  assert.throws(()=>cycleDecision({...decision(),...patch},'ORDER'),/supplyCycleInvalid/);
 }
 const manual={...decision(),evidence_mode:'manual_external',external_verified:false,evidence_reference:'Explicit phone observation'};
 assert.equal(cycleDecision(manual,'ORDER'),manual);
});
test('manual mutation cannot overwrite authenticated decision or accept an authenticated receipt',()=>{
 const {order_id,revision,evidence_mode,external_verified,recorded_by,recorded_at,...body}=decision();
 const c={version:1,scope:'SCOPE',order,cycle:Object.fromEntries(Object.entries(page()).filter(([k])=>!['deliveries','limit','offset','has_more'].includes(k))),body,uncertain:false};
 c.body={...body,expected_revision:1};
 assert.throws(()=>decisionCommand(c,'SCOPE'),/supplyCycleInvalid/);
 c.body={...body};c.cycle.decision=null;
 assert.equal(decisionCommand(c,'SCOPE'),c);
 assert.throws(()=>decisionResult({operation_id:'OP',repeated:false,decision:decision()},c,'BUYER'),/supplyCycleInvalid/);
});
test('decision read snapshots match the existing official contract',()=>{
 const official=JSON.parse(fs.readFileSync(new URL('../../docs/api/purchase-supply-cycle.openapi.json',import.meta.url),'utf8')).components.schemas;
 for(const s of [decisions,deliveries])for(const name of ['Decision','Cycle']){
  const p=name==='Decision'?s[name].properties:s[name].properties.decision.properties;
  const o=name==='Decision'?official[name].properties:official[name].properties.decision.properties;
  assert.deepEqual(p.evidence_mode,o.evidence_mode);assert.deepEqual(p.external_verified,o.external_verified);
 }
});
test('authenticated boundary is explicit in all five languages',()=>{
 const texts=['pt-BR','en','es','fr','zh-CN'].map(l=>decisionWords(l).authenticatedBoundary);
 assert.equal(new Set(texts).size,5);for(const text of texts)assert.ok(typeof text==='string'&&text.length>20);
});
