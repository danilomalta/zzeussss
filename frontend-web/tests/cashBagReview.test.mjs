import test from 'node:test';import assert from 'node:assert/strict';
import {bagBody,bagRecord} from '../src/core/cashBags/model.mjs';
import {fixture} from './cashBags.test.mjs';
import {cashBagWords} from '../src/core/cashBags/words.mjs';
test('receipt explanation preserves independent actor, exact variance and original receipt',async()=>{
 const sender=fixture(),r=await sender.api.submit(sender.api.prepare('create','review-bag',{operation_id:'review-create',bag_id:'review-bag',withdrawal_operation_id:'withdraw-one',seal_id:'seal'}));
 const sent=await sender.api.submit(sender.api.prepare('dispatch','review-bag',{operation_id:'review-send'},r.result.bag));
 const receiver=fixture('R',sender.data),before=await receiver.api.read('review-bag');
 const body={operation_id:'review-receive',seal_id:'seal',counted_cents:290,reason:'Independent count explanation'};
 bagBody('receive',body);assert.throws(()=>bagBody('receive',{...body,reason:' '}),/cashBagInvalid/);
 const result=await receiver.api.submit(receiver.api.prepare('receive','review-bag',body,before));
 bagRecord(result.result.bag,'R');assert.equal(result.result.bag.difference_cents,-10);assert.equal(result.result.bag.history.at(-1).reason,body.reason);assert.equal(result.result.bag.receiver_id,'R');
 assert.ok(sent.result.bag.state==='in_transit');
});
test('receive explanation label exists in five languages',()=>{assert.equal(new Set(['pt-BR','en','es','fr','zh-CN'].map(l=>cashBagWords(l).receiveReason)).size,5)});
