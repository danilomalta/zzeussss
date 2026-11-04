import assert from 'node:assert/strict';
import {createReceivablesClient} from '../src/core/receivables/client.mjs';
const base=process.env.TITAN_BRIDGE_URL,token=process.env.TITAN_BRIDGE_TOKEN,session=JSON.parse(process.env.TITAN_BRIDGE_SESSION||'null');
assert.ok(base&&new URL(base).hostname==='127.0.0.1'&&token&&session);
const context={token,session};let posts=0,lose=true;
const client=createReceivablesClient({getCurrent:()=>context,pending:new Map(),fetcher:async(path,options)=>{
 const response=await fetch(base+path,options);if(options.method==='POST')posts++;
 if(lose&&options.method==='POST'){lose=false;await response.arrayBuffer();throw Error('synthetic lost response after real commit');}return response;
}}).bind(context);
const input={operation_id:'bridge-create',id:'bridge-bill',customer_reference:'bridge-customer',document_reference:'bridge-document',description:'Synthetic HTTP integration only',issued_on:'2026-11-01',total_cents:1000,installments:[{id:'one',due_on:'2026-11-05',amount_cents:500},{id:'two',due_on:'2026-11-10',amount_cents:500}]};
const sourceID=process.env.TITAN_BRIDGE_ORDER;
if(sourceID){const source=await client.customerOrder(sourceID);assert.equal(source.message_id,sourceID);input.customer_reference=source.customer_reference;input.customer_order={message_id:sourceID,terms_reference:'Documented agreed monetary terms 42',confirmed:true};}
await assert.rejects(client.create(input));assert.equal(client.pending().operation_id,'bridge-create');
const recovered=await client.recover();assert.equal(recovered.document.id,input.id);assert.equal(client.pending(),null);assert.equal(posts,1);
const receive={operation_id:'bridge-cash-one',installment_id:'one',session_id:'ar-turn',amount_cents:500,expected_received_cents:0};
await client.receive(input.id,receive);const replay=await client.receive(input.id,receive);assert.equal(replay.repeated,true);
await client.receive(input.id,{...receive,operation_id:'bridge-cash-two',installment_id:'two'});
const document=await client.read(input.id);assert.equal(document.state,'settled');assert.equal(document.received_cents,1000);assert.equal(document.receipts.length,2);assert.equal(document.outstanding_cents,0);
assert.equal((await client.list()).total,1);
console.log(JSON.stringify({realOrderSource:!!sourceID,realHTTP:true,lostResponseRecovered:true,duplicateCashPrevented:true,settledCents:1000}));
